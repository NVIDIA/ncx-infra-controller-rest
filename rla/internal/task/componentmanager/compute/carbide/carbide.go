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
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

	// healthOverrideSource is the source tag written into health-report
	// overrides so they can be matched on removal.
	healthOverrideSource = "rla-power-control"
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

// InjectExpectation registers an expected machine with Carbide via AddExpectedMachine.
// The Info field should contain a JSON-encoded carbideapi.AddExpectedMachineRequest.
func (m *Manager) InjectExpectation(
	ctx context.Context,
	target common.Target,
	info operations.InjectExpectationTaskInfo,
) error {
	var req carbideapi.AddExpectedMachineRequest
	if err := json.Unmarshal(info.Info, &req); err != nil {
		return fmt.Errorf("failed to unmarshal AddExpectedMachineRequest: %w", err)
	}

	if m.carbideClient == nil {
		return fmt.Errorf("carbide client is not configured")
	}

	if err := m.carbideClient.AddExpectedMachine(ctx, req); err != nil {
		return fmt.Errorf("failed to add expected machine: %w", err)
	}

	log.Info().
		Str("bmc_mac", req.BMCMACAddress).
		Str("chassis_serial", req.ChassisSerialNumber).
		Msg("Successfully registered expected machine with Carbide")

	return nil
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

	// Track which components had health-report overrides inserted so we
	// can clean them up after the power operation completes.
	var overrideInserted []string

	defer func() {
		// Use a detached context so cleanup is not blocked by the
		// parent context being canceled or timed-out. Each individual
		// gRPC call inside RemoveHealthReportOverride already applies
		// the configured per-call timeout.
		cleanupCtx := context.WithoutCancel(ctx)

		for _, componentID := range overrideInserted {
			if err := m.carbideClient.RemoveHealthReportOverride(
				cleanupCtx, componentID, healthOverrideSource,
			); err != nil {
				log.Warn().Err(err).Str("component", componentID).
					Msg("Failed to remove health report override after power control")
			}
		}
	}()

	for i, componentID := range target.ComponentIDs {
		// Place a health-report override so Carbide marks the machine as
		// under maintenance for the duration of the power operation.
		if err := m.carbideClient.InsertHealthReportOverride(
			ctx, componentID, healthOverrideSource,
		); err != nil {
			log.Warn().Err(err).Str("component", componentID).
				Msg("Failed to insert health report override, proceeding anyway")
		} else {
			overrideInserted = append(overrideInserted, componentID)
		}

		// Set Carbide's power-on gate (desired power state) before issuing the
		// actual power control command so the power manager doesn't conflict.
		if err := m.carbideClient.UpdatePowerOption(
			ctx, componentID, desiredPowerState,
		); err != nil {
			if isAlreadyInDesiredStateError(err) {
				log.Debug().Str("component", componentID).
					Int("desired_state", int(desiredPowerState)).
					Msg("Power option already in desired state, skipping")
			} else {
				return fmt.Errorf(
					"failed to update power option for %s: %w", componentID, err,
				)
			}
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

// FirmwareControl schedules a firmware update via Carbide's SetFirmwareUpdateTimeWindow API.
//
// Before scheduling, it performs two checks:
//  1. Fail-fast: if targetVersion is provided, it must match one of the
//     desired firmware entries configured in Core. Otherwise the update
//     can never succeed and we return an error immediately.
//  2. Idempotent: if all target machines already have UpdateComplete==true,
//     the update already finished and we skip the schedule call.
func (m *Manager) FirmwareControl(ctx context.Context, target common.Target, info operations.FirmwareControlTaskInfo) error {
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

	// Fail-fast: validate targetVersion against Core's desired firmware config.
	if info.TargetVersion != "" {
		parsedTarget, err := parseTargetVersion(info.TargetVersion)
		if err != nil {
			return fmt.Errorf("invalid TargetVersion: %w", err)
		}
		desiredEntries, err := m.carbideClient.GetDesiredFirmwareVersions(ctx)
		if err != nil {
			return fmt.Errorf("failed to query desired firmware versions: %w", err)
		}
		if !isTargetVersionInDesired(parsedTarget, desiredEntries) {
			return fmt.Errorf(
				"target version %q does not match any desired firmware entry in Core; update cannot succeed",
				info.TargetVersion,
			)
		}
	}

	// Idempotent: skip only when every requested machine was returned and all
	// have UpdateComplete=true. A partial lookup must not suppress scheduling
	// because some components may still need the update.
	machines, err := m.carbideClient.FindMachinesByIds(ctx, target.ComponentIDs)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check machine update status, proceeding with schedule")
	} else if len(machines) == len(target.ComponentIDs) && allUpdateComplete(machines) {
		log.Info().
			Str("components", target.String()).
			Msg("All machines already have UpdateComplete=true, skipping firmware schedule")
		return nil
	}

	var startTime, endTime time.Time
	switch {
	case info.StartTime == 0 && info.EndTime == 0:
		startTime = time.Now()
		endTime = startTime.Add(24 * time.Hour)
	case info.StartTime == 0 || info.EndTime == 0:
		return fmt.Errorf("firmware window requires both start_time and end_time, got start=%d end=%d",
			info.StartTime, info.EndTime)
	default:
		startTime = time.Unix(info.StartTime, 0)
		endTime = time.Unix(info.EndTime, 0)
		if !startTime.Before(endTime) {
			return fmt.Errorf("firmware window start_time (%s) must be before end_time (%s)",
				startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
		}
	}

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

// parseTargetVersion parses a targetVersion JSON string into a component
// version map. targetVersion must be a JSON object with the same schema as
// desired_firmware.versions->>'Versions', e.g. {"bmc":"7.10.30.00","uefi":"2.22.1"}.
func parseTargetVersion(targetVersion string) (map[string]string, error) {
	var target map[string]string
	if err := json.Unmarshal([]byte(targetVersion), &target); err != nil {
		return nil, fmt.Errorf("target_version must be a JSON object: %w", err)
	}
	return target, nil
}

// isTargetVersionInDesired checks whether a pre-parsed component version map
// matches the component_versions of any desired firmware entry.
func isTargetVersionInDesired(target map[string]string, entries []*pb.DesiredFirmwareVersionEntry) bool {
	for _, entry := range entries {
		if versionsEqual(target, entry.GetComponentVersions()) {
			return true
		}
	}
	return false
}

func versionsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func allUpdateComplete(machines []carbideapi.MachineDetail) bool {
	if len(machines) == 0 {
		return false
	}
	for _, m := range machines {
		if !m.UpdateComplete {
			return false
		}
	}
	return true
}

// GetFirmwareStatus returns the current status of firmware updates for the target components.
// It queries Carbide for the current machine firmware version and compares it against the
// target version to determine completion.
func (m *Manager) GetFirmwareStatus(ctx context.Context, target common.Target) (map[string]operations.FirmwareUpdateStatus, error) {
	log.Debug().
		Str("components", target.String()).
		Msg("GetFirmwareStatus called for compute")

	if m.carbideClient == nil {
		return nil, fmt.Errorf("carbide client is not configured")
	}

	machines, err := m.carbideClient.FindMachinesByIds(ctx, target.ComponentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query machines: %w", err)
	}

	machineByID := make(map[string]carbideapi.MachineDetail, len(machines))
	for _, machine := range machines {
		machineByID[machine.MachineID] = machine
	}

	result := make(map[string]operations.FirmwareUpdateStatus, len(target.ComponentIDs))
	for _, id := range target.ComponentIDs {
		machine, ok := machineByID[id]
		if !ok {
			result[id] = operations.FirmwareUpdateStatus{
				ComponentID: id,
				State:       operations.FirmwareUpdateStateUnknown,
				Error:       "machine not found in Carbide",
			}
			continue
		}

		status := operations.FirmwareUpdateStatus{ComponentID: id}
		switch {
		case machine.UpdateComplete:
			status.State = operations.FirmwareUpdateStateCompleted
		case machine.State == "HostReprovisioning":
			status.State = operations.FirmwareUpdateStateVerifying
		default:
			status.State = operations.FirmwareUpdateStateQueued
		}

		log.Debug().
			Str("machine_id", id).
			Str("machine_state", machine.State).
			Bool("update_complete", machine.UpdateComplete).
			Str("firmware_status", status.State.String()).
			Msg("Firmware status for compute machine")

		result[id] = status
	}

	return result, nil
}

// BringUpControl opens the Carbide power-on gate for
// each compute component, allowing bring-up and power on.
func (m *Manager) BringUpControl(
	ctx context.Context,
	target common.Target,
) error {
	log.Debug().
		Str("components", target.String()).
		Msg("BringUpControl for compute")

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
				"BringUpControl failed for %s: %w",
				componentID, err,
			)
		}
		log.Info().
			Str("component_id", componentID).
			Msg("BringUpControl succeeded")
	}

	return nil
}

// GetBringUpStatus returns the bring-up state for each
// compute component via Carbide.
func (m *Manager) GetBringUpStatus(
	ctx context.Context,
	target common.Target,
) (map[string]operations.MachineBringUpState, error) {
	log.Debug().
		Str("components", target.String()).
		Msg("GetBringUpStatus for compute")

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
				"GetBringUpStatus failed for %s: %w",
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

// isAlreadyInDesiredStateError returns true when Carbide reports that the
// power option is already set to the requested state (idempotent no-op).
func isAlreadyInDesiredStateError(err error) bool {
	if s, ok := status.FromError(err); ok && s.Code() == codes.InvalidArgument {
		return strings.Contains(s.Message(), "already set as")
	}
	return false
}
