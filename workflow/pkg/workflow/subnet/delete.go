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

package subnet

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"go.temporal.io/sdk/client"

	subnetActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/subnet"
	"github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/queue"
)

// DeleteSubnet is a Temporal workflow to delete an existing Subnet via Site Agent
func DeleteSubnet(ctx workflow.Context, subnetID uuid.UUID, vpcID uuid.UUID) error {
	logger := log.With().Str("Workflow", "Subnet").Str("Action", "Delete").Str("Subnet ID", subnetID.String()).
		Str("VPC ID", vpcID.String()).Logger()

	logger.Info().Msg("starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    10,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 3 * time.Minute,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var subnetManager subnetActivity.ManageSubnet

	err := workflow.ExecuteActivity(ctx, subnetManager.DeleteSubnetViaSiteAgent, subnetID, vpcID).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to delete activity: DeleteSubnetViaSiteAgent")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// ExecuteDeleteSubnetWorkflow is a helper function to trigger execution of delete Subnet workflow
func ExecuteDeleteSubnetWorkflow(ctx context.Context, tc client.Client, subnetID uuid.UUID, vpcID uuid.UUID) (*string, error) {
	uid := uuid.New()

	workflowOptions := client.StartWorkflowOptions{
		ID:        "subnet-delete-" + uid.String(),
		TaskQueue: queue.CloudTaskQueue,
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, DeleteSubnet, subnetID, vpcID)

	if err != nil {
		log.Error().Err(err).Msg("failed to execute workflow: DeleteSubnet")
		return nil, err
	}

	wid := we.GetID()

	return &wid, nil
}
