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
	"maps"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	"github.com/nvidia/bare-metal-manager-rest/api/internal/config"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/handler/util/common"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model"
	cerr "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	sutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	cdbp "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"
)

// ~~~~~ Machine GPU Stats Handler ~~~~~ //

// GetMachineGPUStatsHandler is the API Handler for retrieving GPU stats for machines at a site
type GetMachineGPUStatsHandler struct {
	dbSession  *cdb.Session
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetMachineGPUStatsHandler initializes and returns a new handler for machine GPU stats
func NewGetMachineGPUStatsHandler(dbSession *cdb.Session, cfg *config.Config) GetMachineGPUStatsHandler {
	return GetMachineGPUStatsHandler{
		dbSession:  dbSession,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Retrieve GPU stats for machines at a site
// @Description Returns GPU summary stats grouped by GPU name for machines at the specified site
// @Tags machine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param orgName path string true "Name of NGC organization"
// @Param siteId query string true "Site ID"
// @Success 200 {array} model.APIMachineGPUStats
// @Router /v2/org/{orgName}/carbide/machine/gpu/stats [get]
func (gmgsh GetMachineGPUStatsHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Machine", "GetGPUStats", c, gmgsh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	infrastructureProvider, apiError := common.IsProvider(ctx, logger, gmgsh.dbSession, org, dbUser, false)
	if apiError != nil {
		return cerr.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	siteIDStr := c.QueryParam("siteId")
	if siteIDStr == "" {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "siteId query parameter is required", nil)
	}
	site, err := common.GetSiteFromIDString(ctx, nil, siteIDStr, gmgsh.dbSession)
	if err != nil {
		logger.Error().Err(err).Str("siteId", siteIDStr).Msg("error parsing or retrieving site")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID specified in query param", nil)
	}

	if site.InfrastructureProviderID != infrastructureProvider.ID {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have access to the specified site", nil)
	}

	siteID := &site.ID

	// Fetch all machines for the site (exclude metadata for performance)
	machineDAO := cdbm.NewMachineDAO(gmgsh.dbSession)
	machines, _, err := machineDAO.GetAll(ctx, nil, cdbm.MachineFilterInput{
		SiteID:          siteID,
		ExcludeMetadata: true,
	}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving machines for site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve machines", nil)
	}

	if len(machines) == 0 {
		return c.JSON(http.StatusOK, []model.APIMachineGPUStats{})
	}

	machineIDs := lo.Map(machines, func(m cdbm.Machine, _ int) string { return m.ID })

	// Fetch GPU capabilities for all machines
	mcDAO := cdbm.NewMachineCapabilityDAO(gmgsh.dbSession)
	gpuType := cdbm.MachineCapabilityTypeGPU
	capabilities, _, err := mcDAO.GetAll(ctx, nil, machineIDs, nil, &gpuType,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, cdb.GetIntPtr(cdbp.TotalLimit), nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving GPU capabilities")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve GPU capabilities", nil)
	}

	result := model.NewAPIMachineGPUStatsList(capabilities)

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, result)
}

// ~~~~~ Tenant Instance Type Stats Handler ~~~~~ //

// GetTenantInstanceTypeStatsHandler is the API Handler for retrieving per-tenant instance type allocation stats
type GetTenantInstanceTypeStatsHandler struct {
	dbSession  *cdb.Session
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetTenantInstanceTypeStatsHandler initializes and returns a new handler for tenant instance type stats
func NewGetTenantInstanceTypeStatsHandler(dbSession *cdb.Session, cfg *config.Config) GetTenantInstanceTypeStatsHandler {
	return GetTenantInstanceTypeStatsHandler{
		dbSession:  dbSession,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Retrieve per-tenant instance type allocation stats for a site
// @Description Returns instance type allocation stats grouped by tenant for the specified site
// @Tags tenant
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param orgName path string true "Name of NGC organization"
// @Param siteId query string true "Site ID"
// @Success 200 {array} model.APITenantInstanceTypeStats
// @Router /v2/org/{orgName}/carbide/tenant/instance-type/stats [get]
func (gtitsh GetTenantInstanceTypeStatsHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Tenant", "GetInstanceTypeStats", c, gtitsh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	infrastructureProvider, apiError := common.IsProvider(ctx, logger, gtitsh.dbSession, org, dbUser, false)
	if apiError != nil {
		return cerr.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	siteIDStr := c.QueryParam("siteId")
	if siteIDStr == "" {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "siteId query parameter is required", nil)
	}
	site, err := common.GetSiteFromIDString(ctx, nil, siteIDStr, gtitsh.dbSession)
	if err != nil {
		logger.Error().Err(err).Str("siteId", siteIDStr).Msg("error parsing or retrieving site")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID specified in query param", nil)
	}

	if site.InfrastructureProviderID != infrastructureProvider.ID {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have access to the specified site", nil)
	}

	siteID := &site.ID
	siteIDs := []uuid.UUID{*siteID}

	// 1. Fetch all instance types for the site
	itDAO := cdbm.NewInstanceTypeDAO(gtitsh.dbSession)
	instanceTypes, _, err := itDAO.GetAll(ctx, nil, cdbm.InstanceTypeFilterInput{SiteIDs: siteIDs},
		nil, nil, nil, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving instance types")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve instance types", nil)
	}

	instanceTypeIDs := lo.Map(instanceTypes, func(it cdbm.InstanceType, _ int) uuid.UUID { return it.ID })
	instanceTypeMap := lo.KeyBy(instanceTypes, func(it cdbm.InstanceType) uuid.UUID { return it.ID })

	// 2. Fetch all allocations for the site (IDs needed to filter constraints)
	aDAO := cdbm.NewAllocationDAO(gtitsh.dbSession)
	allocations, _, err := aDAO.GetAll(ctx, nil, cdbm.AllocationFilterInput{SiteIDs: siteIDs},
		cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving allocations")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve allocations", nil)
	}

	allocationIDs := lo.Map(allocations, func(a cdbm.Allocation, _ int) uuid.UUID { return a.ID })

	// 3. Fetch allocation constraints with Allocation.Tenant
	var constraints []cdbm.AllocationConstraint
	if len(allocationIDs) > 0 {
		acDAO := cdbm.NewAllocationConstraintDAO(gtitsh.dbSession)
		constraints, _, err = acDAO.GetAll(ctx, nil, allocationIDs,
			cdb.GetStrPtr(cdbm.AllocationResourceTypeInstanceType), instanceTypeIDs,
			nil, nil, []string{"Allocation.Tenant"}, nil, cdb.GetIntPtr(cdbp.TotalLimit), nil)
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving allocation constraints")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve allocation constraints", nil)
		}
	}

	// 4. Fetch all instances for the site
	iDAO := cdbm.NewInstanceDAO(gtitsh.dbSession)
	instances, _, err := iDAO.GetAll(ctx, nil, cdbm.InstanceFilterInput{SiteIDs: siteIDs},
		cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving instances")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve instances", nil)
	}

	// 5. Fetch all machines with instance types for the site (exclude metadata)
	machineDAO := cdbm.NewMachineDAO(gtitsh.dbSession)
	machines, _, err := machineDAO.GetAll(ctx, nil, cdbm.MachineFilterInput{
		SiteID:          siteID,
		InstanceTypeIDs: instanceTypeIDs,
		ExcludeMetadata: true,
	}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving machines")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve machines", nil)
	}

	machineByID := lo.KeyBy(machines, func(m cdbm.Machine) string { return m.ID })

	// Build usage maps
	tenantITUsed := make(map[uuid.UUID]map[uuid.UUID]*model.APIMachineStatusBreakdown)
	for _, inst := range instances {
		if inst.InstanceTypeID == nil || inst.MachineID == nil {
			continue
		}
		tID := inst.TenantID
		itID := *inst.InstanceTypeID
		if tenantITUsed[tID] == nil {
			tenantITUsed[tID] = make(map[uuid.UUID]*model.APIMachineStatusBreakdown)
		}
		if tenantITUsed[tID][itID] == nil {
			tenantITUsed[tID][itID] = &model.APIMachineStatusBreakdown{}
		}
		if m, ok := machineByID[*inst.MachineID]; ok {
			tenantITUsed[tID][itID].AddMachineStatusCounts(m)
		}
	}

	// Group constraints by tenantID -> instanceTypeID
	tenantITAllocs := make(map[uuid.UUID]map[uuid.UUID][]cdbm.AllocationConstraint)
	for _, ac := range constraints {
		tID := ac.Allocation.TenantID
		itID := ac.ResourceTypeID
		if tenantITAllocs[tID] == nil {
			tenantITAllocs[tID] = make(map[uuid.UUID][]cdbm.AllocationConstraint)
		}
		tenantITAllocs[tID][itID] = append(tenantITAllocs[tID][itID], ac)
	}

	// Healthy (non-error) machines per instance type for maxAllocatable
	healthyAssignedMachines := lo.Filter(machines, func(m cdbm.Machine, _ int) bool {
		return m.InstanceTypeID != nil && m.Status != cdbm.MachineStatusError && m.Status != cdbm.MachineStatusMaintenance
	})
	healthyMachineCountByIT := lo.CountValuesBy(healthyAssignedMachines, func(m cdbm.Machine) uuid.UUID { return *m.InstanceTypeID })

	// Used machines per instance type across all tenants (in any state)
	usedMachinesByIT := make(map[uuid.UUID]int)
	for _, itMap := range tenantITUsed {
		for itID, stats := range itMap {
			usedMachinesByIT[itID] += stats.Total
		}
	}

	// Build response grouped by tenant
	tenantStatsMap := make(map[uuid.UUID]model.APITenantInstanceTypeStats)
	for tID, itAllocs := range tenantITAllocs {
		var t *cdbm.Tenant
		for _, acs := range itAllocs {
			if len(acs) > 0 {
				t = acs[0].Allocation.Tenant
				break
			}
		}

		tenantOrgDisplay := t.Org
		if t.OrgDisplayName != nil {
			tenantOrgDisplay = *t.OrgDisplayName
		}

		ts := model.APITenantInstanceTypeStats{
			ID:             t.ID.String(),
			Org:            t.Org,
			OrgDisplayName: tenantOrgDisplay,
		}

		for itID, details := range itAllocs {
			it, ok := instanceTypeMap[itID]
			if !ok {
				continue
			}
			allocated := lo.Reduce(details, func(acc int, ac cdbm.AllocationConstraint, _ int) int {
				return acc + ac.ConstraintValue
			}, 0)

			apiAllocs := lo.Map(details, func(ac cdbm.AllocationConstraint, _ int) model.APITenantInstanceTypeAllocation {
				return model.APITenantInstanceTypeAllocation{
					ID:    ac.Allocation.ID.String(),
					Name:  ac.Allocation.Name,
					Total: ac.ConstraintValue,
				}
			})

			used := model.APIMachineStatusBreakdown{}
			if tenantITUsed[tID] != nil && tenantITUsed[tID][itID] != nil {
				used = *tenantITUsed[tID][itID]
			}

			maxAlloc := healthyMachineCountByIT[itID] - usedMachinesByIT[itID]
			if maxAlloc < 0 {
				maxAlloc = 0
			}

			ts.InstanceTypes = append(ts.InstanceTypes, model.APITenantInstanceTypeStatsEntry{
				ID:               it.ID.String(),
				Name:             it.Name,
				Allocated:        allocated,
				UsedMachineStats: used,
				MaxAllocatable:   maxAlloc,
				Allocations:      apiAllocs,
			})
		}

		tenantStatsMap[tID] = ts
	}

	result := slices.Collect(maps.Values(tenantStatsMap))

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, result)
}

// ~~~~~ Machine Instance Type Summary Handler ~~~~~ //

// GetMachineInstanceTypeSummaryHandler is the API Handler for retrieving assigned vs unassigned machine summary
type GetMachineInstanceTypeSummaryHandler struct {
	dbSession  *cdb.Session
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetMachineInstanceTypeSummaryHandler initializes and returns a new handler for machine instance type summary
func NewGetMachineInstanceTypeSummaryHandler(dbSession *cdb.Session, cfg *config.Config) GetMachineInstanceTypeSummaryHandler {
	return GetMachineInstanceTypeSummaryHandler{
		dbSession:  dbSession,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Retrieve machine instance type assignment summary for a site
// @Description Returns machine counts grouped by assigned (has instance type) vs unassigned, broken down by status
// @Tags machine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param orgName path string true "Name of NGC organization"
// @Param siteId query string true "Site ID"
// @Success 200 {object} model.APIMachineInstanceTypeSummary
// @Router /v2/org/{orgName}/carbide/machine/instance-type/stats/summary [get]
func (gmitsh GetMachineInstanceTypeSummaryHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Machine", "GetInstanceTypeSummary", c, gmitsh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	infrastructureProvider, apiError := common.IsProvider(ctx, logger, gmitsh.dbSession, org, dbUser, false)
	if apiError != nil {
		return cerr.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	siteIDStr := c.QueryParam("siteId")
	if siteIDStr == "" {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "siteId query parameter is required", nil)
	}
	site, err := common.GetSiteFromIDString(ctx, nil, siteIDStr, gmitsh.dbSession)
	if err != nil {
		logger.Error().Err(err).Str("siteId", siteIDStr).Msg("error parsing or retrieving site")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID specified in query param", nil)
	}

	if site.InfrastructureProviderID != infrastructureProvider.ID {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have access to the specified site", nil)
	}

	siteID := &site.ID

	// Fetch all machines for the site (exclude metadata for performance)
	machineDAO := cdbm.NewMachineDAO(gmitsh.dbSession)
	machines, _, err := machineDAO.GetAll(ctx, nil, cdbm.MachineFilterInput{
		SiteID:          siteID,
		ExcludeMetadata: true,
	}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving machines for site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve machines", nil)
	}

	// Partition into assigned vs unassigned, count by status
	var assigned, unassigned model.APIMachineStatusBreakdown
	for _, m := range machines {
		bd := &assigned
		if m.InstanceTypeID == nil {
			bd = &unassigned
		}
		bd.AddMachineStatusCounts(m)
	}

	result := model.APIMachineInstanceTypeSummary{
		Assigned:   assigned,
		Unassigned: unassigned,
	}

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, result)
}

// ~~~~~ Machine Instance Type Detailed Stats Handler ~~~~~ //

// GetMachineInstanceTypeStatsHandler is the API Handler for retrieving detailed per-instance-type machine stats
type GetMachineInstanceTypeStatsHandler struct {
	dbSession  *cdb.Session
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetMachineInstanceTypeStatsHandler initializes and returns a new handler for machine instance type stats
func NewGetMachineInstanceTypeStatsHandler(dbSession *cdb.Session, cfg *config.Config) GetMachineInstanceTypeStatsHandler {
	return GetMachineInstanceTypeStatsHandler{
		dbSession:  dbSession,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Retrieve detailed per-instance-type machine stats for a site
// @Description Returns machine stats for each instance type including allocation details and tenant breakdown
// @Tags machine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param orgName path string true "Name of NGC organization"
// @Param siteId query string true "Site ID"
// @Success 200 {array} model.APIMachineInstanceTypeStats
// @Router /v2/org/{orgName}/carbide/machine/instance-type/stats [get]
func (gmitsh GetMachineInstanceTypeStatsHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Machine", "GetInstanceTypeStats", c, gmitsh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	infrastructureProvider, apiError := common.IsProvider(ctx, logger, gmitsh.dbSession, org, dbUser, false)
	if apiError != nil {
		return cerr.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	siteIDStr := c.QueryParam("siteId")
	if siteIDStr == "" {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "siteId query parameter is required", nil)
	}
	site, err := common.GetSiteFromIDString(ctx, nil, siteIDStr, gmitsh.dbSession)
	if err != nil {
		logger.Error().Err(err).Str("siteId", siteIDStr).Msg("error parsing or retrieving site")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID specified in query param", nil)
	}

	if site.InfrastructureProviderID != infrastructureProvider.ID {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have access to the specified site", nil)
	}

	siteID := &site.ID
	siteIDs := []uuid.UUID{*siteID}

	// 1. Fetch all instance types for the site
	itDAO := cdbm.NewInstanceTypeDAO(gmitsh.dbSession)
	instanceTypes, _, err := itDAO.GetAll(ctx, nil, cdbm.InstanceTypeFilterInput{SiteIDs: siteIDs},
		nil, nil, nil, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving instance types")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve instance types", nil)
	}

	if len(instanceTypes) == 0 {
		return c.JSON(http.StatusOK, []model.APIMachineInstanceTypeStats{})
	}

	instanceTypeIDs := lo.Map(instanceTypes, func(it cdbm.InstanceType, _ int) uuid.UUID { return it.ID })

	// 2. Fetch all machines for the site (exclude metadata)
	machineDAO := cdbm.NewMachineDAO(gmitsh.dbSession)
	machines, _, err := machineDAO.GetAll(ctx, nil, cdbm.MachineFilterInput{
		SiteID:          siteID,
		ExcludeMetadata: true,
	}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving machines")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve machines", nil)
	}

	machineByID := lo.KeyBy(machines, func(m cdbm.Machine) string { return m.ID })

	assignedMachines := lo.Filter(machines, func(m cdbm.Machine, _ int) bool { return m.InstanceTypeID != nil })
	machinesByIT := lo.GroupBy(assignedMachines, func(m cdbm.Machine) uuid.UUID { return *m.InstanceTypeID })

	// 3. Fetch all allocations for the site (IDs needed to filter constraints)
	aDAO := cdbm.NewAllocationDAO(gmitsh.dbSession)
	allocations, _, err := aDAO.GetAll(ctx, nil, cdbm.AllocationFilterInput{SiteIDs: siteIDs},
		cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving allocations")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve allocations", nil)
	}

	allocationIDs := lo.Map(allocations, func(a cdbm.Allocation, _ int) uuid.UUID { return a.ID })

	// 4. Fetch allocation constraints with Allocation.Tenant
	var constraints []cdbm.AllocationConstraint
	if len(allocationIDs) > 0 {
		acDAO := cdbm.NewAllocationConstraintDAO(gmitsh.dbSession)
		constraints, _, err = acDAO.GetAll(ctx, nil, allocationIDs,
			cdb.GetStrPtr(cdbm.AllocationResourceTypeInstanceType), instanceTypeIDs,
			nil, nil, []string{"Allocation.Tenant"}, nil, cdb.GetIntPtr(cdbp.TotalLimit), nil)
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving allocation constraints")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve allocation constraints", nil)
		}
	}

	// 5. Fetch all instances for the site
	iDAO := cdbm.NewInstanceDAO(gmitsh.dbSession)
	instances, _, err := iDAO.GetAll(ctx, nil, cdbm.InstanceFilterInput{SiteIDs: siteIDs},
		cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving instances")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve instances", nil)
	}

	// Build aggregation maps using shared helpers
	itUsed, tenantITUsed := model.GetInstanceTypeMachineUsageMap(instances, machineByID)
	constraintsByIT := make(map[uuid.UUID][]cdbm.AllocationConstraint)
	for _, ac := range constraints {
		constraintsByIT[ac.ResourceTypeID] = append(constraintsByIT[ac.ResourceTypeID], ac)
	}

	// Build response
	result := lo.Map(instanceTypes, func(it cdbm.InstanceType, _ int) model.APIMachineInstanceTypeStats {
		return model.NewAPIMachineInstanceTypeStats(it, machinesByIT[it.ID], constraintsByIT[it.ID], itUsed, tenantITUsed)
	})

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, result)
}
