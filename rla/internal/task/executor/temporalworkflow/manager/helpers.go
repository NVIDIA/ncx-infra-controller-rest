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

package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.temporal.io/api/enums/v1"
	temporalclient "go.temporal.io/sdk/client"

	taskcommon "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
)

var (
	workflowStatusToTaskStatus = map[enums.WorkflowExecutionStatus]taskcommon.TaskStatus{ //nlint
		enums.WORKFLOW_EXECUTION_STATUS_RUNNING:          taskcommon.TaskStatusRunning,
		enums.WORKFLOW_EXECUTION_STATUS_COMPLETED:        taskcommon.TaskStatusCompleted,
		enums.WORKFLOW_EXECUTION_STATUS_FAILED:           taskcommon.TaskStatusFailed,
		enums.WORKFLOW_EXECUTION_STATUS_CANCELED:         taskcommon.TaskStatusTerminated,
		enums.WORKFLOW_EXECUTION_STATUS_TERMINATED:       taskcommon.TaskStatusTerminated,
		enums.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW: taskcommon.TaskStatusRunning,
		enums.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:        taskcommon.TaskStatusTerminated,
	}
)

func taskStatusFromTemporalWorkflowStatus(
	workflowStatus enums.WorkflowExecutionStatus,
) taskcommon.TaskStatus {
	if taskStatus, ok := workflowStatusToTaskStatus[workflowStatus]; ok {
		return taskStatus
	}
	return taskcommon.TaskStatusUnknown
}

type executeWorkflowParams struct {
	workflowName string
	timeout      time.Duration
	req          *task.ExecutionRequest
	info         any
}

func executeWorkflow(
	ctx context.Context,
	client temporalclient.Client,
	params executeWorkflowParams,
) (*task.ExecutionResponse, error) {
	r, err := client.ExecuteWorkflow(
		ctx,
		temporalclient.StartWorkflowOptions{
			TaskQueue:                WorkflowQueue,
			ID:                       params.req.Info.TaskID.String(),
			WorkflowExecutionTimeout: params.timeout,
		},
		params.workflowName,
		params.req.Info,
		params.info,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute workflow: %v", err)
	}

	executionID := &common.ExecutionID{
		WorkflowID: r.GetID(),
		RunID:      r.GetRunID(),
	}

	log.Info().Msgf("Temporal workflow %s started", executionID.String())

	encodedExecutionID, err := executionID.Encode()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to encode execution ID %s: %v", executionID.String(), err,
		)
	}

	if !params.req.Async {
		// For synchronous requests, block until the workflow is completed.
		if err := r.Get(ctx, nil); err != nil {
			return nil, fmt.Errorf("failed to get workflow result: %v", err)
		}
	}

	return &task.ExecutionResponse{
		ExecutionID: encodedExecutionID,
	}, nil
}
