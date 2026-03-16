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

package expectedswitch

import (
	swa "github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/activity"
	sww "github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/workflow"
)

// RegisterSubscriber registers the ExpectedSwitchWorkflows with the Temporal client
func (api *API) RegisterSubscriber() error {
	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedSwitch: Registering the subscribers")

	// Register workflows
	// Register CreateExpectedSwitch workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.CreateExpectedSwitch)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedSwitch: successfully registered the CreateExpectedSwitch workflow")

	// Register UpdateExpectedSwitch workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.UpdateExpectedSwitch)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedSwitch: successfully registered the UpdateExpectedSwitch workflow")

	// Register DeleteExpectedSwitch workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DeleteExpectedSwitch)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedSwitch: successfully registered the DeleteExpectedSwitch workflow")

	// Register activities
	expectedSwitchManager := swa.NewManageExpectedSwitch(ManagerAccess.Data.EB.Managers.Carbide.Client)

	// Register CreateExpectedSwitchOnSite activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(expectedSwitchManager.CreateExpectedSwitchOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedSwitch: successfully registered the CreateExpectedSwitchOnSite activity")

	// Register UpdateExpectedSwitchOnSite activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(expectedSwitchManager.UpdateExpectedSwitchOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedSwitch: successfully registered the UpdateExpectedSwitchOnSite activity")

	// Register DeleteExpectedSwitchOnSite activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(expectedSwitchManager.DeleteExpectedSwitchOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedSwitch: successfully registered the DeleteExpectedSwitchOnSite activity")

	return nil
}
