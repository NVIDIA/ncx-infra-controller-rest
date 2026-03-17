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

package infinibandpartition

import (
	"context"
	"errors"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.temporal.io/sdk/temporal"
)

// CreateInfiniBandPartitionActivity - Create InfiniBandPartition Activity
func (ibpw *Workflows) CreateInfiniBandPartitionActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.CreateInfiniBandPartitionRequest) (*wflows.InfiniBandPartitionInfo, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().Str("Activity", "CreateInfiniBandPartitionActivity").Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting activity")

	var infiniBandPartitionRequest *wflows.InfiniBandPartitionInfo
	var err error

	// Initialize response
	infiniBandPartitionRequest = &wflows.InfiniBandPartitionInfo{
		IbPartition: &wflows.IBPartition{},
	}

	wflowMetadata := &ibpWorkflowMetadata{
		activity: activityCreate,
		response: &wflows.InfiniBandPartitionInfo{
			IbPartition: infiniBandPartitionRequest.IbPartition,
		},
	}

	// Validate the ResourceReq
	if ResourceReq == nil {
		// Return error here
		// Bail out earlier
		err = errors.New("invalid or empty request provided as activity argument")
		wflowMetadata.response.StatusMsg = err.Error()
		wflowMetadata.response.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		logger.Error().Err(err).Msg("Failed to create InfiniBand Partition, invalid request")
		return nil, temporal.NewApplicationError(err.Error(), "", &wflowMetadata.response)
	}
	logger.Info().Msg("Validated InfiniBand Partition request, proceeding with creation")

	ibpResponse, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, ResourceReq, wflowMetadata)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create InfiniBand Partition on site via orchestrator")
		return nil, err
	}

	ResourceResp := wflowMetadata.response
	if resp, ok := ibpResponse.(*wflows.IBPartition); ok {
		ResourceResp.IbPartition = resp
		logger.Info().Msg("Successfully completed activity")
	} else {
		logger.Warn().Msg("Unexpected response type for InfiniBandPartition creation")
	}

	return ResourceResp, nil
}

// DeleteInfiniBandPartitionActivity deletes the InfiniBandPartition at carbide
func (ibpw *Workflows) DeleteInfiniBandPartitionActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq *wflows.DeleteInfiniBandPartitionRequest) (*wflows.InfiniBandPartitionInfo, error) {
	// Initialize logger
	logger := ManagerAccess.Data.EB.Log.With().
		Str("Activity", "DeleteInfiniBandPartitionActivity").
		Str("ResourceID", ResourceID).Logger()
	logger.Info().Msg("Starting activity")

	// Initialize response
	infiniBandPartitionRequest := &wflows.InfiniBandPartitionInfo{
		IbPartition: &wflows.IBPartition{},
	}

	wflowMetadata := &ibpWorkflowMetadata{
		activity: activityDelete,
		response: &wflows.InfiniBandPartitionInfo{
			IbPartition: infiniBandPartitionRequest.IbPartition,
		},
	}

	// Validate the ResourceReq
	if ResourceReq == nil {
		err := errors.New("invalid or empty request provided as argument")
		logger.Error().Err(err).Msg("Failed to delete InfiniBand Partition, invalid request")
		wflowMetadata.response.StatusMsg = err.Error()
		wflowMetadata.response.Status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		return nil, temporal.NewApplicationError(err.Error(), "", &wflowMetadata.response)
	}
	logger.Info().Msg("Validated InfiniBand delete partition request, proceeding with deletion")

	_, err := ManagerAccess.API.Orchestrator.DoActivity(ctx, ResourceVer, ResourceID, ResourceReq, wflowMetadata)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to delete Infinity Band Partition on Site via Orchestrator")
		return nil, err
	}

	logger.Info().Msg("Successfully completed activity")
	return wflowMetadata.response, nil
}
