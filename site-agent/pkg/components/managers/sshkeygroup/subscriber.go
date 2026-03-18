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

package sshkeygroup

import (
	"go.temporal.io/sdk/activity"
	workflow "go.temporal.io/sdk/workflow"

	swa "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/activity"
	sww "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/workflow"
)

// RegisterSubscriber registers the SSHKeyGroupWorkflows with the Temporal client
func (api *API) RegisterSubscriber() error {

	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: Registering the subscribers")

	// Get sshkeygroup workflow interface
	sshkeyinterface := NewSSHKeyGroupWorkflows(
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Publisher,
		ManagerAccess.Data.EB.Managers.Workflow.Temporal.Subscriber,
		ManagerAccess.Conf.EB,
	)

	sshkeygroupManager := swa.NewManageSSHKeyGroup(ManagerAccess.Data.EB.Managers.Carbide.Client)

	// Register Site Workflows

	// Register CreateSSHKeyGroupV2 workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.CreateSSHKeyGroupV2)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the CreateSSHKeyGroupV2 workflow")

	// Register UpdateSSHKeyGroupV2 workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.UpdateSSHKeyGroupV2)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the UpdateSSHKeyGroupV2 workflow")

	// Register DeleteSSHKeyGroupV2 workflow
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflow(sww.DeleteSSHKeyGroupV2)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the DeleteSSHKeyGroupV2 workflow")

	// Register Site Workflow Activities

	// Register CreateSSHKeyGroupActivityV2 activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(sshkeygroupManager.CreateSSHKeyGroupOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the CreateSSHKeyGroupOnSiteV1 activity")

	// Register UpdateSSHKeyGroupActivityV2 activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(sshkeygroupManager.UpdateSSHKeyGroupOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the UpdateSSHKeyGroupOnSiteV1 activity")

	// Register DeleteSSHKeyGroupActivityV2 activity
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivity(sshkeygroupManager.DeleteSSHKeyGroupOnSite)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the DeleteSSHKeyGroupOnSiteV1 activity")

	// Register legacy workflows, to be removed in future
	wflowRegisterOptions := workflow.RegisterOptions{
		Name: "CreateSSHKeyGroup",
	}
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflowWithOptions(
		ManagerAccess.API.SSHKeyGroup.CreateSSHKeyGroup, wflowRegisterOptions,
	)

	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the create SSHKeyGroup workflow")

	wflowRegisterOptions = workflow.RegisterOptions{
		Name: "UpdateSSHKeyGroup",
	}
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflowWithOptions(
		ManagerAccess.API.SSHKeyGroup.UpdateSSHKeyGroup, wflowRegisterOptions,
	)

	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the create SSHKeyGroup workflow")

	wflowRegisterOptions = workflow.RegisterOptions{
		Name: "DeleteSSHKeyGroup",
	}
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflowWithOptions(
		ManagerAccess.API.SSHKeyGroup.DeleteSSHKeyGroup, wflowRegisterOptions,
	)

	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the delete SSHKeyGroup workflow")

	wflowRegisterOptions = workflow.RegisterOptions{
		Name: "GetSSHKeyGroup",
	}

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterWorkflowWithOptions(
		ManagerAccess.API.SSHKeyGroup.GetSSHKeyGroup, wflowRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the GetSSHKeyGroup SSHKeyGroup workflow")

	// Register legacy activities, to be removed in the future
	activityRegisterOptions := activity.RegisterOptions{
		Name: "CreateSSHKeyGroupActivity",
	}

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		sshkeyinterface.CreateSSHKeyGroupActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the Create SSHKeyGroup activity")

	activityRegisterOptions = activity.RegisterOptions{
		Name: "UpdateSSHKeyGroupActivity",
	}

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		sshkeyinterface.UpdateSSHKeyGroupActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the Update SSHKeyGroup activity")

	activityRegisterOptions = activity.RegisterOptions{
		Name: "DeleteSSHKeyGroupActivity",
	}

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		sshkeyinterface.DeleteSSHKeyGroupActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the Delete SSHKeyGroup activity")

	activityRegisterOptions = activity.RegisterOptions{
		Name: "GetSSHKeyGroupActivity",
	}
	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		sshkeyinterface.GetSSHKeyGroupActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: successfully registered the GetSSHKeyGroupByName SSHKeyGroup activity")

	return nil
}

// RegisterSubscribers - this is method 2 of registering the subscriber
func RegisterSubscribers() {
	// Register the subscribers here
	ManagerAccess.Data.EB.Log.Info().Msg("SSHKeyGroup: Registering the subscribers")
	ManagerAccess.API.Orchestrator.AddWorkflow(ManagerAccess.API.SSHKeyGroup.CreateSSHKeyGroup)
}
