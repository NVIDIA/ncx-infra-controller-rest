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

package inventorysync

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/carbideapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/common/utils"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/config"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db/model"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/psmapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

// TestInventory is the main test for the inventory package
func TestInventory(t *testing.T) {
	ctx := context.Background()

	if os.Getenv("DB_PORT") == "" {
		log.Warn().Msgf("Not running unit test due to no DB environment specified")
		t.SkipNow()
	}

	// Reset package-level state to avoid cross-test pollution
	lastUpdateMachineIDs = time.Time{}

	dbConf, err := db.BuildDBConfigFromEnv()
	assert.Nil(t, err)
	pool, err := utils.UnitTestDB(ctx, t, dbConf)
	assert.Nil(t, err)

	cfg := config.UnitTestConfig()

	grpcMock := carbideapi.NewMockClient()

	// Create a basic faked GRPC environment
	serial1 := "serial1"
	serial2 := "serial2"
	serial3 := "serial3"
	grpcMock.AddMachine(carbideapi.Machine{MachineID: "id1", ChassisSerial: &serial1})
	grpcMock.AddMachine(carbideapi.Machine{MachineID: "id2", ChassisSerial: &serial2})
	grpcMock.AddMachine(carbideapi.Machine{MachineID: "id3", ChassisSerial: &serial3})
	grpcMock.AddMachine(carbideapi.Machine{MachineID: "id4", ChassisSerial: nil})
	grpcMock.AddPowerState("id2", carbideapi.PowerStateOn)

	// Create a rack (required for components due to NOT NULL constraint)
	rack := model.Rack{
		Name:         "test-rack",
		Manufacturer: "TestMfg",
		SerialNumber: "rack-serial-001",
	}
	err = rack.Create(ctx, pool.DB())
	assert.Nil(t, err)

	// Create components with required fields (manufacturer and rack_id are NOT NULL)
	c := model.Component{SerialNumber: "serial2", Manufacturer: "TestMfg", RackID: rack.ID}
	err = c.Create(ctx, pool.DB())
	assert.Nil(t, err)
	c = model.Component{SerialNumber: "serial4", Manufacturer: "TestMfg2", RackID: rack.ID}
	err = c.Create(ctx, pool.DB())
	assert.Nil(t, err)

	psmMock := psmapi.NewMockClient()
	runInventoryOne(ctx, &cfg, pool, grpcMock, psmMock)

	rows, err := pool.DB().Query("SELECT serial_number, power_state FROM component;")
	assert.NotNil(t, rows)
	assert.Nil(t, err)
	defer rows.Close()

	var found int
	for rows.Next() {
		var serial string
		var state *carbideapi.PowerState
		rows.Scan(&serial, &state)

		switch serial {
		case "serial2":
			assert.Equal(t, *state, carbideapi.PowerStateOn)
			found++
		case "serial4":
			assert.Nil(t, state)
			found++
		default:
			panic(fmt.Sprintf("Invalid row found: %v %v", serial, state))
		}
	}
	assert.Equal(t, 2, found)
}

// TestHandleExpectedPowershelves tests that expected powershelves are registered with PSM
func TestHandleExpectedPowershelves(t *testing.T) {
	ctx := context.Background()

	if os.Getenv("DB_PORT") == "" {
		log.Warn().Msgf("Not running unit test due to no DB environment specified")
		t.SkipNow()
	}

	// Reset package-level state to avoid cross-test pollution
	lastUpdateMachineIDs = time.Time{}

	dbConf, err := db.BuildDBConfigFromEnv()
	assert.Nil(t, err)
	pool, err := utils.UnitTestDB(ctx, t, dbConf)
	assert.Nil(t, err)

	// Create mock clients
	carbideMock := carbideapi.NewMockClient()
	psmMock := psmapi.NewMockClient()

	// Create a rack (required for components)
	rack := model.Rack{
		Name:         "test-rack",
		Manufacturer: "TestMfg",
		SerialNumber: "rack-serial-001",
	}
	err = rack.Create(ctx, pool.DB())
	assert.Nil(t, err)

	// Create PowerShelf components with PMCs (BMCs)
	// PMC 1: Expected powershelf with DHCPed interface
	ps1 := model.Component{
		Name:         "powershelf-1",
		Type:         devicetypes.ComponentTypePowerShelf.String(),
		Manufacturer: "LiteonMfg",
		SerialNumber: "ps-serial-001",
		RackID:       rack.ID,
	}
	err = ps1.Create(ctx, pool.DB())
	assert.Nil(t, err)

	// Add PMC (BMC) for powershelf 1
	pmc1 := model.BMC{
		MacAddress:  "aa:bb:cc:dd:ee:01",
		Type:        devicetypes.BMCTypeHost.String(),
		ComponentID: ps1.ID,
	}
	err = pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return pmc1.Create(ctx, tx)
	})
	assert.Nil(t, err)

	// PMC 2: Expected powershelf with DHCPed interface
	ps2 := model.Component{
		Name:         "powershelf-2",
		Type:         devicetypes.ComponentTypePowerShelf.String(),
		Manufacturer: "LiteonMfg",
		SerialNumber: "ps-serial-002",
		RackID:       rack.ID,
	}
	err = ps2.Create(ctx, pool.DB())
	assert.Nil(t, err)

	pmc2 := model.BMC{
		MacAddress:  "aa:bb:cc:dd:ee:02",
		Type:        devicetypes.BMCTypeHost.String(),
		ComponentID: ps2.ID,
	}
	err = pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return pmc2.Create(ctx, tx)
	})
	assert.Nil(t, err)

	// PMC 3: Expected powershelf but NOT DHCPed (no interface in carbide)
	ps3 := model.Component{
		Name:         "powershelf-3",
		Type:         devicetypes.ComponentTypePowerShelf.String(),
		Manufacturer: "LiteonMfg",
		SerialNumber: "ps-serial-003",
		RackID:       rack.ID,
	}
	err = ps3.Create(ctx, pool.DB())
	assert.Nil(t, err)

	pmc3 := model.BMC{
		MacAddress:  "aa:bb:cc:dd:ee:03",
		Type:        devicetypes.BMCTypeHost.String(),
		ComponentID: ps3.ID,
	}
	err = pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return pmc3.Create(ctx, tx)
	})
	assert.Nil(t, err)

	// PMC 4: Already registered powershelf - should update firmware version and power state
	ps4 := model.Component{
		Name:            "powershelf-4",
		Type:            devicetypes.ComponentTypePowerShelf.String(),
		Manufacturer:    "LiteonMfg",
		SerialNumber:    "ps-serial-004",
		RackID:          rack.ID,
		FirmwareVersion: "1.0.0", // Old firmware version
	}
	err = ps4.Create(ctx, pool.DB())
	assert.Nil(t, err)

	pmc4 := model.BMC{
		MacAddress:  "aa:bb:cc:dd:ee:04",
		Type:        devicetypes.BMCTypeHost.String(),
		ComponentID: ps4.ID,
	}
	err = pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return pmc4.Create(ctx, tx)
	})
	assert.Nil(t, err)

	// Pre-register PMC 4 in PSM with updated firmware version and PSUs
	psmMock.AddPowershelf(psmapi.PowerShelf{
		PMC: psmapi.PowerManagementController{
			MACAddress:      "aa:bb:cc:dd:ee:04",
			IPAddress:       "10.0.0.104",
			FirmwareVersion: "2.0.0", // New firmware version
		},
		PSUs: []psmapi.PowerSupplyUnit{
			{PowerState: true}, // On
			{PowerState: true}, // On
		},
	})

	// PMC 5: Already registered powershelf with mixed PSU power states (should be Unknown)
	ps5 := model.Component{
		Name:            "powershelf-5",
		Type:            devicetypes.ComponentTypePowerShelf.String(),
		Manufacturer:    "LiteonMfg",
		SerialNumber:    "ps-serial-005",
		RackID:          rack.ID,
		FirmwareVersion: "1.0.0",
	}
	err = ps5.Create(ctx, pool.DB())
	assert.Nil(t, err)

	pmc5 := model.BMC{
		MacAddress:  "aa:bb:cc:dd:ee:05",
		Type:        devicetypes.BMCTypeHost.String(),
		ComponentID: ps5.ID,
	}
	err = pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return pmc5.Create(ctx, tx)
	})
	assert.Nil(t, err)

	// Pre-register PMC 5 in PSM with mixed PSU power states
	psmMock.AddPowershelf(psmapi.PowerShelf{
		PMC: psmapi.PowerManagementController{
			MACAddress:      "aa:bb:cc:dd:ee:05",
			IPAddress:       "10.0.0.105",
			FirmwareVersion: "2.1.0",
		},
		PSUs: []psmapi.PowerSupplyUnit{
			{PowerState: true},  // On
			{PowerState: false}, // Off - mixed state
		},
	})

	// PMC 6: Already registered powershelf with all PSUs off (should be PowerStateOff)
	ps6 := model.Component{
		Name:            "powershelf-6",
		Type:            devicetypes.ComponentTypePowerShelf.String(),
		Manufacturer:    "LiteonMfg",
		SerialNumber:    "ps-serial-006",
		RackID:          rack.ID,
		FirmwareVersion: "1.0.0",
	}
	err = ps6.Create(ctx, pool.DB())
	assert.Nil(t, err)

	pmc6 := model.BMC{
		MacAddress:  "aa:bb:cc:dd:ee:06",
		Type:        devicetypes.BMCTypeHost.String(),
		ComponentID: ps6.ID,
	}
	err = pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return pmc6.Create(ctx, tx)
	})
	assert.Nil(t, err)

	// Pre-register PMC 6 in PSM with all PSUs off
	psmMock.AddPowershelf(psmapi.PowerShelf{
		PMC: psmapi.PowerManagementController{
			MACAddress:      "aa:bb:cc:dd:ee:06",
			IPAddress:       "10.0.0.106",
			FirmwareVersion: "2.2.0",
		},
		PSUs: []psmapi.PowerSupplyUnit{
			{PowerState: false}, // Off
			{PowerState: false}, // Off
		},
	})

	// PMC 7: DHCPed but with multiple IP addresses (should skip registration)
	ps7 := model.Component{
		Name:         "powershelf-7",
		Type:         devicetypes.ComponentTypePowerShelf.String(),
		Manufacturer: "LiteonMfg",
		SerialNumber: "ps-serial-007",
		RackID:       rack.ID,
	}
	err = ps7.Create(ctx, pool.DB())
	assert.Nil(t, err)

	pmc7 := model.BMC{
		MacAddress:  "aa:bb:cc:dd:ee:07",
		Type:        devicetypes.BMCTypeHost.String(),
		ComponentID: ps7.ID,
	}
	err = pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return pmc7.Create(ctx, tx)
	})
	assert.Nil(t, err)

	// Add machine interfaces to carbide mock
	// PMC 1 has DHCPed
	carbideMock.AddMachineInterface(carbideapi.MachineInterface{
		MacAddress: "aa:bb:cc:dd:ee:01",
		Addresses:  []string{"10.0.0.101"},
	})

	// PMC 2 has DHCPed
	carbideMock.AddMachineInterface(carbideapi.MachineInterface{
		MacAddress: "aa:bb:cc:dd:ee:02",
		Addresses:  []string{"10.0.0.102"},
	})

	// Random BMC interfaces (not expected powershelves)
	carbideMock.AddMachineInterface(carbideapi.MachineInterface{
		MacAddress: "ff:ff:ff:ff:ff:01",
		Addresses:  []string{"10.0.0.201"},
	})
	carbideMock.AddMachineInterface(carbideapi.MachineInterface{
		MacAddress: "ff:ff:ff:ff:ff:02",
		Addresses:  []string{"10.0.0.202"},
	})

	// PMC 7 has multiple IP addresses (unexpected, should skip registration)
	carbideMock.AddMachineInterface(carbideapi.MachineInterface{
		MacAddress: "aa:bb:cc:dd:ee:07",
		Addresses:  []string{"10.0.0.107", "10.0.0.108"}, // Multiple IPs
	})

	// PMC 3 is NOT in carbide (hasn't DHCPed)

	// Verify we have 3 pre-registered powershelves before running inventory
	preRegistered, err := psmMock.GetPowershelves(ctx, []string{})
	assert.Nil(t, err)
	assert.Equal(t, 3, len(preRegistered), "Should have 3 pre-registered powershelves (PMC 4, 5, 6)")

	// Run the inventory loop
	cfg := config.UnitTestConfig()
	runInventoryOne(ctx, &cfg, pool, carbideMock, psmMock)

	// Verify that only expected PMCs that have DHCPed were registered with PSM
	registeredPowershelves, err := psmMock.GetPowershelves(ctx, []string{})
	assert.Nil(t, err)

	// Should have 5 total powershelves: PMC 4, 5, 6 (pre-registered) + PMC 1, 2 (newly registered)
	// PMC 3 (not DHCPed), PMC 7 (multiple IPs), and the random ones should NOT be registered
	assert.Equal(t, 5, len(registeredPowershelves))

	// Build a map for easier verification
	registeredByMac := make(map[string]psmapi.PowerShelf)
	for _, ps := range registeredPowershelves {
		registeredByMac[ps.PMC.MACAddress] = ps
	}

	// Verify PMC 1 was registered with correct IP
	ps1Registered, ok := registeredByMac["aa:bb:cc:dd:ee:01"]
	assert.True(t, ok, "PMC 1 should be registered")
	assert.Equal(t, "10.0.0.101", ps1Registered.PMC.IPAddress)

	// Verify PMC 2 was registered with correct IP
	ps2Registered, ok := registeredByMac["aa:bb:cc:dd:ee:02"]
	assert.True(t, ok, "PMC 2 should be registered")
	assert.Equal(t, "10.0.0.102", ps2Registered.PMC.IPAddress)

	// Verify PMC 3 was NOT registered (not DHCPed)
	_, ok = registeredByMac["aa:bb:cc:dd:ee:03"]
	assert.False(t, ok, "PMC 3 should NOT be registered (not DHCPed)")

	// Verify random BMCs were NOT registered
	_, ok = registeredByMac["ff:ff:ff:ff:ff:01"]
	assert.False(t, ok, "Random BMC 1 should NOT be registered")
	_, ok = registeredByMac["ff:ff:ff:ff:ff:02"]
	assert.False(t, ok, "Random BMC 2 should NOT be registered")

	// Verify power state and firmware version updates in the component table
	// PMC 4: Should have updated firmware version and PowerStateOn (all PSUs on)
	var updatedPs4 model.Component
	err = pool.DB().NewSelect().Model(&updatedPs4).Where("id = ?", ps4.ID).Scan(ctx)
	assert.Nil(t, err)
	assert.Equal(t, "2.0.0", updatedPs4.FirmwareVersion, "PMC 4 firmware version should be updated")
	assert.NotNil(t, updatedPs4.PowerState, "PMC 4 power state should be set")
	assert.Equal(t, carbideapi.PowerStateOn, *updatedPs4.PowerState, "PMC 4 should be PowerStateOn (all PSUs on)")

	// PMC 5: Should have updated firmware version and PowerStateUnknown (mixed PSU states)
	var updatedPs5 model.Component
	err = pool.DB().NewSelect().Model(&updatedPs5).Where("id = ?", ps5.ID).Scan(ctx)
	assert.Nil(t, err)
	assert.Equal(t, "2.1.0", updatedPs5.FirmwareVersion, "PMC 5 firmware version should be updated")
	assert.NotNil(t, updatedPs5.PowerState, "PMC 5 power state should be set")
	assert.Equal(t, carbideapi.PowerStateUnknown, *updatedPs5.PowerState, "PMC 5 should be PowerStateUnknown (mixed PSU states)")

	// PMC 6: Should have updated firmware version and PowerStateOff (all PSUs off)
	var updatedPs6 model.Component
	err = pool.DB().NewSelect().Model(&updatedPs6).Where("id = ?", ps6.ID).Scan(ctx)
	assert.Nil(t, err)
	assert.Equal(t, "2.2.0", updatedPs6.FirmwareVersion, "PMC 6 firmware version should be updated")
	assert.NotNil(t, updatedPs6.PowerState, "PMC 6 power state should be set")
	assert.Equal(t, carbideapi.PowerStateOff, *updatedPs6.PowerState, "PMC 6 should be PowerStateOff (all PSUs off)")

	// PMC 7: Should NOT be registered (multiple IP addresses)
	_, ok = registeredByMac["aa:bb:cc:dd:ee:07"]
	assert.False(t, ok, "PMC 7 should NOT be registered (multiple IP addresses)")
}
