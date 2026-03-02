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

package executor

import (
	"context"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
)

type Executor interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Type() common.ExecutorType
	PowerControl(ctx context.Context, req *task.ExecutionRequest, info operations.PowerControlTaskInfo) (*task.ExecutionResponse, error)           //nolint
	FirmwareControl(ctx context.Context, req *task.ExecutionRequest, info operations.FirmwareControlTaskInfo) (*task.ExecutionResponse, error)     //nolint
	InjectExpectation(ctx context.Context, req *task.ExecutionRequest, info operations.InjectExpectationTaskInfo) (*task.ExecutionResponse, error) //nolint
	BringUp(ctx context.Context, req *task.ExecutionRequest, info operations.BringUpTaskInfo) (*task.ExecutionResponse, error)                     //nolint
	CheckStatus(ctx context.Context, executionID string) (common.TaskStatus, error)
}

type ExecutorConfig interface {
	Validate() error
	Build(ctx context.Context) (Executor, error)
}

func New(
	ctx context.Context,
	executorConfig ExecutorConfig,
) (Executor, error) {
	if err := executorConfig.Validate(); err != nil {
		return nil, err
	}

	return executorConfig.Build(ctx)
}
