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

package carbide

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/carbideapi"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/componentmanager"
	carbideprovider "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/componentmanager/providers/carbide"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/operations"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/common/devicetypes"
)

const (
	// ImplementationName is the name used to identify this implementation.
	ImplementationName = "carbide"
)

// Manager manages NVLink switch components via the Carbide API.
type Manager struct {
	carbideClient carbideapi.Client
}

// New creates a new Carbide-based NVLSwitch Manager instance.
func New(carbideClient carbideapi.Client) *Manager {
	return &Manager{
		carbideClient: carbideClient,
	}
}

// Factory creates a new Manager from the provided providers.
// It retrieves the CarbideProvider from the registry and uses its client.
func Factory(providerRegistry *componentmanager.ProviderRegistry) (componentmanager.ComponentManager, error) {
	provider, err := componentmanager.GetTyped[*carbideprovider.Provider](
		providerRegistry,
		carbideprovider.ProviderName,
	)
	if err != nil {
		return nil, fmt.Errorf("nvlswitch/carbide requires carbide provider: %w", err)
	}

	return New(provider.Client()), nil
}

// Register registers the Carbide NVLSwitch manager factory with the given registry.
func Register(registry *componentmanager.Registry) {
	registry.RegisterFactory(devicetypes.ComponentTypeNVLSwitch, ImplementationName, Factory)
}

// Type returns the component type this manager handles.
func (m *Manager) Type() devicetypes.ComponentType {
	return devicetypes.ComponentTypeNVLSwitch
}

// InjectExpectation registers an expected switch with Carbide via AddExpectedSwitch.
// The Info field should contain a JSON-encoded carbideapi.AddExpectedSwitchRequest.
func (m *Manager) InjectExpectation(
	ctx context.Context,
	target common.Target,
	info operations.InjectExpectationTaskInfo,
) error {
	var req carbideapi.AddExpectedSwitchRequest
	if err := json.Unmarshal(info.Info, &req); err != nil {
		return fmt.Errorf("failed to unmarshal AddExpectedSwitchRequest: %w", err)
	}

	if m.carbideClient == nil {
		return fmt.Errorf("carbide client is not configured")
	}

	if err := m.carbideClient.AddExpectedSwitch(ctx, req); err != nil {
		return fmt.Errorf("failed to add expected switch: %w", err)
	}

	log.Info().
		Str("bmc_mac", req.BMCMACAddress).
		Str("switch_serial", req.SwitchSerialNumber).
		Msg("Successfully registered expected switch with Carbide")

	return nil
}

// PowerControl performs power operations on an NVLink switch via Carbide API.
func (m *Manager) PowerControl(
	ctx context.Context,
	target common.Target,
	info operations.PowerControlTaskInfo,
) error {
	log.Debug().Msgf(
		"NVLSwitch power control %s op %s activity received",
		target.String(),
		info.Operation.String(),
	)

	if m.carbideClient == nil {
		return fmt.Errorf("carbide client is not configured")
	}

	if err := target.Validate(); err != nil {
		return fmt.Errorf("target is invalid: %w", err)
	}

	// Map common.PowerOperation to carbideapi.SystemPowerControl
	var action carbideapi.SystemPowerControl
	var desiredPowerState carbideapi.PowerState
	switch info.Operation {
	// Power On
	case operations.PowerOperationPowerOn:
		action = carbideapi.PowerControlOn
		desiredPowerState = carbideapi.PowerStateOn
	case operations.PowerOperationForcePowerOn:
		action = carbideapi.PowerControlForceOn
		desiredPowerState = carbideapi.PowerStateOn
	// Power Off
	case operations.PowerOperationPowerOff:
		action = carbideapi.PowerControlGracefulShutdown
		desiredPowerState = carbideapi.PowerStateOff
	case operations.PowerOperationForcePowerOff:
		action = carbideapi.PowerControlForceOff
		desiredPowerState = carbideapi.PowerStateOff
	// Restart (OS level)
	case operations.PowerOperationRestart:
		action = carbideapi.PowerControlGracefulRestart
		desiredPowerState = carbideapi.PowerStateOn
	case operations.PowerOperationForceRestart:
		action = carbideapi.PowerControlForceRestart
		desiredPowerState = carbideapi.PowerStateOn
	// Reset (hardware level)
	case operations.PowerOperationWarmReset:
		action = carbideapi.PowerControlWarmReset
		desiredPowerState = carbideapi.PowerStateOn
	case operations.PowerOperationColdReset:
		action = carbideapi.PowerControlColdReset
		desiredPowerState = carbideapi.PowerStateOn
	default:
		return fmt.Errorf("unknown power operation: %v", info.Operation)
	}

	for _, componentID := range target.ComponentIDs {
		// Set Carbide's power-on gate (desired power state) before issuing the
		// actual power control command so the power manager doesn't conflict.
		if err := m.carbideClient.UpdatePowerOption(ctx, componentID, desiredPowerState); err != nil {
			return fmt.Errorf("failed to update power option for %s: %w", componentID, err)
		}

		if err := m.carbideClient.AdminPowerControl(ctx, componentID, action); err != nil {
			return fmt.Errorf("failed to perform power control on %s: %w",
				componentID, err)
		}
	}

	log.Info().Msgf("power control %s on %s completed",
		info.Operation.String(), target.String())

	return nil
}

// GetPowerStatus retrieves the power status of NVLink switches via Carbide API.
func (m *Manager) GetPowerStatus(
	ctx context.Context,
	target common.Target,
) (map[string]operations.PowerStatus, error) {
	log.Debug().Msgf(
		"NVLSwitch get power status %s activity received",
		target.String(),
	)

	if m.carbideClient == nil {
		return nil, fmt.Errorf("carbide client is not configured")
	}

	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("target is invalid: %w", err)
	}

	powerStates, err := m.carbideClient.GetPowerStates(ctx, target.ComponentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get power states: %w", err)
	}

	result := make(map[string]operations.PowerStatus, len(powerStates))
	for _, state := range powerStates {
		result[state.MachineID] = carbidePowerStateToOperationsPowerStatus(state.PowerState)
	}

	log.Info().Msgf("get power status for %s completed, got %d results",
		target.String(), len(result))

	return result, nil
}

// carbidePowerStateToOperationsPowerStatus converts carbide PowerState to operations PowerStatus.
func carbidePowerStateToOperationsPowerStatus(state carbideapi.PowerState) operations.PowerStatus {
	switch state {
	case carbideapi.PowerStateOn:
		return operations.PowerStatusOn
	case carbideapi.PowerStateOff, carbideapi.PowerStateDisabled:
		return operations.PowerStatusOff
	default:
		return operations.PowerStatusUnknown
	}
}

// FirmwareControl schedules a firmware update via Carbide's SetFirmwareUpdateTimeWindow API.
// This sets the time window during which Carbide will automatically perform the firmware update.
// Returns immediately after the schedule request is accepted.
func (m *Manager) FirmwareControl(ctx context.Context, target common.Target, info operations.FirmwareControlTaskInfo) error {
	log.Debug().
		Str("components", target.String()).
		Str("target_version", info.TargetVersion).
		Msg("Scheduling firmware update for NVLSwitch via Carbide")

	if m.carbideClient == nil {
		return fmt.Errorf("carbide client is not configured")
	}

	if err := target.Validate(); err != nil {
		return fmt.Errorf("target is invalid: %w", err)
	}

	startTime := time.Unix(info.StartTime, 0)
	endTime := time.Unix(info.EndTime, 0)

	if err := m.carbideClient.SetFirmwareUpdateTimeWindow(ctx, target.ComponentIDs, startTime, endTime); err != nil {
		return fmt.Errorf("failed to schedule firmware update for NVLSwitch: %w", err)
	}

	log.Info().
		Str("components", target.String()).
		Time("start_time", startTime).
		Time("end_time", endTime).
		Msg("Firmware update scheduled for NVLSwitch")

	return nil
}

// GetFirmwareStatus returns the current status of firmware updates for the target components.
// Carbide does not have a dedicated firmware update status API; we read the current firmware version
// to determine if the update completed.
// TODO: Implement proper status checking once Carbide exposes a firmware update status API.
func (m *Manager) GetFirmwareStatus(ctx context.Context, target common.Target) (map[string]operations.FirmwareUpdateStatus, error) { //nolint
	log.Debug().
		Str("components", target.String()).
		Msg("GetFirmwareStatus called for NVLSwitch")

	result := make(map[string]operations.FirmwareUpdateStatus, len(target.ComponentIDs))
	for _, id := range target.ComponentIDs {
		result[id] = operations.FirmwareUpdateStatus{
			ComponentID: id,
			State:       operations.FirmwareUpdateStateUnknown,
		}
	}

	return result, nil
}
