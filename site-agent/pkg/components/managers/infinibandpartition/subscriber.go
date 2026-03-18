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

package infinibandpartition

import (
	"go.temporal.io/sdk/activity"
	workflow "go.temporal.io/sdk/workflow"

	swa "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/activity"
	sww "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/workflow"
)

// RegisterSubscriber registers the InfiniBandPartitionWorkflows with the Temporal client
func (api *API) RegisterSubscriber() error {

	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("InfiniBandPartition: Registering the subscribers")

	ibpManager := swa.NewManageInfiniBandPartition(ManagerAccess.Data.EB.Managers.Carbide.Client)

	// Get InfiniBandPartition workflow interface
	infiniBandPartitionInterface := NewInfiniBandPartitionWorkflows(
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Publisher,
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Subscriber,
		ManagerAccess.Conf.EB,
	)

	// Register worfklow

	// Sync workflows

	// CreateInfiniBandPartitionV2
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.CreateInfiniBandPartitionV2)
	ManagerAccess.Data.EB.Log.Info().Msg("InfiniBandPartition: successfully registered CreateInfiniBandPartitionV2 workflow")

	// DeleteInfiniBandPartitionV2
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DeleteInfiniBandPartitionV2)
	ManagerAccess.Data.EB.Log.Info().Msg("InfiniBandPartition: successfully registered DeleteInfiniBandPartitionV2 workflow")

	wflowRegisterOptions := workflow.RegisterOptions{
		Name: "CreateInfiniBandPartition",
	}
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflowWithOptions(
		ManagerAccess.API.InfiniBandPartition.CreateInfiniBandPartition, wflowRegisterOptions,
	)

	ManagerAccess.Data.EB.Log.Info().Msg("InfiniBandPartition: successfully registered the create InfiniBandPartition workflow")

	wflowRegisterOptions = workflow.RegisterOptions{
		Name: "DeleteInfiniBandPartition",
	}
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflowWithOptions(
		ManagerAccess.API.InfiniBandPartition.DeleteInfiniBandPartition, wflowRegisterOptions,
	)

	ManagerAccess.Data.EB.Log.Info().Msg("InfiniBandPartition: successfully registered the delete InfiniBandPartition workflow")

	// Register activity

	// Sync workflow activities

	// CreateInfiniBandPartitionOnSite
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(ibpManager.CreateInfiniBandPartitionOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("VPC: successfully registered the CreateInfiniBandPartitionOnSite activity")

	// DeleteInfiniBandPartitionOnSite
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(ibpManager.DeleteInfiniBandPartitionOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("VPC: successfully registered the DeleteInfiniBandPartitionOnSite activity")

	activityRegisterOptions := activity.RegisterOptions{
		Name: "CreateInfiniBandPartitionActivity",
	}

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		infiniBandPartitionInterface.CreateInfiniBandPartitionActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("InfiniBandPartition: successfully registered the Create InfiniBandPartition activity")

	activityRegisterOptions = activity.RegisterOptions{
		Name: "DeleteInfiniBandPartitionActivity",
	}

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		infiniBandPartitionInterface.DeleteInfiniBandPartitionActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("InfiniBandPartition: successfully registered the Delete InfiniBandPartition activity")

	return nil
}

// RegisterSubscribers - this is method 2 of registering the subscriber
func RegisterSubscribers() {
	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("InfiniBandPartition: Registering the subscribers")
	ManagerAccess.API.Orchestrator.AddWorkflow(ManagerAccess.API.InfiniBandPartition.CreateInfiniBandPartition)
}
