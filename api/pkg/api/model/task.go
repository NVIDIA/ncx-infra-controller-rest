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

package model

import (
	"fmt"

	rlav1 "github.com/nvidia/bare-metal-manager-rest/workflow-schema/rla/protobuf/v1"
)

var ProtoToAPITaskStatusName = map[rlav1.TaskStatus]string{
	rlav1.TaskStatus_TASK_STATUS_UNKNOWN:   "unknown",
	rlav1.TaskStatus_TASK_STATUS_PENDING:   "pending",
	rlav1.TaskStatus_TASK_STATUS_RUNNING:   "running",
	rlav1.TaskStatus_TASK_STATUS_COMPLETED: "succeeded",
	rlav1.TaskStatus_TASK_STATUS_FAILED:    "failed",
}

// APITask is the API response model for a task.
type APITask struct {
	ID          string            `json:"id"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	StartTime   string            `json:"startTime"`
	EndTime     string            `json:"endTime"`
	Message     string            `json:"message"`
	Metadata    map[string]string `json:"metadata"`
}

func (t *APITask) FromProto(task *rlav1.Task) {
	if task == nil {
		return
	}
	if task.GetId() != nil {
		t.ID = task.GetId().GetId()
	}
	t.Status = enumOr(ProtoToAPITaskStatusName, task.GetStatus(), "unknown")
	t.Description = task.GetDescription()
	t.Message = task.GetMessage()
}

func NewAPITask(task *rlav1.Task) *APITask {
	t := &APITask{}
	t.FromProto(task)
	return t
}

// APIGetTaskRequest captures query parameters for getting a task by ID.
type APIGetTaskRequest struct {
	SiteID string `query:"siteId"`
}

func (r *APIGetTaskRequest) Validate() error {
	if r.SiteID == "" {
		return fmt.Errorf("siteId query parameter is required")
	}
	return nil
}
