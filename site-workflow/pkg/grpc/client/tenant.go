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

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

var (
	ErrInvalidTenantRequest        = errors.New("gRPC-lib: Tenant - invalid request")
	ErrInvalidTenantOrganizationID = errors.New("gRPC-lib: Tenant - invalid Organization ID")
	ErrInvalidTenantName           = errors.New("gRPC-lib: Tenant - invalid name")
)

type TenantInterface interface {
	// Tenant Interfaces
	CreateTenant(ctx context.Context, request *wflows.CreateTenantRequest) (response *wflows.CreateTenantResponse, err error)
	UpdateTenant(ctx context.Context, request *wflows.UpdateTenantRequest) (response *wflows.UpdateTenantResponse, err error)

	FindTenantOrganizationIDs(ctx context.Context, request *wflows.TenantSearchFilter) (response *wflows.TenantOrganizationIdList, err error)
	FindTenantsByOrganizationIDs(ctx context.Context, request *wflows.TenantByOrganizationIdsRequest) (response *wflows.TenantList, err error)
}

func (tenant *compute) CreateTenant(ctx context.Context, request *wflows.CreateTenantRequest) (response *wflows.CreateTenantResponse, err error) {
	log.Info().Interface("request", request).Msg("CreateTenant: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-CreateTenant")
	defer span.End()

	if request == nil {
		log.Err(ErrInvalidMachineID).Msg("CreateTenant: invalid request")
		return response, ErrInvalidTenantRequest
	}

	if request.OrganizationId == "" {
		log.Err(ErrInvalidMachineID).Msg("CreateTenant: invalid Organization ID")
		return response, ErrInvalidTenantOrganizationID
	}

	if request.Metadata != nil && request.Metadata.Name == "" {
		log.Err(ErrInvalidMachineID).Msg("CreateTenant: invalid Name")
		return response, ErrInvalidTenantName
	}

	response, err = tenant.carbide.CreateTenant(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("CreateTenant: error")
		return nil, err
	}
	return
}

func (tenant *compute) UpdateTenant(ctx context.Context, request *wflows.UpdateTenantRequest) (response *wflows.UpdateTenantResponse, err error) {
	log.Info().Interface("request", request).Msg("UpdateTenant: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-UpdateTenant")
	defer span.End()

	if request == nil {
		log.Err(ErrInvalidMachineID).Msg("CreateTenant: invalid request")
		return response, ErrInvalidTenantRequest
	}

	if request.OrganizationId == "" {
		log.Err(ErrInvalidMachineID).Msg("CreateTenant: invalid Organization ID")
		return response, ErrInvalidTenantOrganizationID
	}

	if request.Metadata != nil && request.Metadata.Name == "" {
		log.Err(ErrInvalidMachineID).Msg("CreateTenant: invalid Name")
		return response, ErrInvalidTenantName
	}

	response, err = tenant.carbide.UpdateTenant(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("UpdateTenant: error")
		return nil, err
	}
	return
}

func (tenant *compute) FindTenantOrganizationIDs(ctx context.Context, request *wflows.TenantSearchFilter) (response *wflows.TenantOrganizationIdList, err error) {
	log.Info().Interface("request", request).Msg("FindTenantOrganizationIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-FindTenantOrganizationIDs")
	defer span.End()

	if request == nil {
		request = &wflows.TenantSearchFilter{}
	}
	response, err = tenant.carbide.FindTenantOrganizationIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindTenantOrganizationIds: error")
		return nil, err
	}
	return
}

func (tenant *compute) FindTenantsByOrganizationIDs(ctx context.Context, request *wflows.TenantByOrganizationIdsRequest) (response *wflows.TenantList, err error) {
	log.Info().Interface("request", request).Msg("FindTenantsByOrganizationIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-FindTenantsByOrganizationIDs")
	defer span.End()

	if request == nil {
		request = &wflows.TenantByOrganizationIdsRequest{}
	}
	response, err = tenant.carbide.FindTenantsByOrganizationIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msgf("FindTenantsByOrganizationIds: error")
		return nil, err
	}

	return
}
