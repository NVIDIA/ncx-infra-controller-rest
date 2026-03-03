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

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

var injectExpectationActivityOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 10 * time.Minute,
	RetryPolicy: &temporal.RetryPolicy{
		MaximumAttempts:    3,
		InitialInterval:    5 * time.Second,
		MaximumInterval:    1 * time.Minute,
		BackoffCoefficient: 2,
	},
}

// InjectExpectation orchestrates injecting expected component configurations
// to their respective backend services (Carbide for compute/switch, PSM for
// powershelves). Each component is processed via the InjectExpectation activity
// which delegates to the appropriate component manager.
func InjectExpectation(
	ctx workflow.Context,
	reqInfo task.ExecutionInfo,
	info *operations.InjectExpectationTaskInfo,
) error {
	if reqInfo.Rack == nil || len(reqInfo.Rack.Components) == 0 {
		return fmt.Errorf("no components provided")
	}

	ctx = workflow.WithActivityOptions(ctx, injectExpectationActivityOptions)

	if err := updateRunningTaskStatus(ctx, reqInfo.TaskID); err != nil {
		return err
	}

	typeToTargets := buildTargets(&reqInfo)

	if err := injectExpectationForAll(ctx, typeToTargets, info); err != nil {
		return updateFinishedTaskStatus(ctx, reqInfo.TaskID, err)
	}

	return updateFinishedTaskStatus(ctx, reqInfo.TaskID, nil)
}

// injectExpectationForAll iterates over each component type and calls
// the InjectExpectation activity. Each component type is handled
// sequentially to keep error reporting clear.
func injectExpectationForAll(
	ctx workflow.Context,
	typeToTargets map[devicetypes.ComponentType]common.Target,
	info *operations.InjectExpectationTaskInfo,
) error {
	for compType, target := range typeToTargets {
		log.Info().
			Str("component_type", devicetypes.ComponentTypeToString(compType)).
			Int("count", len(target.ComponentIDs)).
			Msg("Injecting expectations for component type")

		err := workflow.ExecuteActivity(
			ctx, "InjectExpectation", target, *info,
		).Get(ctx, nil)
		if err != nil {
			return fmt.Errorf(
				"InjectExpectation failed for %s: %w",
				devicetypes.ComponentTypeToString(compType), err,
			)
		}

		log.Info().
			Str("component_type", devicetypes.ComponentTypeToString(compType)).
			Msg("InjectExpectation completed for component type")
	}

	return nil
}
