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

package leakdetection

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/carbideapi"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/config"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/operation"
	taskmanager "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/manager"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/operations"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/common/devicetypes"
)

// RunLeakDetection will loop and handle various leak detection tasks
func RunLeakDetection(ctx context.Context, taskMgr taskmanager.Manager) {
	config := config.ReadConfig()
	if config.DisableLeakDetection {
		log.Info().Msg("Leak detection disabled by configuration")
		return
	}

	carbideClient, err := carbideapi.NewClient(config.GRPCTimeout)
	if err != nil {
		// Use whether CARBIDE_API_URL is set to determine if we're running in a production environment (fail hard) or not (just complain and do nothing)
		// Note that this doesn't actually create a connection immediately, so it won't fail just because carbide-api hasn't started yet.
		msg := fmt.Sprintf("Unable to create GRPC client (pre-connect): %v", err)
		if os.Getenv("CARBIDE_API_URL") == "" {
			log.Error().Msg(msg)
			return
		} else {
			log.Fatal().Msg(msg)
		}
	}

	log.Info().Msg("Starting leak detection loop")

	for {
		runLeakDetectionOne(ctx, &config, carbideClient, taskMgr)
		time.Sleep(config.LeakDetectionInterval)
	}
}

func runLeakDetectionOne(ctx context.Context, config *config.Config, carbideClient carbideapi.Client, taskMgr taskmanager.Manager) {
	log.Info().Msg("Running leak detection")

	leakingMachineIds, err := carbideClient.GetLeakingMachineIds(ctx)
	if err != nil {
		log.Error().Msgf("Unable to retrieve leaking machine IDs from Carbide: %v", err)
		return
	}

	log.Info().Msgf("Found %d leaking machine IDs", len(leakingMachineIds))

	for _, machineID := range leakingMachineIds {
		log.Info().Msgf("Leaking machine ID: %s, submitting force power-off task", machineID)

		if err := submitPowerOffTask(ctx, taskMgr, machineID); err != nil {
			log.Error().Err(err).Str("machine_id", machineID).
				Msg("Failed to submit power-off task for leaking machine")
		}
	}
}

func submitPowerOffTask(ctx context.Context, taskMgr taskmanager.Manager, machineID string) error {
	info := &operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationForcePowerOff,
		Forced:    true,
	}

	raw, err := info.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal power control info: %w", err)
	}

	req := &operation.Request{
		Operation: operation.Wrapper{
			Type: info.Type(),
			Code: info.CodeString(),
			Info: raw,
		},
		TargetSpec: operation.TargetSpec{
			Components: []operation.ComponentTarget{
				{
					External: &operation.ExternalRef{
						Type: devicetypes.ComponentTypeCompute,
						ID:   machineID,
					},
				},
			},
		},
		Description:      fmt.Sprintf("Leak detection: force power-off machine %s", machineID),
		ConflictStrategy: operation.ConflictStrategyQueue,
	}

	taskIDs, err := taskMgr.SubmitTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to submit task: %w", err)
	}

	if len(taskIDs) == 0 {
		return fmt.Errorf("failed to create any power-off tasks for leaking machine %s", machineID)
	}

	log.Info().
		Str("machine_id", machineID).
		Int("task_count", len(taskIDs)).
		Msg("Power-off task submitted for leaking machine")

	return nil
}
