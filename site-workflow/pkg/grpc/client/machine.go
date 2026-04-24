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
	"os"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
)

// MachineInterface grpc client interface
type MachineInterface interface {
	// Machine Interfaces
	GetAllMachines(ctx context.Context, request *wflows.MachineSearchConfig, pageSize int) (response *wflows.MachineList, err error)
	FindMachineIDs(ctx context.Context, request *wflows.MachineSearchConfig) (response *wflows.MachineIdList, err error)
	FindMachinesByIDs(ctx context.Context, request *wflows.MachinesByIdsRequest) (response *wflows.MachineList, err error)
}

func (machine *compute) GetAllMachines(ctx context.Context, request *wflows.MachineSearchConfig, pageSize int) (response *wflows.MachineList, err error) {
	log.Info().Interface("request", request).Msg("GetAllMachines: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-GetAllMachines")
	defer span.End()

	if request == nil {
		request = &wflows.MachineSearchConfig{}
	}

	idList, err := machine.nico.FindMachineIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindMachineIds: error")
		return nil, err
	}
	response = &wflows.MachineList{}
	idChunks := SliceToChunks(idList.MachineIds, pageSize)
	for i, chunk := range idChunks {
		list, err := machine.nico.FindMachinesByIds(ctx, &wflows.MachinesByIdsRequest{MachineIds: chunk})
		if err != nil {
			log.Error().Err(err).Msgf("FindMachinesByIds: error on chunk index %d", i)
			return nil, err
		}
		response.Machines = append(response.Machines, list.Machines...)
	}
	log.Info().Int("MachineListLen", len(idList.MachineIds)).Msg("GetMachines: received result")
	return response, err
}

func (machine *compute) FindMachineIDs(ctx context.Context, request *wflows.MachineSearchConfig) (response *wflows.MachineIdList, err error) {
	log.Info().Interface("request", request).Msg("FindMachineIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-FindMachineIDs")
	defer span.End()

	if request == nil {
		request = &wflows.MachineSearchConfig{}
	}

	response, err = machine.nico.FindMachineIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("FindMachineIds: error")
		return nil, err
	}
	return
}

func (machine *compute) FindMachinesByIDs(ctx context.Context, request *wflows.MachinesByIdsRequest) (response *wflows.MachineList, err error) {
	log.Info().Interface("request", request).Msg("FindMachinesByIDs: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-FindMachinesByIDs")
	defer span.End()

	if request == nil {
		request = &wflows.MachinesByIdsRequest{}
	}

	response, err = machine.nico.FindMachinesByIds(ctx, request)
	if err != nil {
		log.Error().Err(err).Msgf("FindMachinesByIds: error")
		return nil, err
	}
	return
}
