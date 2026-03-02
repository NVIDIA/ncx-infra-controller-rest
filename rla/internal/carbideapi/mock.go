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

package carbideapi

import (
	"context"
	"time"
)

type mockClient struct {
	machines                    map[string]Machine
	powerStates                 map[string]PowerState
	machineInterfaces           map[string]MachineInterface
	firmwareUpdateTimeWindowErr error // If set, SetFirmwareUpdateTimeWindow will return this error
	adminPowerControlErr        error // If set, AdminPowerControl will return this error
}

// NewMockClient returns a "GRPC" client that returns mock values so it can be used in unit tests.
func NewMockClient() Client {
	return &mockClient{
		machines:          map[string]Machine{},
		powerStates:       map[string]PowerState{},
		machineInterfaces: map[string]MachineInterface{},
	}
}

func (c *mockClient) Version(ctx context.Context) (string, error) {
	return "1.2.3", nil
}

func (c *mockClient) GetMachines(ctx context.Context) (ret []Machine, err error) {
	for _, machine := range c.machines {
		ret = append(ret, machine)
	}

	return ret, nil
}

func (c *mockClient) GetPowerStates(ctx context.Context, machineIds []string) (ret []MachinePowerState, err error) {
	for _, cur := range machineIds {
		if state, ok := c.powerStates[cur]; ok {
			ret = append(ret, MachinePowerState{MachineID: cur, PowerState: state})
		}
	}

	return ret, nil
}

func (c *mockClient) SetFirmwareUpdateTimeWindow(ctx context.Context, machineIds []string, startTime, endTime time.Time) error {
	return c.firmwareUpdateTimeWindowErr
}

func (c *mockClient) AdminPowerControl(ctx context.Context, machineID string, action SystemPowerControl) error {
	return c.adminPowerControlErr
}

func (c *mockClient) UpdatePowerOption(ctx context.Context, machineID string, desiredState PowerState) error {
	return nil
}

func (c *mockClient) AddMachine(machine Machine) {
	c.machines[machine.MachineID] = machine
}

func (c *mockClient) AddPowerState(machineID string, state PowerState) {
	c.powerStates[machineID] = state
}

func (c *mockClient) SetFirmwareUpdateTimeWindowError(err error) {
	c.firmwareUpdateTimeWindowErr = err
}

func (c *mockClient) SetAdminPowerControlError(err error) {
	c.adminPowerControlErr = err
}

func (c *mockClient) FindInterfaces(ctx context.Context) (map[string]MachineInterface, error) {
	// Return a copy of the map
	interfaces := make(map[string]MachineInterface)
	for mac, iface := range c.machineInterfaces {
		interfaces[mac] = iface
	}
	return interfaces, nil
}

func (c *mockClient) AddMachineInterface(iface MachineInterface) {
	c.machineInterfaces[iface.MacAddress] = iface
}

func (c *mockClient) FindMachinesByIds(ctx context.Context, machineIds []string) ([]MachineDetail, error) {
	// Mock implementation returns empty for now
	return nil, nil
}

func (c *mockClient) GetMachinePositionInfo(ctx context.Context, machineIds []string) ([]MachinePosition, error) {
	// Mock implementation returns empty for now
	return nil, nil
}

func (c *mockClient) AllowIngestionAndPowerOn(
	ctx context.Context,
	bmcIP string,
	bmcMAC string,
) error {
	return nil
}

func (c *mockClient) DetermineMachineIngestionState(
	ctx context.Context,
	bmcIP string,
	bmcMAC string,
) (BringUpState, error) {
	return BringUpStateMachineCreated, nil
}
