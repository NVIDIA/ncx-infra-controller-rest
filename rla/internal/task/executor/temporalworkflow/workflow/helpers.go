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

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/alert"
	taskcommon "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operationrules"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
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
		return errors.Join(err, fmt.Errorf("failed to update task status: %v", lerr))
	}

	return err
}

func buildTargets(info *task.ExecutionInfo) map[devicetypes.ComponentType]common.Target {
	if info.Rack == nil {
		return nil
	}

	// Group component IDs by type
	mapOnType := make(map[devicetypes.ComponentType][]string)
	for _, c := range info.Rack.Components {
		// NOTE: we skip checking if the component ID is empty, because it's
		// possible that the component ID is not set up for local testing case.
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

// executeGenericStageParallel executes all steps in a stage concurrently for any operation type
// Each component type in the stage runs as a child workflow (cross-type parallelism)
// Within each type, components are batched according to max_parallel
func executeGenericStageParallel(
	ctx workflow.Context,
	steps []operationrules.SequenceStep,
	typeToTargets map[devicetypes.ComponentType]common.Target,
	activityName string,
	activityInfo any,
) error {
	// Launch child workflow for each component type in parallel
	futures := make([]workflow.ChildWorkflowFuture, 0, len(steps))

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

		// Execute each component type as a child workflow
		// Use step.Timeout for child workflow (applies to entire pre+main+post)
		childWorkflowTimeout := step.Timeout
		if childWorkflowTimeout == 0 {
			childWorkflowTimeout = 30 * time.Minute // Fallback default
		}

		childOptions := workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("component-step-%s-%s",
				workflow.GetInfo(ctx).WorkflowExecution.ID,
				devicetypes.ComponentTypeToString(step.ComponentType)),
			WorkflowExecutionTimeout: childWorkflowTimeout,
		}
		childCtx := workflow.WithChildOptions(ctx, childOptions)

		future := workflow.ExecuteChildWorkflow(
			childCtx,
			GenericComponentStepWorkflow,
			step,
			target,
			activityName,
			activityInfo,
			typeToTargets, // Pass allTargets for cross-component verification
		)
		futures = append(futures, future)
	}

	// Wait for all component type child workflows to complete
	for i, future := range futures {
		if err := future.Get(ctx, nil); err != nil {
			return fmt.Errorf("component type %s failed: %w",
				devicetypes.ComponentTypeToString(steps[i].ComponentType), err)
		}

		log.Info().
			Str("component_type", devicetypes.ComponentTypeToString(steps[i].ComponentType)).
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
