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
)

// RegisterSubscriber registers the SubnetWorkflows with the Temporal client
func (api *API) RegisterSubscriber() error {
	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: Registering the subscribers")

	// Get Subnet workflow interface
	Subnetinterface := NewSubnetWorkflows(
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Publisher,
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Subscriber,
		ManagerAccess.Conf.EB,
	)

	subnetManager := swa.NewManageSubnet(ManagerAccess.Data.EB.Managers.Carbide.Client)

	//  Register Workflows

	// Sync workflows
	// Register CreateSubnet worfklow
	// TODO: Once all Site Agents are updated, remove the legacy CreateSubnet workflow, duplicate register Site Workflow as CreateSubnet
	// Once all Site Agents are updated with duplicate workflow, switch Cloud API to use call CreateSubnet, then de-register CreateSubnetV2 workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.CreateSubnetV2)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered Create Subnet v2 workflow")

	// Register DeleteSubnet worfklow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DeleteSubnetV2)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered Delete Subnet v2 workflow")

	// Legacy workflows
	// Register CreateSubnet worfklow (deprecated)
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(ManagerAccess.API.Subnet.CreateSubnet)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered deprecated Create Subnet workflow")

	// Register DeleteSubnet worfklow (deprecated)
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(ManagerAccess.API.Subnet.DeleteSubnet)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered Delete Subnet workflow")

	// Regsiter Activities

	// Sync workflow activities
	// Register CreateSubnetOnSite
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(subnetManager.CreateSubnetOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered Create Subnet activity")

	// Register DeleteSubnetOnSite
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(subnetManager.DeleteSubnetOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered Delete Subnet activity")

	// Legacy workflow activities
	// Register CreateSubnetActivity (deprecated)
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(Subnetinterface.CreateSubnetActivity)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered deprecated Create Subnet Activity")

	// Register DeleteSubnetActivity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(Subnetinterface.DeleteSubnetActivity)
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: successfully registered Delete Subnet activity")

	return nil
}

// RegisterSubscribers - this is method 2 of registering the subscriber
func RegisterSubscribers() {
	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: Registering the subscribers")
	ManagerAccess.API.Orchestrator.AddWorkflow(ManagerAccess.API.Subnet.CreateSubnet)
}
