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

package instance

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"go.temporal.io/sdk/client"

	instanceActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/instance"
	"github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/queue"
)

// RebootInstance is a Temporal workflow to reboot a machine associated with Instance via Site Agent
func RebootInstance(ctx workflow.Context, instanceID uuid.UUID, rebootWithCustomIpxe bool, applyUpdatesOnReboot bool) error {
	logger := log.With().Str("Workflow", "Instance").Str("Action", "Reboot").Str("Instance ID", instanceID.String()).Logger()

	logger.Info().Msg("starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    15,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 2 * time.Minute,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var instanceManager instanceActivity.ManageInstance

	err := workflow.ExecuteActivity(ctx, instanceManager.RebootInstanceViaSiteAgent, instanceID, rebootWithCustomIpxe, applyUpdatesOnReboot).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to execute activity: RebootInstanceViaSiteAgent")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// ExecuteRebootInstanceWorkflow is a helper function to trigger execution of reboot Instance workflow
func ExecuteRebootInstanceWorkflow(ctx context.Context, tc client.Client, instanceID uuid.UUID, rebootWithCustomIpxe bool, applyUpdatesOnReboot bool) (*string, error) {
	uid := uuid.New()

	workflowOptions := client.StartWorkflowOptions{
		ID:        "instance-reboot-" + uid.String(),
		TaskQueue: queue.CloudTaskQueue,
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, RebootInstance, instanceID, rebootWithCustomIpxe, applyUpdatesOnReboot)

	if err != nil {
		log.Error().Err(err).Msg("failed to execute workflow: RebootInstance")
		return nil, err
	}

	wid := we.GetID()

	return &wid, nil
}
