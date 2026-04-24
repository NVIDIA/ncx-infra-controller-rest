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

package activity

import (
	"context"
	"errors"
	"time"

	swe "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/error"
	"github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/grpc/client"
	cClient "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/grpc/client"
	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ManageVpcPrefix is an activity wrapper for VpcPrefix management
type ManageVpcPrefix struct {
	NicoAtomicClient *client.NicoAtomicClient
}

// NewManageVpcPrefix returns a new ManageVpcPrefix client
func NewManageVpcPrefix(nicoClient *client.NicoAtomicClient) ManageVpcPrefix {
	return ManageVpcPrefix{
		NicoAtomicClient: nicoClient,
	}
}

// Function to create VpcPrefix with Nico
func (mvp *ManageVpcPrefix) CreateVpcPrefixOnSite(ctx context.Context, request *cwssaws.VpcPrefixCreationRequest) error {
	logger := log.With().Str("Activity", "CreateVpcPrefixOnSite").Logger()

	logger.Info().Msg("Starting activity")

	var err error

	// Validate request
	if request == nil {
		err = errors.New("received empty create VPC prefix request")
	} else if request.Name == "" {
		err = errors.New("received create VPC prefix request missing Name")
	} else if request.Prefix == "" {
		err = errors.New("received create VPC prefix request missing Prefix")
	} else if request.Id == nil || request.Id.Value == "" {
		// Don't let a request come in without a cloud-provided ID
		// or nico will generate one and cloud won't know the relationship.
		err = errors.New("received create VPC prefix request missing ID")
	} else if request.VpcId == nil || request.VpcId.Value == "" {
		err = errors.New("received create VPC prefix request missing VPC ID")
	}

	if err != nil {
		return temporal.NewNonRetryableApplicationError(err.Error(), swe.ErrTypeInvalidRequest, err)
	}

	// Call Site Controller gRPC endpoint
	nicoClient := mvp.NicoAtomicClient.GetClient()
	if nicoClient == nil {
		return client.ErrClientNotConnected
	}
	nicoClient := nicoClient.Nico()

	_, err = nicoClient.CreateVpcPrefix(ctx, request)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create VPC prefix using Site Controller API")
		return swe.WrapErr(err)
	}

	logger.Info().Msg("Completed activity")

	return nil
}

// Function to update VpcPrefix with Nico
func (mvp *ManageVpcPrefix) UpdateVpcPrefixOnSite(ctx context.Context, request *cwssaws.VpcPrefixUpdateRequest) error {
	logger := log.With().Str("Activity", "UpdateVpcPrefixOnSite").Logger()

	logger.Info().Msg("Starting activity")

	var err error

	// Validate request
	if request == nil {
		err = errors.New("received empty update VPC prefix request")
	} else if request.Id == nil || request.Id.Value == "" {
		err = errors.New("received update VPC prefix request missing ID")
	}

	if err != nil {
		return temporal.NewNonRetryableApplicationError(err.Error(), swe.ErrTypeInvalidRequest, err)
	}

	// Call Site Controller gRPC endpoint
	nicoClient := mvp.NicoAtomicClient.GetClient()
	if nicoClient == nil {
		return client.ErrClientNotConnected
	}
	nicoClient := nicoClient.Nico()

	_, err = nicoClient.UpdateVpcPrefix(ctx, request)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to update VPC prefix using Site Controller API")
		return swe.WrapErr(err)
	}

	logger.Info().Msg("Completed activity")

	return nil
}

// Function to delete VpcPrefix on Nico
func (mvp *ManageVpcPrefix) DeleteVpcPrefixOnSite(ctx context.Context, request *cwssaws.VpcPrefixDeletionRequest) error {
	logger := log.With().Str("Activity", "DeleteVpcPrefixOnSite").Logger()

	logger.Info().Msg("Starting activity")

	var err error

	// Validate request
	if request == nil {
		err = errors.New("received empty delete VPC prefix request")
	} else if request.Id == nil || request.Id.Value == "" {
		err = errors.New("reveived delete VPC prefix missing VPC Prefix ID")
	}

	if err != nil {
		return temporal.NewNonRetryableApplicationError(err.Error(), swe.ErrTypeInvalidRequest, err)
	}

	// Call Site Controller gRPC endpoint
	nicoClient := mvp.NicoAtomicClient.GetClient()
	if nicoClient == nil {
		return client.ErrClientNotConnected
	}
	nicoClient := nicoClient.Nico()

	_, err = nicoClient.DeleteVpcPrefix(ctx, request)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to delete VPC prefix using Site Controller API")
		return swe.WrapErr(err)
	}

	logger.Info().Msg("Completed activity")

	return nil
}

// ManageVpcPrefixInventory is an activity wrapper for VpcPrefix inventory collection and publishing
type ManageVpcPrefixInventory struct {
	config ManageInventoryConfig
}

// NewManageVpcPrefixInventory returns a ManageInventory implementation for VpcPrefix
func NewManageVpcPrefixInventory(config ManageInventoryConfig) ManageVpcPrefixInventory {
	return ManageVpcPrefixInventory{
		config: config,
	}
}

// DiscoverVpcPrefixInventory is an activity to collect VpcPrefix inventory and publish to Temporal queue
func (moii *ManageVpcPrefixInventory) DiscoverVpcPrefixInventory(ctx context.Context) error {
	logger := log.With().Str("Activity", "DiscoverVpcPrefixInventory").Logger()
	logger.Info().Msg("Starting activity")

	inventoryImpl := manageInventoryImpl[*cwssaws.VpcPrefixId, *cwssaws.VpcPrefix, *cwssaws.VpcPrefixInventory]{
		itemType:               "VpcPrefix",
		config:                 moii.config,
		internalFindIDs:        VpcPrefixFindIDs,
		internalFindByIDs:      VpcPrefixFindByIDs,
		internalPagedInventory: VpcPrefixPagedInventory,
		internalFindFallback:   VpcPrefixFindFallback,
	}
	return inventoryImpl.CollectAndPublishInventory(ctx, &logger)
}

func VpcPrefixFindIDs(ctx context.Context, nicoClient *cClient.NicoClient) ([]*cwssaws.VpcPrefixId, error) {
	idList, err := nicoClient.Nico().SearchVpcPrefixes(ctx, &cwssaws.VpcPrefixSearchQuery{})
	if err != nil {
		return nil, err
	}
	return idList.VpcPrefixIds, nil
}

func VpcPrefixFindByIDs(ctx context.Context, nicoClient *cClient.NicoClient, ids []*cwssaws.VpcPrefixId) ([]*cwssaws.VpcPrefix, error) {
	list, err := nicoClient.Nico().GetVpcPrefixes(ctx, &cwssaws.VpcPrefixGetRequest{
		VpcPrefixIds: ids,
	})

	if err != nil {
		return nil, err
	}
	return list.GetVpcPrefixes(), nil
}

func VpcPrefixPagedInventory(allItemIDs []*cwssaws.VpcPrefixId, pagedItems []*cwssaws.VpcPrefix, input *pagedInventoryInput) *cwssaws.VpcPrefixInventory {
	itemIDs := []string{}
	for _, id := range allItemIDs {
		itemIDs = append(itemIDs, id.GetValue())
	}

	// Create an inventory page with the subset of VpcPrefixs
	inventory := &cwssaws.VpcPrefixInventory{
		VpcPrefixes: pagedItems,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
		InventoryStatus: input.status,
		StatusMsg:       input.statusMessage,
		InventoryPage:   input.buildPage(),
	}
	if inventory.InventoryPage != nil {
		inventory.InventoryPage.ItemIds = itemIDs
	}
	return inventory
}

func VpcPrefixFindFallback(ctx context.Context, nicoClient *cClient.NicoClient) ([]*cwssaws.VpcPrefixId, []*cwssaws.VpcPrefix, error) {
	request := &cwssaws.VpcPrefixGetRequest{}
	items, err := nicoClient.Nico().GetVpcPrefixes(ctx, request)
	if err != nil {
		return nil, nil, err
	}

	var ids []*cwssaws.VpcPrefixId
	for _, it := range items.GetVpcPrefixes() {
		ids = append(ids, it.GetId())
	}
	return ids, items.GetVpcPrefixes(), nil
}
