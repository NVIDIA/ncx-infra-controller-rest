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
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/worker"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/clients/temporal"
	taskcommon "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/activity"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/workflow"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
)

const (
	WorkflowQueue = "rla-tasks"
)

type Config struct {
	ClientConf    temporal.Config
	WorkerOptions map[string]worker.Options

	// ComponentManagerRegistry is the registry containing initialized component managers.
	ComponentManagerRegistry *componentmanager.Registry
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("configuration for Temporal Executor is nil")
	}

	if err := c.ClientConf.Validate(); err != nil {
		return err
	}

	queueMap := make(map[string]bool)
	for queue := range c.WorkerOptions {
		if queueMap[queue] {
			return fmt.Errorf("queue %s is defined multiple times", queue)
		}
		queueMap[queue] = true
	}

	return nil
}

type Manager struct {
	conf             Config
	publisherClient  *temporal.Client
	subscriberClient *temporal.Client
	workers          map[string]worker.Worker
}

func (c *Config) Build(ctx context.Context) (executor.Executor, error) {
	// Set the component manager registry for activities
	if c.ComponentManagerRegistry != nil {
		activity.SetComponentManagerRegistry(c.ComponentManagerRegistry)
	} else {
		log.Warn().Msg("No component manager registry configured, activities may fail")
	}

	publisherClient, err := temporal.New(c.ClientConf)
	if err != nil {
		return nil, err
	}

	subscriberClient, err := temporal.New(c.ClientConf)
	if err != nil {
		return nil, err
	}

	workers := make(map[string]worker.Worker)
	for queue, options := range c.WorkerOptions {
		worker := worker.New(subscriberClient.Client(), queue, options)
		for _, a := range activity.GetAllActivities() {
			worker.RegisterActivity(a)
		}

		for _, wf := range workflow.GetAllWorkflows() {
			worker.RegisterWorkflow(wf)
		}

		workers[queue] = worker
	}

	return &Manager{
		conf:             *c,
		publisherClient:  publisherClient,
		subscriberClient: subscriberClient,
		workers:          workers,
	}, nil
}

func (m *Manager) Start(ctx context.Context) error {
	for queue, worker := range m.workers {
		log.Info().Msgf("Starting temporal worker for queue %s", queue)
		if err := worker.Start(); err != nil {
			return fmt.Errorf("failed to start temporal worker: %v", err)
		}
		log.Info().Msgf("Temporal worker started for queue %s", queue)
	}

	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	for queue, worker := range m.workers {
		log.Info().Msgf("Stopping temporal worker for queue %s", queue)
		worker.Stop()
		log.Info().Msgf("Temporal worker stopped for queue %s", queue)
	}

	m.publisherClient.Client().Close()
	m.subscriberClient.Client().Close()

	return nil
}

func (m *Manager) Type() taskcommon.ExecutorType {
	return taskcommon.ExecutorTypeTemporal
}

func (m *Manager) CheckStatus(
	ctx context.Context,
	encodedExecutionID string,
) (taskcommon.TaskStatus, error) {
	executionID, err := common.NewFromEncoded(encodedExecutionID)
	if err != nil {
		return taskcommon.TaskStatusUnknown, err
	}

	// Use empty runID to get the latest execution.
	resp, err := m.publisherClient.Client().DescribeWorkflowExecution(
		ctx,
		executionID.WorkflowID,
		"",
	)
	if err != nil {
		return taskcommon.TaskStatusUnknown, fmt.Errorf(
			"failed to describe temporal workflow execution %s: %v",
			executionID.String(),
			err,
		)
	}

	return taskStatusFromTemporalWorkflowStatus(
		resp.GetWorkflowExecutionInfo().GetStatus(),
	), nil
}

func (m *Manager) PowerControl(
	ctx context.Context,
	req *task.ExecutionRequest,
	info operations.PowerControlTaskInfo,
) (*task.ExecutionResponse, error) {
	if err := info.Validate(); err != nil {
		return nil, err
	}

	return executeWorkflow(
		ctx,
		m.publisherClient.Client(),
		executeWorkflowParams{
			workflowName: workflow.PowerControlWorkflowName,
			timeout:      operations.GetOperationOptions(taskcommon.TaskTypePowerControl).Timeout,
			req:          req,
			info:         info,
		},
	)
}

func (m *Manager) FirmwareControl(
	ctx context.Context,
	req *task.ExecutionRequest,
	info operations.FirmwareControlTaskInfo,
) (*task.ExecutionResponse, error) {
	if err := info.Validate(); err != nil {
		return nil, err
	}

	return executeWorkflow(
		ctx,
		m.publisherClient.Client(),
		executeWorkflowParams{
			workflowName: workflow.FirmwareControlWorkflowName,
			timeout:      operations.GetOperationOptions(taskcommon.TaskTypeFirmwareControl).Timeout,
			req:          req,
			info:         info,
		},
	)
}

func (m *Manager) InjectExpectation(
	ctx context.Context,
	req *task.ExecutionRequest,
	info operations.InjectExpectationTaskInfo,
) (*task.ExecutionResponse, error) {
	if err := info.Validate(); err != nil {
		return nil, err
	}

	return executeWorkflow(
		ctx,
		m.publisherClient.Client(),
		executeWorkflowParams{
			workflowName: workflow.InjectExpectationWorkflowName,
			timeout:      operations.GetOperationOptions(taskcommon.TaskTypeInjectExpectation).Timeout,
			req:          req,
			info:         info,
		},
	)
}

func (m *Manager) BringUp(
	ctx context.Context,
	req *task.ExecutionRequest,
	info operations.BringUpTaskInfo,
) (*task.ExecutionResponse, error) {
	if err := info.Validate(); err != nil {
		return nil, err
	}

	return executeWorkflow(
		ctx,
		m.publisherClient.Client(),
		executeWorkflowParams{
			workflowName: workflow.BringUpWorkflowName,
			timeout:      operations.GetOperationOptions(taskcommon.TaskTypeBringUp).Timeout,
			req:          req,
			info:         info,
		},
	)
}
