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
	"testing"

	rlav1 "github.com/nvidia/bare-metal-manager-rest/workflow-schema/rla/protobuf/v1"
	"github.com/stretchr/testify/assert"
)

func TestNewAPITask(t *testing.T) {
	tests := []struct {
		name     string
		task     *rlav1.Task
		expected *APITask
	}{
		{
			name:     "nil task returns empty APITask",
			task:     nil,
			expected: &APITask{},
		},
		{
			name: "task with all fields",
			task: &rlav1.Task{
				Id:          &rlav1.UUID{Id: "task-123"},
				Operation:   "power_on",
				RackId:      &rlav1.UUID{Id: "rack-456"},
				Description: "Power on rack components",
				Status:      rlav1.TaskStatus_TASK_STATUS_RUNNING,
				Message:     "Processing 3 of 5 components",
			},
			expected: &APITask{
				ID:          "task-123",
				Status:      "running",
				Description: "Power on rack components",
				Message:     "Processing 3 of 5 components",
			},
		},
		{
			name: "task with pending status",
			task: &rlav1.Task{
				Id:          &rlav1.UUID{Id: "task-001"},
				Description: "Firmware upgrade",
				Status:      rlav1.TaskStatus_TASK_STATUS_PENDING,
			},
			expected: &APITask{
				ID:          "task-001",
				Status:      "pending",
				Description: "Firmware upgrade",
			},
		},
		{
			name: "task with completed status maps to succeeded",
			task: &rlav1.Task{
				Id:          &rlav1.UUID{Id: "task-002"},
				Description: "Bring up rack",
				Status:      rlav1.TaskStatus_TASK_STATUS_COMPLETED,
				Message:     "All components ready",
			},
			expected: &APITask{
				ID:          "task-002",
				Status:      "succeeded",
				Description: "Bring up rack",
				Message:     "All components ready",
			},
		},
		{
			name: "task with failed status",
			task: &rlav1.Task{
				Id:          &rlav1.UUID{Id: "task-003"},
				Description: "Power off rack",
				Status:      rlav1.TaskStatus_TASK_STATUS_FAILED,
				Message:     "BMC unreachable",
			},
			expected: &APITask{
				ID:          "task-003",
				Status:      "failed",
				Description: "Power off rack",
				Message:     "BMC unreachable",
			},
		},
		{
			name: "task with unknown status",
			task: &rlav1.Task{
				Id:     &rlav1.UUID{Id: "task-004"},
				Status: rlav1.TaskStatus_TASK_STATUS_UNKNOWN,
			},
			expected: &APITask{
				ID:     "task-004",
				Status: "unknown",
			},
		},
		{
			name: "task with nil ID",
			task: &rlav1.Task{
				Description: "Orphan task",
				Status:      rlav1.TaskStatus_TASK_STATUS_PENDING,
			},
			expected: &APITask{
				Status:      "pending",
				Description: "Orphan task",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAPITask(tt.task)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected.ID, result.ID)
			assert.Equal(t, tt.expected.Status, result.Status)
			assert.Equal(t, tt.expected.Description, result.Description)
			assert.Equal(t, tt.expected.Message, result.Message)
			assert.Empty(t, result.StartTime)
			assert.Empty(t, result.EndTime)
			assert.Nil(t, result.Metadata)
		})
	}
}

func TestAPIGetTaskRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request APIGetTaskRequest
		wantErr bool
	}{
		{
			name:    "valid request",
			request: APIGetTaskRequest{SiteID: "550e8400-e29b-41d4-a716-446655440000"},
			wantErr: false,
		},
		{
			name:    "missing siteId",
			request: APIGetTaskRequest{},
			wantErr: true,
		},
		{
			name:    "empty siteId",
			request: APIGetTaskRequest{SiteID: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
