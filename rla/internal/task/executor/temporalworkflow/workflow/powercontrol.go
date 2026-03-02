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
	"time"

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common" //nolint
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operationrules"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

var (
	powerControlActivityOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 20 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    1 * time.Minute,
			BackoffCoefficient: 2,
		},
	}
)

func PowerControl(
	ctx workflow.Context,
	reqInfo task.ExecutionInfo,
	info operations.PowerControlTaskInfo,
) (err error) {
	if reqInfo.Rack == nil || len(reqInfo.Rack.Components) == 0 {
		return nil
	}

	// Set default activity options for UpdateTaskStatus calls
	ctx = workflow.WithActivityOptions(ctx, powerControlActivityOptions)

	if err := updateRunningTaskStatus(ctx, reqInfo.TaskID); err != nil {
		// XXX -- The workflow will be terminated, but the task status won't be
		// updated. We need to add a background process to try to detect and
		// fix this situation in the future.
		return err
	}

	typeToTargets := buildTargets(&reqInfo)

	// Execute power control using operation rules
	err = executePowerControl(
		ctx,
		typeToTargets,
		info,
		reqInfo.RuleDefinition,
	)

	return updateFinishedTaskStatus(ctx, reqInfo.TaskID, err)
}

// executePowerControl executes power control using operation rules.
// All pre/post-operation activities (delays, verification, reachability checks)
// are handled in child workflows via action-based configuration.
func executePowerControl(
	ctx workflow.Context,
	typeToTargets map[devicetypes.ComponentType]common.Target,
	info operations.PowerControlTaskInfo,
	ruleDef *operationrules.RuleDefinition,
) error {
	if ruleDef == nil {
		return fmt.Errorf(
			"rule definition is nil (resolver should never return nil)",
		)
	}

	if len(ruleDef.Steps) == 0 {
		return fmt.Errorf(
			"rule definition has no steps for operation %s",
			info.Operation.String(),
		)
	}

	log.Info().
		Int("step_count", len(ruleDef.Steps)).
		Msg("Executing power control with operation rules")

	// Execute stages sequentially
	// All pre/post-operation activities now handled in child workflow
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
			"PowerControl",
			info,
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

	log.Info().Msg("Power control completed successfully")

	return nil
}
