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

func (instance *compute) RebootInstance(ctx context.Context, resourceRequest *wflows.RebootInstanceRequest) (result *wflows.InstancePowerResult, err error) {
	log.Info().Interface("request", resourceRequest).Msg("RebootInstance: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "NicoClient-RebootInstance")
	defer span.End()

	// Validations
	if resourceRequest.MachineId == nil {
		// Name is mandatory
		log.Err(ErrInvalidMachineID).Msg("RebootInstance: invalid request")
		return result, ErrInvalidMachineID
	}
	nicoRequest := &wflows.InstancePowerRequest{MachineId: &wflows.MachineId{}}

	// Convert Resource Request to the type needed by Site controller
	machineID := nicoRequest.MachineId
	machineID.Id = resourceRequest.MachineId.Id
	nicoRequest.MachineId = machineID
	nicoRequest.BootWithCustomIpxe = resourceRequest.BootWithCustomIpxe
	nicoRequest.Operation = wflows.InstancePowerRequest_POWER_RESET
	nicoRequest.ApplyUpdatesOnReboot = resourceRequest.ApplyUpdatesOnReboot
	grpcResponse, err := instance.nico.InvokeInstancePower(ctx, nicoRequest)
	log.Info().Interface("request", nicoRequest).Interface("response", grpcResponse).Msg("RebootInstance: sent gRPC request")
	return grpcResponse, err
}
