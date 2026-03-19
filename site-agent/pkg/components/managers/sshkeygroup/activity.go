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
	"context"
	"errors"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.temporal.io/sdk/temporal"
)

// CreateSSHKeyGroupActivity - Create SSHKeyGroup Activity
func (ac *Workflows) CreateSSHKeyGroupActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.CreateSSHKeyGroupRequest) (*wflows.SSHKeyGroupInfo, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "CreateSSHKeyGroupActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting activity")

	var sshkeygroupRequest *wflows.SSHKeyGroupInfo

	sshkeygroupRequest = &wflows.SSHKeyGroupInfo{
		TenantKeyset: &wflows.TenantKeyset{},
	}

	wflowMetadata := &sshkgWorkflowMetadata{
		activity: activityCreate,
		response: &wflows.SSHKeyGroupInfo{TenantKeyset: sshkeygroupRequest.TenantKeyset},
	}

	// Validate the ResourceReq
	if ResourceReq == nil {
		// Return error here
		// Bail out earlier
		err := errors.New("invalid or empty request provided as activity argument")
		logger.Error().Err(err).Msg("Failed to create SSHKeyGroup, invalid request")
		wflowMetadata.response.StatusMsg = err.Error()
		wflowMetadata.response.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		return nil, temporal.NewApplicationError(err.Error(), "", &wflowMetadata.response)
	}
	logger.Info().Str("KeysetId", ResourceReq.KeysetId).Msg("Validated SSHKeyGroup request, proceeding with creation")

	TenantKeysetresponse, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, ResourceReq, wflowMetadata)
	ResourceResp := wflowMetadata.response
	if err != nil {
		logger.Error().Err(err).Str("KeysetId", ResourceReq.KeysetId).Msg("Error creating SSHKeyGroup on site via Orchestrator")
		// Update response status on failure
		ResourceResp.StatusMsg = err.Error()
		ResourceResp.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		return nil, err
	}

	if resp, ok := TenantKeysetresponse.(*wflows.CreateTenantKeysetResponse); ok {
		ResourceResp.TenantKeyset = resp.Keyset
		logger.Info().Str("KeysetId", ResourceReq.KeysetId).Msg("Successfully completed activity")
	} else {
		err = errors.New("invalid response received from Site Controller")
		logger.Error().Err(err).Msg("Failed to create SSHKeyGroup, invalid response")
		ResourceResp.StatusMsg = err.Error()
		ResourceResp.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		return nil, err
	}

	return ResourceResp, err
}

// UpdateSSHKeyGroupActivity updates the sshkeygroup at carbide
func (ac *Workflows) UpdateSSHKeyGroupActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.UpdateSSHKeyGroupRequest) (*wflows.SSHKeyGroupInfo, error) {
	var sshkeygroupRequest *wflows.SSHKeyGroupInfo
	var err error
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "UpdateSSHKeyGroupActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting Activity")

	sshkeygroupRequest = &wflows.SSHKeyGroupInfo{
		TenantKeyset: &wflows.TenantKeyset{},
	}

	wflowMetadata := &sshkgWorkflowMetadata{
		activity: activityUpdate,
		response: &wflows.SSHKeyGroupInfo{TenantKeyset: sshkeygroupRequest.TenantKeyset},
	}

	// Validate the ResourceReq
	if ResourceReq == nil {
		// Return error here
		// Bail out earlier
		err = errors.New("invalid or empty request provided as activity argument")
		logger.Error().Err(err).Msg("Failed to update SSHKeyGroup, invalid request")
		wflowMetadata.response.StatusMsg = err.Error()
		wflowMetadata.response.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		return nil, temporal.NewApplicationError(err.Error(), "", &wflowMetadata.response)
	}
	logger.Info().Str("ID", ResourceReq.KeysetId).Msg("Validated SSHKeyGroup update request, proceeding with update")

	TenantKeysetresponse, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, ResourceReq, wflowMetadata)
	ResourceResp := wflowMetadata.response
	if err != nil {
		logger.Error().Err(err).Str("KeysetId", ResourceReq.KeysetId).Msg("Error updating SSHKeyGroup on site via Orchestrator")
		return nil, err
	}

	if _, ok := TenantKeysetresponse.(*wflows.UpdateTenantKeysetResponse); ok {
		logger.Info().Str("KeysetId", ResourceReq.KeysetId).Msg("Successfully completed activity")
	} else {
		err = errors.New("unexpected response received from Site Controller")
		logger.Error().Err(err).Msg("Failed to update SSHKeyGroup, unexpected response")
		return nil, err
	}

	return ResourceResp, nil
}

// DeleteSSHKeyGroupActivity deletes the sshkeygroup at carbide
func (ac *Workflows) DeleteSSHKeyGroupActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.DeleteSSHKeyGroupRequest) (*wflows.SSHKeyGroupInfo, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "DeleteSSHKeyGroupActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting Activity")

	sshkeygroupRequest := &wflows.SSHKeyGroupInfo{
		TenantKeyset: &wflows.TenantKeyset{},
	}

	wflowMetadata := &sshkgWorkflowMetadata{
		activity: activityDelete,
		response: &wflows.SSHKeyGroupInfo{TenantKeyset: sshkeygroupRequest.TenantKeyset},
	}

	// Validate the ResourceReq
	if ResourceReq == nil {
		err := errors.New("invalid or empty request provided as activity argument")
		logger.Error().Err(err).Msg("Failed to delete SSHKeyGroup, invalid request")
		wflowMetadata.response.StatusMsg = err.Error()
		wflowMetadata.response.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		return nil, temporal.NewApplicationError(err.Error(), "", wflowMetadata.response)
	}
	logger.Info().Str("ID", ResourceReq.KeysetId).Msg("Validated SSHKeyGroup deletion request, proceeding with deletion")

	_, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, ResourceReq, wflowMetadata)
	if err != nil {
		logger.Error().Str("KeysetId", ResourceReq.KeysetId).Err(err).Msg("Error during SSHKeyGroup deletion from site via Orchestrator")
		wflowMetadata.response.StatusMsg = err.Error()
		wflowMetadata.response.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
	} else {
		logger.Info().Str("KeysetId", ResourceReq.KeysetId).Msg("Successfully completed activity")
		wflowMetadata.response.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_SUCCESS // Ensure response status is updated accordingly
	}

	return wflowMetadata.response, err
}

// GetSSHKeyGroupActivity Gets the sshkeygroup at carbide
func (ac *Workflows) GetSSHKeyGroupActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.GetSSHKeyGroup) (*wflows.GetSSHKeyGroupResponse, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "GetSSHKeyGroupActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting Activity")

	wflowMetadata := &sshkgWorkflowMetadata{
		activity: activityGet,
		respList: &wflows.GetSSHKeyGroupResponse{},
	}

	response, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, ResourceReq, wflowMetadata)
	ResourceResp := wflowMetadata.respList
	if err != nil {
		logger.Error().Err(err).Msg("Error retrieving SSHKeyGroup from site via Orchestrator")
		return nil, err
	}

	if resp, ok := response.(*wflows.TenantKeySetList); ok && resp != nil {
		// Log if the response type assertion fails
		ResourceResp.List = resp
		logger.Info().Msg("Successfully completed activity")
		return ResourceResp, nil
	}

	err = errors.New("invalid or empty response received from Site Controller")
	logger.Error().Err(err).Msg("Failed to retrieve SSHKeyGroup, invalid or empty response")
	return nil, err
}
