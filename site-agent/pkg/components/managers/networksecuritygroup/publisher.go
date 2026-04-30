/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package networksecuritygroup

import (
	"github.com/google/uuid"

	swa "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/activity"
	sww "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/workflow"
)

// RegisterPublisher registers NetworkSecurityGroup inventory workflow and activity with Temporal
func (api *API) RegisterPublisher() error {
	ManagerAccess.Data.EB.Log.Info().Msg("NetworkSecurityGroup: Registering inventory workflow and activity")

	// Register DiscoverNetworkSecurityGroupInventory workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DiscoverNetworkSecurityGroupInventory)
	ManagerAccess.Data.EB.Log.Info().Msg("NetworkSecurityGroup: Successfully registered DiscoverNetworkSecurityGroupInventory workflow")

	// Register DiscoverNetworkSecurityGroupInventory activity
	inventoryManager := swa.NewManageNetworkSecurityGroupInventory(swa.ManageInventoryConfig{
		SiteID:                uuid.MustParse(ManagerAccess.Conf.EB.Temporal.ClusterID),
		CarbideAtomicClient:   ManagerAccess.Data.EB.Managers.Carbide.Client,
		TemporalPublishClient: ManagerAccess.Data.EB.Managers.Workflow.Temporal.Publisher,
		TemporalPublishQueue:  ManagerAccess.Conf.EB.Temporal.TemporalPublishQueue,
		SitePageSize:          InventoryCarbidePageSize,
		CloudPageSize:         InventoryCloudPageSize,
	})

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(inventoryManager.DiscoverNetworkSecurityGroupInventory)
	ManagerAccess.Data.EB.Log.Info().Msg("NetworkSecurityGroup: Successfully registered DiscoverNetworkSecurityGroupInventory activity")

	api.RegisterCron()

	return nil
}
