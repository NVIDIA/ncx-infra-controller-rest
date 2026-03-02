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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/extra/bundebug"

	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/handler/util/common"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model"
	"github.com/nvidia/bare-metal-manager-rest/common/pkg/otelecho"
	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	cdbu "github.com/nvidia/bare-metal-manager-rest/db/pkg/util"
)

// ~~~~~ Test Helpers ~~~~~ //

func testStatsInitDB(t *testing.T) *cdb.Session {
	dbSession := cdbu.GetTestDBSession(t, false)
	dbSession.DB.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(false),
		bundebug.FromEnv("BUNDEBUG"),
	))
	return dbSession
}

func testStatsBuildUser(t *testing.T, dbSession *cdb.Session, orgs []string, roles []string) *cdbm.User {
	uDAO := cdbm.NewUserDAO(dbSession)
	OrgData := cdbm.OrgData{}
	for _, org := range orgs {
		OrgData[org] = cdbm.Org{
			ID:          123,
			Name:        org,
			DisplayName: org,
			OrgType:     "ENTERPRISE",
			Roles:       roles,
		}
	}
	u, err := uDAO.Create(context.Background(), nil, cdbm.UserCreateInput{
		StarfleetID: cdb.GetStrPtr(uuid.NewString()),
		Email:       cdb.GetStrPtr("stats-test@test.com"),
		FirstName:   cdb.GetStrPtr("Stats"),
		LastName:    cdb.GetStrPtr("Tester"),
		OrgData:     OrgData,
	})
	assert.Nil(t, err)
	return u
}

func testStatsBuildInfrastructureProvider(t *testing.T, dbSession *cdb.Session, org, name string) *cdbm.InfrastructureProvider {
	ip := &cdbm.InfrastructureProvider{
		ID:   uuid.New(),
		Name: name,
		Org:  org,
	}
	_, err := dbSession.DB.NewInsert().Model(ip).Exec(context.Background())
	assert.Nil(t, err)
	return ip
}

func testStatsBuildSite(t *testing.T, dbSession *cdb.Session, ip *cdbm.InfrastructureProvider, name string) *cdbm.Site {
	st := &cdbm.Site{
		ID:                          uuid.New(),
		Name:                        name,
		Org:                         ip.Org,
		InfrastructureProviderID:    ip.ID,
		SiteControllerVersion:       cdb.GetStrPtr("1.0.0"),
		SiteAgentVersion:            cdb.GetStrPtr("1.0.0"),
		RegistrationToken:           cdb.GetStrPtr("1234-5678-9012-3456"),
		RegistrationTokenExpiration: cdb.GetTimePtr(cdb.GetCurTime()),
		IsInfinityEnabled:           false,
		Status:                      cdbm.SiteStatusRegistered,
		CreatedBy:                   uuid.New(),
	}
	_, err := dbSession.DB.NewInsert().Model(st).Exec(context.Background())
	assert.Nil(t, err)
	return st
}

func testStatsBuildTenant(t *testing.T, dbSession *cdb.Session, org, name, displayName string) *cdbm.Tenant {
	tn := &cdbm.Tenant{
		ID:             uuid.New(),
		Name:           name,
		Org:            org,
		OrgDisplayName: &displayName,
	}
	_, err := dbSession.DB.NewInsert().Model(tn).Exec(context.Background())
	assert.Nil(t, err)
	return tn
}

func testStatsBuildInstanceType(t *testing.T, dbSession *cdb.Session, ip *cdbm.InfrastructureProvider, site *cdbm.Site, name string) *cdbm.InstanceType {
	it := &cdbm.InstanceType{
		ID:                       uuid.New(),
		Name:                     name,
		InfrastructureProviderID: ip.ID,
		SiteID:                   cdb.GetUUIDPtr(site.ID),
		Status:                   cdbm.InstanceTypeStatusReady,
		CreatedBy:                uuid.New(),
		Version:                  "1.0",
	}
	_, err := dbSession.DB.NewInsert().Model(it).Exec(context.Background())
	assert.Nil(t, err)
	return it
}

func testStatsBuildMachine(t *testing.T, dbSession *cdb.Session, ip *cdbm.InfrastructureProvider, site *cdbm.Site, itID *uuid.UUID, status string, isNetworkDegraded bool) *cdbm.Machine {
	mid := uuid.NewString()
	m := &cdbm.Machine{
		ID:                       mid,
		InfrastructureProviderID: ip.ID,
		SiteID:                   site.ID,
		InstanceTypeID:           itID,
		ControllerMachineID:      mid,
		DefaultMacAddress:        cdb.GetStrPtr("00:1B:44:11:3A:B7"),
		IsAssigned:               itID != nil,
		IsNetworkDegraded:        isNetworkDegraded,
		Status:                   status,
	}
	_, err := dbSession.DB.NewInsert().Model(m).Exec(context.Background())
	assert.Nil(t, err)
	return m
}

func testStatsBuildMachineCapability(t *testing.T, dbSession *cdb.Session, machineID string, capType, name string, count int) *cdbm.MachineCapability {
	mc := &cdbm.MachineCapability{
		ID:        uuid.New(),
		MachineID: &machineID,
		Type:      capType,
		Name:      name,
		Count:     &count,
		Created:   cdb.GetCurTime(),
		Updated:   cdb.GetCurTime(),
	}
	_, err := dbSession.DB.NewInsert().Model(mc).Exec(context.Background())
	assert.Nil(t, err)
	return mc
}

func testStatsBuildAllocation(t *testing.T, dbSession *cdb.Session, ip *cdbm.InfrastructureProvider, tenant *cdbm.Tenant, site *cdbm.Site, name string) *cdbm.Allocation {
	al := &cdbm.Allocation{
		ID:                       uuid.New(),
		Name:                     name,
		InfrastructureProviderID: ip.ID,
		TenantID:                 tenant.ID,
		SiteID:                   site.ID,
		Status:                   cdbm.AllocationStatusRegistered,
		CreatedBy:                uuid.New(),
	}
	_, err := dbSession.DB.NewInsert().Model(al).Exec(context.Background())
	assert.Nil(t, err)
	return al
}

func testStatsBuildAllocationConstraint(t *testing.T, dbSession *cdb.Session, alloc *cdbm.Allocation, instanceTypeID uuid.UUID, constraintValue int) *cdbm.AllocationConstraint {
	ac := &cdbm.AllocationConstraint{
		ID:              uuid.New(),
		AllocationID:    alloc.ID,
		ResourceType:    cdbm.AllocationResourceTypeInstanceType,
		ResourceTypeID:  instanceTypeID,
		ConstraintType:  cdbm.AllocationConstraintTypeReserved,
		ConstraintValue: constraintValue,
		CreatedBy:       uuid.New(),
	}
	_, err := dbSession.DB.NewInsert().Model(ac).Exec(context.Background())
	assert.Nil(t, err)
	return ac
}

func testStatsBuildInstance(t *testing.T, dbSession *cdb.Session, ip *cdbm.InfrastructureProvider, site *cdbm.Site, tenant *cdbm.Tenant, vpc *cdbm.Vpc, itID *uuid.UUID, machineID *string, allocID *uuid.UUID, acID *uuid.UUID, name, status string) *cdbm.Instance {
	inst := &cdbm.Instance{
		ID:                       uuid.New(),
		Name:                     name,
		AllocationID:             allocID,
		AllocationConstraintID:   acID,
		TenantID:                 tenant.ID,
		InfrastructureProviderID: ip.ID,
		SiteID:                   site.ID,
		InstanceTypeID:           itID,
		VpcID:                    vpc.ID,
		MachineID:                machineID,
		Status:                   status,
		CreatedBy:                uuid.New(),
	}
	_, err := dbSession.DB.NewInsert().Model(inst).Exec(context.Background())
	assert.Nil(t, err)
	return inst
}

func testStatsBuildVpc(t *testing.T, dbSession *cdb.Session, ip *cdbm.InfrastructureProvider, site *cdbm.Site, tenant *cdbm.Tenant, name string) *cdbm.Vpc {
	vpc := &cdbm.Vpc{
		ID:                       uuid.New(),
		Name:                     name,
		Org:                      tenant.Org,
		InfrastructureProviderID: ip.ID,
		SiteID:                   site.ID,
		TenantID:                 tenant.ID,
		Status:                   cdbm.VpcStatusReady,
		CreatedBy:                uuid.New(),
	}
	_, err := dbSession.DB.NewInsert().Model(vpc).Exec(context.Background())
	assert.Nil(t, err)
	return vpc
}

func testStatsSetupEchoContext(t *testing.T, org, siteID string, user *cdbm.User) (echo.Context, *httptest.ResponseRecorder) {
	ctx := context.Background()
	tracer, _, ctx := common.TestCommonTraceProviderSetup(t, ctx)
	ctx = context.WithValue(ctx, otelecho.TracerKey, tracer)

	e := echo.New()
	q := url.Values{}
	q.Add("siteId", siteID)

	req := httptest.NewRequest(http.MethodGet, "/?"+q.Encode(), nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	ec := e.NewContext(req, rec)
	ec.SetParamNames("orgName")
	ec.SetParamValues(org)
	ec.Set("user", user)
	ec.SetRequest(ec.Request().WithContext(ctx))

	return ec, rec
}

// ~~~~~ Test Data Setup ~~~~~ //
//
// Test scenario:
//   1 Infrastructure Provider (org: "stats-org")
//   1 Site
//   2 Tenants: "tenant-a" (org: "tenant-a-org", display: "Tenant A Org") and "tenant-b" (org: "tenant-b-org", display: "Tenant B Org")
//   4 Instance Types: "cpu.x100", "gpu.a100", "gpu.h100", "storage.hdd"
//
//   Machines (20 total across instance types + 3 unassigned):
//     cpu.x100: 8 machines (5 ready, 1 inUse, 1 error, 1 maintenance) - 2 degraded
//     gpu.a100: 5 machines (2 ready, 2 inUse, 1 error) - 1 degraded
//     gpu.h100: 4 machines (3 ready, 1 maintenance)
//     storage.hdd: 3 machines (3 ready)
//     unassigned: 3 machines (2 ready, 1 unknown)
//
//   GPU Capabilities:
//     gpu.a100 machines: each has "NVIDIA A100 PCIe" GPU with count=8
//     gpu.h100 machines: each has "NVIDIA H100 PCIe" GPU with count=8
//
//   Allocations:
//     Tenant A: 2 allocations
//       alloc-a-1: cpu.x100 constraint=20, gpu.a100 constraint=3
//       alloc-a-2: cpu.x100 constraint=10
//     Tenant B: 1 allocation
//       alloc-b-1: cpu.x100 constraint=15, gpu.h100 constraint=2
//
//   Instances (machines in use by tenants):
//     Tenant A:
//       3 instances on cpu.x100 machines (1 on error machine, 1 on maintenance machine, 1 on inUse machine)
//       2 instances on gpu.a100 machines (both on inUse machines)
//     Tenant B:
//       1 instance on gpu.h100 machine (on maintenance machine)

func TestStatsHandlers(t *testing.T) {
	dbSession := testStatsInitDB(t)
	defer dbSession.Close()
	common.TestSetupSchema(t, dbSession)

	org := "stats-org"

	// Users
	providerUser := testStatsBuildUser(t, dbSession, []string{org}, []string{"FORGE_PROVIDER_ADMIN"})
	tenantUser := testStatsBuildUser(t, dbSession, []string{org}, []string{"FORGE_TENANT_ADMIN"})

	// Infrastructure provider & site
	ip := testStatsBuildInfrastructureProvider(t, dbSession, org, "stats-provider")
	site := testStatsBuildSite(t, dbSession, ip, "stats-site")

	// Tenants
	tenantA := testStatsBuildTenant(t, dbSession, "tenant-a-org", "tenant-a", "Tenant A Org")
	tenantB := testStatsBuildTenant(t, dbSession, "tenant-b-org", "tenant-b", "Tenant B Org")

	// Instance Types
	itCPU := testStatsBuildInstanceType(t, dbSession, ip, site, "cpu.x100")
	itA100 := testStatsBuildInstanceType(t, dbSession, ip, site, "gpu.a100")
	itH100 := testStatsBuildInstanceType(t, dbSession, ip, site, "gpu.h100")
	itStorage := testStatsBuildInstanceType(t, dbSession, ip, site, "storage.hdd")

	// ~~~~~ Machines ~~~~~ //

	// cpu.x100 machines (8 total)
	cpuMachines := make([]*cdbm.Machine, 8)
	cpuMachines[0] = testStatsBuildMachine(t, dbSession, ip, site, &itCPU.ID, cdbm.MachineStatusReady, false)
	cpuMachines[1] = testStatsBuildMachine(t, dbSession, ip, site, &itCPU.ID, cdbm.MachineStatusReady, false)
	cpuMachines[2] = testStatsBuildMachine(t, dbSession, ip, site, &itCPU.ID, cdbm.MachineStatusReady, true) // degraded
	cpuMachines[3] = testStatsBuildMachine(t, dbSession, ip, site, &itCPU.ID, cdbm.MachineStatusReady, true) // degraded
	cpuMachines[4] = testStatsBuildMachine(t, dbSession, ip, site, &itCPU.ID, cdbm.MachineStatusReady, false)
	cpuMachines[5] = testStatsBuildMachine(t, dbSession, ip, site, &itCPU.ID, cdbm.MachineStatusInUse, false)
	cpuMachines[6] = testStatsBuildMachine(t, dbSession, ip, site, &itCPU.ID, cdbm.MachineStatusError, false)
	cpuMachines[7] = testStatsBuildMachine(t, dbSession, ip, site, &itCPU.ID, cdbm.MachineStatusMaintenance, false)

	// gpu.a100 machines (5 total)
	a100Machines := make([]*cdbm.Machine, 5)
	a100Machines[0] = testStatsBuildMachine(t, dbSession, ip, site, &itA100.ID, cdbm.MachineStatusReady, false)
	a100Machines[1] = testStatsBuildMachine(t, dbSession, ip, site, &itA100.ID, cdbm.MachineStatusReady, false)
	a100Machines[2] = testStatsBuildMachine(t, dbSession, ip, site, &itA100.ID, cdbm.MachineStatusInUse, false)
	a100Machines[3] = testStatsBuildMachine(t, dbSession, ip, site, &itA100.ID, cdbm.MachineStatusInUse, false)
	a100Machines[4] = testStatsBuildMachine(t, dbSession, ip, site, &itA100.ID, cdbm.MachineStatusError, true) // degraded

	// gpu.h100 machines (4 total)
	h100Machines := make([]*cdbm.Machine, 4)
	h100Machines[0] = testStatsBuildMachine(t, dbSession, ip, site, &itH100.ID, cdbm.MachineStatusReady, false)
	h100Machines[1] = testStatsBuildMachine(t, dbSession, ip, site, &itH100.ID, cdbm.MachineStatusReady, false)
	h100Machines[2] = testStatsBuildMachine(t, dbSession, ip, site, &itH100.ID, cdbm.MachineStatusReady, false)
	h100Machines[3] = testStatsBuildMachine(t, dbSession, ip, site, &itH100.ID, cdbm.MachineStatusMaintenance, false)

	// storage.hdd machines (3 total)
	testStatsBuildMachine(t, dbSession, ip, site, &itStorage.ID, cdbm.MachineStatusReady, false)
	testStatsBuildMachine(t, dbSession, ip, site, &itStorage.ID, cdbm.MachineStatusReady, false)
	testStatsBuildMachine(t, dbSession, ip, site, &itStorage.ID, cdbm.MachineStatusReady, false)

	// Unassigned machines (3 total)
	testStatsBuildMachine(t, dbSession, ip, site, nil, cdbm.MachineStatusReady, false)
	testStatsBuildMachine(t, dbSession, ip, site, nil, cdbm.MachineStatusReady, false)
	testStatsBuildMachine(t, dbSession, ip, site, nil, cdbm.MachineStatusUnknown, false)

	// ~~~~~ GPU Capabilities ~~~~~ //

	for _, m := range a100Machines {
		testStatsBuildMachineCapability(t, dbSession, m.ID, cdbm.MachineCapabilityTypeGPU, "NVIDIA A100 PCIe", 8)
	}
	for _, m := range h100Machines {
		testStatsBuildMachineCapability(t, dbSession, m.ID, cdbm.MachineCapabilityTypeGPU, "NVIDIA H100 PCIe", 8)
	}

	// ~~~~~ Allocations & Constraints ~~~~~ //

	allocA1 := testStatsBuildAllocation(t, dbSession, ip, tenantA, site, "alloc-a-1")
	testStatsBuildAllocationConstraint(t, dbSession, allocA1, itCPU.ID, 20) // 20 cpu.x100 for Tenant A alloc-1
	testStatsBuildAllocationConstraint(t, dbSession, allocA1, itA100.ID, 3) // 3 gpu.a100 for Tenant A alloc-1

	allocA2 := testStatsBuildAllocation(t, dbSession, ip, tenantA, site, "alloc-a-2")
	testStatsBuildAllocationConstraint(t, dbSession, allocA2, itCPU.ID, 10) // 10 cpu.x100 for Tenant A alloc-2

	allocB1 := testStatsBuildAllocation(t, dbSession, ip, tenantB, site, "alloc-b-1")
	testStatsBuildAllocationConstraint(t, dbSession, allocB1, itCPU.ID, 15) // 15 cpu.x100 for Tenant B alloc-1
	testStatsBuildAllocationConstraint(t, dbSession, allocB1, itH100.ID, 2) // 2 gpu.h100 for Tenant B alloc-1

	// ~~~~~ Instances (machines associated with tenant instances) ~~~~~ //

	// VPCs needed for instances
	vpcA := testStatsBuildVpc(t, dbSession, ip, site, tenantA, "vpc-a")
	vpcB := testStatsBuildVpc(t, dbSession, ip, site, tenantB, "vpc-b")

	// Tenant A instances on cpu.x100: 3 instances
	testStatsBuildInstance(t, dbSession, ip, site, tenantA, vpcA, &itCPU.ID, &cpuMachines[5].ID, &allocA1.ID, nil, "a-cpu-inst-1", cdbm.InstanceStatusReady) // on inUse machine
	testStatsBuildInstance(t, dbSession, ip, site, tenantA, vpcA, &itCPU.ID, &cpuMachines[6].ID, &allocA1.ID, nil, "a-cpu-inst-2", cdbm.InstanceStatusReady) // on error machine
	testStatsBuildInstance(t, dbSession, ip, site, tenantA, vpcA, &itCPU.ID, &cpuMachines[7].ID, &allocA2.ID, nil, "a-cpu-inst-3", cdbm.InstanceStatusReady) // on maintenance machine

	// Tenant A instances on gpu.a100: 2 instances
	testStatsBuildInstance(t, dbSession, ip, site, tenantA, vpcA, &itA100.ID, &a100Machines[2].ID, &allocA1.ID, nil, "a-a100-inst-1", cdbm.InstanceStatusReady) // on inUse machine
	testStatsBuildInstance(t, dbSession, ip, site, tenantA, vpcA, &itA100.ID, &a100Machines[3].ID, &allocA1.ID, nil, "a-a100-inst-2", cdbm.InstanceStatusReady) // on inUse machine

	// Tenant B instances on gpu.h100: 1 instance
	testStatsBuildInstance(t, dbSession, ip, site, tenantB, vpcB, &itH100.ID, &h100Machines[3].ID, &allocB1.ID, nil, "b-h100-inst-1", cdbm.InstanceStatusReady) // on maintenance machine

	cfg := common.GetTestConfig()

	// ~~~~~ Test: Machine GPU Stats ~~~~~ //

	t.Run("GetMachineGPUStats", func(t *testing.T) {
		ec, rec := testStatsSetupEchoContext(t, org, site.ID.String(), providerUser)

		handler := NewGetMachineGPUStatsHandler(dbSession, cfg)
		err := handler.Handle(ec)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var result []model.APIMachineGPUStats
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.Nil(t, err)

		// Should have 2 GPU types: NVIDIA A100 PCIe and NVIDIA H100 PCIe
		assert.Equal(t, 2, len(result))

		gpuByName := make(map[string]model.APIMachineGPUStats)
		for _, g := range result {
			gpuByName[g.Name] = g
		}

		// A100: 5 machines x 8 GPUs each = 40 GPUs
		a100Stats := gpuByName["NVIDIA A100 PCIe"]
		assert.Equal(t, 40, a100Stats.GPUs)
		assert.Equal(t, 5, a100Stats.Machines)

		// H100: 4 machines x 8 GPUs each = 32 GPUs
		h100Stats := gpuByName["NVIDIA H100 PCIe"]
		assert.Equal(t, 32, h100Stats.GPUs)
		assert.Equal(t, 4, h100Stats.Machines)
	})

	// ~~~~~ Test: Machine Instance Type Summary ~~~~~ //

	t.Run("GetMachineInstanceTypeSummary", func(t *testing.T) {
		ec, rec := testStatsSetupEchoContext(t, org, site.ID.String(), providerUser)

		handler := NewGetMachineInstanceTypeSummaryHandler(dbSession, cfg)
		err := handler.Handle(ec)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var result model.APIMachineInstanceTypeSummary
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.Nil(t, err)

		// Assigned: 8 cpu + 5 a100 + 4 h100 + 3 storage = 20 machines
		assert.Equal(t, 20, result.Assigned.Total)
		assert.Equal(t, 13, result.Assigned.Ready)      // 5 cpu + 2 a100 + 3 h100 + 3 storage
		assert.Equal(t, 3, result.Assigned.InUse)       // 1 cpu + 2 a100
		assert.Equal(t, 2, result.Assigned.Error)       // 1 cpu + 1 a100
		assert.Equal(t, 2, result.Assigned.Maintenance) // 1 cpu + 1 h100
		assert.Equal(t, 0, result.Assigned.Unknown)

		// Unassigned: 3 machines (2 ready, 1 unknown)
		assert.Equal(t, 3, result.Unassigned.Total)
		assert.Equal(t, 2, result.Unassigned.Ready)
		assert.Equal(t, 0, result.Unassigned.InUse)
		assert.Equal(t, 0, result.Unassigned.Error)
		assert.Equal(t, 0, result.Unassigned.Maintenance)
		assert.Equal(t, 1, result.Unassigned.Unknown)
	})

	// ~~~~~ Test: Machine Instance Type Stats (detailed) ~~~~~ //

	t.Run("GetMachineInstanceTypeStats", func(t *testing.T) {
		ec, rec := testStatsSetupEchoContext(t, org, site.ID.String(), providerUser)

		handler := NewGetMachineInstanceTypeStatsHandler(dbSession, cfg)
		err := handler.Handle(ec)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var result []model.APIMachineInstanceTypeStats
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.Nil(t, err)

		// Should have 4 instance types
		assert.Equal(t, 4, len(result))

		statsByName := make(map[string]model.APIMachineInstanceTypeStats)
		for _, s := range result {
			statsByName[s.Name] = s
		}

		// cpu.x100: 8 assigned (5 Ready, 1 InUse, 1 Error, 1 Maintenance), 3 used, maxAllocatable=(7 healthy)-(3 used)=4
		cpuStats := statsByName["cpu.x100"]
		assert.Equal(t, itCPU.ID.String(), cpuStats.ID)
		// assignedMachineStats: all 8 machines assigned to this IT
		assert.Equal(t, 8, cpuStats.AssignedMachineStats.Total)
		assert.Equal(t, 1, cpuStats.AssignedMachineStats.Error)
		assert.Equal(t, 1, cpuStats.AssignedMachineStats.Maintenance)
		assert.Equal(t, 45, cpuStats.Allocated)     // 20 + 10 (Tenant A) + 15 (Tenant B)
		assert.Equal(t, 3, cpuStats.MaxAllocatable) // (8 - 1 error - 1 maintenance) - 3 used = 3
		// usedMachineStats: 3 total (Tenant A: 3), 1 error, 1 maintenance
		assert.Equal(t, 3, cpuStats.UsedMachineStats.Total)
		assert.Equal(t, 1, cpuStats.UsedMachineStats.Error)
		assert.Equal(t, 1, cpuStats.UsedMachineStats.Maintenance)
		// 2 tenants for cpu.x100
		assert.Equal(t, 2, len(cpuStats.Tenants))

		// gpu.a100: 5 assigned (2 Ready, 2 InUse, 1 Error), 2 used, maxAllocatable=(4 healthy)-(2 used)=2
		a100Stats := statsByName["gpu.a100"]
		// assignedMachineStats: all 5 machines
		assert.Equal(t, 5, a100Stats.AssignedMachineStats.Total)
		assert.Equal(t, 1, a100Stats.AssignedMachineStats.Error)
		assert.Equal(t, 0, a100Stats.AssignedMachineStats.Maintenance)
		assert.Equal(t, 3, a100Stats.Allocated)      // 3 from Tenant A
		assert.Equal(t, 2, a100Stats.MaxAllocatable) // (5 - 1 error) - 2 used = 2
		assert.Equal(t, 2, a100Stats.UsedMachineStats.Total)
		assert.Equal(t, 0, a100Stats.UsedMachineStats.Error)
		assert.Equal(t, 0, a100Stats.UsedMachineStats.Maintenance)
		// 1 tenant for gpu.a100
		assert.Equal(t, 1, len(a100Stats.Tenants))

		// gpu.h100: 4 assigned (3 Ready, 1 Maintenance), 1 used, maxAllocatable=(4 - 0 error - 1 maintenance)-(1 used)=2
		h100Stats := statsByName["gpu.h100"]
		// assignedMachineStats: all 4 machines
		assert.Equal(t, 4, h100Stats.AssignedMachineStats.Total)
		assert.Equal(t, 0, h100Stats.AssignedMachineStats.Error)
		assert.Equal(t, 1, h100Stats.AssignedMachineStats.Maintenance)
		assert.Equal(t, 2, h100Stats.Allocated)      // 2 from Tenant B
		assert.Equal(t, 2, h100Stats.MaxAllocatable) // (4 - 0 error - 1 maintenance) - 1 used = 2
		assert.Equal(t, 1, h100Stats.UsedMachineStats.Total)
		assert.Equal(t, 0, h100Stats.UsedMachineStats.Error)
		assert.Equal(t, 1, h100Stats.UsedMachineStats.Maintenance) // Tenant B instance on maintenance machine
		// 1 tenant for gpu.h100
		assert.Equal(t, 1, len(h100Stats.Tenants))

		// storage.hdd: 3 assigned (3 Ready), 0 allocated, maxAllocatable=3
		storageStats := statsByName["storage.hdd"]
		// assignedMachineStats: all 3 machines, all ready
		assert.Equal(t, 3, storageStats.AssignedMachineStats.Total)
		assert.Equal(t, 0, storageStats.AssignedMachineStats.Error)
		assert.Equal(t, 0, storageStats.AssignedMachineStats.Maintenance)
		assert.Equal(t, 0, storageStats.Allocated)
		assert.Equal(t, 3, storageStats.MaxAllocatable) // 3 - 0 = 3
		assert.Equal(t, 0, storageStats.UsedMachineStats.Total)
		assert.Equal(t, 0, len(storageStats.Tenants))
	})

	// ~~~~~ Test: Tenant Instance Type Stats ~~~~~ //

	t.Run("GetTenantInstanceTypeStats", func(t *testing.T) {
		ec, rec := testStatsSetupEchoContext(t, org, site.ID.String(), providerUser)

		handler := NewGetTenantInstanceTypeStatsHandler(dbSession, cfg)
		err := handler.Handle(ec)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var result []model.APITenantInstanceTypeStats
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.Nil(t, err)

		// Should have 2 tenants
		assert.Equal(t, 2, len(result))

		tenantByOrg := make(map[string]model.APITenantInstanceTypeStats)
		for _, ts := range result {
			tenantByOrg[ts.Org] = ts
		}

		// Tenant A
		tenantAStats := tenantByOrg["tenant-a-org"]
		assert.Equal(t, tenantA.ID.String(), tenantAStats.ID)
		assert.Equal(t, "Tenant A Org", tenantAStats.OrgDisplayName)
		// Tenant A has allocations for cpu.x100 and gpu.a100
		assert.Equal(t, 2, len(tenantAStats.InstanceTypes))

		tenantAByIT := make(map[string]model.APITenantInstanceTypeStatsEntry)
		for _, it := range tenantAStats.InstanceTypes {
			tenantAByIT[it.Name] = it
		}

		// Tenant A cpu.x100: allocated=30 (20+10), used=3 (1 error, 1 maintenance), 2 allocations
		tenantACPU := tenantAByIT["cpu.x100"]
		assert.Equal(t, 30, tenantACPU.Allocated)
		assert.Equal(t, 3, tenantACPU.UsedMachineStats.Total)
		assert.Equal(t, 1, tenantACPU.UsedMachineStats.Error)
		assert.Equal(t, 1, tenantACPU.UsedMachineStats.Maintenance)
		assert.Equal(t, 2, len(tenantACPU.Allocations))

		// Tenant A gpu.a100: allocated=3, used=2, 1 allocation
		tenantAA100 := tenantAByIT["gpu.a100"]
		assert.Equal(t, 3, tenantAA100.Allocated)
		assert.Equal(t, 2, tenantAA100.UsedMachineStats.Total)
		assert.Equal(t, 0, tenantAA100.UsedMachineStats.Error)
		assert.Equal(t, 0, tenantAA100.UsedMachineStats.Maintenance)
		assert.Equal(t, 1, len(tenantAA100.Allocations))

		// Tenant B
		tenantBStats := tenantByOrg["tenant-b-org"]
		assert.Equal(t, tenantB.ID.String(), tenantBStats.ID)
		assert.Equal(t, "Tenant B Org", tenantBStats.OrgDisplayName)
		// Tenant B has allocations for cpu.x100 and gpu.h100
		assert.Equal(t, 2, len(tenantBStats.InstanceTypes))

		tenantBByIT := make(map[string]model.APITenantInstanceTypeStatsEntry)
		for _, it := range tenantBStats.InstanceTypes {
			tenantBByIT[it.Name] = it
		}

		// Tenant B cpu.x100: allocated=15, used=0 (no instances), 1 allocation
		tenantBCPU := tenantBByIT["cpu.x100"]
		assert.Equal(t, 15, tenantBCPU.Allocated)
		assert.Equal(t, 0, tenantBCPU.UsedMachineStats.Total)
		assert.Equal(t, 1, len(tenantBCPU.Allocations))

		// Tenant B gpu.h100: allocated=2, used=1 (maintenance), 1 allocation
		tenantBH100 := tenantBByIT["gpu.h100"]
		assert.Equal(t, 2, tenantBH100.Allocated)
		assert.Equal(t, 1, tenantBH100.UsedMachineStats.Total)
		assert.Equal(t, 0, tenantBH100.UsedMachineStats.Error)
		assert.Equal(t, 1, tenantBH100.UsedMachineStats.Maintenance)
		assert.Equal(t, 1, len(tenantBH100.Allocations))
	})

	// ~~~~~ Test: Auth - Tenant user should be denied ~~~~~ //

	t.Run("GetMachineGPUStats_TenantUserDenied", func(t *testing.T) {
		ec, rec := testStatsSetupEchoContext(t, org, site.ID.String(), tenantUser)

		handler := NewGetMachineGPUStatsHandler(dbSession, cfg)
		err := handler.Handle(ec)
		require.Nil(t, err)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	// ~~~~~ Test: Missing siteId query param ~~~~~ //

	t.Run("GetMachineGPUStats_MissingSiteId", func(t *testing.T) {
		ctx := context.Background()
		tracer, _, ctx := common.TestCommonTraceProviderSetup(t, ctx)
		ctx = context.WithValue(ctx, otelecho.TracerKey, tracer)

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil) // no siteId
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		ec := e.NewContext(req, rec)
		ec.SetParamNames("orgName")
		ec.SetParamValues(org)
		ec.Set("user", providerUser)
		ec.SetRequest(ec.Request().WithContext(ctx))

		handler := NewGetMachineGPUStatsHandler(dbSession, cfg)
		err := handler.Handle(ec)
		require.Nil(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	// ~~~~~ Test: Invalid siteId ~~~~~ //

	t.Run("GetMachineGPUStats_InvalidSiteId", func(t *testing.T) {
		ec, rec := testStatsSetupEchoContext(t, org, "not-a-uuid", providerUser)

		handler := NewGetMachineGPUStatsHandler(dbSession, cfg)
		err := handler.Handle(ec)
		require.Nil(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	// ~~~~~ Test: Empty site (no machines) ~~~~~ //

	t.Run("GetMachineGPUStats_EmptySite", func(t *testing.T) {
		emptySite := testStatsBuildSite(t, dbSession, ip, "empty-site")
		ec, rec := testStatsSetupEchoContext(t, org, emptySite.ID.String(), providerUser)

		handler := NewGetMachineGPUStatsHandler(dbSession, cfg)
		err := handler.Handle(ec)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var result []model.APIMachineGPUStats
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.Nil(t, err)
		assert.Equal(t, 0, len(result))
	})

	t.Run("GetMachineInstanceTypeSummary_EmptySite", func(t *testing.T) {
		emptySite := testStatsBuildSite(t, dbSession, ip, "empty-site-summary")
		ec, rec := testStatsSetupEchoContext(t, org, emptySite.ID.String(), providerUser)

		handler := NewGetMachineInstanceTypeSummaryHandler(dbSession, cfg)
		err := handler.Handle(ec)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var result model.APIMachineInstanceTypeSummary
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.Nil(t, err)
		assert.Equal(t, 0, result.Assigned.Total)
		assert.Equal(t, 0, result.Unassigned.Total)
	})

	_ = tenantUser // used above
}
