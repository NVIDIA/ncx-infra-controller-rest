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
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/carbideapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager"
	carbideprovider "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/providers/carbide"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

const (
	// ImplementationName is the name used to identify this implementation.
	ImplementationName = "carbide"
)

// Manager manages compute node components via the Carbide API.
type Manager struct {
	carbideClient carbideapi.Client
	// powerDelay is inserted between sequential power control calls to
	// avoid overwhelming the power delivery system when commanding
	// multiple compute trays. 0 means no delay.
	powerDelay time.Duration
}

// New creates a new Carbide-based compute Manager instance.
func New(carbideClient carbideapi.Client, powerDelay time.Duration) *Manager {
	return &Manager{
		carbideClient: carbideClient,
		powerDelay:    powerDelay,
	}
}

// Register registers the Carbide compute manager factory with the given registry.
// powerDelay is the inter-component stagger for power control calls.
func Register(registry *componentmanager.Registry, powerDelay time.Duration) {
	factory := func(
		providerRegistry *componentmanager.ProviderRegistry,
	) (componentmanager.ComponentManager, error) {
		provider, err := componentmanager.GetTyped[*carbideprovider.Provider](
			providerRegistry,
			carbideprovider.ProviderName,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"compute/carbide requires carbide provider: %w", err,
			)
		}
		return New(provider.Client(), powerDelay), nil
	}
	registry.RegisterFactory(devicetypes.ComponentTypeCompute, ImplementationName, factory)
}

// Type returns the component type this manager handles.
func (m *Manager) Type() devicetypes.ComponentType {
	return devicetypes.ComponentTypeCompute
}

// InjectExpectation injects expected configuration or state information for a compute node.
func (m *Manager) InjectExpectation(
	ctx context.Context,
	target common.Target,
	info operations.InjectExpectationTaskInfo,
) error {
	// TODO: Implement logic to inject expected configuration/state
	// This might involve validating and storing expected firmware versions,
	// hardware configurations, etc.
	return fmt.Errorf("InjectExpectation not yet implemented for compute")
}

// PowerControl performs power operations on a compute node via Carbide API.
func (m *Manager) PowerControl(
	ctx context.Context,
	target common.Target,
	info operations.PowerControlTaskInfo,
) error {
	log.Debug().Msgf(
		"compute power control %s op %s activity received",
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

	for i, componentID := range target.ComponentIDs {
		// Set Carbide's power-on gate (desired power state) before issuing the
		// actual power control command so the power manager doesn't conflict.
		if err := m.carbideClient.UpdatePowerOption(
			ctx, componentID, desiredPowerState,
		); err != nil {
			return fmt.Errorf(
				"failed to update power option for %s: %w", componentID, err,
			)
		}

		if err := m.carbideClient.AdminPowerControl(ctx, componentID, action); err != nil {
			return fmt.Errorf(
				"failed to perform power control on %s: %w", componentID, err,
			)
		}

		// Stagger calls to avoid overwhelming the power delivery system
		if m.powerDelay > 0 && i < len(target.ComponentIDs)-1 {
			time.Sleep(m.powerDelay)
		}
	}

	log.Info().Msgf("power control %s on %s completed",
		info.Operation.String(), target.String())

	return nil
}

// GetPowerStatus retrieves the power status of compute nodes via Carbide API.
func (m *Manager) GetPowerStatus(
	ctx context.Context,
	target common.Target,
) (map[string]operations.PowerStatus, error) {
	log.Debug().Msgf(
		"compute get power status %s activity received",
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

// FirmwareControl performs firmware operations on a compute node.
func (m *Manager) FirmwareControl(
	ctx context.Context,
	target common.Target,
	info operations.FirmwareControlTaskInfo,
) error {
	// TODO: Implement firmware control
	switch info.Operation {
	case operations.FirmwareOperationUpgrade:
		// Implement firmware upgrade
		return fmt.Errorf("firmware upgrade not yet implemented for compute")
	case operations.FirmwareOperationDowngrade:
		// Implement firmware downgrade
		return fmt.Errorf("firmware downgrade not yet implemented for compute")
	case operations.FirmwareOperationRollback:
		// Implement firmware rollback
		return fmt.Errorf("firmware rollback not yet implemented for compute")
	case operations.FirmwareOperationVersion:
		// Implement firmware version
		return fmt.Errorf("firmware version not yet implemented for compute")
	default:
		return fmt.Errorf("unknown firmware operation: %v", info.Operation)
	}
}

// StartFirmwareUpdate schedules a firmware update via Carbide's SetFirmwareUpdateTimeWindow API.
// This sets the time window during which Carbide will automatically perform the firmware update.
// Returns immediately after the schedule request is accepted.
func (m *Manager) StartFirmwareUpdate(ctx context.Context, target common.Target, info operations.FirmwareControlTaskInfo) error {
	log.Debug().
		Str("components", target.String()).
		Str("target_version", info.TargetVersion).
		Msg("Scheduling firmware update for compute via Carbide")

	if m.carbideClient == nil {
		return fmt.Errorf("carbide client is not configured")
	}

	if err := target.Validate(); err != nil {
		return fmt.Errorf("target is invalid: %w", err)
	}

	startTime := time.Unix(info.StartTime, 0)
	endTime := time.Unix(info.EndTime, 0)

	if err := m.carbideClient.SetFirmwareUpdateTimeWindow(ctx, target.ComponentIDs, startTime, endTime); err != nil {
		return fmt.Errorf("failed to schedule firmware update for compute: %w", err)
	}

	log.Info().
		Str("components", target.String()).
		Time("start_time", startTime).
		Time("end_time", endTime).
		Msg("Firmware update scheduled for compute")

	return nil
}

// GetFirmwareUpdateStatus returns the current status of firmware updates for the target components.
// Carbide does not have a dedicated firmware update status API; we read the current firmware version
// to determine if the update completed.
// TODO: Implement proper status checking once Carbide exposes a firmware update status API.
func (m *Manager) GetFirmwareUpdateStatus(ctx context.Context, target common.Target) (map[string]operations.FirmwareUpdateStatus, error) { //nolint
	log.Debug().
		Str("components", target.String()).
		Msg("GetFirmwareUpdateStatus called for compute")

	// Carbide doesn't have a firmware update status API. Return unknown status for all components.
	// The workflow polling will rely on timeout to determine completion.
	// TODO: Implement proper status check (e.g., compare current firmware version with target).
	result := make(map[string]operations.FirmwareUpdateStatus, len(target.ComponentIDs))
	for _, id := range target.ComponentIDs {
		result[id] = operations.FirmwareUpdateStatus{
			ComponentID: id,
			State:       operations.FirmwareUpdateStateUnknown,
		}
	}

	return result, nil
}

// AllowBringUpAndPowerOn opens the Carbide power-on gate for
// each compute component, allowing bring-up and power on.
func (m *Manager) AllowBringUpAndPowerOn(
	ctx context.Context,
	target common.Target,
) error {
	log.Debug().
		Str("components", target.String()).
		Msg("AllowBringUpAndPowerOn for compute")

	if m.carbideClient == nil {
		return fmt.Errorf("carbide client is not configured")
	}

	if err := target.Validate(); err != nil {
		return fmt.Errorf("target is invalid: %w", err)
	}

	for _, componentID := range target.ComponentIDs {
		if err := m.carbideClient.AllowIngestionAndPowerOn(
			ctx, componentID, "",
		); err != nil {
			return fmt.Errorf(
				"AllowBringUpAndPowerOn failed for %s: %w",
				componentID, err,
			)
		}
		log.Info().
			Str("component_id", componentID).
			Msg("AllowBringUpAndPowerOn succeeded")
	}

	return nil
}

// GetBringUpState returns the bring-up state for each
// compute component via Carbide.
func (m *Manager) GetBringUpState(
	ctx context.Context,
	target common.Target,
) (map[string]operations.MachineBringUpState, error) {
	log.Debug().
		Str("components", target.String()).
		Msg("GetBringUpState for compute")

	if m.carbideClient == nil {
		return nil, fmt.Errorf("carbide client is not configured")
	}

	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("target is invalid: %w", err)
	}

	result := make(
		map[string]operations.MachineBringUpState,
		len(target.ComponentIDs),
	)
	for _, componentID := range target.ComponentIDs {
		state, err := m.carbideClient.DetermineMachineIngestionState(
			ctx, componentID, "",
		)
		if err != nil {
			return nil, fmt.Errorf(
				"GetBringUpState failed for %s: %w",
				componentID, err,
			)
		}
		result[componentID] = carbideToBringUpState(state)
	}

	return result, nil
}

func carbideToBringUpState(
	s carbideapi.BringUpState,
) operations.MachineBringUpState {
	switch s {
	case carbideapi.BringUpStateWaitingForIngestion:
		return operations.MachineBringUpStateWaitingForIngestion
	case carbideapi.BringUpStateMachineNotCreated:
		return operations.MachineBringUpStateMachineNotCreated
	case carbideapi.BringUpStateMachineCreated:
		return operations.MachineBringUpStateMachineCreated
	default:
		return operations.MachineBringUpStateNotDiscovered
	}
}
