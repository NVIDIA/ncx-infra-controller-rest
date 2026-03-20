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
	"time"

	"github.com/google/uuid"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/operation"
	taskcommon "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/operationrules"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/common/devicetypes"
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
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time

	// QueueExpiresAt is the deadline for a waiting task to be promoted.
	// After this time the Promoter terminates the task automatically.
	// Nil for non-waiting tasks.
	QueueExpiresAt *time.Time
}

// WorkflowComponent holds the minimal component data needed to execute
// a workflow. All fields are plain JSON-safe types.
type WorkflowComponent struct {
	Type        devicetypes.ComponentType `json:"type"`
	ComponentID string                    `json:"component_id"`
}

// ExecutionInfo contains the information needed to execute a task.
// RuleDefinition contains the resolved operation rule
// (resolved at task creation time).
type ExecutionInfo struct {
	TaskID         uuid.UUID
	Components     []WorkflowComponent
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

	if len(r.Info.Components) == 0 {
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
