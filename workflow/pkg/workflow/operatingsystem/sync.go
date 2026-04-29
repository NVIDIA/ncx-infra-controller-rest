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

package operatingsystem

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	temporalEnums "go.temporal.io/api/enums/v1"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	osActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/operatingsystem"
	"github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/queue"
)

const (
	SyncOperationCreate = "Create"
	SyncOperationUpdate = "Update"
	SyncOperationDelete = "Delete"
)

// SynchronizeOperatingSystem is a cloud-namespace Temporal workflow that propagates
// Operating System changes (create/update/delete) to all associated sites asynchronously.
func SynchronizeOperatingSystem(ctx workflow.Context, osID uuid.UUID, operation string) error {
	logger := log.With().Str("Workflow", "SynchronizeOperatingSystem").
		Str("OS ID", osID.String()).Str("Operation", operation).Logger()

	logger.Info().Msg("starting workflow")

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    15,
	}
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var osPushManager osActivity.ManageOperatingSystemPush

	err := workflow.ExecuteActivity(ctx, osPushManager.SynchronizeOperatingSystemToSites, osID, operation).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to execute activity: SynchronizeOperatingSystemToSites")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// ExecuteSynchronizeOperatingSystemWorkflow triggers the SynchronizeOperatingSystem workflow
// asynchronously on the cloud task queue. The handler does not wait for completion.
func ExecuteSynchronizeOperatingSystemWorkflow(ctx context.Context, tc client.Client, osID uuid.UUID, operation string) (*string, error) {
	workflowOptions := client.StartWorkflowOptions{
		ID:                    "synchronize-operating-system-" + osID.String() + "-" + operation,
		TaskQueue:             queue.CloudTaskQueue,
		WorkflowIDReusePolicy: temporalEnums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, SynchronizeOperatingSystem, osID, operation)
	if err != nil {
		log.Error().Err(err).Msg("failed to execute SynchronizeOperatingSystem workflow")
		return nil, err
	}

	wid := we.GetID()
	return &wid, nil
}
