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
	"encoding/json"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	temporalClient "go.temporal.io/sdk/client"

	"github.com/google/uuid"

	"github.com/labstack/echo/v4"

	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	"github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"

	"github.com/nvidia/bare-metal-manager-rest/api/internal/config"
	common "github.com/nvidia/bare-metal-manager-rest/api/pkg/api/handler/util/common"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/pagination"
	auth "github.com/nvidia/bare-metal-manager-rest/auth/pkg/authorization"
	cerr "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	cutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
)

// ~~~~~ GetAll Instance NVLinkInterface Handler ~~~~~ //

// GetAllInstanceNVLinkInterfaceHandler is the API Handler for retrieving all NVLinkInterfaces for an Instance
type GetAllInstanceNVLinkInterfaceHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewGetAllInstanceNVLinkInterfaceHandler initializes and returns a new handler for retrieving all NVLinkInterfaces for an Instance
func NewGetAllInstanceNVLinkInterfaceHandler(dbSession *cdb.Session, tc temporalClient.Client, cfg *config.Config) GetAllInstanceNVLinkInterfaceHandler {
	return GetAllInstanceNVLinkInterfaceHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Retrieve all Interfaces for an Instance
// @Description Retrieve all Interfaces for an Instance
// @Tags interface
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param instanceId path string true "ID of Instance"
// @Param status query string false "Filter by status" e.g. 'Pending', 'Error'"
// @Param includeRelation query string false "Related entities to include in response e.g. 'Instance', 'Subnet'"
// @Param pageNumber query integer false "Page number of results returned"
// @Param pageSize query integer false "Number of results per page"
// @Param orderBy query string false "Order by field"
// @Success 200 {object} model.APIInterface
// @Router /v2/org/{org}/carbide/instance/{instance_id}/interface [get]
func (ganvliih GetAllInstanceNVLinkInterfaceHandler) Handle(c echo.Context) error {
	return cerr.NewAPIErrorResponse(c, http.StatusNotImplemented, "Not implemented", nil)
}

// ~~~~~ GetAll NVLinkInterface Handler ~~~~~ //

// GetAllNVLinkInterfaceHandler is the API Handler for retrieving all NVLinkInterfaces
type GetAllNVLinkInterfaceHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewGetAllNVLinkInterfaceHandler initializes and returns a new handler for retrieving all NVLinkInterfaces
func NewGetAllNVLinkInterfaceHandler(dbSession *cdb.Session, tc temporalClient.Client, cfg *config.Config) GetAllNVLinkInterfaceHandler {
	return GetAllNVLinkInterfaceHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Retrieve all NVLinkInterfaces
// @Description Retrieve all NVLinkInterfaces
// @Tags NVLinkInterface
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param siteId query string true "ID of Site"
// @Param instanceId path string true "ID of Instance"
// @Param nvlinkLogicalPartitionId path string true "ID of NVLinkLogicalPartition"
// @Param nvLinkDomainId path string true "ID of NVLinkDomain"
// @Param status query string false "Filter by status" e.g. 'Pending', 'Error'"
// @Param includeRelation query string false "Related entities to include in response e.g. 'NVLinkLogicalPartition, Instance'"
// @Param pageNumber query integer false "Page number of results returned"
// @Param pageSize query integer false "Number of results per page"
// @Param orderBy query string false "Order by field"
// @Success 200 {object} model.APINVLinkInterface
// @Router /v2/org/{org}/carbide/nvlink-interface [get]
func (gaish GetAllNVLinkInterfaceHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("NVLinkInterface", "GetAll", c, gaish.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}
	if dbUser == nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org
	ok, err := auth.ValidateOrgMembership(dbUser, org)
	if !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership for User in request")
		} else {
			logger.Warn().Msg("could not validate org membership for user, access denied")
		}
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	// Validate role, only Tenant Admins are allowed to retrieve Instances
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Tenant Admin role, access denied")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}

	// Validate pagination request
	pageRequest := pagination.PageRequest{}
	err = c.Bind(&pageRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding pagination request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request pagination data", nil)
	}

	// Validate pagination request attributes
	err = pageRequest.Validate(cdbm.NVLinkInterfaceOrderByFields)
	if err != nil {
		logger.Warn().Err(err).Msg("error validating pagination request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest,
			"Failed to validate pagination request data", err)
	}

	// Get and validate includeRelation params
	qParams := c.QueryParams()
	qIncludeRelations, errMsg := common.GetAndValidateQueryRelations(qParams, cdbm.NVLinkInterfaceRelatedEntities)
	if errMsg != "" {
		logger.Warn().Msg(errMsg)
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, errMsg, nil)
	}

	// Get Tenant for this org
	tnDAO := cdbm.NewTenantDAO(gaish.dbSession)

	tenants, err := tnDAO.GetAllByOrg(ctx, nil, org, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Tenant for this org")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant", nil)
	}

	if len(tenants) == 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Org does not have a Tenant associated", nil)
	}
	tenant := tenants[0]

	// Get site ID from query param
	var siteIDs []uuid.UUID
	siteIDStr := qParams["siteId"]
	tsDAO := cdbm.NewTenantSiteDAO(gaish.dbSession)
	for _, siteIDStr := range siteIDStr {
		site, err := common.GetSiteFromIDString(ctx, nil, siteIDStr, gaish.dbSession)
		if err != nil {
			logger.Warn().Err(err).Msg("error getting site in request")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to retrieve Site specified in query param, invalid ID or DB error", nil)
		}
		siteIDs = append(siteIDs, site.ID)

		// Check Site association with Tenant
		_, err = tsDAO.GetByTenantIDAndSiteID(ctx, nil, tenant.ID, site.ID, nil)
		if err != nil {
			if err == cdb.ErrDoesNotExist {
				return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant does not have access to this Site", nil)
			}
			logger.Error().Err(err).Msg("error retrieving TenantSite from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to determine Tenant access to Site, DB error", nil)
		}
	}

	// Get Instance ID from query param
	var instanceIDs []uuid.UUID
	instanceIDStr := qParams["instanceId"]
	instanceDAO := cdbm.NewInstanceDAO(gaish.dbSession)
	for _, instanceIDStr := range instanceIDStr {
		instanceID, err := uuid.Parse(instanceIDStr)
		if err != nil {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Instance ID in URL", nil)
		}

		// Get Instance
		instance, err := instanceDAO.GetByID(ctx, nil, instanceID, nil)
		if err != nil {
			if err == cdb.ErrDoesNotExist {
				return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find Instance with specified ID", nil)
			}
			logger.Error().Err(err).Msg("error retrieving Instance from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Instance", nil)
		}

		// Check if Instance belongs to Tenant
		if instance.TenantID != tenant.ID {
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Instance does not belong to current Tenant", nil)
		}

		instanceIDs = append(instanceIDs, instanceID)
	}

	// Get NVLink Logical Partition ID from URL param
	var nvlinkLogicalPartitionIDs []uuid.UUID
	nvlinkLogicalPartitionIDStr := qParams["nvLinkLogicalPartitionId"]

	nvllpDAO := cdbm.NewNVLinkLogicalPartitionDAO(gaish.dbSession)
	for _, nvlinkLogicalPartitionIDStr := range nvlinkLogicalPartitionIDStr {
		nvlinkLogicalPartitionID, err := uuid.Parse(nvlinkLogicalPartitionIDStr)
		if err != nil {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid NVLink Logical Partition ID in URL", nil)
		}

		// Get NVLink Logical Partition
		nvllp, err := nvllpDAO.GetByID(ctx, nil, nvlinkLogicalPartitionID, nil)
		if err != nil {
			if err == cdb.ErrDoesNotExist {
				return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find NVLink Logical Partition with specified ID", nil)
			}
			logger.Error().Err(err).Msg("error retrieving NVLink Logical Partition from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve NVLink Logical Partition", nil)
		}

		// Check if NVLink Logical Partition belongs to Tenant
		if nvllp.TenantID != tenant.ID {
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "NVLink Logical Partition does not belong to current Tenant", nil)
		}

		nvlinkLogicalPartitionIDs = append(nvlinkLogicalPartitionIDs, nvlinkLogicalPartitionID)
	}

	// Get NVLink Domain ID from query param
	var nvlinkDomainIDs []uuid.UUID
	nvlinkDomainIDStr := qParams["nvLinkDomainId"]
	for _, nvlinkDomainIDStr := range nvlinkDomainIDStr {
		nvlinkDomainID, err := uuid.Parse(nvlinkDomainIDStr)
		if err != nil {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid NVLink Domain ID in query param", nil)
		}
		nvlinkDomainIDs = append(nvlinkDomainIDs, nvlinkDomainID)
	}

	// Get status from query param
	var statuses []string
	statusQuery := qParams["status"]
	for _, statusQuery := range statusQuery {
		gaish.tracerSpan.SetAttribute(handlerSpan, attribute.String("status", statusQuery), logger)
		_, ok := cdbm.NVLinkInterfaceStatusMap[statusQuery]
		if !ok {
			logger.Warn().Msg(fmt.Sprintf("invalid value in status query: %v", statusQuery))
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Status value in query", nil)
		}
		statuses = append(statuses, statusQuery)
	}

	// Get the NVLink Logical Partition NVLink Interfaces record from the db
	nvlIfcDAO := cdbm.NewNVLinkInterfaceDAO(gaish.dbSession)

	filterInput := cdbm.NVLinkInterfaceFilterInput{
		SiteIDs:                   siteIDs,
		InstanceIDs:               instanceIDs,
		NVLinkLogicalPartitionIDs: nvlinkLogicalPartitionIDs,
		NVLinkDomainIDs:           nvlinkDomainIDs,
		Statuses:                  statuses,
	}

	pageInput := paginator.PageInput{
		Limit:   pageRequest.Limit,
		Offset:  pageRequest.Offset,
		OrderBy: pageRequest.OrderBy,
	}

	dbNVLinkInterfaces, total, err := nvlIfcDAO.GetAll(ctx, nil, filterInput, pageInput, qIncludeRelations)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving NVLink Interface Details from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve NVLink Interface ", nil)
	}

	// Create response
	apiNVLinkInterfaces := []model.APINVLinkInterface{}
	for _, dbnvlifc := range dbNVLinkInterfaces {
		curnvlifc := dbnvlifc
		apiNVLinkInterfaces = append(apiNVLinkInterfaces, *model.NewAPINVLinkInterface(&curnvlifc))
	}

	// Create pagination response header
	pageReponse := pagination.NewPageResponse(*pageRequest.PageNumber, *pageRequest.PageSize, total, pageRequest.OrderByStr)
	pageHeader, err := json.Marshal(pageReponse)
	if err != nil {
		logger.Error().Err(err).Msg("error marshaling pagination response")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to generate pagination response header", nil)
	}
	c.Response().Header().Set(pagination.ResponseHeaderName, string(pageHeader))

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusOK, apiNVLinkInterfaces)
}
