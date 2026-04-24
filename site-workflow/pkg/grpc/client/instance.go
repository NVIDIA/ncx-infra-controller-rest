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
	"github.com/gogo/status"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/codes"
)

var (
	ErrInvalidInstanceName = errors.New("gRPC-lib: Instance - invalid name")
	ErrInvalidInstanceID   = errors.New("gRPC-lib: Instance - invalid instance id")
	ErrInvalidMachineID    = errors.New("gRPC-lib: Instance - invalid machine id")
	ErrInvalidSegmentID    = errors.New("gRPC-lib: Instance - invalid segment id")
	ErrInvalidIPxe         = errors.New("gRPC-lib: Instance - invalid custom ipxe")
	ErrInvalidRequest      = errors.New("gRPC-lib: Instance - invalid request")
)

type InstanceInterface interface {
	// Instance Interfaces
	CreateInstance(ctx context.Context, request *wflows.CreateInstanceRequest) (response *wflows.Instance, err error)
	CreateInstances(ctx context.Context, request *wflows.BatchInstanceAllocationRequest) (response *wflows.BatchInstanceAllocationResponse, err error)
	DeleteInstance(ctx context.Context, request *wflows.DeleteInstanceRequest) (response *wflows.InstanceReleaseResult, err error)
	RebootInstance(ctx context.Context, request *wflows.RebootInstanceRequest) (response *wflows.InstancePowerResult, err error)

	FindInstanceIDs(ctx context.Context, request *wflows.InstanceSearchFilter) (response *wflows.InstanceIdList, err error)
	FindInstancesByIDs(ctx context.Context, request *wflows.InstancesByIdsRequest) (response *wflows.InstanceList, err error)

	GetAllInstances(ctx context.Context, request *wflows.InstanceSearchFilter, pageSize int) (response *wflows.InstanceList, err error)
}

func (instance *compute) GetAllInstances(ctx context.Context, request *wflows.InstanceSearchFilter, pageSize int) (response *wflows.InstanceList, err error) {
	log.Info().Interface("request", request).Msg("GetAllInstances: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-GetAllInstances")
	defer span.End()

	if request == nil {
		request = &wflows.InstanceSearchFilter{}
	}

	idList, err := instance.nico.FindInstanceIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindInstanceIds: error")
		return nil, err
	}
	response = &wflows.InstanceList{}
	idChunks := SliceToChunks(idList.InstanceIds, pageSize)
	for i, chunk := range idChunks {
		list, err := instance.nico.FindInstancesByIds(ctx, &wflows.InstancesByIdsRequest{InstanceIds: chunk})
		if err != nil {
			log.Error().Err(err).Msgf("FindInstancesByIds: error on chunk index %d", i)
			return nil, err
		}
		response.Instances = append(response.Instances, list.Instances...)
	}
	log.Info().Int("InstanceListLen", len(idList.InstanceIds)).Msg("GetInstances: received result")
	return response, err
}

func (instance *compute) FindInstanceIDs(ctx context.Context, request *wflows.InstanceSearchFilter) (response *wflows.InstanceIdList, err error) {
	log.Info().Interface("request", request).Msg("FindInstanceIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-FindInstanceIDs")
	defer span.End()

	if request == nil {
		request = &wflows.InstanceSearchFilter{}
	}
	response, err = instance.nico.FindInstanceIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindInstanceIds: error")
		return nil, err
	}
	return
}

func (instance *compute) FindInstancesByIDs(ctx context.Context, request *wflows.InstancesByIdsRequest) (response *wflows.InstanceList, err error) {
	log.Info().Interface("request", request).Msg("FindInstancesByIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-FindInstancesByIDs")
	defer span.End()

	if request == nil {
		request = &wflows.InstancesByIdsRequest{}
	}
	response, err = instance.nico.FindInstancesByIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msgf("FindInstancesByIds: error")
		return nil, err
	}

	return
}

func (instance *compute) CreateInstance(ctx context.Context, request *wflows.CreateInstanceRequest) (response *wflows.Instance, err error) {
	log.Info().Interface("request", request).Msg("CreateInstance: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-CreateInstance")
	defer span.End()

	// Validations
	if request.MachineId == nil {
		// Id is mandatory
		log.Err(ErrInvalidMachineID).Msg("CreateInstance: invalid request")
		return response, ErrInvalidMachineID
	}

	// Nico request
	// Convert Resource Request to the type needed by Site controller
	machineID := wflows.MachineId{}
	machineID.Id = request.MachineId.Id
	nicoRequest := &wflows.InstanceAllocationRequest{
		MachineId: &wflows.MachineId{},
	}
	if request.InstanceId != nil {
		nicoRequest.InstanceId = &wflows.InstanceId{Value: request.InstanceId.Value}
	}
	nicoRequest.MachineId = &machineID
	nicoRequest.Config = &wflows.InstanceConfig{}
	nicoRequest.Config.Tenant = &wflows.TenantConfig{
		TenantOrganizationId: request.TenantOrg,
		TenantKeysetIds:      request.TenantKeysetIds,
	}

	nicoRequest.Config.Os = &wflows.OperatingSystem{
		PhoneHomeEnabled: request.PhoneHomeEnabled,
	}

	if request.CustomIpxe != nil {
		nicoRequest.Config.Os.Variant = &wflows.OperatingSystem_Ipxe{
			Ipxe: &wflows.InlineIpxe{
				IpxeScript: *request.CustomIpxe,
			},
		}
	}

	if request.UserData != nil {
		nicoRequest.Config.Os.UserData = request.UserData
	}

	if request.AlwaysBootWithCustomIpxe != nil {
		nicoRequest.Config.Os.RunProvisioningInstructionsOnEveryBoot = *request.AlwaysBootWithCustomIpxe
	}

	nicoRequest.Config.Network = &wflows.InstanceNetworkConfig{}
	nicoRequest.Config.Network.Interfaces = request.Interfaces

	// InfiniBand Interfaces
	if request.IbInterfaces != nil {
		nicoRequest.Config.Infiniband = &wflows.InstanceInfinibandConfig{}
		nicoRequest.Config.Infiniband.IbInterfaces = request.IbInterfaces
	}

	// Instance labels metadata
	if request.Metadata != nil {
		nicoRequest.Metadata = request.Metadata
	}

	// Lets verify the applicable parameters
	response, err = instance.nico.AllocateInstance(ctx, nicoRequest)
	log.Info().Interface("request", nicoRequest).Msg("CreateInstance: sent gRPC request")
	return response, err
}

// CreateInstances creates multiple instances in a single transaction
// This wraps Nico's AllocateInstances gRPC method
func (instance *compute) CreateInstances(ctx context.Context, request *wflows.BatchInstanceAllocationRequest) (response *wflows.BatchInstanceAllocationResponse, err error) {
	log.Info().Interface("request", request).Int("count", len(request.InstanceRequests)).Msg("CreateInstances: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-CreateInstances")
	defer span.End()

	// Validations
	if len(request.InstanceRequests) == 0 {
		log.Err(ErrInvalidRequest).Msg("CreateInstances: empty instance requests")
		return nil, ErrInvalidRequest
	}

	// Call nico batch API (AllocateInstances is the name in Nico layer)
	response, err = instance.nico.AllocateInstances(ctx, request)
	if err != nil {
		log.Err(err).Msg("CreateInstances: failed")
		return nil, err
	}

	log.Info().Int("count", len(response.Instances)).Msg("CreateInstances: successfully created instances")
	return response, nil
}

func (instance *compute) DeleteInstance(ctx context.Context, request *wflows.DeleteInstanceRequest) (response *wflows.InstanceReleaseResult, err error) {
	log.Info().Interface("request", request).Msg("DeleteInstance: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-DeleteInstance")
	defer span.End()

	// Validations
	if request.InstanceId == nil {
		// Name is mandatory
		log.Err(ErrInvalidInstanceID).Msg("DeleteInstance: invalid request")
		return response, ErrInvalidInstanceID
	}
	nicoRequest := &wflows.InstanceReleaseRequest{}
	if request.InstanceId != nil {
		nicoRequest.Id = &wflows.InstanceId{Value: request.InstanceId.Value}
	}
	response, err = instance.nico.ReleaseInstance(ctx, nicoRequest)
	if err != nil {
		// If site controller don't have Instance, no need to fail the request
		// Check for grpc error code 'NotFound'
		if status.Code(err) == codes.NotFound {
			err = nil
		}
	}
	log.Info().Interface("request", nicoRequest).Msg("DeleteInstance: sent gRPC request")
	return response, err
}
