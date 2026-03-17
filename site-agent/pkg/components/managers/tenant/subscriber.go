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

package tenant

import (
	swa "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/activity"
	sww "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/workflow"
)

// RegisterSubscriber registers Tenant workflows Site Agent subscribes to execute
func (api *API) RegisterSubscriber() error {
	//  Register Workflows

	// Sync workflows
	// Register CreateTenant worfklow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.CreateTenant)
	ManagerAccess.Data.EB.Log.Info().Msg("Tenant: successfully registered CreateTenant workflow")

	// Register UpdateTenant worfklow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.UpdateTenant)
	ManagerAccess.Data.EB.Log.Info().Msg("Tenant: successfully registered UpdateTenant workflow")

	// Regsiter Activities
	tenantManager := swa.NewManageTenant(ManagerAccess.Data.EB.Managers.Carbide.Client)

	// Sync workflow activities
	// Register CreateTenantOnSite
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(tenantManager.CreateTenantOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("Tenant: successfully registered CreateTenantOnSite activity")

	// Register UpdateTenantOnSite
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(tenantManager.UpdateTenantOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("Tenant: successfully registered UpdateTenantOnSite activity")

	return nil
}

// RegisterSubscribers - this is method 2 of registering the subscriber
func RegisterSubscribers() {
	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("Subnet: Registering the subscribers")
	ManagerAccess.API.Orchestrator.AddWorkflow(ManagerAccess.API.Subnet.CreateSubnet)
}
