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
	swa "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/activity"
	sww "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/workflow"
)

// RegisterSubscriber registers the OsImage and OperatingSystem workflows/activities with the Temporal client
func (api *API) RegisterSubscriber() error {
	ManagerAccess.Data.EB.Log.Info().Msg("OperatingSystem: Registering the subscribers")

	osManager := swa.NewManageOperatingSystem(ManagerAccess.Data.EB.Managers.Carbide.Client)

	// ── OsImage workflows (cloud-managed image catalog pushed to site) ─────────────────
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.CreateOsImage)
	ManagerAccess.Data.EB.Log.Info().Msg("OsImage: successfully registered the CreateOsImage workflow")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.UpdateOsImage)
	ManagerAccess.Data.EB.Log.Info().Msg("OsImage: successfully registered the UpdateOsImage workflow")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DeleteOsImage)
	ManagerAccess.Data.EB.Log.Info().Msg("OsImage: successfully registered the DeleteOsImage workflow")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(osManager.CreateOsImageOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("OsImage: successfully registered CreateOsImageOnSite activity")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(osManager.UpdateOsImageOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("OsImage: successfully registered UpdateOsImageOnSite activity")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(osManager.DeleteOsImageOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("OsImage: successfully registered DeleteOsImageOnSite activity")

	// ── OperatingSystem workflows (bi-directional: carbide-rest ↔ carbide-core) ────────
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.CreateOperatingSystem)
	ManagerAccess.Data.EB.Log.Info().Msg("OperatingSystem: successfully registered the CreateOperatingSystem workflow")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.UpdateOperatingSystem)
	ManagerAccess.Data.EB.Log.Info().Msg("OperatingSystem: successfully registered the UpdateOperatingSystem workflow")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DeleteOperatingSystem)
	ManagerAccess.Data.EB.Log.Info().Msg("OperatingSystem: successfully registered the DeleteOperatingSystem workflow")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(osManager.CreateOperatingSystemOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("OperatingSystem: successfully registered CreateOperatingSystemOnSite activity")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(osManager.UpdateOperatingSystemOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("OperatingSystem: successfully registered UpdateOperatingSystemOnSite activity")

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(osManager.DeleteOperatingSystemOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("OperatingSystem: successfully registered DeleteOperatingSystemOnSite activity")

	return nil
}
