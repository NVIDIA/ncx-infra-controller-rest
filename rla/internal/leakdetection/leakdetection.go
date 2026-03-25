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
)

// RunLeakDetection will loop and handle various leak detection tasks
func RunLeakDetection(ctx context.Context) {
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
		runLeakDetectionOne(ctx, &config, carbideClient)
		time.Sleep(config.LeakDetectionInterval)
	}
}

func runLeakDetectionOne(ctx context.Context, config *config.Config, carbideClient carbideapi.Client) {
	log.Info().Msg("Running leak detection")

	leakingMachineIds, err := carbideClient.GetLeakingMachineIds(ctx)
	if err != nil {
		log.Error().Msgf("Unable to retrieve leaking machine IDs from Carbide: %v", err)
		return
	}

	log.Info().Msgf("Found %d leaking machine IDs", len(leakingMachineIds))

	for _, machineID := range leakingMachineIds {
		log.Info().Msgf("Leaking machine ID: %s", machineID)
		// Call workflow routine to call update power options and to turn off this machine
	}
}
