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

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
)

var bringUpActivityOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 20 * time.Minute,
	RetryPolicy: &temporal.RetryPolicy{
		MaximumAttempts:    3,
		InitialInterval:    5 * time.Second,
		MaximumInterval:    1 * time.Minute,
		BackoffCoefficient: 2,
	},
}

// BringUp orchestrates the rack bring-up sequence using operation rules.
// The execution sequence is driven by the RuleDefinition attached to the
// task, falling back to a hardcoded default when no custom rule exists.
func BringUp(
	ctx workflow.Context,
	reqInfo task.ExecutionInfo,
	info *operations.BringUpTaskInfo,
) error {
	if reqInfo.Rack == nil || len(reqInfo.Rack.Components) == 0 {
		return fmt.Errorf("no components provided")
	}

	ctx = workflow.WithActivityOptions(ctx, bringUpActivityOptions)

	if err := updateRunningTaskStatus(ctx, reqInfo.TaskID); err != nil {
		return err
	}

	typeToTargets := buildTargets(&reqInfo)

	err := executeRuleBasedOperation(
		ctx,
		typeToTargets,
		"BringUp",
		info,
		reqInfo.RuleDefinition,
	)

	return updateFinishedTaskStatus(ctx, reqInfo.TaskID, err)
}
