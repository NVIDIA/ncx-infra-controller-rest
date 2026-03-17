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
	sww "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/workflow"
	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"

	swa "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/activity"
)

// RegisterPublisher registers the InstanceWorkflows with the Temporal client
func (api *API) RegisterPublisher() error {

	// Register the publishers here
	ManagerAccess.Data.EB.Log.Info().Msg("Instance: Registering the publishers")

	// Get Instance workflow interface
	Instanceinterface := NewInstanceWorkflows(
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Publisher,
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Subscriber,
		ManagerAccess.Conf.EB,
	)
	activityRegisterOptions := activity.RegisterOptions{
		Name: "PublishInstanceActivity",
	}

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		Instanceinterface.PublishInstanceActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("Instance: successfully registered the Publish Instance activity")

	activityRegisterOptions = activity.RegisterOptions{
		Name: "PublishInstancePowerStatus",
	}
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		Instanceinterface.PublishInstancePowerStatus, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("Instance: successfully registered the PublishInstancePowerStatus activity")

	// Instance Inventory workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DiscoverInstanceInventory)
	ManagerAccess.Data.EB.Log.Info().Msg("Instance: successfully registered the Discover Instance Inventory workflow")

	instanceInventoryManager := swa.NewManageInstanceInventory(swa.ManageInventoryConfig{
		SiteID:                uuid.MustParse(ManagerAccess.Conf.EB.Temporal.ClusterID),
		CarbideAtomicClient:   ManagerAccess.Data.EB.Managers.Carbide.Client,
		TemporalPublishClient: ManagerAccess.Data.EB.Managers.Workflow.Temporal.Publisher,
		TemporalPublishQueue:  ManagerAccess.Conf.EB.Temporal.TemporalPublishQueue,
		SitePageSize:          InventoryCarbidePageSize,
		CloudPageSize:         InventoryCloudPageSize,
	})
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(instanceInventoryManager.DiscoverInstanceInventory)
	ManagerAccess.Data.EB.Log.Info().Msg("Instance: successfully registered the Discover Instance Inventory activity")

	api.RegisterCron()
	return nil
}
