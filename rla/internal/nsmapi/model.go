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
package nsmapi

import (
	"fmt"
	"time"

	pb "github.com/nvidia/bare-metal-manager-rest/rla/internal/nsmapi/gen"
)

// PowerAction enumerates the power actions supported by NV-Switch Manager.
type PowerAction int

const (
	PowerActionUnknown          PowerAction = 0
	PowerActionForceOff         PowerAction = 1
	PowerActionPowerCycle       PowerAction = 2
	PowerActionGracefulShutdown PowerAction = 3
	PowerActionOn               PowerAction = 4
	PowerActionForceOn          PowerAction = 5
	PowerActionGracefulRestart  PowerAction = 6
	PowerActionForceRestart     PowerAction = 7
)

func (a PowerAction) String() string {
	switch a {
	case PowerActionForceOff:
		return "ForceOff"
	case PowerActionPowerCycle:
		return "PowerCycle"
	case PowerActionGracefulShutdown:
		return "GracefulShutdown"
	case PowerActionOn:
		return "On"
	case PowerActionForceOn:
		return "ForceOn"
	case PowerActionGracefulRestart:
		return "GracefulRestart"
	case PowerActionForceRestart:
		return "ForceRestart"
	default:
		return "Unknown"
	}
}

func powerActionToPb(a PowerAction) pb.PowerAction {
	switch a {
	case PowerActionForceOff:
		return pb.PowerAction_POWER_ACTION_FORCE_OFF
	case PowerActionPowerCycle:
		return pb.PowerAction_POWER_ACTION_POWER_CYCLE
	case PowerActionGracefulShutdown:
		return pb.PowerAction_POWER_ACTION_GRACEFUL_SHUTDOWN
	case PowerActionOn:
		return pb.PowerAction_POWER_ACTION_ON
	case PowerActionForceOn:
		return pb.PowerAction_POWER_ACTION_FORCE_ON
	case PowerActionGracefulRestart:
		return pb.PowerAction_POWER_ACTION_GRACEFUL_RESTART
	case PowerActionForceRestart:
		return pb.PowerAction_POWER_ACTION_FORCE_RESTART
	default:
		return pb.PowerAction_POWER_ACTION_UNKNOWN
	}
}

// NVSwitchComponent enumerates the updatable components of an NV-Switch tray.
type NVSwitchComponent int

const (
	NVSwitchComponentUnknown NVSwitchComponent = 0
	NVSwitchComponentBMC     NVSwitchComponent = 1
	NVSwitchComponentCPLD    NVSwitchComponent = 2
	NVSwitchComponentBIOS    NVSwitchComponent = 3
	NVSwitchComponentNVOS    NVSwitchComponent = 4
)

func (c NVSwitchComponent) String() string {
	switch c {
	case NVSwitchComponentBMC:
		return "BMC"
	case NVSwitchComponentCPLD:
		return "CPLD"
	case NVSwitchComponentBIOS:
		return "BIOS"
	case NVSwitchComponentNVOS:
		return "NVOS"
	default:
		return fmt.Sprintf("Unknown(%d)", int(c))
	}
}

func nvSwitchComponentToPb(c NVSwitchComponent) pb.NVSwitchComponent {
	switch c {
	case NVSwitchComponentBMC:
		return pb.NVSwitchComponent_NVSWITCH_COMPONENT_BMC
	case NVSwitchComponentCPLD:
		return pb.NVSwitchComponent_NVSWITCH_COMPONENT_CPLD
	case NVSwitchComponentBIOS:
		return pb.NVSwitchComponent_NVSWITCH_COMPONENT_BIOS
	case NVSwitchComponentNVOS:
		return pb.NVSwitchComponent_NVSWITCH_COMPONENT_NVOS
	default:
		return pb.NVSwitchComponent_NVSWITCH_COMPONENT_UNKNOWN
	}
}

// StatusCode represents the result of an operation.
type StatusCode int

const (
	StatusSuccess         StatusCode = 0
	StatusInvalidArgument StatusCode = 1
	StatusInternalError   StatusCode = 2
)

func statusCodeFromPb(sc pb.StatusCode) StatusCode {
	switch sc {
	case pb.StatusCode_SUCCESS:
		return StatusSuccess
	case pb.StatusCode_INVALID_ARGUMENT:
		return StatusInvalidArgument
	case pb.StatusCode_INTERNAL_ERROR:
		return StatusInternalError
	default:
		return StatusInternalError
	}
}

// PowerControlResult contains the result of a power control operation for a single switch.
type PowerControlResult struct {
	UUID   string
	Status StatusCode
	Error  string
}

func powerControlResultFromPb(resp *pb.NVSwitchResponse) PowerControlResult {
	return PowerControlResult{
		UUID:   resp.GetUuid(),
		Status: statusCodeFromPb(resp.GetStatus()),
		Error:  resp.GetError(),
	}
}

// UpdateState represents the granular state of a firmware update.
type UpdateState int

const (
	UpdateStateUnknown        UpdateState = 0
	UpdateStateQueued         UpdateState = 1
	UpdateStatePowerCycle     UpdateState = 2
	UpdateStateWaitReachable  UpdateState = 3
	UpdateStateCopy           UpdateState = 4
	UpdateStateUpload         UpdateState = 5
	UpdateStateInstall        UpdateState = 6
	UpdateStatePollCompletion UpdateState = 7
	UpdateStateVerify         UpdateState = 8
	UpdateStateCleanup        UpdateState = 9
	UpdateStateCompleted      UpdateState = 10
	UpdateStateFailed         UpdateState = 11
	UpdateStateCancelled      UpdateState = 12
)

func (s UpdateState) IsTerminal() bool {
	return s == UpdateStateCompleted || s == UpdateStateFailed || s == UpdateStateCancelled
}

func updateStateFromPb(s pb.UpdateState) UpdateState {
	return UpdateState(s)
}

// FirmwareUpdateInfo contains full details of a firmware update operation.
type FirmwareUpdateInfo struct {
	ID             string
	SwitchUUID     string
	Component      NVSwitchComponent
	BundleVersion  string
	State          UpdateState
	VersionFrom    string
	VersionTo      string
	VersionActual  string
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	BundleUpdateID string
	SequenceOrder  int32
	PredecessorID  string
}

func firmwareUpdateInfoFromPb(info *pb.FirmwareUpdateInfo) FirmwareUpdateInfo {
	result := FirmwareUpdateInfo{
		ID:             info.GetId(),
		SwitchUUID:     info.GetSwitchUuid(),
		Component:      NVSwitchComponent(info.GetComponent()),
		BundleVersion:  info.GetBundleVersion(),
		State:          updateStateFromPb(info.GetState()),
		VersionFrom:    info.GetVersionFrom(),
		VersionTo:      info.GetVersionTo(),
		VersionActual:  info.GetVersionActual(),
		ErrorMessage:   info.GetErrorMessage(),
		BundleUpdateID: info.GetBundleUpdateId(),
		SequenceOrder:  info.GetSequenceOrder(),
		PredecessorID:  info.GetPredecessorId(),
	}
	if info.GetCreatedAt() != nil {
		result.CreatedAt = info.GetCreatedAt().AsTime()
	}
	if info.GetUpdatedAt() != nil {
		result.UpdatedAt = info.GetUpdatedAt().AsTime()
	}
	return result
}

// ComponentInfo describes a component within a firmware bundle.
type ComponentInfo struct {
	Name     string
	Version  string
	Strategy string
}

// FirmwareBundle represents a firmware package with multiple components.
type FirmwareBundle struct {
	Version     string
	Description string
	Components  []ComponentInfo
}

func firmwareBundleFromPb(bundle *pb.FirmwareBundle) FirmwareBundle {
	result := FirmwareBundle{
		Version:     bundle.GetVersion(),
		Description: bundle.GetDescription(),
	}
	for _, comp := range bundle.GetComponents() {
		result.Components = append(result.Components, ComponentInfo{
			Name:     comp.GetName(),
			Version:  comp.GetVersion(),
			Strategy: comp.GetStrategy(),
		})
	}
	return result
}
