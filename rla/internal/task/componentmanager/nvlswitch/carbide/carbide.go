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

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/carbideapi"
	pb "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/carbideapi/gen"
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

func switchIDsProto(ids []string) *pb.SwitchIdList {
	pbIDs := make([]*pb.SwitchId, len(ids))
	for i, id := range ids {
		pbIDs[i] = &pb.SwitchId{Id: id}
	}
	return &pb.SwitchIdList{Ids: pbIDs}
}

// PowerControl performs power operations on NVLink switches via Carbide's
// ComponentPowerControl RPC.
func (m *Manager) PowerControl(
	ctx context.Context,
	target common.Target,
	info operations.PowerControlTaskInfo,
) error {
	log.Debug().Msgf(
		"NVLSwitch power control %s op %s via Carbide",
		target.String(),
		info.Operation.String(),
	)

	if err := target.Validate(); err != nil {
		return fmt.Errorf("target is invalid: %w", err)
	}

	var action pb.SystemPowerControl
	switch info.Operation {
	case operations.PowerOperationPowerOn, operations.PowerOperationForcePowerOn:
		action = pb.SystemPowerControl_SYSTEM_POWER_CONTROL_ON
	case operations.PowerOperationPowerOff:
		action = pb.SystemPowerControl_SYSTEM_POWER_CONTROL_GRACEFUL_SHUTDOWN
	case operations.PowerOperationForcePowerOff:
		action = pb.SystemPowerControl_SYSTEM_POWER_CONTROL_FORCE_OFF
	case operations.PowerOperationRestart, operations.PowerOperationWarmReset:
		action = pb.SystemPowerControl_SYSTEM_POWER_CONTROL_GRACEFUL_RESTART
	case operations.PowerOperationForceRestart:
		action = pb.SystemPowerControl_SYSTEM_POWER_CONTROL_FORCE_RESTART
	default:
		return fmt.Errorf("unsupported power operation for NVLSwitch: %v", info.Operation)
	}

	req := &pb.ComponentPowerControlRequest{
		Target: &pb.ComponentPowerControlRequest_SwitchIds{
			SwitchIds: switchIDsProto(target.ComponentIDs),
		},
		Action: action,
	}

	resp, err := m.carbideClient.ComponentPowerControl(ctx, req)
	if err != nil {
		return fmt.Errorf("ComponentPowerControl failed: %w", err)
	}

	for _, r := range resp.GetResults() {
		if r.GetStatus() != pb.ComponentManagerStatusCode_COMPONENT_MANAGER_STATUS_CODE_SUCCESS {
			return fmt.Errorf("power control failed for %s: %s", r.GetComponentId(), r.GetError())
		}
	}

	log.Info().Msgf("NVLSwitch power control %s on %s completed via Carbide",
		info.Operation.String(), target.String())
	return nil
}

func (m *Manager) GetPowerStatus(
	ctx context.Context,
	target common.Target,
) (map[string]operations.PowerStatus, error) {
	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("target is invalid: %w", err)
	}

	req := &pb.GetComponentInventoryRequest{
		Target: &pb.GetComponentInventoryRequest_SwitchIds{
			SwitchIds: switchIDsProto(target.ComponentIDs),
		},
	}

	resp, err := m.carbideClient.GetComponentInventory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetComponentInventory failed: %w", err)
	}

	result := make(map[string]operations.PowerStatus, len(target.ComponentIDs))
	for _, id := range target.ComponentIDs {
		result[id] = operations.PowerStatusUnknown
	}

	for _, entry := range resp.GetEntries() {
		compID := entry.GetResult().GetComponentId()
		if ps := carbideprovider.ExtractPowerState(entry.GetReport()); ps != operations.PowerStatusUnknown {
			result[compID] = ps
		}
	}

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
		Msg("Starting firmware update for NVLSwitch via Carbide")

	if err := target.Validate(); err != nil {
		return fmt.Errorf("target is invalid: %w", err)
	}

	req := &pb.UpdateComponentFirmwareRequest{
		Target: &pb.UpdateComponentFirmwareRequest_SwitchIds{
			SwitchIds: switchIDsProto(target.ComponentIDs),
		},
		TargetVersion: info.TargetVersion,
	}

	resp, err := m.carbideClient.UpdateComponentFirmware(ctx, req)
	if err != nil {
		return fmt.Errorf("UpdateComponentFirmware failed: %w", err)
	}

	for _, r := range resp.GetResults() {
		if r.GetStatus() != pb.ComponentManagerStatusCode_COMPONENT_MANAGER_STATUS_CODE_SUCCESS {
			return fmt.Errorf("firmware update failed for %s: %s", r.GetComponentId(), r.GetError())
		}
	}

	log.Info().
		Str("components", target.String()).
		Str("target_version", info.TargetVersion).
		Msg("Firmware update started for NVLSwitch via Carbide")
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

	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("target is invalid: %w", err)
	}

	req := &pb.GetComponentFirmwareStatusRequest{
		Target: &pb.GetComponentFirmwareStatusRequest_SwitchIds{
			SwitchIds: switchIDsProto(target.ComponentIDs),
		},
	}

	resp, err := m.carbideClient.GetComponentFirmwareStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("GetComponentFirmwareStatus failed: %w", err)
	}

	result := make(map[string]operations.FirmwareUpdateStatus, len(resp.GetStatuses()))
	for _, s := range resp.GetStatuses() {
		compID := s.GetResult().GetComponentId()
		result[compID] = operations.FirmwareUpdateStatus{
			ComponentID: compID,
			State:       carbideprovider.MapFirmwareState(s.GetState()),
		}
	}

	return result, nil
}

func (m *Manager) AllowBringUpAndPowerOn(
	ctx context.Context,
	target common.Target,
) error {
	log.Info().
		Str("components", target.String()).
		Msg("NVLSwitch AllowBringUpAndPowerOn: placeholder")
	return nil
}

func (m *Manager) GetBringUpState(
	ctx context.Context,
	target common.Target,
) (map[string]operations.MachineBringUpState, error) {
	log.Info().
		Str("components", target.String()).
		Msg("NVLSwitch GetBringUpState: placeholder")

	result := make(
		map[string]operations.MachineBringUpState,
		len(target.ComponentIDs),
	)
	for _, id := range target.ComponentIDs {
		result[id] = operations.MachineBringUpStateMachineCreated
	}
	return result, nil
}
