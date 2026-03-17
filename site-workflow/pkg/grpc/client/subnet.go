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
	"fmt"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"net"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.opentelemetry.io/otel"

	"github.com/rs/zerolog/log"
)

var (
	// ErrInvalidName subnet - invalid name
	ErrInvalidName = errors.New("gRPC-lib: subnet - invalid name")
	// ErrInvalidID subnet - invalid id"
	ErrInvalidID = errors.New("gRPC-lib: subnet - invalid id")
	// ErrInvalidVPCId subnet - invalid vpc
	ErrInvalidVPCId = errors.New("gRPC-lib: subnet - invalid vpc")
	// ErrEmptyPrefixes subnet - empty prefix list
	ErrEmptyPrefixes = errors.New("gRPC-lib: subnet - empty prefix list")
	// ErrEmptyPrefix  subnet - empty prefix
	ErrEmptyPrefix = errors.New("gRPC-lib: subnet - empty prefix ")
	// ErrInvalidCIDR subnet - invalid CIDR
	ErrInvalidCIDR = errors.New("gRPC-lib: subnet - invalid CIDR ")
)

// SubnetInterface Subnet Interface
type SubnetInterface interface {
	CreateNetworkSegment(ctx context.Context, request *wflows.CreateSubnetRequest) (response *wflows.NetworkSegment, err error)
	// DEPRECATED: use GetAllNetworkSegments or GetNetworkSegment instead
	GetNetworkSegmentDeprecated(ctx context.Context, ID *wflows.UUID) (response *wflows.NetworkSegmentList, err error)
	DeleteNetworkSegment(ctx context.Context, request *wflows.DeleteSubnetRequest) (response *wflows.NetworkSegmentDeletionResult, err error)
	GetAllNetworkSegments(ctx context.Context, request *wflows.NetworkSegmentSearchFilter, pageSize int) (response *wflows.NetworkSegmentList, err error)
	GetNetworkSegment(ctx context.Context, request *wflows.UUID) (response *wflows.NetworkSegment, err error)
	FindNetworkSegmentIds(ctx context.Context, request *wflows.NetworkSegmentSearchFilter) (response *wflows.NetworkSegmentIdList, err error)
	FindNetworkSegmentsByIds(ctx context.Context, request *wflows.NetworkSegmentsByIdsRequest) (response *wflows.NetworkSegmentList, err error)

	ValidatePrefixes(prefixes []*wflows.NetworkPrefixInfo) (err error)
	ValidatePrefix(prefix *wflows.NetworkPrefixInfo) (err error)
}

func (sub *network) CreateNetworkSegment(ctx context.Context, request *wflows.CreateSubnetRequest) (response *wflows.NetworkSegment, err error) {
	log.Info().Interface("request", request).Msg("CreateNetworkSegment: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-CreateNetworkSegment")
	defer span.End()

	// Validations
	if request.Name == "" {
		// Name is mandatory
		log.Err(ErrInvalidName).Msg("CreateNetworkSegment: invalid request")
		return response, ErrInvalidName
	}

	if request.VpcId == nil {
		// Name is mandatory
		log.Err(ErrInvalidVPCId).Msg("CreateNetworkSegment: invalid request")
		return response, ErrInvalidVPCId
	}

	// Validate network prefix
	err = sub.ValidatePrefixes(request.NetworkPrefixes)
	if err != nil {
		log.Err(err).Msg("CreateNetworkSegment: invalid prefix")
		return response, err
	}

	// Carbide request
	carbideRequest := &wflows.NetworkSegmentCreationRequest{
		Name: request.Name,
	}
	if request.SubnetId != nil {
		carbideRequest.Id = &wflows.NetworkSegmentId{Value: request.SubnetId.Value}
	}
	carbideRequest.SubdomainId = nil
	if request.SubdomainId != nil {
		carbideRequest.SubdomainId = &wflows.DomainId{Value: request.SubdomainId.Value}
	}
	if request.VpcId != nil {
		carbideRequest.VpcId = &wflows.VpcId{Value: request.VpcId.Value}
	}
	if request.Mtu != nil {
		carbideRequest.Mtu = request.Mtu
	}

	// Lets verify the applicable parameters
	// We can do policies on these later
	// For now - just do stateless transitions
	for index, prefix := range request.NetworkPrefixes {
		carbideRequest.Prefixes = append(carbideRequest.Prefixes, &wflows.NetworkPrefix{})
		carbideRequest.Prefixes[index].Gateway = prefix.Gateway
		carbideRequest.Prefixes[index].Prefix = prefix.Prefix
		carbideRequest.Prefixes[index].ReserveFirst = prefix.ReserveFirst
		// We need to do additional checks on the resource state later
	}
	response, err = sub.carbide.CreateNetworkSegment(ctx, carbideRequest)
	log.Info().Interface("request", carbideRequest).Msg("CreateNetworkSegment: sent gRPC request")
	return response, err
}

// DEPRECATED: use GetAllNetworkSegments instead
func (sub *network) GetNetworkSegmentDeprecated(ctx context.Context, id *wflows.UUID) (response *wflows.NetworkSegmentList, err error) {
	log.Info().Interface("request", id).Msg("GetNetworkSegment: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-GetNetworkSegment")
	defer span.End()

	// Carbide request
	carbideRequest := &wflows.NetworkSegmentQuery{}
	if id != nil {
		carbideRequest.Id = &wflows.NetworkSegmentId{Value: id.Value}
	}
	response, err = sub.carbide.FindNetworkSegments(ctx, carbideRequest)
	log.Info().Int("NetworkSegmentLen", len(response.NetworkSegments)).Msg("FindNetworkSegment: received result")
	return response, err
}

func (sub *network) GetAllNetworkSegments(ctx context.Context, request *wflows.NetworkSegmentSearchFilter, pageSize int) (response *wflows.NetworkSegmentList, err error) {
	log.Info().Interface("request", request).Msg("GetAllNetworkSegments: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-GetAllNetworkSegments")
	defer span.End()

	if request == nil {
		request = &wflows.NetworkSegmentSearchFilter{}
	}

	idList, err := sub.carbide.FindNetworkSegmentIds(ctx, request)
	if err != nil {
		if grpcStatus, ok := status.FromError(err); ok {
			if grpcStatus.Code() == codes.Unimplemented {
				log.Info().Msg("Using deprecated API to get NetworkSegments")
				return sub.GetNetworkSegmentDeprecated(ctx, nil)
			}
		}
		log.Error().Err(err).Msg("FindNetworkSegmentIds: error")
		return nil, err
	}
	response = &wflows.NetworkSegmentList{}
	idChunks := SliceToChunks(idList.NetworkSegmentsIds, pageSize)
	for i, chunk := range idChunks {
		list, err := sub.carbide.FindNetworkSegmentsByIds(ctx, &wflows.NetworkSegmentsByIdsRequest{NetworkSegmentsIds: chunk})
		if err != nil {
			log.Error().Err(err).Msgf("FindNetworkSegmentsByIds: error on chunk index %d", i)
			return nil, err
		}
		response.NetworkSegments = append(response.NetworkSegments, list.NetworkSegments...)
	}
	log.Info().Int("NetworkSegmentsListLen", len(idList.NetworkSegmentsIds)).Msg("GetNetworkSegments: received result")
	return response, err
}

func (sub *network) GetNetworkSegment(ctx context.Context, request *wflows.UUID) (response *wflows.NetworkSegment, err error) {
	log.Info().Interface("request", request).Msg("GetAllNetworkSegments: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-GetAllNetworkSegments")
	defer span.End()

	if request == nil {
		return nil, fmt.Errorf("NetworkSegment with ID %s not found", request)
	}

	networkSegmentId := &wflows.NetworkSegmentId{Value: request.Value}
	list, err := sub.carbide.FindNetworkSegmentsByIds(ctx, &wflows.NetworkSegmentsByIdsRequest{NetworkSegmentsIds: []*wflows.NetworkSegmentId{networkSegmentId}})
	if err != nil {
		if grpcStatus, ok := status.FromError(err); ok {
			if grpcStatus.Code() == codes.Unimplemented {
				log.Info().Msg("Using deprecated API to get NetworkSegment")
				list, err = sub.GetNetworkSegmentDeprecated(ctx, request)
				if err != nil {
					return nil, err
				}
			}
		} else {
			log.Error().Err(err).Msgf("FindNetworkSegmentsByIds: error")
			return nil, err
		}
	}
	segments := list.GetNetworkSegments()
	if len(segments) == 1 {
		return segments[0], nil
	}
	return nil, fmt.Errorf("NetworkSegment with ID %s not found", request)
}

func (sub *network) FindNetworkSegmentIds(ctx context.Context, request *wflows.NetworkSegmentSearchFilter) (response *wflows.NetworkSegmentIdList, err error) {
	log.Info().Interface("request", request).Msg("FindNetworkSegmentIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-FindNetworkSegmentIDs")
	defer span.End()

	if request == nil {
		request = &wflows.NetworkSegmentSearchFilter{}
	}

	response, err = sub.carbide.FindNetworkSegmentIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindNetworkSegmentIds: error")
		return nil, err
	}
	return
}

func (sub *network) FindNetworkSegmentsByIds(ctx context.Context, request *wflows.NetworkSegmentsByIdsRequest) (response *wflows.NetworkSegmentList, err error) {
	log.Info().Interface("request", request).Msg("FindNetworkSegmentsByIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-FindNetworkSegmentsByIDs")
	defer span.End()

	if request == nil {
		request = &wflows.NetworkSegmentsByIdsRequest{}
	}

	response, err = sub.carbide.FindNetworkSegmentsByIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msgf("FindNetworkSegmentsByIds: error")
		return nil, err
	}
	return
}

// This function is not currently supported
// func (sub *network) UpdateNetworkSegment(ctx context.Context, TransactionID *wflows.TransactionID, request *wflows.UpdateSubnetRequest) (result *wflows.NetworkSegmentUpdateResult, err error) {
// 	log.Info().Interface("request", request).Msg("UpdateNetworkSegment: received request")
// 	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-UpdateNetworkSegment")
// 	defer span.End()

// 	// Validations
// 	if request.NetworkSegmentId == nil {
// 		// Name is mandatory
// 		log.Err(ErrInvalidID).Msg("UpdateNetworkSegment: invalid request")
// 		return result, ErrInvalidID
// 	}

// 	// Carbide request
// 	carbideRequest := &wflows.NetworkSegmentUpdateRequest{
// 		Id: request.NetworkSegmentId,
// 	}
// 	carbideRequest.Mtu = request.Mtu
// 	carbideRequest.Name = request.Name

// 	result, err = sub.carbide.UpdateNetworkSegment(ctx, carbideRequest)
// 	log.Info().Interface("request", carbideRequest).Msg("UpdateNetworkSegment: sent gRPC request")
// 	return result, err
// }

func (sub *network) DeleteNetworkSegment(ctx context.Context, request *wflows.DeleteSubnetRequest) (response *wflows.NetworkSegmentDeletionResult, err error) {
	log.Info().Interface("request", request).Msg("DeleteNetworkSegment: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-DeleteNetworkSegment")
	defer span.End()

	// Validations
	if request.NetworkSegmentId == nil {
		// Name is mandatory
		log.Err(ErrInvalidID).Msg("DeleteNetworkSegment: invalid request")
		return response, ErrInvalidID
	}
	carbideRequest := &wflows.NetworkSegmentDeletionRequest{}
	carbideRequest.Id = &wflows.NetworkSegmentId{Value: request.NetworkSegmentId.Value}
	response, err = sub.carbide.DeleteNetworkSegment(ctx, carbideRequest)
	log.Info().Interface("request", carbideRequest).Msg("DeleteNetworkSegment: sent gRPC request")
	return response, err
}

func (sub *network) ValidatePrefixes(prefixes []*wflows.NetworkPrefixInfo) (err error) {
	log.Info().Msg("ValidatePrefixes: checking the prefixes")
	// Validations
	// 1. Check if the prefixes are filled
	if len(prefixes) == 0 {
		// Invalid list
		log.Err(ErrEmptyPrefixes).Msg("ValidatePrefixes: invalid prefix")
		return ErrEmptyPrefixes
	}

	// There are 2 policies that can be applied here
	// 1. Validate individual prefix and send the error list (or)
	// 2. Fail at initial error
	errlist := []error{}
	var prefixErr error
	for index, prefix := range prefixes {
		log.Info().Int("Index", index).Msg("ValidatePrefixes: checking the prefixes")
		err = sub.ValidatePrefix(prefix)
		if err != nil {
			log.Err(err).Int("Index", index).Msg("ValidatePrefixes: checking the prefixes")
			errlist = append(errlist, err)
			prefixErr = err
		} else {
			errlist = append(errlist, nil)
		}
	}

	return prefixErr
}

func (sub *network) ValidatePrefix(prefix *wflows.NetworkPrefixInfo) (err error) {
	// Validations
	if prefix == nil {
		log.Err(ErrEmptyPrefix).Msg("ValidatePrefix - invalid prefix")
		return ErrEmptyPrefix

	}
	// convert prefix cidr string to net addr info
	// Assume IPv4 address
	_, _, err = net.ParseCIDR(prefix.Prefix)
	if err != nil {
		log.Err(err).Msg("ValidatePrefix - invalid prefix")
		return err
	}

	return nil
}
