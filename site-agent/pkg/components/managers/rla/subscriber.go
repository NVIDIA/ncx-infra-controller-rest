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

package rla

import (
	swa "github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/activity"
	sww "github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/workflow"
)

// RegisterSubscriber registers the RLA Rack and Tray workflows with the Temporal client
func (api *API) RegisterSubscriber() error {
	// Check if RLA is enabled
	if !ManagerAccess.Conf.EB.RLA.Enabled {
		ManagerAccess.Data.EB.Log.Info().Msg("RLA: RLA is disabled, skipping workflow registration")
		return nil
	}

	rackManager := swa.NewManageRack(ManagerAccess.Data.EB.Managers.RLA.Client)
	trayManager := swa.NewManageTray(ManagerAccess.Data.EB.Managers.RLA.Client)

	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: Registering the rack workflows")

	/// Register rack workflows

	// GetRack
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.GetRack)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered GetRack workflow")

	// GetRacks
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.GetRacks)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered GetRacks workflow")

	// ValidateRackComponents
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.ValidateRackComponents)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered ValidateRackComponents workflow")

	// PowerOnRack
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.PowerOnRack)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered PowerOnRack workflow")

	// PowerOffRack
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.PowerOffRack)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered PowerOffRack workflow")

	// PowerResetRack
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.PowerResetRack)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered PowerResetRack workflow")

	// UpgradeFirmware
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.UpgradeFirmware)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered UpgradeFirmware workflow")

	/// Register rack activities

	// GetRack activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(rackManager.GetRack)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered GetRack activity")

	// GetRacks activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(rackManager.GetRacks)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered GetRacks activity")

	// ValidateRackComponents activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(rackManager.ValidateRackComponents)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered ValidateRackComponents activity")

	// PowerOnRack activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(rackManager.PowerOnRack)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered PowerOnRack activity")

	// PowerOffRack activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(rackManager.PowerOffRack)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered PowerOffRack activity")

	// PowerResetRack activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(rackManager.PowerResetRack)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered PowerResetRack activity")

	// UpgradeFirmware activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(rackManager.UpgradeFirmware)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered UpgradeFirmware activity")

	// Register the tray subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: Registering the tray workflows")

	/// Register tray workflows

	// GetTray
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.GetTray)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered GetTray workflow")

	// GetTrays
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.GetTrays)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered GetTrays workflow")

	/// Register tray activities

	// GetTray activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(trayManager.GetTray)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered GetTray activity")

	// GetTrays activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(trayManager.GetTrays)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered GetTrays activity")

	return nil
}
