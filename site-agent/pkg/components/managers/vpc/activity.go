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

package vpc

import (
	"context"
	"errors"
	"reflect"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

// TODO(deprecated): Remove any related DeleteVPC references after carbide-rest-api and agent are updated everywhere.  We've moved to sync workflow.
// TODO: Remove this and and any related references (deprecated).  We've moved to sync workflow.
// CreateVPCActivity - Create VPC Activity
func (ac *Workflows) CreateVPCActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.CreateVPCRequest) (*wflows.VPCInfo, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "CreateVPCActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting Activity")

	var vpcRequest *wflows.Vpc
	var err error
	if !reflect.ValueOf(ManagerAccess.Conf.EB.CloudVersion).IsZero() && !reflect.ValueOf(ManagerAccess.Conf.EB.SiteVersion).IsZero() && ManagerAccess.Conf.EB.CloudVersion != ManagerAccess.Conf.EB.SiteVersion {
		logger.Info().Str("CloudVersion", ManagerAccess.Conf.EB.CloudVersion).
			Str("SiteVersion", ManagerAccess.Conf.EB.SiteVersion).Msg("Transforming VPC Request due to version mismatch")

		// Transform the message according to the version
		transformRequest := &VPCReqTransformer{
			// This is the request coming from cloud
			FromVersion: ManagerAccess.Conf.EB.CloudVersion,
			ToVersion:   ManagerAccess.Conf.EB.SiteVersion,
			Op:          "create",
			Request:     ResourceReq,
		}
		vpcRequest, err = transformRequest.VPCRequestConverter()
		if err != nil {
			// Log error during request transformation
			logger.Error().Err(err).Msg("Failed to transform VPC request")
			return nil, err
		}
	} else {
		// Log VPC creation request
		logger.Info().Str("Name", ResourceReq.Name).Msg("Creating VPC Request directly")
		vpcRequest = &wflows.Vpc{
			Id:                   &wflows.VpcId{Value: ResourceReq.VpcId.Value},
			Name:                 ResourceReq.Name,
			TenantOrganizationId: ResourceReq.TenantOrganizationId,
		}
	}

	wflowMetadata := &vpcWorkflowMetadata{
		activity: activityCreate,
		response: &wflows.VPCInfo{
			Vpc: vpcRequest,
		},
	}

	vpcresponse, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, vpcRequest, wflowMetadata)
	ResourceResp := wflowMetadata.response
	if err != nil {
		// Log any errors encountered during the VPC creation activity
		logger.Error().Err(err).Msg("Failed to create VPC on Site via Orchestrator")
		return ResourceResp, err
	}

	if vpcresp, ok := vpcresponse.(*wflows.Vpc); ok {
		// Log the successful creation of the VPC
		ResourceResp.Vpc = vpcresp
		logger.Info().Str("VPCId", vpcRequest.Id.Value).Msg("Successfully completed activity")
	} else {
		// Log if the response type assertion fails or is unexpectedly nil
		err = errors.New("invalid or empty response received from Site Controller")
		logger.Error().Err(err).Msg("Failed to create subnet, invalid or empty response")
	}

	return ResourceResp, err
}

// UpdateVPCActivity updates the vpc at carbide
func (ac *Workflows) UpdateVPCActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.UpdateVPCRequest) (*wflows.VPCInfo, error) {
	var vpcRequest *wflows.Vpc
	var err error
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "UpdateVPCActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting Activity")

	if !reflect.ValueOf(ManagerAccess.Conf.EB.CloudVersion).IsZero() && !reflect.ValueOf(ManagerAccess.Conf.EB.SiteVersion).IsZero() && ManagerAccess.Conf.EB.CloudVersion != ManagerAccess.Conf.EB.SiteVersion {
		logger.Info().Str("CloudVersion", ManagerAccess.Conf.EB.CloudVersion).
			Str("SiteVersion", ManagerAccess.Conf.EB.SiteVersion).Msg("Transforming VPC Request due to version mismatch")

		transformRequest := &VPCReqTransformer{
			FromVersion: ManagerAccess.Conf.EB.CloudVersion,
			ToVersion:   ManagerAccess.Conf.EB.SiteVersion,
			Op:          "update",
			Request:     ResourceReq,
		}
		vpcRequest, err = transformRequest.VPCRequestConverter()
		if err != nil {
			logger.Error().Err(err).Msg("Failed to transform VPC update request")
			return nil, err
		}
	} else {
		logger.Info().Str("VPCId", ResourceReq.Id.Value).Str("Name", ResourceReq.Name).Msg("Updating VPC Request directly")
		vpcRequest = &wflows.Vpc{
			Id:                   &wflows.VpcId{Value: ResourceReq.Id.Value},
			Name:                 ResourceReq.Name,
			TenantOrganizationId: ResourceReq.TenantOrganizationId,
		}
	}

	wflowMetadata := &vpcWorkflowMetadata{
		activity: activityUpdate,
		response: &wflows.VPCInfo{
			Vpc: vpcRequest,
		},
	}

	_, err = ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, vpcRequest, wflowMetadata)
	if err != nil {
		// Log the successful update of the VPC
		logger.Error().Str("VPCId", vpcRequest.Id.Value).Err(err).Msg("Failed to update VPC on site via Orchestrator")
		return wflowMetadata.response, err
	}

	logger.Info().Str("VPCId", vpcRequest.Id.Value).Msg("Successfully completed activity")
	return wflowMetadata.response, nil
}

// DeleteVPCActivity deletes the vpc at carbide
func (ac *Workflows) DeleteVPCActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.DeleteVPCRequest) (*wflows.VPCInfo, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "DeleteVPCActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting Activity")

	vpcRequest := &wflows.Vpc{
		Id: &wflows.VpcId{Value: ResourceReq.Id.Value},
	}

	wflowMetadata := &vpcWorkflowMetadata{activity: activityDelete,
		response: &wflows.VPCInfo{Vpc: vpcRequest}}
	// Perform the deletion activity
	_, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, ResourceReq.Id.Value, wflowMetadata)
	if err != nil {
		// Log error if deletion fails
		logger.Error().Str("VPCId", ResourceReq.Id.Value).Err(err).Msg("Error deleting VPC from site via Orchestrator")
		return nil, err
	}

	// Log successful deletion
	logger.Info().Str("VPCId", ResourceReq.Id.Value).Msg("Successfully completed activity")
	return wflowMetadata.response, nil
}

// GetVPCByNameActivity Gets the vpc at carbide
func (ac *Workflows) GetVPCByNameActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.GetVPCByNameRequest) (*wflows.GetVPCResponse, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "GetVPCByNameActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting Activity")

	wflowMetadata := &vpcWorkflowMetadata{
		activity: activityGetByName,
		respList: &wflows.GetVPCResponse{},
	}
	vpcRequest := &wflows.VpcSearchFilter{
		Name: &ResourceReq.Name,
	}

	// Execute the orchestrator activity with the VPC search query
	vpcresponse, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, vpcRequest, wflowMetadata)
	ResourceResp := wflowMetadata.respList
	if err != nil {
		logger.Error().Str("VPCName", ResourceReq.Name).Err(err).Msg("Failed to retrieve VPC by name from site via Orchestrator")
		return nil, err
	}

	if vpcresp, ok := vpcresponse.(*wflows.VpcList); ok {
		ResourceResp.List = vpcresp
		logger.Info().Str("VPCName", ResourceReq.Name).Int("VPCCount", len(vpcresp.Vpcs)).Msg("Successfully completed activity")
	} else {
		err = errors.New("invalid or empty response received from Site Controller")
		logger.Error().Err(err).Msg("Failed to retrieve VPC by name, invalid or empty response")
	}

	return ResourceResp, err
}

// CollectVPCListActivity - activity to collect list of VPCs
func (ac *Workflows) CollectVPCListActivity(ctx context.Context, ResourceVer uint64,
	ResourceID string, ResourceReq *wflows.GetVPCByIdRequest) (inv *wflows.GetVPCResponse, err error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "CollectVPCListActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting Activity")

	wflowMd := &vpcWorkflowMetadata{
		activity: activityGetList,
		respList: &wflows.GetVPCResponse{},
	}
	vpcRequest := &wflows.VpcSearchFilter{}

	// Perform the VPC list collection activity
	vpcResponse, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, vpcRequest, wflowMd)
	if err != nil {
		logger.Error().Str("VPCId", ResourceReq.Id.Value).Err(err).Msg("Failed to collect VPC list from site via Orchestrator")
		return nil, err
	}

	if vpcList, ok := vpcResponse.(*wflows.VpcList); ok && vpcList != nil {
		inventory := wflowMd.respList
		inventory.List = vpcList
		logger.Info().Int("Vpc Count", len(vpcList.Vpcs)).Msg("Successfully completed activity")
		return inventory, nil
	}

	err = errors.New("invalid response received from Site Controller")
	logger.Error().Err(err).Msg("Failed to collect VPC list, invalid response")
	return nil, err
}
