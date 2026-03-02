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

package protobuf

import (
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	dbquery "github.com/nvidia/bare-metal-manager-rest/rla/internal/db/query"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/inventoryobjects/bmc"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/inventoryobjects/component"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/inventoryobjects/rack"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/credential"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/deviceinfo"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/location"
	pb "github.com/nvidia/bare-metal-manager-rest/rla/pkg/proto/v1"
)

func TestUUIDFrom(t *testing.T) {
	testID := uuid.New()
	testCases := map[string]struct {
		id       *pb.UUID
		expected uuid.UUID
	}{
		"valid protobuf uuid": {
			id:       &pb.UUID{Id: testID.String()},
			expected: testID,
		},
		"nil protobuf uuid": {
			id:       nil,
			expected: uuid.Nil,
		},
		"invalid protobuf uuid": {
			id:       &pb.UUID{Id: "invalid-uuid"},
			expected: uuid.Nil,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, UUIDFrom(testCase.id))
		})
	}
}

func TestComponentTypeConverter(t *testing.T) {
	for typ, ptype := range componentTypeToMap {
		assert.Equal(t, typ, ComponentTypeFrom(ptype))
		assert.Equal(t, ptype, ComponentTypeTo(typ))
	}

	assert.Equal(t, devicetypes.ComponentTypeUnknown, ComponentTypeFrom(pb.ComponentType(-1)))               //nolint
	assert.Equal(t, pb.ComponentType_COMPONENT_TYPE_UNKNOWN, ComponentTypeTo(devicetypes.ComponentType(-1))) //nolint
}

func TestBMCTypeConverter(t *testing.T) {
	for typ, ptype := range bmcTypeToMap {
		assert.Equal(t, typ, BMCTypeFrom(ptype))
		assert.Equal(t, ptype, BMCTypeTo(typ))
	}

	assert.Equal(t, devicetypes.BMCTypeUnknown, BMCTypeFrom(pb.BMCType(-1)))         //nolint
	assert.Equal(t, pb.BMCType_BMC_TYPE_UNKNOWN, BMCTypeTo(devicetypes.BMCType(-1))) //nolint
}

func TestDeviceInfoConverter(t *testing.T) {
	shared := deviceinfo.NewRandom("some device", 5)

	sharedP := pb.DeviceInfo{
		Id:           &pb.UUID{Id: shared.ID.String()},
		Name:         shared.Name,
		Manufacturer: shared.Manufacturer,
		Model:        &shared.Model,
		SerialNumber: shared.SerialNumber,
		Description:  &shared.Description,
	}

	testCases := map[string]struct {
		source     *deviceinfo.DeviceInfo
		sourceP    *pb.DeviceInfo
		converted  *deviceinfo.DeviceInfo
		convertedP *pb.DeviceInfo
	}{
		"valid": {
			source:     &shared,
			sourceP:    &sharedP,
			converted:  &shared,
			convertedP: &sharedP,
		},
		"nil": {
			source:     nil,
			sourceP:    nil,
			converted:  &deviceinfo.DeviceInfo{},
			convertedP: nil,
		},
		"empty fields": {
			source: &deviceinfo.DeviceInfo{
				ID:           uuid.Nil,
				Name:         shared.Name,
				Manufacturer: shared.Manufacturer,
				Model:        "",
				SerialNumber: shared.SerialNumber,
				Description:  "",
			},
			sourceP: &pb.DeviceInfo{
				Id:           nil,
				Name:         sharedP.Name,
				Manufacturer: sharedP.Manufacturer,
				SerialNumber: sharedP.SerialNumber,
			},
			converted: &deviceinfo.DeviceInfo{
				ID:           uuid.Nil,
				Name:         shared.Name,
				Manufacturer: shared.Manufacturer,
				Model:        "",
				SerialNumber: shared.SerialNumber,
				Description:  "",
			},
			convertedP: &pb.DeviceInfo{
				Id:           nil,
				Name:         sharedP.Name,
				Manufacturer: sharedP.Manufacturer,
				SerialNumber: sharedP.SerialNumber,
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, *testCase.converted, DeviceInfoFrom(testCase.sourceP))
			assert.Equal(t, testCase.convertedP, DeviceInfoTo(testCase.source))
		})
	}
}

func TestLocationConverter(t *testing.T) {
	shared := location.Location{
		Region:     "US",
		DataCenter: "DC1",
		Room:       "Room1",
		Position:   "Pos1",
	}

	sharedP := pb.Location{
		Region:     shared.Region,
		Datacenter: shared.DataCenter,
		Room:       shared.Room,
		Position:   shared.Position,
	}

	testCases := map[string]struct {
		source     *location.Location
		sourceP    *pb.Location
		converted  *location.Location
		convertedP *pb.Location
	}{
		"valid": {
			source:     &shared,
			sourceP:    &sharedP,
			converted:  &shared,
			convertedP: &sharedP,
		},
		"nil": {
			source:     nil,
			sourceP:    nil,
			converted:  &location.Location{},
			convertedP: nil,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, *testCase.converted, LocationFrom(testCase.sourceP))
			assert.Equal(t, testCase.convertedP, LocationTo(testCase.source))
		})
	}
}

func TestRackPositionConverter(t *testing.T) {
	shared := component.InRackPosition{
		SlotID:    1,
		TrayIndex: 2,
		HostID:    3,
	}

	sharedP := pb.RackPosition{
		SlotId:  int32(shared.SlotID),
		TrayIdx: int32(shared.TrayIndex),
		HostId:  int32(shared.HostID),
	}

	testCases := map[string]struct {
		source     *component.InRackPosition
		sourceP    *pb.RackPosition
		converted  *component.InRackPosition
		convertedP *pb.RackPosition
	}{
		"valid": {
			source:     &shared,
			sourceP:    &sharedP,
			converted:  &shared,
			convertedP: &sharedP,
		},
		"nil": {
			source:  nil,
			sourceP: nil,
			converted: &component.InRackPosition{
				SlotID:    -1,
				TrayIndex: -1,
				HostID:    -1,
			},
			convertedP: nil,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, *testCase.converted, RackPositionFrom(testCase.sourceP))
			assert.Equal(t, testCase.convertedP, RackPositionTo(testCase.source))
		})
	}
}

func TestBMCFrom(t *testing.T) {
	// Helper functions for test setup
	stringPtr := func(s string) *string {
		return &s
	}

	mustParseMAC := func(s string) net.HardwareAddr {
		mac, err := net.ParseMAC(s)
		if err != nil {
			panic(err)
		}
		return mac
	}

	credentialPtr := func(cred credential.Credential) *credential.Credential {
		return &cred
	}

	sharedMac := "00:1a:2b:3c:4d:5e"
	sharedIp := "192.168.1.1"
	sharedUser := "admin"
	sharedPassword := "password"

	testCases := map[string]struct {
		name          string
		sourceP       *pb.BMCInfo
		source        *bmc.BMC
		expectedType  devicetypes.BMCType
		expectedBMC   *bmc.BMC
		expectedProto *pb.BMCInfo
		testBMCTo     bool
		testBMCToType devicetypes.BMCType
	}{
		"valid host BMC with all fields": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				IpAddress:  stringPtr(sharedIp),
				User:       stringPtr(sharedUser),
				Password:   stringPtr(sharedPassword),
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC:        mustParseMAC(sharedMac),
				IP:         net.ParseIP(sharedIp),
				Credential: credentialPtr(credential.New(sharedUser, sharedPassword)),
			},
			source: &bmc.BMC{
				MAC:        mustParseMAC(sharedMac),
				IP:         net.ParseIP(sharedIp),
				Credential: credentialPtr(credential.New(sharedUser, sharedPassword)),
			},
			expectedProto: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				IpAddress:  stringPtr(sharedIp),
				User:       stringPtr(sharedUser),
				Password:   stringPtr(sharedPassword),
			},
			testBMCTo:     true,
			testBMCToType: devicetypes.BMCTypeHost,
		},
		"valid DPU BMC": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_DPU,
				MacAddress: sharedMac,
				IpAddress:  stringPtr(sharedIp),
				User:       stringPtr(sharedUser),
				Password:   stringPtr(sharedPassword),
			},
			expectedType: devicetypes.BMCTypeDPU,
			expectedBMC: &bmc.BMC{
				MAC:        mustParseMAC(sharedMac),
				IP:         net.ParseIP(sharedIp),
				Credential: credentialPtr(credential.New(sharedUser, sharedPassword)),
			},
			source: &bmc.BMC{
				MAC:        mustParseMAC(sharedMac),
				IP:         net.ParseIP(sharedIp),
				Credential: credentialPtr(credential.New(sharedUser, sharedPassword)),
			},
			expectedProto: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_DPU,
				MacAddress: sharedMac,
				IpAddress:  stringPtr(sharedIp),
				User:       stringPtr(sharedUser),
				Password:   stringPtr(sharedPassword),
			},
			testBMCTo:     true,
			testBMCToType: devicetypes.BMCTypeDPU,
		},
		"unknown BMC type": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_UNKNOWN,
				MacAddress: sharedMac,
			},
			expectedType: devicetypes.BMCTypeUnknown,
			expectedBMC: &bmc.BMC{
				MAC: mustParseMAC(sharedMac),
			},
			source: &bmc.BMC{
				MAC: mustParseMAC(sharedMac),
			},
			expectedProto: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_UNKNOWN,
				MacAddress: sharedMac,
			},
			testBMCTo:     true,
			testBMCToType: devicetypes.BMCTypeUnknown,
		},
		"nil BMCInfo": {
			sourceP:       nil,
			expectedType:  devicetypes.BMCTypeUnknown,
			expectedBMC:   nil,
			source:        nil,
			expectedProto: nil,
			testBMCTo:     true,
			testBMCToType: devicetypes.BMCTypeHost,
		},
		"invalid MAC address": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: "invalid-mac-address",
				User:       stringPtr(sharedUser),
				Password:   stringPtr(sharedPassword),
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC:        nil,                                                       // Invalid MAC results in nil
				Credential: credentialPtr(credential.New(sharedUser, sharedPassword)), //nolint
			},
			// Can't test BMCTo with invalid MAC as it would cause issues
			testBMCTo: false,
		},
		"empty MAC address": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: "",
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC: nil, // Empty MAC results in nil
			},
			testBMCTo: false,
		},
		"nil IP address": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				IpAddress:  nil,
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC: mustParseMAC(sharedMac),
				IP:  nil,
			},
			source: &bmc.BMC{
				MAC: mustParseMAC(sharedMac),
				IP:  nil,
			},
			expectedProto: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				IpAddress:  nil,
			},
			testBMCTo:     true,
			testBMCToType: devicetypes.BMCTypeHost,
		},
		"empty IP address": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				IpAddress:  stringPtr(""),
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC: mustParseMAC(sharedMac),
				IP:  nil, // Empty IP string results in nil
			},
			testBMCTo: false, // Can't reconstruct empty IP string from nil
		},
		"invalid IP address": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				IpAddress:  stringPtr("invalid-ip"),
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC: mustParseMAC(sharedMac),
				IP:  nil, // Invalid IP results in nil
			},
			testBMCTo: false,
		},
		"IPv6 address": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				IpAddress:  stringPtr("2001:db8::1"),
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC: mustParseMAC(sharedMac),
				IP:  net.ParseIP("2001:db8::1"),
			},
			source: &bmc.BMC{
				MAC: mustParseMAC(sharedMac),
				IP:  net.ParseIP("2001:db8::1"),
			},
			expectedProto: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				IpAddress:  stringPtr("2001:db8::1"),
			},
			testBMCTo:     true,
			testBMCToType: devicetypes.BMCTypeHost,
		},
		"nil credentials": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				User:       nil,
				Password:   nil,
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC:        mustParseMAC(sharedMac),
				Credential: nil,
			},
			source: &bmc.BMC{
				MAC:        mustParseMAC(sharedMac),
				Credential: nil,
			},
			expectedProto: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				User:       nil,
				Password:   nil,
			},
			testBMCTo:     true,
			testBMCToType: devicetypes.BMCTypeHost,
		},
		"partial credentials - only user": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				User:       stringPtr("testuser"),
				Password:   nil,
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC:        mustParseMAC(sharedMac),
				Credential: nil, // BMCFrom requires both user and password
			},
			testBMCTo: false, // Can't reconstruct partial credentials
		},
		"partial credentials - only password": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				User:       nil,
				Password:   stringPtr("testpass"),
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC:        mustParseMAC(sharedMac),
				Credential: nil, // BMCFrom requires both user and password
			},
			testBMCTo: false, // Can't reconstruct partial credentials
		},
		"empty credentials": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_HOST,
				MacAddress: sharedMac,
				User:       stringPtr(""),
				Password:   stringPtr(""),
			},
			expectedType: devicetypes.BMCTypeHost,
			expectedBMC: &bmc.BMC{
				MAC:        mustParseMAC(sharedMac),
				Credential: credentialPtr(credential.New("", "")),
			},
			// Can't test BMCTo with empty credentials as they're converted to nil (invalid)
			testBMCTo: false,
		},
		"different MAC formats": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_DPU,
				MacAddress: "AA-BB-CC-DD-EE-FF", // Different format
			},
			expectedType: devicetypes.BMCTypeDPU,
			expectedBMC: &bmc.BMC{
				MAC: mustParseMAC("AA-BB-CC-DD-EE-FF"),
			},
			// Test that BMCTo normalizes to colon format
			source: &bmc.BMC{
				MAC: mustParseMAC("AA-BB-CC-DD-EE-FF"),
			},
			expectedProto: &pb.BMCInfo{
				Type:       pb.BMCType_BMC_TYPE_DPU,
				MacAddress: "aa:bb:cc:dd:ee:ff", // Normalized to lowercase with colons
			},
			testBMCTo:     true,
			testBMCToType: devicetypes.BMCTypeDPU,
		},
		"invalid BMC type": {
			sourceP: &pb.BMCInfo{
				Type:       pb.BMCType(-1), // Invalid type
				MacAddress: sharedMac,
			},
			expectedType: devicetypes.BMCTypeUnknown, // Should default to unknown
			expectedBMC: &bmc.BMC{
				MAC: mustParseMAC(sharedMac),
			},
			testBMCTo: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Test BMCFrom conversion
			convertedType, converted := BMCFrom(tc.sourceP)
			assert.Equal(t, tc.expectedType, convertedType, "BMCFrom should return expected type") //nolint
			assert.Equal(t, tc.expectedBMC, converted, "BMCFrom should return expected BMC")       //nolint

			// Test BMCTo conversion if applicable
			if tc.testBMCTo && tc.source != nil {
				convertedProto := BMCTo(tc.testBMCToType, tc.source)
				assert.Equal(t, tc.expectedProto, convertedProto, "BMCTo should return expected protobuf BMC") //nolint
			} else if tc.testBMCTo && tc.source == nil {
				convertedProto := BMCTo(tc.testBMCToType, tc.source)
				assert.Nil(t, convertedProto, "BMCTo should return nil for nil BMC") //nolint
			}
		})
	}
}

func TestComponentConverter(t *testing.T) {
	shared := component.Component{
		Type:            devicetypes.ComponentTypeCompute,
		Info:            deviceinfo.NewRandom("TestComponent", 6),
		FirmwareVersion: "1.0.0",
		Position: component.InRackPosition{
			SlotID:    26,
			TrayIndex: 12,
			HostID:    0,
		},
		BmcsByType: make(map[devicetypes.BMCType][]bmc.BMC),
	}

	sharedP := pb.Component{
		Type: pb.ComponentType_COMPONENT_TYPE_COMPUTE,
		Info: &pb.DeviceInfo{
			Id:           &pb.UUID{Id: shared.Info.ID.String()},
			Name:         shared.Info.Name,
			Manufacturer: shared.Info.Manufacturer,
			Model:        &shared.Info.Model,
			SerialNumber: shared.Info.SerialNumber,
			Description:  &shared.Info.Description,
		},
		FirmwareVersion: shared.FirmwareVersion,
		Position: &pb.RackPosition{
			SlotId:  int32(shared.Position.SlotID),
			TrayIdx: int32(shared.Position.TrayIndex),
			HostId:  int32(shared.Position.HostID),
		},
		Bmcs: make([]*pb.BMCInfo, 0),
	}

	testCases := map[string]struct {
		source     *component.Component
		sourceP    *pb.Component
		converted  *component.Component
		convertedP *pb.Component
	}{
		"valid": {
			source:     &shared,
			sourceP:    &sharedP,
			converted:  &shared,
			convertedP: &sharedP,
		},
		"nil": {
			source:     nil,
			sourceP:    nil,
			converted:  nil,
			convertedP: nil,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.converted, ComponentFrom(testCase.sourceP))
			assert.Equal(t, testCase.convertedP, ComponentTo(testCase.source))
		})
	}
}

func TestRackConverter(t *testing.T) {
	shared := rack.Rack{
		Info: deviceinfo.NewRandom("TestRack", 12),
		Loc: location.Location{
			Region:     "US",
			DataCenter: "Santa Clara",
			Room:       "Mars",
			Position:   "Row 12",
		},
		Components: make([]component.Component, 0),
	}

	sharedP := pb.Rack{
		Info: &pb.DeviceInfo{
			Id:           &pb.UUID{Id: shared.Info.ID.String()},
			Name:         shared.Info.Name,
			Manufacturer: shared.Info.Manufacturer,
			Model:        &shared.Info.Model,
			SerialNumber: shared.Info.SerialNumber,
			Description:  &shared.Info.Description,
		},
		Location: &pb.Location{
			Region:     shared.Loc.Region,
			Datacenter: shared.Loc.DataCenter,
			Room:       shared.Loc.Room,
			Position:   shared.Loc.Position,
		},
		Components: make([]*pb.Component, 0),
	}

	testCases := map[string]struct {
		source     *rack.Rack
		sourceP    *pb.Rack
		converted  *rack.Rack
		convertedP *pb.Rack
	}{
		"valid": {
			source:     &shared,
			sourceP:    &sharedP,
			converted:  &shared,
			convertedP: &sharedP,
		},
		"nil": {
			source:     nil,
			sourceP:    nil,
			converted:  nil,
			convertedP: nil,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.converted, RackFrom(testCase.sourceP))
			assert.Equal(t, testCase.convertedP, RackTo(testCase.source))
		})
	}
}

func TestOrderByConverter(t *testing.T) {
	testCases := map[string]struct {
		source     *pb.OrderBy
		sourceDB   *dbquery.OrderBy
		queryType  QueryType // QueryTypeRack or QueryTypeComponent
		converted  *dbquery.OrderBy
		convertedP *pb.OrderBy
	}{
		"rack name ASC": {
			source: &pb.OrderBy{
				Field:     &pb.OrderBy_RackField{RackField: pb.RackOrderByField_RACK_ORDER_BY_FIELD_NAME},
				Direction: "ASC",
			},
			sourceDB: &dbquery.OrderBy{
				Column:    "name",
				Direction: dbquery.OrderAscending,
			},
			queryType: QueryTypeRack,
			converted: &dbquery.OrderBy{
				Column:    "name",
				Direction: dbquery.OrderAscending,
			},
			convertedP: &pb.OrderBy{
				Field:     &pb.OrderBy_RackField{RackField: pb.RackOrderByField_RACK_ORDER_BY_FIELD_NAME},
				Direction: "ASC",
			},
		},
		"rack manufacturer DESC": {
			source: &pb.OrderBy{
				Field:     &pb.OrderBy_RackField{RackField: pb.RackOrderByField_RACK_ORDER_BY_FIELD_MANUFACTURER},
				Direction: "DESC",
			},
			sourceDB: &dbquery.OrderBy{
				Column:    "manufacturer",
				Direction: dbquery.OrderDescending,
			},
			queryType: QueryTypeRack,
			converted: &dbquery.OrderBy{
				Column:    "manufacturer",
				Direction: dbquery.OrderDescending,
			},
			convertedP: &pb.OrderBy{
				Field:     &pb.OrderBy_RackField{RackField: pb.RackOrderByField_RACK_ORDER_BY_FIELD_MANUFACTURER},
				Direction: "DESC",
			},
		},
		"component name ASC": {
			source: &pb.OrderBy{
				Field:     &pb.OrderBy_ComponentField{ComponentField: pb.ComponentOrderByField_COMPONENT_ORDER_BY_FIELD_NAME},
				Direction: "ASC",
			},
			sourceDB: &dbquery.OrderBy{
				Column:    "name",
				Direction: dbquery.OrderAscending,
			},
			queryType: QueryTypeComponent,
			converted: &dbquery.OrderBy{
				Column:    "name",
				Direction: dbquery.OrderAscending,
			},
			convertedP: &pb.OrderBy{
				Field:     &pb.OrderBy_ComponentField{ComponentField: pb.ComponentOrderByField_COMPONENT_ORDER_BY_FIELD_NAME},
				Direction: "ASC",
			},
		},
		"component type DESC": {
			source: &pb.OrderBy{
				Field:     &pb.OrderBy_ComponentField{ComponentField: pb.ComponentOrderByField_COMPONENT_ORDER_BY_FIELD_TYPE},
				Direction: "DESC",
			},
			sourceDB: &dbquery.OrderBy{
				Column:    "type",
				Direction: dbquery.OrderDescending,
			},
			queryType: QueryTypeComponent,
			converted: &dbquery.OrderBy{
				Column:    "type",
				Direction: dbquery.OrderDescending,
			},
			convertedP: &pb.OrderBy{
				Field:     &pb.OrderBy_ComponentField{ComponentField: pb.ComponentOrderByField_COMPONENT_ORDER_BY_FIELD_TYPE},
				Direction: "DESC",
			},
		},
		"nil protobuf": {
			source:     nil,
			sourceDB:   nil,
			queryType:  QueryTypeRack,
			converted:  nil,
			convertedP: nil,
		},
		"nil dbquery": {
			source:     nil,
			sourceDB:   nil,
			queryType:  QueryTypeRack,
			converted:  nil,
			convertedP: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Test OrderByFrom conversion
			converted := OrderByFrom(tc.source)
			assert.Equal(t, tc.converted, converted, "OrderByFrom should return expected OrderBy")

			// Test OrderByTo conversion
			convertedP := OrderByTo(tc.sourceDB, tc.queryType)
			assert.Equal(t, tc.convertedP, convertedP, "OrderByTo should return expected protobuf OrderBy")
		})
	}
}
