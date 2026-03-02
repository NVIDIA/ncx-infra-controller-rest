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

package task

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/inventoryobjects/rack"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/operation"
	taskcommon "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operationrules"
)

// Task defines the details of a task. It includes:
// -- ID: The unique identifier of the task.
// -- Operation: The operation to be performed by the task.
// -- RackID: The rack this task operates on (1 task = 1 rack).
// -- ComponentUUIDs: The component UUIDs in this rack.
// -- Description: The description of the task provided by the user.
// -- ExecutorType: The type of executor to be used for the task.
// -- ExecutionID: The identifier of the execution of the task.
// -- Status: The status of the task.
// -- Message: Status message or error details.
// -- AppliedRuleID: The ID of the operation rule that was applied (if any).
type Task struct {
	ID             uuid.UUID
	Operation      operation.Wrapper
	RackID         uuid.UUID   // The rack this task operates on (1 task = 1 rack)
	ComponentUUIDs []uuid.UUID // Component UUIDs in this rack
	Description    string
	ExecutorType   taskcommon.ExecutorType
	ExecutionID    string
	Status         taskcommon.TaskStatus
	Message        string
	AppliedRuleID  *uuid.UUID // The ID of the operation rule that was applied
}

// ExecutionInfo contains the information needed to execute a task.
// Rack contains rack info and the components to be operated on (see rack.Rack NOTE).
// RuleDefinition contains the resolved operation rule (resolved at task creation time).
type ExecutionInfo struct {
	TaskID         uuid.UUID
	Rack           *rack.Rack
	RuleDefinition *operationrules.RuleDefinition
}

type ExecutionRequest struct {
	Info  ExecutionInfo
	Async bool
}

type ExecutionResponse struct {
	ExecutionID string
}

func (r *ExecutionRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("request is nil")
	}

	if r.Info.TaskID == uuid.Nil {
		return fmt.Errorf("task ID is nil")
	}

	if r.Info.Rack == nil {
		return fmt.Errorf("rack is nil")
	}

	if len(r.Info.Rack.Components) == 0 {
		return fmt.Errorf("components list is empty")
	}

	return nil
}

func (r *ExecutionResponse) IsValid() bool {
	if r == nil {
		return false
	}

	if r.ExecutionID == "" {
		return false
	}

	return true
}

type TaskStatusUpdate struct {
	ID      uuid.UUID
	Status  taskcommon.TaskStatus
	Message string
}

type TaskStatusUpdater interface {
	UpdateTaskStatus(ctx context.Context, arg *TaskStatusUpdate) error
}
