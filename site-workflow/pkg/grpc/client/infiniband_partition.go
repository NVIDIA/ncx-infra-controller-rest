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

package client

import (
	"context"
	"errors"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
)

// InfiniBandPartitionInterface is the interface for the InfiniBandPartition client
type InfiniBandPartitionInterface interface {
	CreateInfiniBandPartition(ctx context.Context, request *wflows.CreateInfiniBandPartitionRequest) (response *wflows.IBPartition, err error)
	DeleteInfiniBandPartition(ctx context.Context, request *wflows.DeleteInfiniBandPartitionRequest) (response *wflows.IBPartitionDeletionResult, err error)
	// DEPRECATED: use GetAllInfiniBandPartitions instead
	GetInfiniBandPartition(ctx context.Context, request *wflows.GetInfiniBandPartitionRequest) (response *wflows.IBPartitionList, err error)
	// DEPRECATED: use GetAllInfiniBandPartitions instead
	ListInfiniBandPartition(ctx context.Context) (response *wflows.IBPartitionList, err error)
	GetAllInfiniBandPartitions(ctx context.Context, request *wflows.IBPartitionSearchFilter, pageSize int) (response *wflows.IBPartitionList, err error)
	FindInfinibandPartitionIDs(ctx context.Context, request *wflows.IBPartitionSearchFilter) (response *wflows.IBPartitionIdList, err error)
	FindInfinibandPartitionsByIDs(ctx context.Context, request *wflows.IBPartitionsByIdsRequest) (response *wflows.IBPartitionList, err error)
}

// CreateInfiniBandPartition creates a InfiniBandPartition
func (ibp *network) CreateInfiniBandPartition(ctx context.Context, request *wflows.CreateInfiniBandPartitionRequest) (response *wflows.IBPartition, err error) {
	log.Info().Interface("request", request).Msg("CreateInfiniBandPartition: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-CreateInfiniBandPartition")
	defer span.End()

	// Validate the request
	if request == nil {
		err = errors.New("CreateInfiniBandPartition: invalid request")
		log.Error().Err(err).Msg("CreateInfiniBandPartition: invalid request")
		return nil, err
	}

	// Translate the workflow request to the carbide request
	carbideRequest := &wflows.IBPartitionCreationRequest{
		Id: &wflows.IBPartitionId{Value: request.IbPartitionId.Value},
		Config: &wflows.IBPartitionConfig{
			Name:                 request.Name,
			TenantOrganizationId: request.TenantOrganizationId,
		},
	}

	response, err = ibp.carbide.CreateIBPartition(ctx, carbideRequest)
	return response, err
}

// DeleteInfiniBandPartition deletes a InfiniBandPartition
func (ibp *network) DeleteInfiniBandPartition(ctx context.Context, request *wflows.DeleteInfiniBandPartitionRequest) (response *wflows.IBPartitionDeletionResult, err error) {
	log.Info().Interface("request", request).Msg("DeleteInfiniBandPartition: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-DeleteInfiniBandPartition")
	defer span.End()

	// Validate the request
	if request == nil {
		err = errors.New("DeleteInfiniBandPartition: invalid request")
		log.Error().Err(err).Msg("DeleteInfiniBandPartition: invalid request")
		return nil, err
	}

	// Translate the workflow request to the carbide request
	carbideRequest := &wflows.IBPartitionDeletionRequest{
		Id: &wflows.IBPartitionId{Value: request.Id.Value},
	}
	response, err = ibp.carbide.DeleteIBPartition(ctx, carbideRequest)
	return response, err
}

// GetInfiniBandPartition gets a InfiniBandPartition
// DEPRECATED: use GetAllInfiniBandPartitions instead
func (ibp *network) GetInfiniBandPartition(ctx context.Context, request *wflows.GetInfiniBandPartitionRequest) (response *wflows.IBPartitionList, err error) {
	log.Info().Interface("request", request).Msg("GetInfiniBandPartition: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-GetInfiniBandPartition")
	defer span.End()

	// Validate the request
	if request == nil || request.Id == nil {
		if request == nil {
			err = errors.New("GetInfiniBandPartition: invalid request")
		} else if request.Id == nil {
			err = errors.New("GetInfiniBandPartition: invalid id")
		}
		log.Error().Err(err).Msg("GetInfiniBandPartition: invalid request")
		return nil, err
	}

	carbideRequest := &wflows.IBPartitionQuery{
		Id: &wflows.IBPartitionId{Value: request.Id.Value},
	}
	log.Info().Interface("request", carbideRequest).Msg("GetInfiniBandPartition: converted FindIBPartitions request")

	response, err = ibp.carbide.FindIBPartitions(ctx, carbideRequest)
	if err != nil {
		log.Error().Err(err).Msg("GetInfiniBandPartition: error")
		return nil, err
	}
	log.Info().Int("IbPartitionsLen", len(response.IbPartitions)).Msg("GetInfiniBandPartition: received result")
	return response, err
}

// ListInfiniBandPartition returns list of InfiniBandPartition
// DEPRECATED: use GetAllInfiniBandPartitions instead
func (ibp *network) ListInfiniBandPartition(ctx context.Context) (response *wflows.IBPartitionList, err error) {
	log.Info().Msg("ListInfiniBandPartition: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-ListInfiniBandPartition")
	defer span.End()

	carbiderequest := &wflows.IBPartitionQuery{}
	response, err = ibp.carbide.FindIBPartitions(ctx, carbiderequest)
	if err != nil {
		log.Error().Err(err).Msg("ListInfiniBandPartition: error")
		return nil, err
	}
	log.Info().Int("IbPartitionsLen", len(response.IbPartitions)).Msg("ListInfiniBandPartition: received result")
	return response, err
}

func (ibp *network) GetAllInfiniBandPartitions(ctx context.Context, request *wflows.IBPartitionSearchFilter, pageSize int) (response *wflows.IBPartitionList, err error) {
	log.Info().Interface("request", request).Msg("GetAllInfiniBandPartitions: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-GetAllInfiniBandPartitions")
	defer span.End()

	if request == nil {
		request = &wflows.IBPartitionSearchFilter{}
	}

	idList, err := ibp.carbide.FindIBPartitionIds(ctx, request)
	if err != nil {
		if grpcStatus, ok := status.FromError(err); ok {
			if grpcStatus.Code() == codes.Unimplemented {
				log.Info().Msg("Using deprecated API to get IBPartitions")
				return ibp.ListInfiniBandPartition(ctx)
			}
		}
		log.Error().Err(err).Msg("FindIBPartitionIds: error")
		return nil, err
	}
	response = &wflows.IBPartitionList{}
	idChunks := SliceToChunks(idList.IbPartitionIds, pageSize)
	for i, chunk := range idChunks {
		list, err := ibp.carbide.FindIBPartitionsByIds(ctx, &wflows.IBPartitionsByIdsRequest{IbPartitionIds: chunk})
		if err != nil {
			log.Error().Err(err).Msgf("FindIBPartitionsByIds: error on chunk index %d", i)
			return nil, err
		}
		response.IbPartitions = append(response.IbPartitions, list.IbPartitions...)
	}
	log.Info().Int("IBPartitionListLen", len(idList.IbPartitionIds)).Msg("GetInfiniBandPartitions: received result")
	return response, err
}

func (ibp *network) FindInfinibandPartitionIDs(ctx context.Context, request *wflows.IBPartitionSearchFilter) (response *wflows.IBPartitionIdList, err error) {
	log.Info().Interface("request", request).Msg("FindInfinibandPartitionIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-FindInfinibandPartitionIDs")
	defer span.End()

	if request == nil {
		request = &wflows.IBPartitionSearchFilter{}
	}

	response, err = ibp.carbide.FindIBPartitionIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindIBPartitionIds: error")
		return nil, err
	}
	return
}

func (ibp *network) FindInfinibandPartitionsByIDs(ctx context.Context, request *wflows.IBPartitionsByIdsRequest) (response *wflows.IBPartitionList, err error) {
	log.Info().Interface("request", request).Msg("FindInfinibandPartitionsByIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-FindInfinibandPartitionsByIDs")
	defer span.End()

	if request == nil {
		request = &wflows.IBPartitionsByIdsRequest{}
	}

	response, err = ibp.carbide.FindIBPartitionsByIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindIBPartitionsByIds: error")
		return nil, err
	}
	return
}
