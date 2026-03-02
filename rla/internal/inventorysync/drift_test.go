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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/carbideapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db/model"
)

// ptr is a generic helper that returns a pointer to the given value.
// Useful for constructing test structs with pointer fields (e.g. *int32, *string).
func ptr[T any](v T) *T { return &v }

func TestCompareMachineFieldsForDrift_NoMismatch(t *testing.T) {
	expected := &model.Component{
		SerialNumber:    "SN001",
		FirmwareVersion: "1.0.0",
		SlotID:          2,
		TrayIndex:       1,
		HostID:          5,
	}
	actual := carbideapi.MachineDetail{
		ChassisSerial:   ptr("SN001"),
		FirmwareVersion: "1.0.0",
	}
	position := carbideapi.MachinePosition{
		PhysicalSlotNum:  ptr(int32(2)),
		ComputeTrayIndex: ptr(int32(1)),
		TopologyID:       ptr(int32(5)),
	}

	diffs := compareMachineFieldsForDrift(expected, actual, position)
	assert.Empty(t, diffs)
}

func TestCompareMachineFieldsForDrift_AllFieldsMismatch(t *testing.T) {
	expected := &model.Component{
		SerialNumber:    "SN001",
		FirmwareVersion: "1.0.0",
		SlotID:          2,
		TrayIndex:       1,
		HostID:          5,
	}
	actual := carbideapi.MachineDetail{
		ChassisSerial:   ptr("SN999"),
		FirmwareVersion: "2.0.0",
	}
	position := carbideapi.MachinePosition{
		PhysicalSlotNum:  ptr(int32(10)),
		ComputeTrayIndex: ptr(int32(3)),
		TopologyID:       ptr(int32(7)),
	}

	diffs := compareMachineFieldsForDrift(expected, actual, position)
	assert.Len(t, diffs, 5)

	diffByField := make(map[string]model.FieldDiff)
	for _, d := range diffs {
		diffByField[d.FieldName] = d
	}

	assert.Equal(t, "2", diffByField["slot_id"].ExpectedValue)
	assert.Equal(t, "10", diffByField["slot_id"].ActualValue)

	assert.Equal(t, "1", diffByField["tray_index"].ExpectedValue)
	assert.Equal(t, "3", diffByField["tray_index"].ActualValue)

	assert.Equal(t, "5", diffByField["host_id"].ExpectedValue)
	assert.Equal(t, "7", diffByField["host_id"].ActualValue)

	assert.Equal(t, "1.0.0", diffByField["firmware_version"].ExpectedValue)
	assert.Equal(t, "2.0.0", diffByField["firmware_version"].ActualValue)

	assert.Equal(t, "SN001", diffByField["serial_number"].ExpectedValue)
	assert.Equal(t, "SN999", diffByField["serial_number"].ActualValue)
}

func TestCompareMachineFieldsForDrift_NilPositionFieldsSkipped(t *testing.T) {
	expected := &model.Component{
		SerialNumber:    "SN001",
		FirmwareVersion: "1.0.0",
		SlotID:          2,
		TrayIndex:       1,
		HostID:          5,
	}
	actual := carbideapi.MachineDetail{
		ChassisSerial:   ptr("SN001"),
		FirmwareVersion: "1.0.0",
	}
	// All position fields nil — should not produce diffs even if expected values differ
	position := carbideapi.MachinePosition{}

	diffs := compareMachineFieldsForDrift(expected, actual, position)
	assert.Empty(t, diffs)
}

func TestCompareMachineFieldsForDrift_EmptyActualFirmwareSkipped(t *testing.T) {
	expected := &model.Component{
		FirmwareVersion: "1.0.0",
	}
	// Empty firmware version from Carbide — should not flag as mismatch
	actual := carbideapi.MachineDetail{
		FirmwareVersion: "",
	}
	position := carbideapi.MachinePosition{}

	diffs := compareMachineFieldsForDrift(expected, actual, position)
	assert.Empty(t, diffs)
}

func TestCompareMachineFieldsForDrift_NilChassisSerialSkipped(t *testing.T) {
	expected := &model.Component{
		SerialNumber: "SN001",
	}
	// Nil ChassisSerial from Carbide — should not flag as mismatch
	actual := carbideapi.MachineDetail{
		ChassisSerial: nil,
	}
	position := carbideapi.MachinePosition{}

	diffs := compareMachineFieldsForDrift(expected, actual, position)
	assert.Empty(t, diffs)
}

func TestCompareMachineFieldsForDrift_PartialMismatch(t *testing.T) {
	expected := &model.Component{
		SerialNumber:    "SN001",
		FirmwareVersion: "1.0.0",
		SlotID:          2,
		TrayIndex:       1,
		HostID:          5,
	}
	actual := carbideapi.MachineDetail{
		ChassisSerial:   ptr("SN001"), // match
		FirmwareVersion: "2.0.0",      // mismatch
	}
	position := carbideapi.MachinePosition{
		PhysicalSlotNum:  ptr(int32(2)), // match
		ComputeTrayIndex: ptr(int32(1)), // match
		TopologyID:       ptr(int32(9)), // mismatch
	}

	diffs := compareMachineFieldsForDrift(expected, actual, position)
	assert.Len(t, diffs, 2)

	diffByField := make(map[string]model.FieldDiff)
	for _, d := range diffs {
		diffByField[d.FieldName] = d
	}

	assert.Contains(t, diffByField, "firmware_version")
	assert.Contains(t, diffByField, "host_id")
	assert.NotContains(t, diffByField, "slot_id")
	assert.NotContains(t, diffByField, "tray_index")
	assert.NotContains(t, diffByField, "serial_number")
}

func TestCompareMachineFieldsForDrift_ExpectedEmptyFirmwareActualHasValue(t *testing.T) {
	expected := &model.Component{
		FirmwareVersion: "", // empty in DB
	}
	actual := carbideapi.MachineDetail{
		FirmwareVersion: "2.0.0", // Carbide has value
	}
	position := carbideapi.MachinePosition{}

	diffs := compareMachineFieldsForDrift(expected, actual, position)
	assert.Len(t, diffs, 1)
	assert.Equal(t, "firmware_version", diffs[0].FieldName)
	assert.Equal(t, "", diffs[0].ExpectedValue)
	assert.Equal(t, "2.0.0", diffs[0].ActualValue)
}
