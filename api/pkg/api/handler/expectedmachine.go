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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nvidia/bare-metal-manager-rest/api/internal/config"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/handler/util/common"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model/util"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/pagination"
	sc "github.com/nvidia/bare-metal-manager-rest/api/pkg/client/site"
	cutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	cwutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	"github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"
	cwssaws "github.com/nvidia/bare-metal-manager-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/nvidia/bare-metal-manager-rest/workflow/pkg/queue"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	tclient "go.temporal.io/sdk/client"
)

// ValidateProviderOrTenantSiteAccess validates if the provider or tenant has access to the site
func ValidateProviderOrTenantSiteAccess(ctx context.Context, logger zerolog.Logger, dbSession *cdb.Session, site *cdbm.Site, infrastructureProvider *cdbm.InfrastructureProvider, tenant *cdbm.Tenant) (bool, *cutil.APIError) {
	hasAccess := false

	// Validate if Provider has access to the Site
	if infrastructureProvider != nil && site.InfrastructureProviderID == infrastructureProvider.ID {
		hasAccess = true
	}

	if !hasAccess && tenant != nil {
		// Check Tenant Site relationship
		tsDAO := cdbm.NewTenantSiteDAO(dbSession)
		_, tsCount, err := tsDAO.GetAll(ctx, nil, cdbm.TenantSiteFilterInput{
			TenantIDs: []uuid.UUID{tenant.ID},
			SiteIDs:   []uuid.UUID{site.ID},
		}, paginator.PageInput{}, []string{})
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving Tenant Site relationship")
			return false, cutil.NewAPIError(http.StatusInternalServerError, "Failed to check Tenant/Site association due to DB error", nil)
		}

		hasAccess = tsCount > 0

		// Check if Tenant is privileged
		if !hasAccess && tenant.Config.TargetedInstanceCreation {
			// Check if privileged tenant has an account with the Site's Infrastructure Provider
			taDAO := cdbm.NewTenantAccountDAO(dbSession)
			_, taCount, err := taDAO.GetAll(ctx, nil, cdbm.TenantAccountFilterInput{
				InfrastructureProviderID: &site.InfrastructureProviderID,
				TenantIDs:                []uuid.UUID{tenant.ID},
			}, paginator.PageInput{}, []string{})
			if err != nil {
				logger.Error().Err(err).Msg("error retrieving Tenant Account for Site")
				return false, cutil.NewAPIError(http.StatusInternalServerError, "Failed to retrieve Tenant's Account with Site's Provider due to DB error", nil)
			}

			hasAccess = taCount > 0
		}
	}

	return hasAccess, nil
}

// ~~~~~ Create Handler ~~~~~ //

// CreateExpectedMachineHandler is the API Handler for creating new ExpectedMachine
type CreateExpectedMachineHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewCreateExpectedMachineHandler initializes and returns a new handler for creating ExpectedMachine
func NewCreateExpectedMachineHandler(dbSession *cdb.Session, tc tclient.Client, scp *sc.ClientPool, cfg *config.Config) CreateExpectedMachineHandler {
	return CreateExpectedMachineHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        scp,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Create an ExpectedMachine
// @Description Create an ExpectedMachine
// @Tags ExpectedMachine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param message body model.APIExpectedMachineCreateRequest true "ExpectedMachine creation request"
// @Success 201 {object} model.APIExpectedMachine
// @Router /v2/org/{org}/carbide/expected-machine [post]
func (cemh CreateExpectedMachineHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Create", "ExpectedMachine", c, cemh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}
	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// ensure our user is a provider or tenant for the org
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, cemh.dbSession, org, dbUser, false, true)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Validate request
	// Bind request data to API model
	apiRequest := model.APIExpectedMachineCreateRequest{}
	err := c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate()
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating Expected Machine creation request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate Expected Machine creation data", verr)
	}

	// Validate that SKU exists if specified
	if apiRequest.SkuID != nil {
		skuDAO := cdbm.NewSkuDAO(cemh.dbSession)
		_, err = skuDAO.Get(ctx, nil, *apiRequest.SkuID)
		if err != nil {
			if errors.Is(err, cdb.ErrDoesNotExist) {
				logger.Warn().Msg("SKU ID specified in request does not exist")
				return cutil.NewAPIErrorResponse(c, http.StatusUnprocessableEntity, "SKU ID specified in request does not exist", nil)
			}
			logger.Warn().Err(err).Msg("error validating SKU ID in request data")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate SKU ID in request data due to DB error", nil)
		}
	}

	// Retrieve the Site from the DB
	site, err := common.GetSiteFromIDString(ctx, nil, apiRequest.SiteID, cemh.dbSession)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data does not exist", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site specified in request data due to DB error", nil)
	}

	// Validate ProviderTenantSite relationship and site state
	hasAccess, apiError := ValidateProviderOrTenantSiteAccess(ctx, logger, cemh.dbSession, site, infrastructureProvider, tenant)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	if !hasAccess {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have access to Site", nil)
	}

	// Check if Site is in Registered state
	if site.Status != cdbm.SiteStatusRegistered {
		logger.Warn().Msg("Site is not in Registered state")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site is not in Registered state, cannot perform operation", nil)
	}

	// Check for duplicate MAC address
	// Notes: We do not allow multiple Expected Machines with the same MAC address, but it's not a DB unique constraint so we check here
	emDAO := cdbm.NewExpectedMachineDAO(cemh.dbSession)
	ems, count, err := emDAO.GetAll(ctx, nil, cdbm.ExpectedMachineFilterInput{
		BmcMacAddresses: []string{apiRequest.BmcMacAddress},
		SiteIDs:         []uuid.UUID{site.ID},
	}, paginator.PageInput{
		Limit: cdb.GetIntPtr(1),
	}, nil)

	if err != nil {
		logger.Error().Err(err).Msg("error checking for duplicate MAC address on Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to validate MAC address uniqueness on Site due to DB error", nil)
	}

	if count > 0 {
		logger.Warn().Str("MacAddress", apiRequest.BmcMacAddress).Msg("Expected Machine with specified MAC address already exists on Site")

		return cutil.NewAPIErrorResponse(c, http.StatusConflict, "Expected Machine with specified MAC address already exists on Site", validation.Errors{
			"id": errors.New(ems[0].ID.String()),
		})
	}

	// Start a db transaction
	tx, err := cdb.BeginTx(ctx, cemh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Expected Machine due to DB transaction error", nil)
	}
	// this variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Create the ExpectedMachine in DB
	// Note: DefaultBmcUsername and BmcPassword are not stored in DB, only passed to workflow
	expectedMachine, err := emDAO.Create(
		ctx,
		tx,
		cdbm.ExpectedMachineCreateInput{
			ExpectedMachineID:        uuid.New(),
			SiteID:                   site.ID,
			BmcMacAddress:            apiRequest.BmcMacAddress,
			ChassisSerialNumber:      apiRequest.ChassisSerialNumber,
			SkuID:                    apiRequest.SkuID,
			FallbackDpuSerialNumbers: apiRequest.FallbackDPUSerialNumbers,
			Labels:                   apiRequest.Labels,
			CreatedBy:                dbUser.ID,
		},
	)
	if err != nil {
		logger.Error().Err(err).Msg("error creating ExpectedMachine record in DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Expected Machine due to DB error", nil)
	}

	// Build the create request for workflow
	// BMC credentials come from API request since they're not stored in DB
	createExpectedMachineRequest := &cwssaws.ExpectedMachine{
		Id:                       &cwssaws.UUID{Value: expectedMachine.ID.String()},
		BmcMacAddress:            expectedMachine.BmcMacAddress,
		ChassisSerialNumber:      expectedMachine.ChassisSerialNumber,
		FallbackDpuSerialNumbers: expectedMachine.FallbackDpuSerialNumbers,
		SkuId:                    expectedMachine.SkuID,
	}

	if apiRequest.DefaultBmcUsername != nil {
		createExpectedMachineRequest.BmcUsername = *apiRequest.DefaultBmcUsername
	}

	if apiRequest.DefaultBmcPassword != nil {
		createExpectedMachineRequest.BmcPassword = *apiRequest.DefaultBmcPassword
	}

	protoLabels := util.ProtobufLabelsFromAPILabels(apiRequest.Labels)
	if protoLabels != nil {
		createExpectedMachineRequest.Metadata = &cwssaws.Metadata{
			Labels: protoLabels,
		}
	}

	logger.Info().Msg("triggering Expected Machine create workflow on Site")

	// Create workflow options
	workflowOptions := tclient.StartWorkflowOptions{
		ID:                       "expected-machine-create-" + expectedMachine.ID.String(),
		WorkflowExecutionTimeout: cwutil.WorkflowExecutionTimeout,
		TaskQueue:                queue.SiteTaskQueue,
	}

	// Get the temporal client for the site we are working with
	stc, err := cemh.scp.GetClientByID(site.ID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	// Run workflow
	apiErr := common.ExecuteSyncWorkflow(ctx, logger, stc, "CreateExpectedMachine", workflowOptions, createExpectedMachineRequest)
	if apiErr != nil {
		return cutil.NewAPIErrorResponse(c, apiErr.Code, apiErr.Message, apiErr.Data)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing ExpectedMachine transaction to DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Expected Machine due to DB transaction error", nil)
	}
	// Set committed so, deferred cleanup functions will do nothing
	txCommitted = true

	// Create response
	apiExpectedMachine := model.NewAPIExpectedMachine(expectedMachine)

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusCreated, apiExpectedMachine)
}

// ~~~~~ GetAll Handler ~~~~~ //

// GetAllExpectedMachineHandler is the API Handler for getting all ExpectedMachines
type GetAllExpectedMachineHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewGetAllExpectedMachineHandler initializes and returns a new handler for getting all ExpectedMachines
func NewGetAllExpectedMachineHandler(dbSession *cdb.Session, tc tclient.Client, cfg *config.Config) GetAllExpectedMachineHandler {
	return GetAllExpectedMachineHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get all ExpectedMachines
// @Description Get all ExpectedMachines
// @Tags ExpectedMachine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param siteId query string false "ID of Site (optional, filters results to specific site)"
// @Param pageNumber query integer false "Page number of results returned"
// @Param includeRelation query string false "Related entities to include in response e.g. 'Site', 'SKU'"
// @Param pageSize query integer false "Number of results per page"
// @Param orderBy query string false "Order by field"
// @Success 200 {object} []model.APIExpectedMachine
// @Router /v2/org/{org}/carbide/expected-machine [get]
func (gaemh GetAllExpectedMachineHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("GetAll", "ExpectedMachine", c, gaemh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}
	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// ensure our user is a provider for the org
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, gaemh.dbSession, org, dbUser, true, true)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	filterInput := cdbm.ExpectedMachineFilterInput{}

	// Get Site ID from query param if specified
	siteIDStr := c.QueryParam("siteId")
	if siteIDStr != "" {
		site, err := common.GetSiteFromIDString(ctx, nil, siteIDStr, gaemh.dbSession)
		if err != nil {
			if errors.Is(err, cdb.ErrDoesNotExist) {
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data does not exist", nil)
			}
			logger.Error().Err(err).Msg("error retrieving Site from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site specified in request data due to DB error", nil)
		}

		// Validate ProviderTenantSite relationship and site state
		hasAccess, apiError := ValidateProviderOrTenantSiteAccess(ctx, logger, gaemh.dbSession, site, infrastructureProvider, tenant)
		if apiError != nil {
			return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
		}

		if !hasAccess {
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Current org is not associated with the Site specified in query", nil)
		}

		filterInput.SiteIDs = []uuid.UUID{site.ID}
	} else if tenant != nil {
		// Tenants must specify a Site ID
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site ID must be specified in query when retrieving Expected Machines as a Tenant", nil)
	} else {
		// Get all Sites for the org's Infrastructure Provider
		siteDAO := cdbm.NewSiteDAO(gaemh.dbSession)
		sites, _, err := siteDAO.GetAll(ctx, nil,
			cdbm.SiteFilterInput{InfrastructureProviderIDs: []uuid.UUID{infrastructureProvider.ID}},
			paginator.PageInput{Limit: cdb.GetIntPtr(math.MaxInt)},
			nil,
		)
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving Sites from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Sites for org due to DB error", nil)
		}

		siteIDs := make([]uuid.UUID, 0, len(sites))
		for _, site := range sites {
			siteIDs = append(siteIDs, site.ID)
		}
		filterInput.SiteIDs = siteIDs
	}

	// Get and validate includeRelation params
	qParams := c.QueryParams()
	qIncludeRelations, errStr := common.GetAndValidateQueryRelations(qParams, cdbm.ExpectedMachineRelatedEntities)
	if errStr != "" {
		logger.Warn().Msg(errStr)
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, errStr, nil)
	}

	// Validate pagination request
	pageRequest := pagination.PageRequest{}
	err := c.Bind(&pageRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding pagination request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request pagination data", nil)
	}

	// Validate pagination attributes
	err = pageRequest.Validate(cdbm.ExpectedMachineOrderByFields)
	if err != nil {
		logger.Warn().Err(err).Msg("error validating pagination request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate pagination request data", err)
	}

	// Get Expected Machines from DB
	emDAO := cdbm.NewExpectedMachineDAO(gaemh.dbSession)
	expectedMachines, total, err := emDAO.GetAll(
		ctx,
		nil,
		filterInput,
		paginator.PageInput{
			Offset:  pageRequest.Offset,
			Limit:   pageRequest.Limit,
			OrderBy: pageRequest.OrderBy,
		}, qIncludeRelations,
	)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Expected Machines from db")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Expected Machines due to DB error", nil)
	}

	// Create response
	apiExpectedMachines := []*model.APIExpectedMachine{}
	for _, em := range expectedMachines {
		apiExpectedMachine := model.NewAPIExpectedMachine(&em)
		apiExpectedMachines = append(apiExpectedMachines, apiExpectedMachine)
	}

	// Create pagination response header
	pageResponse := pagination.NewPageResponse(*pageRequest.PageNumber, *pageRequest.PageSize, total, pageRequest.OrderByStr)
	pageHeader, err := json.Marshal(pageResponse)
	if err != nil {
		logger.Error().Err(err).Msg("error marshaling pagination response")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to generate pagination response header", nil)
	}

	c.Response().Header().Set(pagination.ResponseHeaderName, string(pageHeader))

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusOK, apiExpectedMachines)
}

// ~~~~~ Get Handler ~~~~~ //

// GetExpectedMachineHandler is the API Handler for retrieving ExpectedMachine
type GetExpectedMachineHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewGetExpectedMachineHandler initializes and returns a new handler to retrieve ExpectedMachine
func NewGetExpectedMachineHandler(dbSession *cdb.Session, tc tclient.Client, cfg *config.Config) GetExpectedMachineHandler {
	return GetExpectedMachineHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Retrieve the ExpectedMachine
// @Description Retrieve the ExpectedMachine by ID
// @Tags ExpectedMachine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Expected Machine"
// @Param includeRelation query string false "Related entities to include in response e.g. 'Site', 'SKU'"
// @Success 200 {object} model.APIExpectedMachine
// @Router /v2/org/{org}/carbide/expected-machine/{id} [get]
func (gemh GetExpectedMachineHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Get", "ExpectedMachine", c, gemh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}
	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// ensure our user is a provider for the org
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, gemh.dbSession, org, dbUser, true, true)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Get Expected Machine ID from URL param
	expectedMachineIDStr := c.Param("id")
	expectedMachineID, err := uuid.Parse(expectedMachineIDStr)
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Expected Machine ID in URL", nil)
	}

	logger = logger.With().Str("ExpectedMachineID", expectedMachineID.String()).Logger()

	gemh.tracerSpan.SetAttribute(handlerSpan, attribute.String("expected_machine_id", expectedMachineID.String()), logger)

	// Get and validate includeRelation params
	qParams := c.QueryParams()
	qIncludeRelations, errStr := common.GetAndValidateQueryRelations(qParams, cdbm.ExpectedMachineRelatedEntities)
	if errStr != "" {
		logger.Warn().Msg(errStr)
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, errStr, nil)
	}

	// Get ExpectedMachine from DB by ID
	emDAO := cdbm.NewExpectedMachineDAO(gemh.dbSession)
	expectedMachine, err := emDAO.Get(ctx, nil, expectedMachineID, qIncludeRelations, false)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find Expected Machine with ID: %s", expectedMachineID.String()), nil)
		}
		logger.Error().Err(err).Msg("error retrieving Expected Machine from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Expected Machine due to DB error", nil)
	}

	// Get Site for the Expected Machine
	siteDAO := cdbm.NewSiteDAO(gemh.dbSession)
	site, err := siteDAO.GetByID(ctx, nil, expectedMachine.SiteID, nil, false)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site details for Expected Machine due to DB error", nil)
	}

	// Validate ProviderTenantSite relationship and site state
	hasAccess, apiError := ValidateProviderOrTenantSiteAccess(ctx, logger, gemh.dbSession, site, infrastructureProvider, tenant)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	if !hasAccess {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Current org is not associated with the Site of the Expected Machine", nil)
	}

	// Create response
	apiExpectedMachine := model.NewAPIExpectedMachine(expectedMachine)

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, apiExpectedMachine)
}

// ~~~~~ Update Handler ~~~~~ //

// UpdateExpectedMachineHandler is the API Handler for updating a ExpectedMachine
type UpdateExpectedMachineHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewUpdateExpectedMachineHandler initializes and returns a new handler for updating ExpectedMachine
func NewUpdateExpectedMachineHandler(dbSession *cdb.Session, tc tclient.Client, scp *sc.ClientPool, cfg *config.Config) UpdateExpectedMachineHandler {
	return UpdateExpectedMachineHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        scp,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Update an existing ExpectedMachine
// @Description Update an existing ExpectedMachine by ID
// @Tags ExpectedMachine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Expected Machine"
// @Param message body model.APIExpectedMachineUpdateRequest true "ExpectedMachine update request"
// @Success 200 {object} model.APIExpectedMachine
// @Router /v2/org/{org}/carbide/expected-machine/{id} [patch]
func (uemh UpdateExpectedMachineHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Update", "ExpectedMachine", c, uemh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Ensure our user is a provider for the org
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, uemh.dbSession, org, dbUser, false, true)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Get Expected Machine ID from URL param
	expectedMachineID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Expected Machine ID in URL", nil)
	}
	logger = logger.With().Str("ExpectedMachineID", expectedMachineID.String()).Logger()

	uemh.tracerSpan.SetAttribute(handlerSpan, attribute.String("expected_machine_id", expectedMachineID.String()), logger)

	// Validate request
	// Bind request data to API model
	apiRequest := model.APIExpectedMachineUpdateRequest{}
	err = c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}
	// Validate request attributes
	verr := apiRequest.Validate()
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating ExpectedMachine update request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate ExpectedMachine update request data", verr)
	}

	// If ID is provided in body, it must match the path ID
	if apiRequest.ID != nil && *apiRequest.ID != expectedMachineID.String() {
		logger.Warn().
			Str("URLID", expectedMachineID.String()).
			Str("RequestDataID", *apiRequest.ID).
			Msg("Mismatched Expected Machine ID between path and body")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "If provided, Expected Machine ID specified in request data must match URL request value", nil)
	}

	// Validate that SKU exists if specified
	if apiRequest.SkuID != nil {
		skuDAO := cdbm.NewSkuDAO(uemh.dbSession)
		_, err = skuDAO.Get(ctx, nil, *apiRequest.SkuID)
		if err != nil {
			if errors.Is(err, cdb.ErrDoesNotExist) {
				logger.Warn().Msg("SKU ID specified in request does not exist")
				return cutil.NewAPIErrorResponse(c, http.StatusUnprocessableEntity, "SKU ID specified in request does not exist", nil)
			}
			logger.Warn().Err(err).Msg("error validating SKU ID in request data")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate SKU ID in request data due to DB error", nil)
		}
	}

	// Get ExpectedMachine from DB by ID
	emDAO := cdbm.NewExpectedMachineDAO(uemh.dbSession)
	expectedMachine, err := emDAO.Get(ctx, nil, expectedMachineID, []string{cdbm.SiteRelationName}, false)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find Expected Machine with ID: %s", expectedMachineID.String()), nil)
		}
		logger.Error().Err(err).Msg("error retrieving Expected Machine from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Expected Machine due to DB error", nil)
	}

	// Validate that Site relation exists for the Expected Machine
	site := expectedMachine.Site
	if site == nil {
		logger.Error().Msg("no Site relation found for Expected Machine")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site details for Expected Machine", nil)
	}

	// Validate ProviderTenantSite relationship and site state
	hasAccess, apiError := ValidateProviderOrTenantSiteAccess(ctx, logger, uemh.dbSession, site, infrastructureProvider, tenant)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	if !hasAccess {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Current org is not associated with the Site of the Expected Machine", nil)
	}

	// Start a db tx
	tx, err := cdb.BeginTx(ctx, uemh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Expected Machine due to DB transaction error", nil)
	}
	// this variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Update ExpectedMachine in DB
	// Note: DefaultBmcUsername and BmcPassword are not stored in DB, only passed to workflow

	updatedExpectedMachine, err := emDAO.Update(
		ctx,
		tx,
		cdbm.ExpectedMachineUpdateInput{
			ExpectedMachineID:        expectedMachine.ID,
			BmcMacAddress:            apiRequest.BmcMacAddress,
			ChassisSerialNumber:      apiRequest.ChassisSerialNumber,
			SkuID:                    apiRequest.SkuID,
			FallbackDpuSerialNumbers: apiRequest.FallbackDPUSerialNumbers,
			Labels:                   apiRequest.Labels,
		},
	)
	if err != nil {
		logger.Error().Err(err).Msg("failed to update ExpectedMachine record in DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Expected Machine due to DB error", nil)
	}

	// Build the update request for workflow
	// BMC credentials come from API request since they're not stored in DB
	updateExpectedMachineRequest := &cwssaws.ExpectedMachine{
		Id:                       &cwssaws.UUID{Value: expectedMachine.ID.String()},
		BmcMacAddress:            updatedExpectedMachine.BmcMacAddress,
		ChassisSerialNumber:      updatedExpectedMachine.ChassisSerialNumber,
		FallbackDpuSerialNumbers: updatedExpectedMachine.FallbackDpuSerialNumbers,
		SkuId:                    updatedExpectedMachine.SkuID,
	}

	if apiRequest.DefaultBmcUsername != nil {
		updateExpectedMachineRequest.BmcUsername = *apiRequest.DefaultBmcUsername
	}

	if apiRequest.DefaultBmcPassword != nil {
		updateExpectedMachineRequest.BmcPassword = *apiRequest.DefaultBmcPassword
	}

	protoLabels := util.ProtobufLabelsFromAPILabels(apiRequest.Labels)
	if protoLabels != nil {
		updateExpectedMachineRequest.Metadata = &cwssaws.Metadata{
			Labels: protoLabels,
		}
	}

	logger.Info().Msg("triggering ExpectedMachine update workflow")

	// Create workflow options
	workflowOptions := tclient.StartWorkflowOptions{
		ID:                       "expected-machine-update-" + expectedMachine.ID.String(),
		WorkflowExecutionTimeout: cwutil.WorkflowExecutionTimeout,
		TaskQueue:                queue.SiteTaskQueue,
	}

	// Get the Temporal client for the site we are working with
	stc, err := uemh.scp.GetClientByID(site.ID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	// Run workflow
	apiErr := common.ExecuteSyncWorkflow(ctx, logger, stc, "UpdateExpectedMachine", workflowOptions, updateExpectedMachineRequest)
	if apiErr != nil {
		return cutil.NewAPIErrorResponse(c, apiErr.Code, apiErr.Message, apiErr.Data)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing ExpectedMachine update transaction to DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update ExpectedMachine", nil)
	}
	// Set committed so, deferred cleanup functions will do nothing
	txCommitted = true

	// Create response
	apiExpectedMachine := model.NewAPIExpectedMachine(updatedExpectedMachine)

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusOK, apiExpectedMachine)
}

// ~~~~~ Delete Handler ~~~~~ //

// DeleteExpectedMachineHandler is the API Handler for deleting a ExpectedMachine
type DeleteExpectedMachineHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewDeleteExpectedMachineHandler initializes and returns a new handler for deleting ExpectedMachine
func NewDeleteExpectedMachineHandler(dbSession *cdb.Session, tc tclient.Client, scp *sc.ClientPool, cfg *config.Config) DeleteExpectedMachineHandler {
	return DeleteExpectedMachineHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        scp,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Delete an existing ExpectedMachine
// @Description Delete an existing ExpectedMachine by ID
// @Tags ExpectedMachine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Expected Machine"
// @Success 204
// @Router /v2/org/{org}/carbide/expected-machine/{id} [delete]
func (demh DeleteExpectedMachineHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Delete", "ExpectedMachine", c, demh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}
	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Ensure our user is a provider for the org
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, demh.dbSession, org, dbUser, false, true)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Get Expected Machine ID from URL param
	expectedMachineID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Expected Machine ID in URL", nil)
	}
	logger = logger.With().Str("ExpectedMachineID", expectedMachineID.String()).Logger()

	demh.tracerSpan.SetAttribute(handlerSpan, attribute.String("expected_machine_id", expectedMachineID.String()), logger)

	// Get ExpectedMachine from DB by ID
	emDAO := cdbm.NewExpectedMachineDAO(demh.dbSession)
	expectedMachine, err := emDAO.Get(ctx, nil, expectedMachineID, []string{cdbm.SiteRelationName}, false)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find Expected Machine with ID: %s", expectedMachineID.String()), nil)
		}
		logger.Error().Err(err).Msg("error retrieving Expected Machine from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Expected Machine due to DB error", nil)
	}

	// Validate that Site relation exists for the Expected Machine
	site := expectedMachine.Site
	if site == nil {
		logger.Error().Msg("no Site relation found for Expected Machine")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site details for Expected Machine", nil)
	}

	// Validate ProviderTenantSite relationship and site state
	hasAccess, apiError := ValidateProviderOrTenantSiteAccess(ctx, logger, demh.dbSession, site, infrastructureProvider, tenant)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	if !hasAccess {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Current org is not associated with the Site of the Expected Machine", nil)
	}

	// Start a db tx
	tx, err := cdb.BeginTx(ctx, demh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Expected Machine due to DB error", nil)
	}
	// this variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Delete ExpectedMachine from DB
	err = emDAO.Delete(ctx, tx, expectedMachine.ID)
	if err != nil {
		logger.Error().Err(err).Msg("unable to delete ExpectedMachine record from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Expected Machine due to DB error", nil)
	}

	// Build the delete request for workflow
	deleteExpectedMachineRequest := &cwssaws.ExpectedMachineRequest{
		Id: &cwssaws.UUID{Value: expectedMachine.ID.String()},
	}

	logger.Info().Msg("triggering ExpectedMachine delete workflow")

	// Create workflow options
	workflowOptions := tclient.StartWorkflowOptions{
		ID:                       "expected-machine-delete-" + expectedMachine.ID.String(),
		WorkflowExecutionTimeout: cwutil.WorkflowExecutionTimeout,
		TaskQueue:                queue.SiteTaskQueue,
	}

	// Get the temporal client for the site we are working with
	stc, err := demh.scp.GetClientByID(site.ID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	// Run workflow
	apiErr := common.ExecuteSyncWorkflow(ctx, logger, stc, "DeleteExpectedMachine", workflowOptions, deleteExpectedMachineRequest)
	if apiErr != nil {
		return cutil.NewAPIErrorResponse(c, apiErr.Code, apiErr.Message, apiErr.Data)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing ExpectedMachine delete transaction to DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Expected Machine due to DB transaction error", nil)
	}
	// Set committed so, deferred cleanup functions will do nothing
	txCommitted = true

	logger.Info().Msg("finishing API handler")

	return c.NoContent(http.StatusNoContent)
}

// ~~~~~ CreateExpectedMachines Handler ~~~~~ //

// CreateExpectedMachinesHandler is the API Handler for creating multiple ExpectedMachines
type CreateExpectedMachinesHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewCreateExpectedMachinesHandler initializes and returns a new handler for creating multiple ExpectedMachines
func NewCreateExpectedMachinesHandler(dbSession *cdb.Session, tc tclient.Client, scp *sc.ClientPool, cfg *config.Config) CreateExpectedMachinesHandler {
	return CreateExpectedMachinesHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        scp,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Create multiple ExpectedMachines
// @Description Create multiple ExpectedMachines in a single request. All machines must belong to the same site.
// @Tags ExpectedMachine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param message body []model.APIExpectedMachineCreateRequest true "ExpectedMachine batch creation request"
// @Success 201 {object} model.APIExpectedMachineBatchResponse
// @Router /v2/org/{org}/carbide/expected-machine/batch [post]
func (cemh CreateExpectedMachinesHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("CreateMultiple", "ExpectedMachine", c, cemh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// ensure our user is a provider or tenant for the org
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, cemh.dbSession, org, dbUser, false, true)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Validate request
	// Bind request data to API model (array payload)
	apiRequests := []model.APIExpectedMachineCreateRequest{}
	err := c.Bind(&apiRequests)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	if len(apiRequests) == 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Request data must contain at least 1 Expected Machine entry", nil)
	}

	if len(apiRequests) > model.ExpectedMachineMaxBatchItems {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("At most %d Expected Machine entries can be created in a batch request", model.ExpectedMachineMaxBatchItems), nil)
	}

	// Validate each item, also collect requested sku IDs
	// - SiteID is required, a valid UUID and unique across all items
	// - BMC address is required and must be unique
	// - Serial Number is required and must be unique
	// Note: this is early partial validation before we try to call the DB.
	validationErrors := validation.Errors{} //
	var foundSiteID *uuid.UUID
	bmcMacMap := make(map[string]int)
	serialMap := make(map[string]int)
	requestedSkuIDs := make(map[string]bool)
	for i, req := range apiRequests {
		strIndex := strconv.Itoa(i) // index/key as string for validation errors map
		itemErrors := validation.Errors{}

		verr := req.Validate()
		if verr != nil {
			var ok bool
			itemErrors, ok = verr.(validation.Errors)
			if !ok {
				common.AddToValidationErrors(itemErrors, "validation", verr)
			}
		}

		// SiteID is mandatory for batch create and must be the same for all items
		if req.SiteID == "" {
			common.AddToValidationErrors(itemErrors, "siteID", fmt.Errorf("Site ID is required"))
		}
		siteID, _ := uuid.Parse(req.SiteID) // already validated
		if foundSiteID == nil {
			foundSiteID = &siteID
		} else {
			if siteID != *foundSiteID {
				common.AddToValidationErrors(itemErrors, "siteID", fmt.Errorf(
					"Expected Machine does not belong to the same Site (%s) as other Expected Machines in request", req.SiteID))
			}
		}

		lowerMac := strings.ToLower(req.BmcMacAddress)
		if prev, ok := bmcMacMap[lowerMac]; ok {
			common.AddToValidationErrors(itemErrors, "bmcMacAddress", fmt.Errorf(
				"duplicate BMC MAC address '%s' found at indices %d and %d", req.BmcMacAddress, prev, i))
		}
		bmcMacMap[lowerMac] = i

		lowerSerial := strings.ToLower(req.ChassisSerialNumber)
		if prev, ok := serialMap[lowerSerial]; ok {
			common.AddToValidationErrors(itemErrors, "chassisSerialNumber", fmt.Errorf(
				"duplicate chassis serial number '%s' found at indices %d and %d", req.ChassisSerialNumber, prev, i))
		}
		serialMap[lowerSerial] = i

		if req.SkuID != nil {
			requestedSkuIDs[*req.SkuID] = true
		}

		if len(itemErrors) > 0 {
			validationErrors[strIndex] = itemErrors
		}
	}
	if len(validationErrors) > 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate Expected Machine create data", validationErrors)
	}
	siteID := *foundSiteID

	// Retrieve the Site from the DB
	site, err := common.GetSiteFromIDString(ctx, nil, siteID.String(), cemh.dbSession)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data does not exist", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site specified in request data due to DB error", nil)
	}

	// Validate access to Site
	hasAccess, apiError := ValidateProviderOrTenantSiteAccess(ctx, logger, cemh.dbSession, site, infrastructureProvider, tenant)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}
	if !hasAccess {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Current org is not associated with the Site", nil)
	}

	// Start a db transaction
	tx, err := cdb.BeginTx(ctx, cemh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Expected Machines due to DB transaction error", nil)
	}
	// this variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Retrieve all ExpectedMachines on Site from DB to allow unicity checks at Site level
	emDAO := cdbm.NewExpectedMachineDAO(cemh.dbSession)
	existingMachinesOnSite, _, err := emDAO.GetAll(ctx, tx, cdbm.ExpectedMachineFilterInput{
		SiteIDs: []uuid.UUID{siteID},
	}, paginator.PageInput{
		Limit: cdb.GetIntPtr(paginator.TotalLimit), // we want ALL records on site
	}, []string{})
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Expected Machines from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Expected Machines due to DB error", nil)
	}
	existingMacAddressMap := make(map[string]bool)
	existingSerialMap := make(map[string]bool)
	for _, machine := range existingMachinesOnSite {
		existingMacAddressMap[machine.BmcMacAddress] = true
		existingSerialMap[machine.ChassisSerialNumber] = true
	}

	// Retrieve all SKUs on Site to validate existence of SKU IDs in request
	skuDAO := cdbm.NewSkuDAO(cemh.dbSession)
	existingSkus, _, err := skuDAO.GetAll(ctx, tx, cdbm.SkuFilterInput{
		SiteIDs: []uuid.UUID{siteID},
	}, paginator.PageInput{
		Limit: cdb.GetIntPtr(len(requestedSkuIDs)),
	})
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving SKUs from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve SKUs due to DB error", nil)
	}
	existingSkuIDsMap := make(map[string]bool)
	for _, sku := range existingSkus {
		existingSkuIDsMap[sku.ID] = true
	}

	// Final checks: unicity of MAC, Serial, and existence of SKUs
	validationErrors = validation.Errors{}
	for i, req := range apiRequests {
		itemErrors := validation.Errors{}
		strIndex := strconv.Itoa(i) // index/key as string for validation errors map

		// Check MAC unicity
		if existingMacAddressMap[req.BmcMacAddress] {
			common.AddToValidationErrors(itemErrors, "bmcMacAddress", fmt.Errorf(
				"Expected Machine with BMC MAC Address: %s already exist", req.BmcMacAddress))
		}
		// Check Serial unicity
		if existingSerialMap[req.ChassisSerialNumber] {
			common.AddToValidationErrors(itemErrors, "chassisSerialNumber", fmt.Errorf(
				"Expected Machine with Chassis Serial Number: %s already exists", req.ChassisSerialNumber))
		}
		// Check SKU existence
		if req.SkuID != nil && !existingSkuIDsMap[*req.SkuID] {
			common.AddToValidationErrors(itemErrors, "skuID", fmt.Errorf(
				"the SkuID specified for Expected Machine does not exist in DB: %s", *req.SkuID))
		}

		// Collect errors
		if len(itemErrors) > 0 {
			validationErrors[strIndex] = itemErrors
		}
	}
	if len(validationErrors) > 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate Expected Machine update data", validationErrors)
	}

	createInputs := make([]cdbm.ExpectedMachineCreateInput, 0, len(apiRequests))
	for _, machineReq := range apiRequests {
		createInputs = append(createInputs, cdbm.ExpectedMachineCreateInput{
			ExpectedMachineID:        uuid.New(),
			SiteID:                   site.ID,
			BmcMacAddress:            machineReq.BmcMacAddress,
			ChassisSerialNumber:      machineReq.ChassisSerialNumber,
			SkuID:                    machineReq.SkuID,
			FallbackDpuSerialNumbers: machineReq.FallbackDPUSerialNumbers,
			Labels:                   machineReq.Labels,
			CreatedBy:                dbUser.ID,
		})
	}

	createdExpectedMachines, err := emDAO.CreateMultiple(ctx, tx, createInputs)
	if err != nil {
		logger.Error().Err(err).Msg("error creating ExpectedMachine records in DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Expected Machine due to DB error", nil)
	}

	workflowMachines := make([]*cwssaws.ExpectedMachine, 0, len(createdExpectedMachines))
	for i, createdMachine := range createdExpectedMachines {
		workflowMachine := &cwssaws.ExpectedMachine{
			Id:                       &cwssaws.UUID{Value: createdMachine.ID.String()},
			BmcMacAddress:            createdMachine.BmcMacAddress,
			ChassisSerialNumber:      createdMachine.ChassisSerialNumber,
			FallbackDpuSerialNumbers: createdMachine.FallbackDpuSerialNumbers,
			SkuId:                    createdMachine.SkuID,
		}

		if apiRequests[i].DefaultBmcUsername != nil {
			workflowMachine.BmcUsername = *apiRequests[i].DefaultBmcUsername
		}

		if apiRequests[i].DefaultBmcPassword != nil {
			workflowMachine.BmcPassword = *apiRequests[i].DefaultBmcPassword
		}

		protoLabels := util.ProtobufLabelsFromAPILabels(apiRequests[i].Labels)
		if protoLabels != nil {
			workflowMachine.Metadata = &cwssaws.Metadata{
				Labels: protoLabels,
			}
		}

		workflowMachines = append(workflowMachines, workflowMachine)
	}

	logger.Info().Int("Count", len(workflowMachines)).Msg("triggering CreateExpectedMachines workflow on Site")

	// Create workflow request
	workflowRequest := &cwssaws.BatchExpectedMachineOperationRequest{
		ExpectedMachines:     &cwssaws.ExpectedMachineList{ExpectedMachines: workflowMachines},
		AcceptPartialResults: false,
	}

	// Create workflow options
	workflowID := fmt.Sprintf("create-expected-machines-%s-%d", site.ID.String(), len(workflowMachines))
	workflowOptions := tclient.StartWorkflowOptions{
		ID:                       workflowID,
		WorkflowExecutionTimeout: cwutil.WorkflowExecutionTimeout,
		TaskQueue:                queue.SiteTaskQueue,
	}

	// Get the temporal client for the site we are working with
	stc, err := cemh.scp.GetClientByID(site.ID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	// Execute workflow and get results
	workflowRun, err := stc.ExecuteWorkflow(ctx, workflowOptions, "CreateExpectedMachines", workflowRequest)
	if err != nil {
		logger.Error().Err(err).Msg("failed to schedule CreateExpectedMachines workflow on Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to schedule batch Expected Machine creation workflow on Site: %v", err), nil)
	}

	workflowRunID := workflowRun.GetID()
	logger = logger.With().Str("WorkflowID", workflowRunID).Logger()
	logger.Info().Msg("executing CreateExpectedMachines workflow on Site")

	// Get workflow results
	var workflowResult cwssaws.BatchExpectedMachineOperationResponse

	err = workflowRun.Get(ctx, &workflowResult)
	if err != nil {
		logger.Error().Err(err).Msg("error executing CreateExpectedMachines workflow on Site")
		// Workflow failed entirely - don't commit transaction
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to execute batch Expected Machine creation workflow on Site: %v", err), nil)
	}

	// sanity checks since this is all-or-nothing
	if len(workflowResult.GetResults()) != len(createdExpectedMachines) {
		logger.Warn().Msgf("Workflow returned a different number of Expected Machines (expected %d but got %d)", len(createdExpectedMachines), len(workflowResult.GetResults()))
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing ExpectedMachine CreateExpectedMachines transaction to DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Expected Machines due to DB transaction error", nil)
	}
	txCommitted = true

	logger.Info().
		Int("SuccessCount", len(createdExpectedMachines)).
		Msg("finishing CreateExpectedMachines API handler")

	// Return only successful machines
	return c.JSON(http.StatusCreated, createdExpectedMachines)
}

// ~~~~~ Batch Update Handler ~~~~~ //

// UpdateExpectedMachinesHandler is the API Handler for batch updating ExpectedMachines
type UpdateExpectedMachinesHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewUpdateExpectedMachinesHandler initializes and returns a new handler for batch updating ExpectedMachines
func NewUpdateExpectedMachinesHandler(dbSession *cdb.Session, tc tclient.Client, scp *sc.ClientPool, cfg *config.Config) UpdateExpectedMachinesHandler {
	return UpdateExpectedMachinesHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        scp,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Batch update ExpectedMachines
// @Description Update multiple ExpectedMachines in a single request. All machines must belong to the same site.
// @Tags ExpectedMachine
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param message body []model.APIExpectedMachineUpdateRequest true "ExpectedMachine UpdateExpectedMachines request"
// @Success 200 {object} model.APIExpectedMachineBatchResponse
// @Router /v2/org/{org}/carbide/expected-machine/batch [patch]
func (uemh UpdateExpectedMachinesHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("UpdateMultiple", "ExpectedMachine", c, uemh.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Ensure our user is a provider or tenant for the org
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, uemh.dbSession, org, dbUser, false, true)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Validate request
	// Bind request data to API model (array payload)
	apiRequests := []model.APIExpectedMachineUpdateRequest{}
	err := c.Bind(&apiRequests)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	if len(apiRequests) == 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Request data must contain at least 1 Expected Machine entry", nil)
	}

	if len(apiRequests) > model.ExpectedMachineMaxBatchItems {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("At most %d Expected Machine entries can be created in a batch request", model.ExpectedMachineMaxBatchItems), nil)
	}

	// Validate each item, also collect requested sku IDs
	// - ID is required and must be unique
	// - BMC address is optional but must be unique
	// - Serial Number is optional but must be unique
	// Note: this is early partial validation before we try to call the DB.
	validationErrors := validation.Errors{} //
	idMap := make(map[uuid.UUID]int)        // Map Expected Machine ID to its index in the request array
	bmcMacMap := make(map[string]int)
	serialMap := make(map[string]int)
	requestedSkuIDs := make(map[string]bool)
	for i, req := range apiRequests {
		strIndex := strconv.Itoa(i) // index/key as string for validation errors map
		itemErrors := validation.Errors{}

		verr := req.Validate()
		if verr != nil {
			var ok bool
			itemErrors, ok = verr.(validation.Errors)
			if !ok {
				common.AddToValidationErrors(itemErrors, "validation", verr)
			}
		}

		// validation must accept nil ID for single update use case so we need to check for nil ID here
		if req.ID == nil {
			common.AddToValidationErrors(itemErrors, "id", fmt.Errorf("Missing required Expected Machine ID"))
		} else {
			// extract already validated UUID
			mid, _ := uuid.Parse(*req.ID)

			if prev, ok := idMap[mid]; ok {
				common.AddToValidationErrors(itemErrors, "id", fmt.Errorf(
					"duplicate Expected Machine ID '%s' found at indices %d and %d", *req.ID, prev, i))
			}
			idMap[mid] = i
		}

		if req.BmcMacAddress != nil {
			lowerMac := strings.ToLower(*req.BmcMacAddress)
			if prev, ok := bmcMacMap[lowerMac]; ok {
				common.AddToValidationErrors(itemErrors, "bmcMacAddress", fmt.Errorf(
					"duplicate BMC MAC address '%s' found at indices %d and %d", *req.BmcMacAddress, prev, i))
			}
			bmcMacMap[lowerMac] = i
		}

		if req.ChassisSerialNumber != nil {
			lowerSerial := strings.ToLower(*req.ChassisSerialNumber)
			if prev, ok := serialMap[lowerSerial]; ok {
				common.AddToValidationErrors(itemErrors, "chassisSerialNumber", fmt.Errorf(
					"duplicate chassis serial number '%s' found at indices %d and %d", *req.ChassisSerialNumber, prev, i))
			}
			serialMap[lowerSerial] = i
		}

		if req.SkuID != nil {
			requestedSkuIDs[*req.SkuID] = true
		}

		if len(itemErrors) > 0 {
			validationErrors[strIndex] = itemErrors
		}
	}
	if len(validationErrors) > 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate Expected Machine update data", validationErrors)
	}

	logger.Info().Int("MachineCount", len(apiRequests)).Msg("processing UpdateExpectedMachines request")

	// Since we only have a list of Expected Machine ID as input we can only learn the SiteIDs involved by querying the DB
	// but we also want to retrieve full Expected Machines from Site to check for Serial uniqueness.
	// We will split into three queries:
	// 1. Retrieve SiteID and Site by loading one ExpectedMachine record
	// 2. Retrieve Expected Machines for Site to check for serial uniqueness.
	// TODO: now that we have a unique index on (mac,siteID) we should reconsider adding unique indices on (serial,siteID).
	//       At this time it is expected that existing serial data may not be unique so we
	//       cannot add such an index without cleaning existing data first.

	// Start a db transaction
	tx, err := cdb.BeginTx(ctx, uemh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Expected Machines due to DB transaction error", nil)
	}
	// this variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// We need to acquire the siteID: we retrieve the first ExpectedMachine record to update and use it as reference.
	// We do not have a reference SiteID so we need to load all the requested Expected Machine to be able to validate they all belong to the same Site.
	emDAO := cdbm.NewExpectedMachineDAO(uemh.dbSession)
	requestedExpectedMachine, _, err := emDAO.GetAll(ctx, tx, cdbm.ExpectedMachineFilterInput{
		ExpectedMachineIDs: slices.Collect(maps.Keys(idMap)),
	}, paginator.PageInput{
		Limit: cdb.GetIntPtr(paginator.TotalLimit),
	}, []string{cdbm.SiteRelationName})
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Expected Machines from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Expected Machines due to DB error", nil)
	}
	if len(requestedExpectedMachine) == 0 {
		logger.Warn().Msg("No Expected Machines found for provided IDs")
		return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "No Expected Machines found for provided IDs", nil)
	}
	requestedEmMap := make(map[uuid.UUID]cdbm.ExpectedMachine)
	for i := range requestedExpectedMachine {
		em := &requestedExpectedMachine[i]
		requestedEmMap[em.ID] = *em
	}

	// Iterate on retrieved records to check siteID and IDs
	var foundSiteID *uuid.UUID
	validationErrors = validation.Errors{}
	for i, req := range apiRequests {
		strIndex := strconv.Itoa(i) // index/key as string for validation errors map
		itemErrors := validation.Errors{}

		// Check ID
		mid, _ := uuid.Parse(*req.ID) // we know the ID in request is valid from previous validation
		em, ok := requestedEmMap[mid]
		if !ok {
			common.AddToValidationErrors(itemErrors, "id", fmt.Errorf(
				"Expected Machine with ID %s not found in DB", *req.ID))
			validationErrors[strIndex] = itemErrors
			continue
		}

		// Check SiteID consistency
		if foundSiteID == nil {
			foundSiteID = &em.SiteID
		} else {
			if em.SiteID != *foundSiteID {
				common.AddToValidationErrors(itemErrors, "siteID", fmt.Errorf(
					"Expected Machine with ID %s does not belong to the same Site (%s) as other Expected Machines in request", *req.ID, *foundSiteID))
			}
		}

		if len(itemErrors) > 0 {
			validationErrors[strIndex] = itemErrors
		}
	}
	if len(validationErrors) > 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate Expected Machine update data", validationErrors)
	}

	// Get our unique Site ID and Site record
	siteID := requestedExpectedMachine[0].SiteID
	site := requestedExpectedMachine[0].Site // we get the site record from the relation loaded with our Expected Machine
	if site == nil {
		logger.Warn().Msg("No Site relation found for Expected Machines")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "No Site found for Expected Machines", nil)
	}

	// Validate ProviderTenantSite relationship and state
	hasAccess, apiError := ValidateProviderOrTenantSiteAccess(ctx, logger, uemh.dbSession, site, infrastructureProvider, tenant)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}
	if !hasAccess {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Current org is not associated with the Site", nil)
	}

	// Retrieve all ExpectedMachines on Site from DB to allow unicity checks at Site level
	emDAO = cdbm.NewExpectedMachineDAO(uemh.dbSession)
	expectedMachinesOnSite, _, err := emDAO.GetAll(ctx, tx, cdbm.ExpectedMachineFilterInput{
		SiteIDs: []uuid.UUID{siteID},
	}, paginator.PageInput{
		Limit: cdb.GetIntPtr(paginator.TotalLimit), // we want ALL records on site
	}, []string{})
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Expected Machines from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Expected Machines due to DB error", nil)
	}
	emMap := make(map[uuid.UUID]cdbm.ExpectedMachine)
	for i := range expectedMachinesOnSite {
		em := &expectedMachinesOnSite[i]
		emMap[em.ID] = *em
	}

	// Retrieve all SKUs on Site to validate existence of SKU IDs in request
	skuDAO := cdbm.NewSkuDAO(uemh.dbSession)
	skus, _, err := skuDAO.GetAll(ctx, tx, cdbm.SkuFilterInput{
		SiteIDs: []uuid.UUID{siteID},
	}, paginator.PageInput{
		Limit: cdb.GetIntPtr(len(requestedSkuIDs)),
	})
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving SKUs from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve SKUs due to DB error", nil)
	}
	uniqueSkuIDsOnSite := make(map[string]bool)
	for _, sku := range skus {
		uniqueSkuIDsOnSite[sku.ID] = true
	}

	// Verify unicity of BMC MAC Addresses and Serial Numbers with existing records on Site
	expectedMachineMacAddressChecker := common.NewUniqueChecker[uuid.UUID]()
	expectedMachineSerialNumberChecker := common.NewUniqueChecker[uuid.UUID]()

	// Load DB data into checkers
	for i := range expectedMachinesOnSite { // iterate on ALL Expected Machine on Site
		em := &expectedMachinesOnSite[i]
		expectedMachineMacAddressChecker.Update(em.ID, em.BmcMacAddress)
		if em.ChassisSerialNumber != "" {
			expectedMachineSerialNumberChecker.Update(em.ID, em.ChassisSerialNumber)
		}
	}

	// Apply changes to MAC and Serial to checkers
	for _, req := range apiRequests {
		mid, _ := uuid.Parse(*req.ID)
		if req.BmcMacAddress != nil {
			expectedMachineMacAddressChecker.Update(mid, *req.BmcMacAddress)
		}
		if req.ChassisSerialNumber != nil {
			expectedMachineSerialNumberChecker.Update(mid, *req.ChassisSerialNumber)
		}
	}

	// Final checks: unicity of MAC, Serial, and existence of SKUs
	validationErrors = validation.Errors{}
	for i, req := range apiRequests {
		itemErrors := validation.Errors{}
		strIndex := strconv.Itoa(i) // index/key as string for validation errors map
		mid, _ := uuid.Parse(*req.ID)

		// Check MAC unicity
		if req.BmcMacAddress != nil && expectedMachineMacAddressChecker.DoesIDHaveConflict(mid) {
			common.AddToValidationErrors(itemErrors, "bmcMacAddress", fmt.Errorf(
				"Expected Machine with BMC MAC Address: %s already exist", *req.BmcMacAddress))
		}
		// Check Serial unicity
		if req.ChassisSerialNumber != nil && expectedMachineSerialNumberChecker.DoesIDHaveConflict(mid) {
			common.AddToValidationErrors(itemErrors, "chassisSerialNumber", fmt.Errorf(
				"Expected Machine with Chassis Serial Number: %s already exists", *req.ChassisSerialNumber))
		}
		// Check SKU existence
		if req.SkuID != nil && !uniqueSkuIDsOnSite[*req.SkuID] {
			common.AddToValidationErrors(itemErrors, "skuID", fmt.Errorf(
				"the SkuID specified for Expected Machine does not exist in DB: %s", *req.SkuID))
		}

		// Collect errors
		if len(itemErrors) > 0 {
			validationErrors[strIndex] = itemErrors
		}
	}
	if len(validationErrors) > 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate Expected Machine update data", validationErrors)
	}

	// Prepare ExpectedMachines input for DB
	updateInputs := make([]cdbm.ExpectedMachineUpdateInput, 0, len(apiRequests))
	for _, machineReq := range apiRequests {
		// APIExpectedMachineUpdateRequest must allow nil ID for single update use case. If present here, it has already been validated.
		if machineReq.ID == nil {
			logger.Error().Msg("Expected Machine ID cannot be nil")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Expected Machine ID cannot be nil", nil)
		}

		emID, _ := uuid.Parse(*machineReq.ID)
		updateInputs = append(updateInputs, cdbm.ExpectedMachineUpdateInput{
			ExpectedMachineID:        emID,
			BmcMacAddress:            machineReq.BmcMacAddress,
			ChassisSerialNumber:      machineReq.ChassisSerialNumber,
			SkuID:                    machineReq.SkuID,
			FallbackDpuSerialNumbers: machineReq.FallbackDPUSerialNumbers,
			Labels:                   machineReq.Labels,
		})
	}

	// Update provided ExpectedMachines in DB
	updatedExpectedMachines, err := emDAO.UpdateMultiple(ctx, tx, updateInputs)
	if err != nil {
		logger.Error().Err(err).Msg("error updating ExpectedMachine records in DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Expected Machine due to DB error", nil)
	}

	workflowMachines := make([]*cwssaws.ExpectedMachine, 0, len(updatedExpectedMachines))
	for i, updatedMachine := range updatedExpectedMachines {
		workflowMachine := &cwssaws.ExpectedMachine{
			Id:                       &cwssaws.UUID{Value: updatedMachine.ID.String()},
			BmcMacAddress:            updatedMachine.BmcMacAddress,
			ChassisSerialNumber:      updatedMachine.ChassisSerialNumber,
			FallbackDpuSerialNumbers: updatedMachine.FallbackDpuSerialNumbers,
			SkuId:                    updatedMachine.SkuID,
		}

		if apiRequests[i].DefaultBmcUsername != nil {
			workflowMachine.BmcUsername = *apiRequests[i].DefaultBmcUsername
		}

		if apiRequests[i].DefaultBmcPassword != nil {
			workflowMachine.BmcPassword = *apiRequests[i].DefaultBmcPassword
		}

		protoLabels := util.ProtobufLabelsFromAPILabels(updatedMachine.Labels)
		if protoLabels != nil {
			workflowMachine.Metadata = &cwssaws.Metadata{
				Labels: protoLabels,
			}
		}

		workflowMachines = append(workflowMachines, workflowMachine)
	}

	logger.Info().Int("Count", len(workflowMachines)).Msg("triggering Expected Machine update workflow")

	// Create workflow request
	workflowRequest := &cwssaws.BatchExpectedMachineOperationRequest{
		ExpectedMachines:     &cwssaws.ExpectedMachineList{ExpectedMachines: workflowMachines},
		AcceptPartialResults: false,
	}

	// Create workflow options
	workflowID := fmt.Sprintf("expected-machines-update-batch-%s-%d", site.ID.String(), len(workflowMachines))
	workflowOptions := tclient.StartWorkflowOptions{
		ID:                       workflowID,
		WorkflowExecutionTimeout: cwutil.WorkflowExecutionTimeout,
		TaskQueue:                queue.SiteTaskQueue,
	}

	// Get the Temporal client for the site we are working with
	stc, err := uemh.scp.GetClientByID(site.ID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	// Execute workflow and get results
	workflowRun, err := stc.ExecuteWorkflow(ctx, workflowOptions, "UpdateExpectedMachines", workflowRequest)
	if err != nil {
		logger.Error().Err(err).Msg("failed to schedule batch Expected Machine update workflow on Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to schedule batch Expected Machine update workflow on Site: %v", err), nil)
	}

	workflowRunID := workflowRun.GetID()
	logger = logger.With().Str("WorkflowID", workflowRunID).Logger()
	logger.Info().Msg("executing Expected Machine update workflow on Site")

	// Get workflow results
	var workflowResult cwssaws.BatchExpectedMachineOperationResponse

	err = workflowRun.Get(ctx, &workflowResult)
	if err != nil {
		logger.Error().Err(err).Msg("error executing batch Expected Machine update workflow on Site")
		// Workflow failed entirely - don't commit transaction, changes will be rolled back
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to execute batch Expected Machine update workflow on Site: %v", err), nil)
	}

	// sanity checks since this is all-or-nothing
	if len(workflowResult.GetResults()) != len(updatedExpectedMachines) {
		logger.Warn().Msgf("workflow returned a different number of Expected Machines (expected %d but got %d)", len(updatedExpectedMachines), len(workflowResult.GetResults()))
	}

	// Commit transaction - only successful updates remain in DB
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing ExpectedMachine UpdateExpectedMachines transaction to DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Expected Machines due to DB transaction error", nil)
	}
	txCommitted = true

	logger.Info().
		Int("SuccessCount", len(updatedExpectedMachines)).
		Msg("finishing UpdateExpectedMachines API handler")

	// Return only successful machines
	return c.JSON(http.StatusOK, updatedExpectedMachines)
}
