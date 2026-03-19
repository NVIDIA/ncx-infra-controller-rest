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

package instance

import (
	"context"
	"errors"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.temporal.io/sdk/temporal"
)

// CreateInstanceActivity - temporal activity
// TODO: Remove (Deprecated)
func (ac *Workflows) CreateInstanceActivity(ctx context.Context, resourceVer uint64, resourceID string, resourceReq *wflows.CreateInstanceRequest) (*wflows.InstanceInfo, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "CreateInstanceActivity").Str("ResourceID", resourceID).Logger()
	logger.Info().Msg("Starting activity")

	wflowMd := &instanceWorkflowMetadata{
		activity: activityCreate,
		response: &wflows.InstanceInfo{Instance: &wflows.Instance{}},
	}

	response, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, resourceVer, resourceID, resourceReq, wflowMd)
	if err != nil {
		logger.Error().Err(err).Msg("Error during instance creation on site via Orchestrator")
		return nil, err
	}

	ResourceResp := wflowMd.response
	if ResourceResp == nil {
		err = errors.New("invalid or empty response received from Site Controller")
		logger.Error().Err(err).Msg("Failed to create instance, invalid or empty response")
		return ResourceResp, err
	}

	if resp, ok := response.(*wflows.Instance); ok && resp != nil {
		ResourceResp.Instance = resp
		logger.Info().Msg("Successfully completed activity")
	} else {
		err = errors.New("invalid response received from Site Controller")
		logger.Error().Err(err).Msg("Failed to create instance, invalid response")
		return nil, err
	}

	return ResourceResp, nil
}

// DeleteInstanceActivity - temporal activity
func (ac *Workflows) DeleteInstanceActivity(ctx context.Context, resourceVer uint64, resourceID string, resourceReq string) (*wflows.InstanceInfo, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "DeleteInstanceActivity").Str("ResourceID", resourceID).Logger()
	logger.Info().Msg("Starting activity")

	wflowMd := &instanceWorkflowMetadata{
		activity: activityDelete,
		response: &wflows.InstanceInfo{Instance: &wflows.Instance{}},
	}

	if resourceReq == "" {
		err := errors.New("invalid or empty instance ID provided as activity argument")
		logger.Error().Err(err).Msg("Failed to delete instance, invalid or empty instance ID")
		wflowMd.response.StatusMsg = err.Error()
		wflowMd.response.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		return nil, temporal.NewApplicationError(err.Error(), "", wflowMd.response)
	}
	logger.Info().Msg("Validated instance ID, proceeding with deletion")

	request := &wflows.DeleteInstanceRequest{
		InstanceId: &wflows.UUID{
			Value: resourceReq,
		},
	}
	wflowMd.response.Instance.Id = &wflows.InstanceId{Value: request.InstanceId.Value}
	_, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, resourceVer, resourceID, request, wflowMd)
	if err != nil {
		logger.Error().Err(err).Msg("Error during instance deletion from Site via Orchestrator")
		return nil, err
	}

	logger.Info().Msg("Successfully completed activity")
	return wflowMd.response, nil
}

// RebootInstanceActivity - temporal activity
// TODO: Remove (deprecated)
func (ac *Workflows) RebootInstanceActivity(ctx context.Context, resourceVer uint64, resourceID string, resourceReq *wflows.RebootInstanceRequest) (*wflows.InstanceRebootInfo, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "RebootInstanceActivity").Str("ResourceID", resourceID).Logger()
	logger.Info().Msg("Starting activity")

	wflowMd := &instanceWorkflowMetadata{
		activity:       activityReboot,
		rebootResponse: &wflows.InstanceRebootInfo{MachineId: &wflows.MachineId{}},
	}

	if resourceReq == nil || resourceReq.MachineId == nil || resourceReq.MachineId.Id == "" {
		errMsg := "invalid or empty reboot request provided as activity argument"
		err := errors.New(errMsg)
		logger.Error().Err(err).Msg("Failed to reboot instance, invalid or empty request")
		wflowMd.rebootResponse.StatusMsg = errMsg
		wflowMd.rebootResponse.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		return nil, temporal.NewApplicationError(errMsg, "", wflowMd.rebootResponse)
	}

	wflowMd.rebootResponse.MachineId = resourceReq.MachineId

	logger.Info().Str("MachineID", resourceReq.MachineId.Id).Msg("Validated reboot request, proceeding with reboot")

	_, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, resourceVer, resourceID, resourceReq, wflowMd)
	if err != nil {
		logger.Error().Str("MachineID", resourceReq.MachineId.Id).Err(err).Msg("Error during instance reboot from site via Orchestrator")
		return nil, err
	}

	logger.Info().Str("MachineID", resourceReq.MachineId.Id).Msg("Successfully completed activity")
	return wflowMd.rebootResponse, nil
}
