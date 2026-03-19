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
	swa "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/activity"
	sww "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/workflow"
	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
)

// RegisterPublisher registers the SubnetWorkflows with the Temporal client
func (api *API) RegisterPublisher() error {

	// Register the publishers here
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: Registering the publishers")

	// Get Subnet workflow interface
	Subnetinterface := NewSubnetWorkflows(
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Publisher,
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Subscriber,
		ManagerAccess.Conf.EB,
	)
	activityRegisterOptions := activity.RegisterOptions{
		Name: "PublishSubnetActivity",
	}
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		Subnetinterface.PublishSubnetActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered the Publish Subnet activity")

	// Subnet Inventory workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DiscoverSubnetInventory)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered the Discover Subnet Inventory workflow")

	inventoryManager := swa.NewManageSubnetInventory(swa.ManageInventoryConfig{
		SiteID:                uuid.MustParse(ManagerAccess.Conf.EB.Temporal.ClusterID),
		CarbideAtomicClient:   ManagerAccess.Data.EB.Managers.Carbide.Client,
		TemporalPublishClient: ManagerAccess.Data.EB.Managers.Workflow.Temporal.Publisher,
		TemporalPublishQueue:  ManagerAccess.Conf.EB.Temporal.TemporalPublishQueue,
		SitePageSize:          InventoryCarbidePageSize,
		CloudPageSize:         InventoryCloudPageSize,
	})
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(inventoryManager.DiscoverSubnetInventory)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered the Discover Subnet Inventory activity")

	api.RegisterCron()

	return nil
}
