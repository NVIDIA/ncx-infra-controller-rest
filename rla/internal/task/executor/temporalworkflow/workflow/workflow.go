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
	"fmt"

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/workflow"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operationrules"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

const (
	PowerControlWorkflowName      = "PowerControl"
	FirmwareControlWorkflowName   = "FirmwareControl"
	InjectExpectationWorkflowName = "InjectExpectation"
	BringUpWorkflowName           = "BringUp"
)

func GetAllWorkflows() []any {
	return []any{
		PowerControl,
		FirmwareControl,
		InjectExpectation,
		BringUp,
		GenericComponentStepWorkflow,
	}
}

// GenericComponentStepWorkflow is a generic child workflow that handles
// any operation for a single component type. It processes components in
// batches according to the step's max_parallel setting. This provides
// better isolation, visibility, and independent lifecycle per component type.
func GenericComponentStepWorkflow(
	ctx workflow.Context,
	step operationrules.SequenceStep,
	target common.Target,
	activityName string,
	activityInfo any,
	allTargets map[devicetypes.ComponentType]common.Target,
) error {
	log.Info().
		Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
		Int("component_count", len(target.ComponentIDs)).
		Int("max_parallel", step.MaxParallel).
		Str("activity", activityName).
		Msg("Component step workflow started")

	// Build activity options from step configuration
	activityOpts := buildActivityOptions(step)
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	// 1. Execute pre-operation actions
	if shouldDo, actions := step.DoPreOperations(); shouldDo {
		log.Debug().
			Int("action_count", len(actions)).
			Msg("Executing pre-operation actions")
		if err := executeActionList(ctx, actions, target, allTargets, activityInfo); err != nil {
			return fmt.Errorf("pre-operation failed: %w", err)
		}
	}

	// 2. Execute main operation
	if shouldDo, action := step.DoMainOperation(); shouldDo {
		// New action-based configuration
		log.Debug().
			Str("action", action.Name).
			Msg("Executing main operation action")
		if err := executeAction(ctx, action, target, allTargets, activityInfo); err != nil {
			return fmt.Errorf("main operation failed: %w", err)
		}
	} else {
		// Backward compatibility: use legacy activityName parameter
		if activityName == "" {
			return fmt.Errorf(
				"no main operation configured and no legacy activityName provided",
			)
		}
		log.Debug().
			Str("activity", activityName).
			Msg("Executing main operation (legacy)")
		if err := executeGenericBatchedComponents(ctx, step, target, activityName, activityInfo); err != nil {
			return fmt.Errorf("main operation (legacy) failed: %w", err)
		}
	}

	// 3. Execute post-operation actions
	if shouldDo, actions := step.DoPostOperations(); shouldDo {
		log.Debug().
			Int("action_count", len(actions)).
			Msg("Executing post-operation actions")
		if err := executeActionList(ctx, actions, target, allTargets, activityInfo); err != nil {
			return fmt.Errorf("post-operation failed: %w", err)
		}
	}

	// Apply delay_after (legacy field, after all actions complete)
	if step.DelayAfter > 0 {
		log.Info().
			Dur("delay", step.DelayAfter).
			Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
			Msg("Applying delay after step (legacy)")
		workflow.Sleep(ctx, step.DelayAfter)
	}

	log.Info().
		Str("component_type", devicetypes.ComponentTypeToString(step.ComponentType)).
		Msg("Component step workflow completed successfully")

	return nil
}
