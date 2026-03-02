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

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIMachineGPUStats_JSON(t *testing.T) {
	stats := APIMachineGPUStats{
		Name:     "NVIDIA A100 PCIe",
		GPUs:     40,
		Machines: 5,
	}

	data, err := json.Marshal(stats)
	require.Nil(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.Nil(t, err)

	assert.Equal(t, "NVIDIA A100 PCIe", parsed["name"])
	assert.Equal(t, float64(40), parsed["gpus"])
	assert.Equal(t, float64(5), parsed["machines"])

	var roundTrip APIMachineGPUStats
	err = json.Unmarshal(data, &roundTrip)
	require.Nil(t, err)
	assert.Equal(t, stats, roundTrip)
}

func TestAPIMachineGPUStats_ZeroValues(t *testing.T) {
	stats := APIMachineGPUStats{}

	data, err := json.Marshal(stats)
	require.Nil(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.Nil(t, err)

	assert.Equal(t, "", parsed["name"])
	assert.Equal(t, float64(0), parsed["gpus"])
	assert.Equal(t, float64(0), parsed["machines"])
}

func TestAPIMachineStatusBreakdown_JSON(t *testing.T) {
	bd := APIMachineStatusBreakdown{
		Total:        23,
		Initializing: 1,
		Ready:        12,
		InUse:        3,
		Error:        2,
		Maintenance:  2,
		Unknown:      3,
	}

	data, err := json.Marshal(bd)
	require.Nil(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.Nil(t, err)

	assert.Equal(t, float64(23), parsed["total"])
	assert.Equal(t, float64(1), parsed["initializing"])
	assert.Equal(t, float64(12), parsed["ready"])
	assert.Equal(t, float64(3), parsed["inUse"])
	assert.Equal(t, float64(2), parsed["error"])
	assert.Equal(t, float64(2), parsed["maintenance"])
	assert.Equal(t, float64(3), parsed["unknown"])

	var roundTrip APIMachineStatusBreakdown
	err = json.Unmarshal(data, &roundTrip)
	require.Nil(t, err)
	assert.Equal(t, bd, roundTrip)
}

func TestAPIMachineInstanceTypeSummary_JSON(t *testing.T) {
	summary := APIMachineInstanceTypeSummary{
		Assigned: APIMachineStatusBreakdown{
			Total: 20, Ready: 13, InUse: 3, Error: 2, Maintenance: 2,
		},
		Unassigned: APIMachineStatusBreakdown{
			Total: 3, Ready: 2, Unknown: 1,
		},
	}

	data, err := json.Marshal(summary)
	require.Nil(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.Nil(t, err)

	assigned := parsed["assigned"].(map[string]interface{})
	assert.Equal(t, float64(20), assigned["total"])
	assert.Equal(t, float64(13), assigned["ready"])

	unassigned := parsed["unassigned"].(map[string]interface{})
	assert.Equal(t, float64(3), unassigned["total"])
	assert.Equal(t, float64(1), unassigned["unknown"])

	var roundTrip APIMachineInstanceTypeSummary
	err = json.Unmarshal(data, &roundTrip)
	require.Nil(t, err)
	assert.Equal(t, summary, roundTrip)
}

func TestAPIMachineInstanceTypeStats_JSON(t *testing.T) {
	stats := APIMachineInstanceTypeStats{
		ID:   "it-123",
		Name: "cpu.x100",
		AssignedMachineStats: APIMachineStatusBreakdown{
			Total: 8, Ready: 5, InUse: 1, Error: 1, Maintenance: 1,
		},
		Allocated:      45,
		MaxAllocatable: 0,
		UsedMachineStats: APIMachineStatusBreakdown{
			Total: 3, InUse: 1, Error: 1, Maintenance: 1,
		},
		Tenants: []APIMachineInstanceTypeTenant{
			{
				ID:        "t-1",
				Name:      "tenant-a-org",
				Allocated: 30,
				UsedMachineStats: APIMachineStatusBreakdown{
					Total: 3, InUse: 1, Error: 1, Maintenance: 1,
				},
				Allocations: []APIMachineInstanceTypeTenantAllocation{
					{ID: "a-1", Name: "alloc-a-1", Allocated: 20},
					{ID: "a-2", Name: "alloc-a-2", Allocated: 10},
				},
			},
			{
				ID:               "t-2",
				Name:             "tenant-b-org",
				Allocated:        15,
				UsedMachineStats: APIMachineStatusBreakdown{},
				Allocations: []APIMachineInstanceTypeTenantAllocation{
					{ID: "b-1", Name: "alloc-b-1", Allocated: 15},
				},
			},
		},
	}

	data, err := json.Marshal(stats)
	require.Nil(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.Nil(t, err)

	assert.Equal(t, "it-123", parsed["id"])
	assert.Equal(t, "cpu.x100", parsed["name"])
	assert.Equal(t, float64(45), parsed["allocated"])
	assert.Equal(t, float64(0), parsed["maxAllocatable"])

	assignedStats := parsed["assignedMachineStats"].(map[string]interface{})
	assert.Equal(t, float64(8), assignedStats["total"])
	assert.Equal(t, float64(5), assignedStats["ready"])
	assert.Equal(t, float64(1), assignedStats["inUse"])
	assert.Equal(t, float64(1), assignedStats["error"])
	assert.Equal(t, float64(1), assignedStats["maintenance"])

	usedStats := parsed["usedMachineStats"].(map[string]interface{})
	assert.Equal(t, float64(3), usedStats["total"])
	assert.Equal(t, float64(1), usedStats["inUse"])
	assert.Equal(t, float64(1), usedStats["error"])
	assert.Equal(t, float64(1), usedStats["maintenance"])

	tenants := parsed["tenants"].([]interface{})
	assert.Equal(t, 2, len(tenants))

	tenantA := tenants[0].(map[string]interface{})
	assert.Equal(t, "tenant-a-org", tenantA["name"])
	assert.Equal(t, float64(30), tenantA["allocated"])

	tenantAUsed := tenantA["usedMachineStats"].(map[string]interface{})
	assert.Equal(t, float64(3), tenantAUsed["total"])
	assert.Equal(t, float64(1), tenantAUsed["inUse"])

	allocs := tenantA["allocations"].([]interface{})
	assert.Equal(t, 2, len(allocs))
	assert.Equal(t, "alloc-a-1", allocs[0].(map[string]interface{})["name"])
	assert.Equal(t, float64(20), allocs[0].(map[string]interface{})["allocated"])

	var roundTrip APIMachineInstanceTypeStats
	err = json.Unmarshal(data, &roundTrip)
	require.Nil(t, err)
	assert.Equal(t, stats, roundTrip)
}

func TestAPIMachineInstanceTypeStats_EmptyTenants(t *testing.T) {
	stats := APIMachineInstanceTypeStats{
		ID:                   "it-456",
		Name:                 "storage.hdd",
		AssignedMachineStats: APIMachineStatusBreakdown{Total: 3, Ready: 3},
		MaxAllocatable:       3,
		Tenants:              nil,
	}

	data, err := json.Marshal(stats)
	require.Nil(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.Nil(t, err)

	assert.Nil(t, parsed["tenants"])
	assert.Equal(t, float64(3), parsed["maxAllocatable"])
}

func TestAPITenantInstanceTypeStats_JSON(t *testing.T) {
	stats := APITenantInstanceTypeStats{
		ID:             "tenant-1",
		Org:            "tenant-a-org",
		OrgDisplayName: "Tenant A Org",
		InstanceTypes: []APITenantInstanceTypeStatsEntry{
			{
				ID:        "it-1",
				Name:      "cpu.x100",
				Allocated: 30,
				UsedMachineStats: APIMachineStatusBreakdown{
					Total: 3, InUse: 1, Error: 1, Maintenance: 1,
				},
				MaxAllocatable: 0,
				Allocations: []APITenantInstanceTypeAllocation{
					{ID: "a-1", Name: "alloc-a-1", Total: 20},
					{ID: "a-2", Name: "alloc-a-2", Total: 10},
				},
			},
			{
				ID:        "it-2",
				Name:      "gpu.a100",
				Allocated: 3,
				UsedMachineStats: APIMachineStatusBreakdown{
					Total: 2, InUse: 2,
				},
				MaxAllocatable: 2,
				Allocations: []APITenantInstanceTypeAllocation{
					{ID: "a-1", Name: "alloc-a-1", Total: 3},
				},
			},
		},
	}

	data, err := json.Marshal(stats)
	require.Nil(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.Nil(t, err)

	assert.Equal(t, "tenant-1", parsed["id"])
	assert.Equal(t, "tenant-a-org", parsed["org"])
	assert.Equal(t, "Tenant A Org", parsed["orgDisplayName"])

	instanceTypes := parsed["instanceTypes"].([]interface{})
	assert.Equal(t, 2, len(instanceTypes))

	cpuEntry := instanceTypes[0].(map[string]interface{})
	assert.Equal(t, "cpu.x100", cpuEntry["name"])
	assert.Equal(t, float64(30), cpuEntry["allocated"])
	assert.Equal(t, float64(0), cpuEntry["maxAllocatable"])

	cpuUsed := cpuEntry["usedMachineStats"].(map[string]interface{})
	assert.Equal(t, float64(3), cpuUsed["total"])
	assert.Equal(t, float64(1), cpuUsed["inUse"])
	assert.Equal(t, float64(1), cpuUsed["error"])
	assert.Equal(t, float64(1), cpuUsed["maintenance"])

	gpuEntry := instanceTypes[1].(map[string]interface{})
	assert.Equal(t, "gpu.a100", gpuEntry["name"])
	gpuUsed := gpuEntry["usedMachineStats"].(map[string]interface{})
	assert.Equal(t, float64(2), gpuUsed["total"])
	assert.Equal(t, float64(2), gpuUsed["inUse"])

	cpuAllocs := cpuEntry["allocations"].([]interface{})
	assert.Equal(t, 2, len(cpuAllocs))
	assert.Equal(t, "alloc-a-1", cpuAllocs[0].(map[string]interface{})["name"])
	assert.Equal(t, float64(20), cpuAllocs[0].(map[string]interface{})["total"])

	var roundTrip APITenantInstanceTypeStats
	err = json.Unmarshal(data, &roundTrip)
	require.Nil(t, err)
	assert.Equal(t, stats, roundTrip)
}

func TestAPITenantInstanceTypeStats_EmptyInstanceTypes(t *testing.T) {
	stats := APITenantInstanceTypeStats{
		ID:             "tenant-empty",
		Org:            "empty-org",
		OrgDisplayName: "Empty Org",
		InstanceTypes:  nil,
	}

	data, err := json.Marshal(stats)
	require.Nil(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.Nil(t, err)

	assert.Equal(t, "tenant-empty", parsed["id"])
	assert.Nil(t, parsed["instanceTypes"])
}
