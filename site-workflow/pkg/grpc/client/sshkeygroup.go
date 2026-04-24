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

// SSHKeyGroupInterface is the interface for the SSHKeyGroup client
type SSHKeyGroupInterface interface {
	CreateSSHKeyGroup(ctx context.Context, request *wflows.CreateSSHKeyGroupRequest) (response *wflows.CreateTenantKeysetResponse, err error)
	UpdateSSHKeyGroup(ctx context.Context, request *wflows.UpdateSSHKeyGroupRequest) (response *wflows.UpdateTenantKeysetResponse, err error)
	DeleteSSHKeyGroup(ctx context.Context, request *wflows.DeleteSSHKeyGroupRequest) (response *wflows.DeleteTenantKeysetResponse, err error)
	GetAllSSHKeyGroups(ctx context.Context, request *wflows.TenantKeysetSearchFilter, pageSize int) (response *wflows.TenantKeySetList, err error)
	FindSSHKeyGroupIDs(ctx context.Context, request *wflows.TenantKeysetSearchFilter) (response *wflows.TenantKeysetIdList, err error)
	FindSSHKeyGroupsByIDs(ctx context.Context, request *wflows.TenantKeysetsByIdsRequest) (response *wflows.TenantKeySetList, err error)
}

// CreateSSHKeyGroup creates a SSHKeyGroup
func (skg *compute) CreateSSHKeyGroup(ctx context.Context, request *wflows.CreateSSHKeyGroupRequest) (response *wflows.CreateTenantKeysetResponse, err error) {
	log.Info().Interface("request", request).Msg("CreateSSHKeyGroup: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-CreateSSHKeyGroup")
	defer span.End()

	// Validate the request
	if request == nil {
		err = errors.New("CreateSSHKeyGroup: invalid request")
		log.Error().Err(err).Msg("CreateSSHKeyGroup: invalid request")
		return nil, err
	}

	// Translate the workflow request to the nico request
	nicoRequest := &wflows.CreateTenantKeysetRequest{
		KeysetIdentifier: &wflows.TenantKeysetIdentifier{},
		KeysetContent:    &wflows.TenantKeysetContent{},
	}
	// Assign the values to nico request
	nicoRequest.Version = request.Version
	if request.PublicKeys != nil {
		nicoRequest.KeysetContent.PublicKeys = request.PublicKeys
	}
	nicoRequest.KeysetIdentifier.KeysetId = request.KeysetId
	nicoRequest.KeysetIdentifier.OrganizationId = request.TenantOrganizationId

	response, err = skg.nico.CreateTenantKeyset(ctx, nicoRequest)
	return response, err
}

func (skg *compute) GetAllSSHKeyGroups(ctx context.Context, request *wflows.TenantKeysetSearchFilter, pageSize int) (response *wflows.TenantKeySetList, err error) {
	log.Info().Interface("request", request).Msg("GetAllSSHKeyGroups: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-GetAllSSHKeyGroups")
	defer span.End()

	if request == nil {
		request = &wflows.TenantKeysetSearchFilter{}
	}

	idList, err := skg.nico.FindTenantKeysetIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindTenantKeysetIds: error")
		return nil, err
	}
	response = &wflows.TenantKeySetList{}
	idChunks := SliceToChunks(idList.KeysetIds, pageSize)
	for i, chunk := range idChunks {
		list, err := skg.nico.FindTenantKeysetsByIds(ctx, &wflows.TenantKeysetsByIdsRequest{KeysetIds: chunk})
		if err != nil {
			log.Error().Err(err).Msgf("FindTenantKeysetsByIds: error on chunk index %d", i)
			return nil, err
		}
		response.Keyset = append(response.Keyset, list.Keyset...)
	}
	log.Info().Int("SSHKeyGroupsListLen", len(idList.KeysetIds)).Msg("GetSSHKeyGroups: received result")
	return response, err
}

func (skg *compute) FindSSHKeyGroupIDs(ctx context.Context, request *wflows.TenantKeysetSearchFilter) (response *wflows.TenantKeysetIdList, err error) {
	log.Info().Interface("request", request).Msg("FindSSHKeyGroupIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-FindSSHKeyGroupIDs")
	defer span.End()

	if request == nil {
		request = &wflows.TenantKeysetSearchFilter{}
	}

	response, err = skg.nico.FindTenantKeysetIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindTenantKeysetIds: error")
		return nil, err
	}
	return
}

func (skg *compute) FindSSHKeyGroupsByIDs(ctx context.Context, request *wflows.TenantKeysetsByIdsRequest) (response *wflows.TenantKeySetList, err error) {
	log.Info().Interface("request", request).Msg("FindSSHKeyGroupsByIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-FindSSHKeyGroupsByIDs")
	defer span.End()

	if request == nil {
		request = &wflows.TenantKeysetsByIdsRequest{}
	}

	response, err = skg.nico.FindTenantKeysetsByIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msgf("FindTenantKeysetsByIds: error")
		return nil, err
	}
	return
}

// UpdateSSHKeyGroup updates a SSHKeyGroup
func (skg *compute) UpdateSSHKeyGroup(ctx context.Context, request *wflows.UpdateSSHKeyGroupRequest) (response *wflows.UpdateTenantKeysetResponse, err error) {
	log.Info().Interface("request", request).Msg("UpdateSSHKeyGroup: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-UpdateSSHKeyGroup")
	defer span.End()

	// Validate the request
	if request == nil {
		err = errors.New("UpdateSSHKeyGroup: invalid request")
		log.Error().Err(err).Msg("UpdateSSHKeyGroup: invalid request")
		return nil, err
	}

	// Check that the request contains KeysetId
	if request.KeysetId == "" {
		err = errors.New("UpdateSSHKeyGroup: invalid request (no KeysetId)")
		log.Error().Err(err).Msg("UpdateSSHKeyGroup: invalid request - no KeysetId specified")
		return nil, err
	}

	// Check that the request contains Version
	if request.Version == "" {
		err = errors.New("UpdateSSHKeyGroup: invalid request (no Version)")
		log.Error().Err(err).Msg("UpdateSSHKeyGroup: invalid request - no Version value specified")
		return nil, err
	}

	// Check that the request contains TenantOrganizationId
	if request.TenantOrganizationId == "" {
		err = errors.New("UpdateSSHKeyGroup: invalid request (no TenantOrganizationId)")
		log.Error().Err(err).Msg("UpdateSSHKeyGroup: invalid request - no TenantOrganizationId value specified")
		return nil, err
	}

	// Translate the workflow request to the nico request
	nicoRequest := &wflows.UpdateTenantKeysetRequest{}
	nicoRequest.KeysetIdentifier = &wflows.TenantKeysetIdentifier{
		OrganizationId: request.TenantOrganizationId,
		KeysetId:       request.KeysetId,
	}
	nicoRequest.Version = request.Version
	nicoRequest.KeysetContent = &wflows.TenantKeysetContent{
		PublicKeys: request.PublicKeys,
	}
	nicoRequest.IfVersionMatch = request.IfVersionMatch

	response, err = skg.nico.UpdateTenantKeyset(ctx, nicoRequest)
	return response, err
}

// DeleteSSHKeyGroup deletes a SSHKeyGroup
func (skg *compute) DeleteSSHKeyGroup(ctx context.Context, request *wflows.DeleteSSHKeyGroupRequest) (response *wflows.DeleteTenantKeysetResponse, err error) {
	log.Info().Interface("Request", request).Msg("DeleteSSHKeyGroup: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-DeleteSSHKeyGroup")
	defer span.End()

	// Validate the request
	if request == nil {
		err = errors.New("DeleteSSHKeyGroup: invalid request")
		log.Error().Err(err).Msg("DeleteSSHKeyGroup: invalid request")
	}

	// Validate the request
	if request.KeysetId == "" {
		err = errors.New("DeleteSSHKeyGroup: invalid keysetID")
		log.Error().Err(err).Msg("DeleteSSHKeyGroup: invalid request")
	}

	// Validate the request
	if request.TenantOrganizationId == "" {
		err = errors.New("DeleteSSHKeyGroup: invalid TenantOrganizationId")
		log.Error().Err(err).Msg("DeleteSSHKeyGroup: invalid request")
	}

	// Translate the workflow request to the nico request
	nicoRequest := &wflows.DeleteTenantKeysetRequest{
		KeysetIdentifier: &wflows.TenantKeysetIdentifier{},
	}

	nicoRequest.KeysetIdentifier.KeysetId = request.KeysetId
	nicoRequest.KeysetIdentifier.OrganizationId = request.TenantOrganizationId
	response, err = skg.nico.DeleteTenantKeyset(ctx, nicoRequest)
	return response, err
}
