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

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
)

// VPCInterface is the interface for the VPC client
type VPCInterface interface {
	CreateVPC(ctx context.Context, request *wflows.Vpc) (response *wflows.Vpc, err error)
	// DEPRECATED: use GetAllVPCs instead
	ListVPCs(ctx context.Context) (response *wflows.VpcList, err error)
	DeleteVPC(ctx context.Context, id string) (response *wflows.VpcDeletionResult, err error)
	// DEPRECATED: use GetAllVPCs instead
	GetVPC(ctx context.Context, request *wflows.VpcSearchQuery) (response *wflows.VpcList, err error)
	GetAllVPCs(ctx context.Context, request *wflows.VpcSearchFilter, pageSize int) (response *wflows.VpcList, err error)
	FindVPCIDs(ctx context.Context, request *wflows.VpcSearchFilter) (response *wflows.VpcIdList, err error)
	FindVPCsByIDs(ctx context.Context, request *wflows.VpcsByIdsRequest) (response *wflows.VpcList, err error)
}

// CreateVPC creates a VPC
func (vpc *network) CreateVPC(ctx context.Context, request *wflows.Vpc) (response *wflows.Vpc, err error) {
	log.Info().Interface("request", request).Msg("CreateVPC: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-CreateVPC")
	defer span.End()

	// Validate the request
	if request == nil {
		err = errors.New("CreateVPC: invalid request")
		log.Error().Err(err).Msg("CreateVPC: invalid request")
		return nil, err
	}

	// Translate the workflow request to the carbide request
	carbideRequest := &wflows.VpcCreationRequest{
		Id:                   request.Id,
		Name:                 request.Name,
		TenantOrganizationId: request.TenantOrganizationId,
	}

	response, err = vpc.carbide.CreateVpc(ctx, carbideRequest)
	return response, err
}

// GetVPC gets a VPC
// DEPRECATED: use GetAllVPCs instead
func (vpc *network) GetVPC(ctx context.Context, request *wflows.VpcSearchQuery) (response *wflows.VpcList, err error) {
	log.Info().Interface("request", request).Msg("GetVPC: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-GetVPC")
	defer span.End()

	response, err = vpc.carbide.FindVpcs(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("GetVPC: error")
		return nil, err
	}
	log.Info().Int("VpcListLen", len(response.Vpcs)).Msg("GetVPC: received result")
	return response, err

}

// GetVPC gets a VPC
// DEPRECATED: use GetAllVPCs instead
func (vpc *network) ListVPCs(ctx context.Context) (response *wflows.VpcList, err error) {
	log.Info().Msg("ListVPCs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-ListVPCs")
	defer span.End()

	carbiderequest := &wflows.VpcSearchQuery{
		Id: &wflows.VpcId{},
	}
	response, err = vpc.carbide.FindVpcs(ctx, carbiderequest)
	if err != nil {
		log.Error().Err(err).Msg("ListVPCs: error")
		return nil, err
	}
	log.Info().Int("VpcListLen", len(response.Vpcs)).Msg("ListVPC: received result")
	return response, err
}

func (vpc *network) GetAllVPCs(ctx context.Context, request *wflows.VpcSearchFilter, pageSize int) (response *wflows.VpcList, err error) {
	log.Info().Interface("request", request).Msg("GetAllVPCs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-GetAllVPCs")
	defer span.End()

	if request == nil {
		request = &wflows.VpcSearchFilter{}
	}

	idList, err := vpc.carbide.FindVpcIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindVpcIds: error")
		return nil, err
	}
	response = &wflows.VpcList{}
	idChunks := SliceToChunks(idList.VpcIds, pageSize)
	for i, chunk := range idChunks {
		list, err := vpc.carbide.FindVpcsByIds(ctx, &wflows.VpcsByIdsRequest{VpcIds: chunk})
		if err != nil {
			log.Error().Err(err).Msgf("FindVpcsByIds: error on chunk index %d", i)
			return nil, err
		}
		response.Vpcs = append(response.Vpcs, list.Vpcs...)
	}
	log.Info().Int("VpcListLen", len(idList.VpcIds)).Msg("GetVPCs: received result")
	return response, err
}

func (vpc *network) FindVPCIDs(ctx context.Context, request *wflows.VpcSearchFilter) (response *wflows.VpcIdList, err error) {
	log.Info().Interface("request", request).Msg("FindVPCIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-FindVPCIDs")
	defer span.End()

	if request == nil {
		request = &wflows.VpcSearchFilter{}
	}

	response, err = vpc.carbide.FindVpcIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindVpcIds: error")
		return nil, err
	}
	return
}

func (vpc *network) FindVPCsByIDs(ctx context.Context, request *wflows.VpcsByIdsRequest) (response *wflows.VpcList, err error) {
	log.Info().Interface("request", request).Msg("FindVPCsByIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-FindVPCsByIDs")
	defer span.End()

	if request == nil {
		request = &wflows.VpcsByIdsRequest{}
	}

	response, err = vpc.carbide.FindVpcsByIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msgf("FindVpcsByIds: error")
		return nil, err
	}
	return
}

// DeleteVPC deletes a VPC
func (vpc *network) DeleteVPC(ctx context.Context, id string) (response *wflows.VpcDeletionResult, err error) {
	log.Info().Str("id", id).Msg("DeleteVPC: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-DeleteVPC")
	defer span.End()

	// Validate the request
	if id == "" {
		err = errors.New("DeleteVPC: invalid request")
		log.Error().Err(err).Msg("DeleteVPC: invalid request")
	}

	// Translate the workflow request to the carbide request
	carbideRequest := &wflows.VpcDeletionRequest{}
	carbideRequest.Id = &wflows.VpcId{Value: id}
	carbideRequest.Id.Value = id
	response, err = vpc.carbide.DeleteVpc(ctx, carbideRequest)
	return response, err
}
