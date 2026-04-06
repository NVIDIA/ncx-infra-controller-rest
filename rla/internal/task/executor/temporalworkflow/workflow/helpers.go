/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/alert"
	taskcommon "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/operationrules"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/task"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/common/devicetypes"
)

// sendAlert logs an alert. Best-effort, never blocks the workflow.
func sendAlert(a alert.Alert) {
	alert.Send(context.Background(), a)
}

func updateRunningTaskStatus(
	ctx workflow.Context,
	taskID uuid.UUID,
) error {
	if taskID == uuid.Nil {
		return fmt.Errorf("task ID is not specified")
	}

	arg := &task.TaskStatusUpdate{
		ID:      taskID,
		Status:  taskcommon.TaskStatusRunning,
		Message: "Running",
	}

	return workflow.ExecuteActivity(ctx, "UpdateTaskStatus", arg).Get(ctx, nil)
}

func updateFinishedTaskStatus(
	ctx workflow.Context,
	taskID uuid.UUID,
	err error,
) error {
	if taskID == uuid.Nil {
		return fmt.Errorf("task ID is not specified")
	}

	var arg *task.TaskStatusUpdate

	if err != nil {
		arg = &task.TaskStatusUpdate{
			ID:      taskID,
			Status:  taskcommon.TaskStatusFailed,
			Message: err.Error(),
		}
	} else {
		arg = &task.TaskStatusUpdate{
			ID:      taskID,
			Status:  taskcommon.TaskStatusCompleted,
			Message: "Completed successfully",
		}
	}

	if lerr := workflow.ExecuteActivity(ctx, "UpdateTaskStatus", arg).Get(ctx, nil); lerr != nil { //nolint
		return errors.Join(err, fmt.Errorf("failed to update task status: %w", lerr))
	}

	return err
}

func buildTargets(
	info *task.ExecutionInfo,
) map[devicetypes.ComponentType]common.Target {
	if info == nil {
		// This is unreachable code, but just in case, handle it anyway.
		// Returns a non-nil map to avoid nil pointer dereferences.
		return map[devicetypes.ComponentType]common.Target{}
	}

	// Group component IDs by type
	mapOnType := make(map[devicetypes.ComponentType][]string)
	for _, c := range info.Components {
		// NOTE: we skip checking if the component ID is empty, because it's
		// possible that the component ID is not set up for local testing.
		mapOnType[c.Type] = append(mapOnType[c.Type], c.ComponentID)
	}

	// Build Target for each type with component IDs only
	results := make(map[devicetypes.ComponentType]common.Target)
	for t, componentIDs := range mapOnType {
		results[t] = common.Target{
			Type:         t,
			ComponentIDs: componentIDs,
		}
	}

	return results
}

// buildActivityOptions constructs activity options from a sequence step
func buildActivityOptions(step operationrules.SequenceStep) workflow.ActivityOptions {
	opts := workflow.ActivityOptions{
		StartToCloseTimeout: 20 * time.Minute, // Default timeout
	}

	// Override timeout if specified in step
	if step.Timeout > 0 {
		opts.StartToCloseTimeout = step.Timeout
	}

	// Set retry policy
	if step.RetryPolicy != nil {
		initialInterval := step.RetryPolicy.InitialInterval
		if initialInterval <= 0 {
			initialInterval = 1 * time.Second
		}

		retryPolicy := &temporal.RetryPolicy{
			MaximumAttempts:    int32(step.RetryPolicy.MaxAttempts),
			InitialInterval:    initialInterval,
			BackoffCoefficient: step.RetryPolicy.BackoffCoefficient,
		}

		if step.RetryPolicy.MaxInterval > 0 {
			retryPolicy.MaximumInterval = step.RetryPolicy.MaxInterval
		}

		opts.RetryPolicy = retryPolicy
	} else {
		// Default retry policy
		opts.RetryPolicy = &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    1 * time.Minute,
			BackoffCoefficient: 2,
		}
	}

	return opts
}

// childWorkflowEntry pairs a launched child workflow future with its component
// type so that error attribution stays correct even when some steps are skipped.
type childWorkflowEntry struct {
	future        workflow.ChildWorkflowFuture
	componentType devicetypes.ComponentType
}

// childWorkflowExecutionTimeout returns a child workflow execution timeout that
// accommodates the full retry budget for activities, the pre/post operation
// durations, and a fixed scheduling buffer.
//
// The child workflow runs: pre-ops → main-op (with retries) → post-ops
// sequentially, so the budget must cover all three phases.
func childWorkflowExecutionTimeout(step operationrules.SequenceStep) time.Duration {
	base := step.Timeout
	if base == 0 {
		base = 30 * time.Minute
	}

	maxAttempts := 1
	var maxBackoff time.Duration
	if step.RetryPolicy != nil && step.RetryPolicy.MaxAttempts > 1 {
		maxAttempts = step.RetryPolicy.MaxAttempts
		if step.RetryPolicy.MaxInterval > 0 {
			maxBackoff = step.RetryPolicy.MaxInterval
		} else {
			maxBackoff = step.RetryPolicy.InitialInterval
		}
	}

	// Main operation: each attempt may take up to base, plus back-off between attempts.
	mainBudget := base*time.Duration(maxAttempts) +
		maxBackoff*time.Duration(maxAttempts-1)

	// Pre/post operation budgets: sum the declared timeouts of each action.
	// Actions without a timeout are assumed to be quick (covered by the buffer).
	var actionBudget time.Duration
	for _, a := range step.PreOperation {
		actionBudget += a.Timeout
	}
	for _, a := range step.PostOperation {
		actionBudget += a.Timeout
	}

	return mainBudget + actionBudget + 2*time.Minute
}

// executeGenericStageParallel executes all steps in a stage concurrently for any operation type.
// Each component type in the stage runs as a child workflow (cross-type parallelism).
// Within each type, components are batched according to the step's max_parallel setting.
func executeGenericStageParallel(
	ctx workflow.Context,
	steps []operationrules.SequenceStep,
	typeToTargets map[devicetypes.ComponentType]common.Target,
	activityName string,
	activityInfo any,
) error {
	// Launch a child workflow for each component type that has targets.
	// Pair each future with its component type so error attribution is always
	// correct even when some steps are skipped (skipped steps shrink the
	// futures slice without a matching change to the steps slice).
	futures := make([]childWorkflowEntry, 0, len(steps))

	for _, step := range steps {
		target, exists := typeToTargets[step.ComponentType]
		if !exists || len(target.ComponentIDs) == 0 {
			log.Info().
				Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
				Msg("Skipping step, no components of this type")
			continue
		}

		log.Info().
			Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
			Int("component_count", len(target.ComponentIDs)).
			Int("max_parallel", step.MaxParallel).
			Str("activity", activityName).
			Msg("Starting component step as child workflow")

		childOptions := workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("component-step-%s-%s",
				workflow.GetInfo(ctx).WorkflowExecution.ID,
				devicetypes.ComponentTypeToString(step.ComponentType)),
			// Give the child workflow enough time to run all retry attempts.
			WorkflowExecutionTimeout: childWorkflowExecutionTimeout(step),
		}
		childCtx := workflow.WithChildOptions(ctx, childOptions)

		future := workflow.ExecuteChildWorkflow(
			childCtx,
			GenericComponentStepWorkflow,
			step,
			target,
			activityName,
			activityInfo,
			typeToTargets,
		)
		futures = append(futures, childWorkflowEntry{
			future:        future,
			componentType: step.ComponentType,
		})
	}

	// Wait for all child workflows and attribute any error to the correct type.
	for _, entry := range futures {
		if err := entry.future.Get(ctx, nil); err != nil {
			return fmt.Errorf("component type %s failed: %w",
				devicetypes.ComponentTypeToString(entry.componentType), err)
		}

		log.Info().
			Str("component_type", devicetypes.ComponentTypeToString(entry.componentType)).
			Msg("Component step completed successfully")
	}

	return nil
}

// executeGenericBatchedComponents executes any operation for all components of a single type
// Components are processed in batches according to the step's max_parallel setting
func executeGenericBatchedComponents(
	ctx workflow.Context,
	step operationrules.SequenceStep,
	target common.Target,
	activityName string,
	activityInfo any,
) error {
	componentIDs := target.ComponentIDs
	maxParallel := step.MaxParallel

	// Handle special cases for maxParallel
	if maxParallel == 0 {
		maxParallel = len(componentIDs) // 0 = unlimited (all at once)
	}
	if maxParallel < 0 {
		maxParallel = 1 // Negative = treat as sequential
	}

	componentCount := len(componentIDs)
	batchCount := (componentCount + maxParallel - 1) / maxParallel

	log.Info().
		Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
		Int("total_components", componentCount).
		Int("max_parallel", maxParallel).
		Int("batch_count", batchCount).
		Str("activity", activityName).
		Msg("Processing components in batches")

	// Process components in batches
	for batchNum := range batchCount {
		start := batchNum * maxParallel
		end := min(start+maxParallel, componentCount)
		batch := componentIDs[start:end]

		log.Info().
			Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
			Int("batch_number", batchNum+1).
			Int("total_batches", batchCount).
			Int("batch_size", len(batch)).
			Msg("Processing batch")

		// Execute all components in this batch in parallel
		futures := make([]workflow.Future, len(batch))
		for i, componentID := range batch {
			singleTarget := common.Target{
				Type:         target.Type,
				ComponentIDs: []string{componentID},
			}

			log.Debug().
				Str("component_id", componentID).
				Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
				Str("activity", activityName).
				Msg("Starting activity for component")

			// Execute activity for single component
			futures[i] = workflow.ExecuteActivity(ctx, activityName, singleTarget, activityInfo)
		}

		// Wait for all components in this batch to complete
		for i, future := range futures {
			if err := future.Get(ctx, nil); err != nil {
				return fmt.Errorf("component %s failed: %w", batch[i], err)
			}

			log.Debug().
				Str("component_id", batch[i]).
				Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
				Str("activity", activityName).
				Msg("Activity succeeded for component")
		}

		log.Info().
			Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
			Int("batch_number", batchNum+1).
			Int("total_batches", batchCount).
			Msg("Batch completed successfully")
	}

	log.Info().
		Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
		Int("total_components", componentCount).
		Int("batch_count", batchCount).
		Msg("All batches completed successfully for component type")

	return nil
}

// parseDurationParam extracts a duration from a parameter value.
// Accepts time.Duration, string (e.g. "30s"), float64, or int (nanoseconds).
func parseDurationParam(val any) time.Duration {
	switch v := val.(type) {
	case time.Duration:
		return v
	case string:
		d, _ := time.ParseDuration(v)
		return d
	case float64:
		return time.Duration(v)
	case int:
		return time.Duration(v)
	default:
		return 0
	}
}

// executeRuleBasedOperation drives any operation through its RuleDefinition.
// Stages execute sequentially; steps within a stage execute in parallel via
// child workflows. The activityName is a legacy fallback used only when a
// step has no MainOperation configured.
func executeRuleBasedOperation(
	ctx workflow.Context,
	typeToTargets map[devicetypes.ComponentType]common.Target,
	activityName string,
	operationInfo any,
	ruleDef *operationrules.RuleDefinition,
) error {
	if ruleDef == nil {
		return fmt.Errorf(
			"rule definition is nil (resolver should never return nil)",
		)
	}

	if len(ruleDef.Steps) == 0 {
		return fmt.Errorf("rule definition has no steps")
	}

	log.Info().
		Int("step_count", len(ruleDef.Steps)).
		Msg("Executing operation with rule definition")

	iter := operationrules.NewStageIterator(ruleDef)
	for stage := iter.Next(); stage != nil; stage = iter.Next() {
		log.Info().
			Int("stage", stage.Number).
			Int("step_count", len(stage.Steps)).
			Msg("Executing stage")

		if err := executeGenericStageParallel(
			ctx,
			stage.Steps,
			typeToTargets,
			activityName,
			operationInfo,
		); err != nil {
			log.Error().
				Err(err).
				Int("stage", stage.Number).
				Msg("Stage execution failed")
			return fmt.Errorf("stage %d failed: %w", stage.Number, err)
		}

		log.Info().
			Int("stage", stage.Number).
			Msg("Stage completed successfully")
	}

	log.Info().Msg("Rule-based operation completed successfully")
	return nil
}
