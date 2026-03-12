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

package expectedpowershelf

import (
	swa "github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/activity"
	sww "github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/workflow"
)

// RegisterSubscriber registers the ExpectedPowerShelfWorkflows with the Temporal client
func (api *API) RegisterSubscriber() error {
	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedPowerShelf: Registering the subscribers")

	// Register workflows
	// Register CreateExpectedPowerShelf workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.CreateExpectedPowerShelf)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedPowerShelf: successfully registered the CreateExpectedPowerShelf workflow")

	// Register UpdateExpectedPowerShelf workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.UpdateExpectedPowerShelf)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedPowerShelf: successfully registered the UpdateExpectedPowerShelf workflow")

	// Register DeleteExpectedPowerShelf workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DeleteExpectedPowerShelf)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedPowerShelf: successfully registered the DeleteExpectedPowerShelf workflow")

	// Register activities
	expectedPowerShelfManager := swa.NewManageExpectedPowerShelf(ManagerAccess.Data.EB.Managers.Carbide.Client)

	// Register CreateExpectedPowerShelfOnSite activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(expectedPowerShelfManager.CreateExpectedPowerShelfOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedPowerShelf: successfully registered the CreateExpectedPowerShelfOnSite activity")

	// Register UpdateExpectedPowerShelfOnSite activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(expectedPowerShelfManager.UpdateExpectedPowerShelfOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedPowerShelf: successfully registered the UpdateExpectedPowerShelfOnSite activity")

	// Register DeleteExpectedPowerShelfOnSite activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(expectedPowerShelfManager.DeleteExpectedPowerShelfOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("ExpectedPowerShelf: successfully registered the DeleteExpectedPowerShelfOnSite activity")

	return nil
}
