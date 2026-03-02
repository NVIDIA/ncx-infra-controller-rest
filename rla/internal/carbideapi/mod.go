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

// Package carbideapi abstracts the GRPC interface used to communicate with carbide-api.  New connection pools can be created with
// NewClient to create a real client or NewMockClient which fakes everything for unit tests.

package carbideapi

import (
	"context"
	"time"
)

// Client allow us to have both a real implemenation and a mock implementation for unit tests which can be switched transparently
type Client interface {
	Version(ctx context.Context) (string, error)
	GetMachines(ctx context.Context) ([]Machine, error)
	GetPowerStates(ctx context.Context, machineIds []string) (ret []MachinePowerState, err error)
	SetFirmwareUpdateTimeWindow(ctx context.Context, machineIds []string, startTime, endTime time.Time) error
	// FindInterfaces returns all machine interfaces known by carbide-api, keyed by MAC address
	FindInterfaces(ctx context.Context) (map[string]MachineInterface, error)

	// AdminPowerControl performs power control operations on a machine
	AdminPowerControl(ctx context.Context, machineID string, action SystemPowerControl) error

	// UpdatePowerOption sets the desired power state for a machine in Carbide's power manager.
	// This controls Carbide's power-on gate: setting desired state to On allows
	// the machine to power on, while Off or Disabled prevents it.
	UpdatePowerOption(ctx context.Context, machineID string, desiredState PowerState) error

	// FindMachinesByIds returns detailed machine information for the given machine IDs
	FindMachinesByIds(ctx context.Context, machineIds []string) ([]MachineDetail, error)

	// GetMachinePositionInfo returns position information for the given machine IDs
	GetMachinePositionInfo(ctx context.Context, machineIds []string) ([]MachinePosition, error)

	// AllowIngestionAndPowerOn opens Carbide's power-on gate for a BMC endpoint,
	// allowing the machine to be ingested and powered on.
	AllowIngestionAndPowerOn(ctx context.Context, bmcIP string, bmcMAC string) error

	// DetermineMachineIngestionState returns the bring-up state of a machine
	// relative to Carbide's power-on gate.
	DetermineMachineIngestionState(ctx context.Context, bmcIP string, bmcMAC string) (BringUpState, error) //nolint

	// The following are only valid in the mock environment and should only be called by unit tests
	AddMachine(Machine)
	AddPowerState(machineID string, state PowerState)
	SetFirmwareUpdateTimeWindowError(err error)
	SetAdminPowerControlError(err error)
	AddMachineInterface(iface MachineInterface)
}
