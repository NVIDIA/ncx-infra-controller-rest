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
package nvswitchmanager

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/nsmapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager"
	nsmprovider "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/providers/nvswitchmanager"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

const (
	// ImplementationName is the name used to identify this implementation.
	ImplementationName = "nvswitchmanager"
)

// Manager manages NVLink switch components via the NV-Switch Manager gRPC API.
type Manager struct {
	nsmClient nsmapi.Client
}

// New creates a new NV-Switch Manager-based NVLSwitch Manager instance.
func New(nsmClient nsmapi.Client) *Manager {
	return &Manager{
		nsmClient: nsmClient,
	}
}

// Factory creates a new Manager from the provided providers.
// It retrieves the NVSwitchManager provider from the registry and uses its client.
func Factory(providerRegistry *componentmanager.ProviderRegistry) (componentmanager.ComponentManager, error) {
	provider, err := componentmanager.GetTyped[*nsmprovider.Provider](
		providerRegistry,
		nsmprovider.ProviderName,
	)
	if err != nil {
		return nil, fmt.Errorf("nvlswitch/nvswitchmanager requires nvswitchmanager provider: %w", err)
	}

	return New(provider.Client()), nil
}

// Register registers the NV-Switch Manager NVLSwitch manager factory with the given registry.
func Register(registry *componentmanager.Registry) {
	registry.RegisterFactory(devicetypes.ComponentTypeNVLSwitch, ImplementationName, Factory)
}

// Type returns the component type this manager handles.
func (m *Manager) Type() devicetypes.ComponentType {
	return devicetypes.ComponentTypeNVLSwitch
}

// InjectExpectation injects expected configuration or state information for an NVLink switch.
func (m *Manager) InjectExpectation(
	_ context.Context,
	_ common.Target,
	_ operations.InjectExpectationTaskInfo,
) error {
	return fmt.Errorf("InjectExpectation not yet implemented for NVLSwitch (nvswitchmanager)")
}

// PowerControl performs power operations on NVLink switches via the NV-Switch Manager API.
func (m *Manager) PowerControl(
	ctx context.Context,
	target common.Target,
	info operations.PowerControlTaskInfo,
) error {
	log.Debug().Msgf(
		"NVLSwitch (nvswitchmanager) power control %s op %s activity received",
		target.String(),
		info.Operation.String(),
	)

	if m.nsmClient == nil {
		return fmt.Errorf("NV-Switch Manager client is not configured")
	}

	if err := target.Validate(); err != nil {
		return fmt.Errorf("target is invalid: %w", err)
	}

	action, err := mapPowerOperation(info.Operation)
	if err != nil {
		return err
	}

	results, err := m.nsmClient.PowerControl(ctx, target.ComponentIDs, action)
	if err != nil {
		return fmt.Errorf("failed to perform power control via NV-Switch Manager: %w", err)
	}

	for _, result := range results {
		if result.Status != nsmapi.StatusSuccess {
			return fmt.Errorf("power control failed for switch %s: %s", result.UUID, result.Error)
		}
	}

	log.Info().Msgf("power control %s on %s completed via NV-Switch Manager",
		info.Operation.String(), target.String())

	return nil
}

// FirmwareControl performs firmware operations on NVLink switches via the NV-Switch Manager API.
func (m *Manager) FirmwareControl(
	ctx context.Context,
	target common.Target,
	info operations.FirmwareControlTaskInfo,
) error {
	log.Debug().Msgf(
		"NVLSwitch (nvswitchmanager) firmware control %s op %s activity received",
		target.String(),
		info.Operation.String(),
	)

	if m.nsmClient == nil {
		return fmt.Errorf("NV-Switch Manager client is not configured")
	}

	if err := target.Validate(); err != nil {
		return fmt.Errorf("target is invalid: %w", err)
	}

	switch info.Operation {
	case operations.FirmwareOperationUpgrade:
		return m.firmwareUpgrade(ctx, target, info)
	case operations.FirmwareOperationVersion:
		return m.firmwareVersion(ctx, target)
	case operations.FirmwareOperationDowngrade:
		return fmt.Errorf("firmware downgrade not supported by NV-Switch Manager; use upgrade with the desired target version")
	case operations.FirmwareOperationRollback:
		return fmt.Errorf("firmware rollback not supported by NV-Switch Manager; use upgrade with the desired target version")
	default:
		return fmt.Errorf("unknown firmware operation: %v", info.Operation)
	}
}

func (m *Manager) firmwareUpgrade(ctx context.Context, target common.Target, info operations.FirmwareControlTaskInfo) error {
	if info.TargetVersion == "" {
		return fmt.Errorf("target_version (bundle version) is required for firmware upgrade")
	}

	for _, componentID := range target.ComponentIDs {
		updates, err := m.nsmClient.QueueUpdate(ctx, componentID, info.TargetVersion, nil)
		if err != nil {
			return fmt.Errorf("failed to queue firmware update for switch %s: %w", componentID, err)
		}

		log.Info().
			Str("switch_uuid", componentID).
			Str("bundle_version", info.TargetVersion).
			Int("updates_queued", len(updates)).
			Msg("Firmware update queued via NV-Switch Manager")
	}

	return nil
}

func (m *Manager) firmwareVersion(ctx context.Context, target common.Target) error {
	for _, componentID := range target.ComponentIDs {
		bundles, err := m.nsmClient.ListBundles(ctx)
		if err != nil {
			return fmt.Errorf("failed to list firmware bundles for switch %s: %w", componentID, err)
		}

		log.Info().
			Str("switch_uuid", componentID).
			Int("bundles_available", len(bundles)).
			Msg("Firmware bundles retrieved via NV-Switch Manager")
	}

	return nil
}

// mapPowerOperation maps RLA's PowerOperation to NV-Switch Manager's PowerAction.
func mapPowerOperation(op operations.PowerOperation) (nsmapi.PowerAction, error) {
	switch op {
	case operations.PowerOperationPowerOn:
		return nsmapi.PowerActionOn, nil
	case operations.PowerOperationForcePowerOn:
		return nsmapi.PowerActionForceOn, nil
	case operations.PowerOperationPowerOff:
		return nsmapi.PowerActionGracefulShutdown, nil
	case operations.PowerOperationForcePowerOff:
		return nsmapi.PowerActionForceOff, nil
	case operations.PowerOperationRestart:
		return nsmapi.PowerActionGracefulRestart, nil
	case operations.PowerOperationForceRestart:
		return nsmapi.PowerActionForceRestart, nil
	case operations.PowerOperationWarmReset:
		log.Warn().Msg("NV-Switch Manager does not distinguish warm/cold reset; using PowerCycle")
		return nsmapi.PowerActionPowerCycle, nil
	case operations.PowerOperationColdReset:
		log.Warn().Msg("NV-Switch Manager does not distinguish warm/cold reset; using PowerCycle")
		return nsmapi.PowerActionPowerCycle, nil
	default:
		return nsmapi.PowerActionUnknown, fmt.Errorf("unknown power operation: %v", op)
	}
}
