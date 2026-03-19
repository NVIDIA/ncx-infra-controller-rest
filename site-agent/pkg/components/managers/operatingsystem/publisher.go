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
	"github.com/google/uuid"

	swa "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/activity"
	sww "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/workflow"
)

// RegisterPublisher registers the OsImage Workflows with the Temporal client
func (api *API) RegisterPublisher() error {
	// Register publisher workflows

	// Collect and Publish OsImage Inventory workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DiscoverOsImageInventory)
	ManagerAccess.Data.EB.Log.Info().Msg("OsImage: successfully registered DiscoverOsImageInventory workflow")

	// Register OsImage activity for discovering and publishing OsImage Inventory
	OsImageInventoryManager := swa.NewManageOsImageInventory(swa.ManageInventoryConfig{
		SiteID:                uuid.MustParse(ManagerAccess.Conf.EB.Temporal.ClusterID),
		CarbideAtomicClient:   ManagerAccess.Data.EB.Managers.Carbide.Client,
		TemporalPublishClient: ManagerAccess.Data.EB.Managers.Workflow.Temporal.Publisher,
		TemporalPublishQueue:  ManagerAccess.Conf.EB.Temporal.TemporalPublishQueue,
		SitePageSize:          InventoryCarbidePageSize,
		CloudPageSize:         InventoryCloudPageSize,
	})

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(OsImageInventoryManager.DiscoverOsImageInventory)
	ManagerAccess.Data.EB.Log.Info().Msg("OsImage: successfully registered DiscoverOsImageInventory activity")

	api.RegisterCron()
	return nil
}
