/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package leakdetection

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/carbideapi"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/config"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/scheduler/types"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/componentmanager"
	carbideprovider "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/componentmanager/providers/carbide" //nolint
	taskmanager "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/manager"
)

// Job implements scheduler.Job for the leak detection task.
type Job struct {
	carbideClient carbideapi.Client
	taskMgr       taskmanager.Manager
}

// New constructs a leak detection Job using the Carbide provider from the
// registry. Returns nil, nil if leak detection is disabled or the Carbide
// provider is not registered (e.g. non-production environment).
func New(
	taskMgr taskmanager.Manager,
	providers *componentmanager.ProviderRegistry,
	cfg config.Config,
) (*Job, error) {
	if cfg.DisableLeakDetection {
		log.Info().Msg("Leak detection disabled by configuration")
		return nil, nil
	}

	carbideProvider, err := componentmanager.GetTyped[*carbideprovider.Provider](
		providers, carbideprovider.ProviderName,
	)
	if err != nil {
		log.Error().Err(err).
			Msg("Carbide provider not available; leak detection disabled")
		return nil, nil
	}

	return &Job{
		carbideClient: carbideProvider.Client(),
		taskMgr:       taskMgr,
	}, nil
}

// Name returns the job name.
func (j *Job) Name() string { return "leak-detection" }

// Run executes one iteration of leak detection.
func (j *Job) Run(ctx context.Context, _ types.Event) error {
	runLeakDetectionOne(ctx, j.carbideClient, j.taskMgr)
	return nil
}
