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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/labstack/echo/v4"

	"go.opentelemetry.io/otel/attribute"
	tOperatorv1 "go.temporal.io/api/operatorservice/v1"
	tWorkflowv1 "go.temporal.io/api/workflowservice/v1"
	tClient "go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/durationpb"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nvidia/bare-metal-manager-rest/api/internal/config"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/handler/util/common"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/pagination"
	auth "github.com/nvidia/bare-metal-manager-rest/auth/pkg/authorization"
	cerr "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	sutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	csm "github.com/nvidia/bare-metal-manager-rest/site-manager/pkg/sitemgr"

	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	"github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"
	cdbp "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"
	siteWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/site"
)

const (
	// SiteRegistrationTokenValidityPeriod is the duration after which a Site registration token expires
	SiteRegistrationTokenValidityPeriod = time.Hour * 24 * 2

	// SiteWorkflowRetentionPeriod is the duration after which completed Site workflows are expunged
	SiteWorkflowRetentionPeriod = time.Hour * 24 * 7
)

// ~~~~~ Create Handler ~~~~~ //

// CreateSiteHandler is the API Handler for creating new Tenant
type CreateSiteHandler struct {
	dbSession  *cdb.Session
	tc         tClient.Client
	tnc        tClient.NamespaceClient
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewCreateSiteHandler initializes and returns a new handler for creating Tenant
func NewCreateSiteHandler(dbSession *cdb.Session, tc tClient.Client, tnc tClient.NamespaceClient, cfg *config.Config) CreateSiteHandler {
	return CreateSiteHandler{
		dbSession:  dbSession,
		tc:         tc,
		tnc:        tnc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Create a Site
// @Description Create a Site for the org. Only Infrastructure Providers can create Site
// @Tags site
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param message body model.APISiteCreateRequest true "Site create request"
// @Success 201 {object} model.APISite
// @Router /v2/org/{org}/carbide/site [post]
func (csh CreateSiteHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "Site").Str("Handler", "Create").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := csh.tracerSpan.CreateChildInContext(ctx, "CreateSiteHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx
		defer handlerSpan.End()
		csh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, csh.tracerSpan, handlerSpan)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org
	ok, err := auth.ValidateOrgMembership(dbUser, org)
	if !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership for User in request")
		} else {
			logger.Warn().Msg("could not validate org membership for user, access denied")
		}
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	// Validate role, only Provider Admins are allowed to create Site
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.ProviderAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Provider Admin role, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Provider Admin role with org", nil)
	}

	// Validate request
	// Bind request data to API model
	apiRequest := model.APISiteCreateRequest{}
	err = c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate()
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating Site creation request data")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating Site creation request data", verr)
	}

	// Get Infrastructure Provider for this org
	ip, err := common.GetInfrastructureProviderForOrg(ctx, nil, csh.dbSession, org)
	if err != nil {
		if err == common.ErrOrgInstrastructureProviderNotFound {
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound,
				fmt.Sprintf("Org '%v' does not have an Infrastructure Provider", org), nil)
		}
		logger.Error().Err(err).Msg("error retrieving Infrastructure Provider for this org")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider", nil)
	}

	// Check for name uniqueness for the Site within the scope of the Provider
	stDAO := cdbm.NewSiteDAO(csh.dbSession)
	sts, tot, err := stDAO.GetAll(ctx, nil, cdbm.SiteFilterInput{Name: &apiRequest.Name, InfrastructureProviderIDs: []uuid.UUID{ip.ID}}, paginator.PageInput{}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("db error checking for name uniqueness of Site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Site, error reading from data store", nil)
	}
	if tot > 0 {
		return cerr.NewAPIErrorResponse(c, http.StatusConflict, "A Site with specified name already exists for Infrastructure Provider", validation.Errors{
			"id": errors.New(sts[0].ID.String()),
		})
	}

	// start a transaction
	tx, err := cdb.BeginTx(ctx, csh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Site, error initiating data store transaction", nil)
	}
	// this variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Create Site
	dbCreateInput := cdbm.SiteCreateInput{
		Name:                     apiRequest.Name,
		Description:              apiRequest.Description,
		Org:                      org,
		InfrastructureProviderID: ip.ID,
		IsInfinityEnabled:        false,
		SerialConsoleHostname:    apiRequest.SerialConsoleHostname,
		IsSerialConsoleEnabled:   false,
		Status:                   cdbm.SiteStatusPending,
		CreatedBy:                dbUser.ID,
		Config: cdbm.SiteConfig{
			NetworkSecurityGroup: true, // The default case for a new site is to support NSGs.
		},
	}
	if apiRequest.Location != nil {
		dbCreateInput.Location = &cdbm.SiteLocation{
			City:    apiRequest.Location.City,
			State:   apiRequest.Location.State,
			Country: apiRequest.Location.Country,
		}
	}
	if apiRequest.Contact != nil {
		dbCreateInput.Contact = &cdbm.SiteContact{
			Email: apiRequest.Contact.Email,
		}
	}
	st, err := stDAO.Create(ctx, tx, dbCreateInput)
	if err != nil {
		logger.Error().Err(err).Msg("error creating Site DB entry")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Site", nil)
	}

	// Create status detail
	sdDAO := cdbm.NewStatusDetailDAO(csh.dbSession)
	ssd, serr := sdDAO.CreateFromParams(ctx, tx, st.ID.String(), *cdb.GetStrPtr(cdbm.SiteStatusPending),
		cdb.GetStrPtr("received site creation request, pending pairing"))
	if serr != nil {
		logger.Error().Err(serr).Msg("error creating Status Detail DB entry")
	}
	if ssd == nil {
		logger.Error().Msg("Status Detail DB entry not returned from CreateFromParams")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to get new Status Detail for Site", nil)
	}

	// Create Site entry in Site Manager
	if csh.cfg.GetSiteManagerEnabled() {
		// create site in site manager
		url := csh.cfg.GetSiteManagerEndpoint()
		provider := st.Org
		if st.InfrastructureProvider != nil {
			provider = st.InfrastructureProvider.Name
		}

		err = csm.CreateSite(ctx, logger, st.ID.String(), st.Name, provider, st.Org, url)
		if err != nil {
			logger.Error().Err(err).Msg("failed to create Site entry in Site Manager")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Unable to create Site Manager entry for Site", nil)
		}
		defer func() {
			if !txCommitted {
				csm.DeleteSite(ctx, logger, st.ID.String(), url)
			}
		}()

		// Retrieve registration token (OTP) from Site Manager
		registrationToken, registrationTokenExpires, serr := csm.GetSiteOTP(ctx, logger, st.ID.String(), url)
		if serr != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to obtain registration token from Site Manager", nil)
		}

		_, err = stDAO.Update(ctx, tx, cdbm.SiteUpdateInput{
			SiteID:                      st.ID,
			RegistrationToken:           registrationToken,
			RegistrationTokenExpiration: registrationTokenExpires,
		})
		if err != nil {
			logger.Error().Err(err).Msg("error updating Site with registration token")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Site with registration token, error communicating with data store", nil)
		}

		// Refresh Site object
		st, serr = stDAO.GetByID(ctx, tx, st.ID, nil, false)
		if serr != nil {
			logger.Error().Err(serr).Msg("error retrieving Site from DB")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to refresh Site data with registration token, error communicating with data store", nil)
		}
	}

	// Create Temporal namespace
	workflowRetentionPeriod := durationpb.New(SiteWorkflowRetentionPeriod)

	regRequest := tWorkflowv1.RegisterNamespaceRequest{
		Namespace:                        st.ID.String(),
		Description:                      fmt.Sprintf("Namespace for Site %v", st.ID),
		WorkflowExecutionRetentionPeriod: workflowRetentionPeriod,
	}

	err = csh.tnc.Register(ctx, &regRequest)
	if err != nil {
		logger.Error().Err(err).Msg("error creating Temporal namespace for Site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create workflow namespace for Site", nil)
	}

	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("failed to commit transaction, error creating site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Site, error finalizing data store transaction", nil)
	}
	txCommitted = true

	// Create response
	apiSite := model.NewAPISite(*st, []cdbm.StatusDetail{*ssd}, nil)

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusCreated, apiSite)
}

// ~~~~~ Update Handler ~~~~~ //

// UpdateSiteHandler is the API Handler for updating a Site
type UpdateSiteHandler struct {
	dbSession  *cdb.Session
	tc         tClient.Client
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewUpdateSiteHandler initializes and returns a new handler for updating Site
func NewUpdateSiteHandler(dbSession *cdb.Session, tc tClient.Client, cfg *config.Config) UpdateSiteHandler {
	return UpdateSiteHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Update an existing Site
// @Description Update an existing Site for the org
// @Tags site
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Site"
// @Param message body model.APISiteUpdateRequest true "Site update request"
// @Success 200 {object} model.APISite
// @Router /v2/org/{org}/carbide/site/{id} [patch]
func (ush UpdateSiteHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "Site").Str("Handler", "Update").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := ush.tracerSpan.CreateChildInContext(ctx, "UpdateSiteHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()

		ush.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, ush.tracerSpan, handlerSpan)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org
	ok, err := auth.ValidateOrgMembership(dbUser, org)
	if !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership for User in request")
		} else {
			logger.Warn().Msg("could not validate org membership for user, access denied")
		}
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	// Validate role, only Provider Admins are allowed to update Site
	isProviderRequest, orgProvider, orgTenant, apiErr := common.GetIsProviderRequest(ctx, logger, ush.dbSession, org, dbUser,
		[]string{auth.ProviderAdminRole}, []string{auth.TenantAdminRole}, c.QueryParams())
	if apiErr != nil {
		return c.JSON(apiErr.Code, apiErr)
	}

	// Get application instance ID from URL param
	stStrID := c.Param("id")

	ush.tracerSpan.SetAttribute(handlerSpan, attribute.String("site_id", stStrID), logger)

	stID, err := uuid.Parse(stStrID)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID in URL", nil)
	}

	stDAO := cdbm.NewSiteDAO(ush.dbSession)

	// Validate request
	// Bind request data to API model
	apiRequest := model.APISiteUpdateRequest{}
	err = c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate(isProviderRequest)
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating Site update request data")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating Site update request data", verr)
	}

	// Check that Site exists
	es, err := stDAO.GetByID(ctx, nil, stID, nil, false)
	if err != nil {
		logger.Warn().Err(err).Msg("error retrieving Site DB entity")
		return cerr.NewAPIErrorResponse(c, http.StatusNotFound, "Could not retrieve Site to update", nil)
	}

	tsDAO := cdbm.NewTenantSiteDAO(ush.dbSession)

	var ts *cdbm.TenantSite

	if isProviderRequest {
		if orgProvider.ID != es.InfrastructureProviderID {
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Site is not associated with org's Infrastructure Provider", nil)
		}
	} else {
		// Check that TenantSite exists
		ts, err = tsDAO.GetByTenantIDAndSiteID(ctx, nil, orgTenant.ID, stID, nil)
		if err != nil {
			if err == cdb.ErrDoesNotExist {
				return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant does not have access to Site", nil)
			}
			logger.Warn().Err(err).Msg("error retrieving TenantSite DB entity")
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound, "Failed to determine Tenant access to Site, DB error", nil)
		}
	}

	var registrationToken *string
	var registrationTokenExpires *time.Time
	var status *string
	var statusMessage *string

	if apiRequest.RenewRegistrationToken != nil && *apiRequest.RenewRegistrationToken {
		// roll site in site manager
		url := ush.cfg.GetSiteManagerEndpoint()
		err = csm.RollSite(ctx, logger, es.ID.String(), es.Name, url)
		if err != nil {
			logger.Warn().Err(err).Msg("error rolling site in site manager")
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Site Manager failure", nil)
		}

		registrationToken, registrationTokenExpires, err = csm.GetSiteOTP(ctx, logger, es.ID.String(), url)
		if err != nil {
			logger.Error().Err(err).Msg("error getting site from site manager")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Site Manager failure", nil)
		}

		// Switch Site status only if it is currently not Registered
		if es.Status != cdbm.SiteStatusRegistered {
			status = cdb.GetStrPtr(cdbm.SiteStatusPending)
			statusMessage = cdb.GetStrPtr("Registration token renewed, pending pairing")
		}
	}

	// Check for name uniqueness for the Site
	if apiRequest.Name != nil && *apiRequest.Name != es.Name {
		sts, tot, serr := stDAO.GetAll(ctx, nil, cdbm.SiteFilterInput{Name: apiRequest.Name, InfrastructureProviderIDs: []uuid.UUID{es.InfrastructureProviderID}}, paginator.PageInput{}, nil)
		if serr != nil {
			logger.Error().Err(serr).Msg("db error checking for name uniqueness of Site")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Site, error reading from data store", nil)
		}
		if tot > 0 {
			return cerr.NewAPIErrorResponse(c, http.StatusConflict, "Another Site with specified name already exists for Provider", validation.Errors{
				"id": errors.New(sts[0].ID.String()),
			})
		}
	}

	// Check if SOL params changed
	if es.Status != cdbm.SiteStatusRegistered {
		if apiRequest.IsSerialConsoleEnabled != nil && *apiRequest.IsSerialConsoleEnabled != es.IsSerialConsoleEnabled {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Cannot enable/disable Serial Console for Site that is not in Registered state", nil)
		}

		if apiRequest.SerialConsoleIdleTimeout != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Cannot update Serial Console idle timeout for Site that is not in Registered state", nil)
		}

		if apiRequest.SerialConsoleMaxSessionLength != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Cannot update Serial Console max session length for Site that is not in Registered state", nil)
		}
	}

	// start a transaction
	tx, err := cdb.BeginTx(ctx, ush.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Site, error initiating data store transaction", nil)
	}
	// this variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Update Site
	var us *cdbm.Site
	var uts *cdbm.TenantSite

	if isProviderRequest {
		dbUpdateInput := cdbm.SiteUpdateInput{
			SiteID:                        stID,
			Name:                          apiRequest.Name,
			Description:                   apiRequest.Description,
			RegistrationToken:             registrationToken,
			RegistrationTokenExpiration:   registrationTokenExpires,
			SerialConsoleHostname:         apiRequest.SerialConsoleHostname,
			IsSerialConsoleEnabled:        apiRequest.IsSerialConsoleEnabled,
			SerialConsoleIdleTimeout:      apiRequest.SerialConsoleIdleTimeout,
			SerialConsoleMaxSessionLength: apiRequest.SerialConsoleMaxSessionLength,
			Status:                        status,
		}
		if apiRequest.Location != nil {
			dbUpdateInput.Location = &cdbm.SiteLocation{
				City:    apiRequest.Location.City,
				State:   apiRequest.Location.State,
				Country: apiRequest.Location.Country,
			}
		}
		if apiRequest.Contact != nil {
			dbUpdateInput.Contact = &cdbm.SiteContact{
				Email: apiRequest.Contact.Email,
			}
		}
		us, err = stDAO.Update(ctx, tx, dbUpdateInput)
		if err != nil {
			logger.Error().Err(err).Msg("error updating site")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update site", nil)
		}
	} else {
		us = es
		uts, err = tsDAO.Update(
			ctx,
			tx,
			cdbm.TenantSiteUpdateInput{
				TenantSiteID:        ts.ID,
				EnableSerialConsole: apiRequest.IsSerialConsoleEnabled,
			},
		)
		if err != nil {
			logger.Error().Err(err).Msg("error updating tenant site")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update tenant site", nil)
		}
	}

	// Add status detail if needed
	if status != nil {
		sdDAO := cdbm.NewStatusDetailDAO(ush.dbSession)
		_, serr := sdDAO.CreateFromParams(ctx, tx, stID.String(), *status, statusMessage)
		if serr != nil {
			logger.Error().Err(serr).Msg("error creating Status Detail DB entry")
		}
	}

	// Get status details
	sdDAO := cdbm.NewStatusDetailDAO(ush.dbSession)

	ssds, _, err := sdDAO.GetAllByEntityID(ctx, tx, stID.String(), nil, cdb.GetIntPtr(pagination.MaxPageSize), nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Status Details for Site from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Status Details for Site", nil)
	}

	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("failed to commit transaction, error updating site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Site, error finalizing data store transaction", nil)
	}
	txCommitted = true

	// Create response
	apiSite := model.NewAPISite(*us, ssds, uts)

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusOK, apiSite)
}

// ~~~~~ Get Handler ~~~~~ //

// GetSiteHandler is the API Handler for getting a Site
type GetSiteHandler struct {
	dbSession  *cdb.Session
	tc         tClient.Client
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetSiteHandler initializes and returns a new handler for getting Site
func NewGetSiteHandler(dbSession *cdb.Session, tc tClient.Client, cfg *config.Config) GetSiteHandler {
	return GetSiteHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get a Site
// @Description Get a Site for the org
// @Tags site
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Site"
// @Param includeRelation query string false "Related entities to include in response e.g. 'InfrastructureProvider'"
// @Success 200 {object} model.APISite
// @Router /v2/org/{org}/carbide/site/{id} [get]
func (gsh GetSiteHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "Site").Str("Handler", "Get").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := gsh.tracerSpan.CreateChildInContext(ctx, "GetSiteHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()

		gsh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, gsh.tracerSpan, handlerSpan)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org
	ok, err := auth.ValidateOrgMembership(dbUser, org)
	if !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership for User in request")
		} else {
			logger.Warn().Msg("could not validate org membership for user, access denied")
		}
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	provider, tenant, apiErr := common.IsProviderOrTenant(ctx, logger, gsh.dbSession, org, dbUser, true, false)
	if apiErr != nil {
		return c.JSON(apiErr.Code, apiErr)
	}

	// Get and validate includeRelation params
	qParams := c.QueryParams()
	qIncludeRelations, errMsg := common.GetAndValidateQueryRelations(qParams, cdbm.SiteRelatedEntities)
	if errMsg != "" {
		logger.Warn().Msg(errMsg)
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, errMsg, nil)
	}

	// Get Site ID from URL param
	stStrID := c.Param("id")

	gsh.tracerSpan.SetAttribute(handlerSpan, attribute.String("site_id", stStrID), logger)

	stID, err := uuid.Parse(stStrID)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID in URL", nil)
	}

	// Get Site
	stDAO := cdbm.NewSiteDAO(gsh.dbSession)

	st, err := stDAO.GetByID(ctx, nil, stID, qIncludeRelations, false)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find Site with specified ID", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site", nil)
	}

	// Check Site association with Provider
	isAssociated := false
	if provider != nil {
		isAssociated = provider.ID == st.InfrastructureProviderID
	}

	var ts *cdbm.TenantSite
	tsDAO := cdbm.NewTenantSiteDAO(gsh.dbSession)

	if !isAssociated && tenant != nil {
		var serr error
		ts, serr = tsDAO.GetByTenantIDAndSiteID(ctx, nil, tenant.ID, stID, nil)
		if serr != nil {
			if serr != cdb.ErrDoesNotExist {
				logger.Error().Err(serr).Msg("error retrieving TenantSite from DB")
				return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to determine org's association with Site, DB error", nil)
			}
		} else {
			isAssociated = true
		}
	}

	if !isAssociated {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Site is not associated with org", nil)
	}

	// Get Site status details
	sdDAO := cdbm.NewStatusDetailDAO(gsh.dbSession)

	ssds, err := sdDAO.GetRecentByEntityIDs(ctx, nil, []string{stID.String()}, common.RECENT_STATUS_DETAIL_COUNT)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Status Details for Site from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Status Details for Site", nil)
	}

	// Create response
	ast := model.NewAPISite(*st, ssds, ts)

	return c.JSON(http.StatusOK, ast)
}

// ~~~~~ GetAll Handler ~~~~~ //

// GetAllSiteHandler is the API Handler for retrieving all Sites
type GetAllSiteHandler struct {
	dbSession  *cdb.Session
	tc         tClient.Client
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetAllSiteHandler initializes and returns a new handler for retrieving all Sites
func NewGetAllSiteHandler(dbSession *cdb.Session, tc tClient.Client, cfg *config.Config) GetAllSiteHandler {
	return GetAllSiteHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get all Sites
// @Description Get all Sites for the org
// @Tags site
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param isNativeNetworkingEnabled query boolean false "Filter by native networking enabled flag"
// @Param isNetworkSecurityGroupEnabled query boolean false "Filter by network security group enabled flag"
// @Param isNVLinkPartitionEnabled query boolean false "Filter by NVLink partition enabled flag"
// @Param isRackLevelAdministrationEnabled query boolean false "Filter by Rack Level Administration enabled flag"
// @Param query query string false "Query input for full text search"
// @Param status query string false "Query input for status"
// @Param includeRelation query string false "Related entities to include in response e.g. 'InfrastructureProvider'"
// @Success 200 {array} []model.APISite
// @Router /v2/org/{org}/carbide/site [get]
func (gash GetAllSiteHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "Site").Str("Handler", "GetAll").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := gash.tracerSpan.CreateChildInContext(ctx, "GetAllSiteHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()

		gash.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, gash.tracerSpan, handlerSpan)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org
	ok, err := auth.ValidateOrgMembership(dbUser, org)
	if !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership for User in request")
		} else {
			logger.Warn().Msg("could not validate org membership for user, access denied")
		}
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	// Validate pagination request
	// Bind request data to API model
	pageRequest := pagination.PageRequest{}
	err = c.Bind(&pageRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding pagination request data into API model")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request pagination data", nil)
	}

	// Validate request attributes
	err = pageRequest.Validate(cdbm.SiteOrderByFields)
	if err != nil {
		logger.Warn().Err(err).Msg("error validating pagination request data")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate pagination request data", err)
	}

	// Get and validate includeRelation params
	qParams := c.QueryParams()
	qIncludeRelations, errMsg := common.GetAndValidateQueryRelations(qParams, cdbm.SiteRelatedEntities)
	if errMsg != "" {
		logger.Warn().Msg(errMsg)
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, errMsg, nil)
	}

	provider, tenant, apiErr := common.IsProviderOrTenant(ctx, logger, gash.dbSession, org, dbUser, true, false)
	if apiErr != nil {
		return c.JSON(apiErr.Code, apiErr)
	}

	filter := cdbm.SiteFilterInput{}

	searchQueryStr := c.QueryParam("query")
	if searchQueryStr != "" {
		gash.tracerSpan.SetAttribute(handlerSpan, attribute.String("query", searchQueryStr), logger)
		filter.SearchQuery = &searchQueryStr
	}

	// Get status from query param
	if statusQuery := qParams["status"]; len(statusQuery) > 0 {
		gash.tracerSpan.SetAttribute(handlerSpan, attribute.StringSlice("status", statusQuery), logger)
		for _, status := range statusQuery {
			_, ok := cdbm.SiteStatusMap[status]
			if !ok {
				logger.Warn().Msg(fmt.Sprintf("invalid value in status query: %v", status))
				statusError := validation.Errors{
					"status": errors.New(status),
				}
				return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Status value in query", statusError)
			}
			filter.Statuses = append(filter.Statuses, status)
		}
	}

	// Check `includeMachineStats` in query
	includeMachineStats := false
	qims := c.QueryParam("includeMachineStats")
	if qims != "" {
		includeMachineStats, err = strconv.ParseBool(qims)
		if err != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid value specified for `includeMachineStats` query param", nil)
		}
	}

	filter.Config = &cdbm.SiteConfigFilterInput{}

	// Check `isNativeNetworkingEnabled` in query
	var isNativeNetworkingEnabled *bool
	qinne := c.QueryParam("isNativeNetworkingEnabled")
	if qinne != "" {
		isnnEnabled, err := strconv.ParseBool(qinne)
		if err != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid value specified for `isNativeNetworkingEnabled` query param", nil)
		}
		isNativeNetworkingEnabled = &isnnEnabled
	}
	filter.Config.NativeNetworking = isNativeNetworkingEnabled

	// Check `isNetworkSecurityGroupEnabled` in query
	var isNetworkSecurityGroupEnabled *bool
	qie := c.QueryParam("isNetworkSecurityGroupEnabled")
	if qie != "" {
		isEnabled, err := strconv.ParseBool(qie)
		if err != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid value specified for `isNativeNetworkingEnabled` query param", nil)
		}
		isNetworkSecurityGroupEnabled = &isEnabled
	}
	filter.Config.NetworkSecurityGroup = isNetworkSecurityGroupEnabled

	// Check `isNVLinkPartitionEnabled` in query
	var isNVLinkPartitionEnabled *bool
	qinlpe := c.QueryParam("isNVLinkPartitionEnabled")
	if qinlpe != "" {
		isEnabled, err := strconv.ParseBool(qinlpe)
		if err != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid value specified for `isNVLinkPartitionEnabled` query param", nil)
		}
		isNVLinkPartitionEnabled = &isEnabled
	}
	filter.Config.NVLinkPartition = isNVLinkPartitionEnabled

	// Check `isRackLevelAdministrationEnabled` in query
	var isRackLevelAdministrationEnabled *bool
	qirlae := c.QueryParam("isRackLevelAdministrationEnabled")
	if qirlae != "" {
		isEnabled, err := strconv.ParseBool(qirlae)
		if err != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid value specified for `isRackLevelAdministrationEnabled` query param", nil)
		}
		isRackLevelAdministrationEnabled = &isEnabled
	}
	filter.Config.RackLevelAdministration = isRackLevelAdministrationEnabled

	// Get machine stats if requested
	var machineStats map[uuid.UUID]*model.APISiteMachineStats

	if includeMachineStats {
		if provider == nil {
			logger.Warn().Msg("`includeMachineStats` is only permitted with Provider Admin role")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User must have Provider Admin role to request `includeMachineStats`", nil)
		}

		machineStats, err = common.GetSiteMachineCountStats(ctx, nil, gash.dbSession, logger, &provider.ID, nil)
		if err != nil {
			logger.Error().Err(err).Msg("unable to request Machine stats for Site")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Unable to request Machine stats for Site", nil)
		}
	}

	stDAO := cdbm.NewSiteDAO(gash.dbSession)

	siteIDs := mapset.NewSet[uuid.UUID]()

	tsMap := map[uuid.UUID]*cdbm.TenantSite{}

	if provider != nil {
		// Retrieve Sites from Provider perspective
		sites, _, serr := stDAO.GetAll(ctx, nil, cdbm.SiteFilterInput{InfrastructureProviderIDs: []uuid.UUID{provider.ID}}, paginator.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
		if serr != nil {
			logger.Error().Err(serr).Msg("error retrieving all Sites by param from DB")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Sites", nil)
		}
		for _, site := range sites {
			siteIDs.Add(site.ID)
		}
	}

	if tenant != nil {
		// Get Sites from Tenant perspective
		tsDAO := cdbm.NewTenantSiteDAO(gash.dbSession)
		tss, _, serr := tsDAO.GetAll(ctx, nil, cdbm.TenantSiteFilterInput{TenantIDs: []uuid.UUID{tenant.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
		if serr != nil {
			logger.Error().Err(serr).Msg("error retrieving Tenant Site associations from DB by Tenant ID")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site associated with Tenant", nil)
		}

		for _, ts := range tss {
			siteIDs.Add(ts.SiteID)
			tsMap[ts.SiteID] = &ts
		}

		// If Tenant is privileged (has TargetedInstanceCreation capability),
		// also retrieve all Sites from Providers they have a Tenant Account with
		if tenant.Config != nil && tenant.Config.TargetedInstanceCreation {
			taDAO := cdbm.NewTenantAccountDAO(gash.dbSession)
			tas, _, serr := taDAO.GetAll(ctx, nil, cdbm.TenantAccountFilterInput{
				TenantIDs: []uuid.UUID{tenant.ID},
				Statuses:  []string{cdbm.TenantAccountStatusReady},
			}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
			if serr != nil {
				logger.Error().Err(serr).Msg("error retrieving Tenant Accounts for privileged Tenant")
				return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant Accounts", nil)
			}

			if len(tas) > 0 {
				providerIDs := make([]uuid.UUID, 0, len(tas))
				for _, ta := range tas {
					providerIDs = append(providerIDs, ta.InfrastructureProviderID)
				}

				providerSites, _, serr := stDAO.GetAll(ctx, nil, cdbm.SiteFilterInput{InfrastructureProviderIDs: providerIDs}, paginator.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
				if serr != nil {
					logger.Error().Err(serr).Msg("error retrieving Sites for Providers from Tenant Accounts")
					return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Sites for one or more Providers", nil)
				}
				for _, site := range providerSites {
					siteIDs.Add(site.ID)
				}
			}
		}
	}

	filter.SiteIDs = siteIDs.ToSlice()

	// Get Sites from DB
	sites, total, err := stDAO.GetAll(ctx, nil, filter, paginator.PageInput{Offset: pageRequest.Offset, Limit: pageRequest.Limit, OrderBy: pageRequest.OrderBy}, qIncludeRelations)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Sites from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Sites, DB error", nil)
	}

	// Get status details
	sdDAO := cdbm.NewStatusDetailDAO(gash.dbSession)

	pagedSiteIDs := []string{}
	for _, site := range sites {
		pagedSiteIDs = append(pagedSiteIDs, site.ID.String())
	}

	// Get status details for the paged Sites
	ssds, serr := sdDAO.GetRecentByEntityIDs(ctx, nil, pagedSiteIDs, common.RECENT_STATUS_DETAIL_COUNT)
	if serr != nil {
		logger.Warn().Err(serr).Msg("error retrieving Status Details for Sites from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to populate status history for Sites", nil)
	}
	ssdMap := map[string][]cdbm.StatusDetail{}
	for _, ssd := range ssds {
		cssd := ssd
		ssdMap[ssd.EntityID] = append(ssdMap[ssd.EntityID], cssd)
	}

	// Create response
	apiSites := []model.APISite{}
	for _, site := range sites {
		apiSite := model.NewAPISite(site, ssdMap[site.ID.String()], tsMap[site.ID])
		// Attach machine stats for the site if they exist.
		apiSite.MachineStats = machineStats[site.ID]
		apiSites = append(apiSites, apiSite)
	}

	// Create pagination response header
	pageReponse := pagination.NewPageResponse(*pageRequest.PageNumber, *pageRequest.PageSize, total, pageRequest.OrderByStr)
	pageHeader, err := json.Marshal(pageReponse)
	if err != nil {
		logger.Error().Err(err).Msg("error marshaling pagination response")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to generate pagination response header", nil)
	}

	c.Response().Header().Set(pagination.ResponseHeaderName, string(pageHeader))

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusOK, apiSites)
}

// ~~~~~ Delete Handler ~~~~~ //

// DeleteSiteHandler is the API Handler for deleting a Site
type DeleteSiteHandler struct {
	dbSession  *cdb.Session
	tc         tClient.Client
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewDeleteSiteHandler initializes and returns a new handler for deleting Site
func NewDeleteSiteHandler(dbSession *cdb.Session, tc tClient.Client, cfg *config.Config) DeleteSiteHandler {
	return DeleteSiteHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Delete a Site
// @Description Delete a Site owned by the infrastructure provider
// @Tags site
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Site"
// @Param purgeMachines query boolean false "Permanently remove all Machines associated with this Site"
// @Success 204
// @Router /v2/org/{org}/carbide/site/{id} [delete]
func (dsh DeleteSiteHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "Site").Str("Handler", "Delete").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := dsh.tracerSpan.CreateChildInContext(ctx, "DeleteSiteHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()

		dsh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, dsh.tracerSpan, handlerSpan)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org
	ok, err := auth.ValidateOrgMembership(dbUser, org)
	if !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership for User in request")
		} else {
			logger.Warn().Msg("could not validate org membership for user, access denied")
		}
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	// Validate role, only Provider Admins are allowed to delete Site
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.ProviderAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Provider Admin role with org, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Provider Admin role with org", nil)
	}

	// Get Site ID from URL param
	stStrID := c.Param("id")

	dsh.tracerSpan.SetAttribute(handlerSpan, attribute.String("site_id", stStrID), logger)

	stID, err := uuid.Parse(stStrID)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID in URL", nil)
	}

	// Check query param for purging Machine data
	purgeMachinesStr := c.QueryParam("purgeMachines")
	purgeMachines := false
	if purgeMachinesStr != "" {
		purgeMachines, err = strconv.ParseBool(purgeMachinesStr)
		if err != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid purgeMachines query param", nil)
		}
	}

	// Check if the org has access to the Site
	// Check if org has an Infrastructure Provider
	ip, err := common.GetInfrastructureProviderForOrg(ctx, nil, dsh.dbSession, org)
	if err != nil {
		if err == common.ErrOrgInstrastructureProviderNotFound {
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "No Infrastructure Provider found for this org", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Infrastructure Provider for this org")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider", nil)
	}

	// Get Site
	stDAO := cdbm.NewSiteDAO(dsh.dbSession)
	st, err := stDAO.GetByID(ctx, nil, stID, nil, false)
	if err != nil {
		logger.Warn().Err(err).Msg("error retrieving Site from DB by ID")
		return cerr.NewAPIErrorResponse(c, http.StatusNotFound, "Could not retrieve Site with ID in URL", nil)
	}

	if st.InfrastructureProviderID != ip.ID {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Site does not belong to org's Infrastructure Provider", nil)
	}

	// Check for Allocations for Site
	aDAO := cdbm.NewAllocationDAO(dsh.dbSession)
	allocationFilter := cdbm.AllocationFilterInput{SiteIDs: []uuid.UUID{st.ID}}
	atotal, err := aDAO.GetCount(ctx, nil, allocationFilter)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Allocations count for Site from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Allocations count for Site", nil)
	}

	if atotal > 0 {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Site has Allocations which must be deleted first", nil)
	}

	// start a transaction
	tx, err := cdb.BeginTx(ctx, dsh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete site, error initiating data store transaction", nil)
	}
	// this variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Delete Site
	err = stDAO.Delete(ctx, tx, stID)
	if err != nil {
		logger.Error().Err(err).Msg("error deleting Site from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Site, error deleting Site in data store", nil)
	}

	// Delete Temporal namespace
	tosc := dsh.tc.OperatorService()
	_, err = tosc.DeleteNamespace(ctx, &tOperatorv1.DeleteNamespaceRequest{
		Namespace: st.ID.String(),
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Warn().Str("Site ID", stStrID).Msg("temporal namespace not found, continuing")
		} else {
			logger.Error().Err(err).Msg("error deleting Temporal namespace for Site")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Site, error deleting workflow namespace", nil)
		}
	}

	if dsh.cfg.GetSiteManagerEnabled() {
		url := dsh.cfg.GetSiteManagerEndpoint()
		err = csm.DeleteSite(ctx, logger, st.ID.String(), url)
		if err == csm.ErrSiteNotFound {
			logger.Warn().Err(err).Msg("Site not found in Site Manager, continuing with deletion")
		} else if err != nil {
			logger.Error().Err(err).Msg("error deleting site in site manager")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Site, error deleting Site Manager entry", nil)
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("failed to commit transaction, error deleting site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete site, error finalizing data store transaction", nil)
	}
	txCommitted = true

	// Trigger Cloud workflow to delete Site Component from DB
	wid, err := siteWorkflow.ExecuteDeleteSiteComponentsWorkflow(ctx, dsh.tc, st.ID, st.InfrastructureProviderID, purgeMachines)
	if err != nil {
		logger.Error().Err(err).Msg("failed to execute delete Site Components workflow")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to initiate Site Component deletion from DB", nil)
	}

	logger.Info().Str("Workflow ID", *wid).Msg("triggered delete Site Component workflow")

	// Create response
	logger.Info().Msg("finishing API handler")

	return c.String(http.StatusAccepted, "Deletion request was accepted")
}

// GetSiteStatusDetailsHandler is the API Handler for getting Site StatusDetail records
type GetSiteStatusDetailsHandler struct {
	dbSession  *cdb.Session
	tracerSpan *sutil.TracerSpan
}

// NewGetSiteStatusDetailsHandler initializes and returns a new handler to retrieve Site StatusDetail records
func NewGetSiteStatusDetailsHandler(dbSession *cdb.Session) GetSiteStatusDetailsHandler {
	return GetSiteStatusDetailsHandler{
		dbSession:  dbSession,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get Site StatusDetails
// @Description Get all StatusDetails for Site
// @Tags Site
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Site"
// @Success 200 {object} []model.APIStatusDetail
// @Router /v2/org/{org}/carbide/Site/{id}/status-history [get]
func (gssdh GetSiteStatusDetailsHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "Site").Str("Handler", "Get").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := gssdh.tracerSpan.CreateChildInContext(ctx, "GetSiteStatusDetailsHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx
		defer handlerSpan.End()
		gssdh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, gssdh.tracerSpan, handlerSpan)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org
	ok, err := auth.ValidateOrgMembership(dbUser, org)
	if !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership for User in request")
		} else {
			logger.Warn().Msg("could not validate org membership for user, access denied")
		}
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	// Validate role, user must be either Provider or Tenant Admin to retrieve a Site
	isProviderRequest, orgProvider, orgTenant, apiErr := common.GetIsProviderRequest(ctx, logger, gssdh.dbSession, org, dbUser,
		[]string{auth.ProviderAdminRole, auth.ProviderViewerRole}, []string{auth.TenantAdminRole}, c.QueryParams())
	if apiErr != nil {
		return c.JSON(apiErr.Code, apiErr)
	}

	// Get Site ID from URL param
	stStrID := c.Param("id")
	gssdh.tracerSpan.SetAttribute(handlerSpan, attribute.String("site_id", stStrID), logger)
	stID, err := uuid.Parse(stStrID)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID in URL", nil)
	}

	// Get Site
	stDAO := cdbm.NewSiteDAO(gssdh.dbSession)
	st, err := stDAO.GetByID(ctx, nil, stID, nil, false)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find Site with specified ID", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site", nil)
	}

	// Check Site association with Provider
	tsDAO := cdbm.NewTenantSiteDAO(gssdh.dbSession)
	if isProviderRequest {
		if orgProvider.ID != st.InfrastructureProviderID {
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Site is not associated with org's Infrastructure Provider", nil)
		}
	} else {
		// Check Site association with Tenant
		_, serr := tsDAO.GetByTenantIDAndSiteID(ctx, nil, orgTenant.ID, stID, nil)
		if serr != nil {
			if serr == cdb.ErrDoesNotExist {
				return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant does not have access to this Site", nil)
			}
			logger.Error().Err(serr).Msg("error retrieving TenantSite from DB")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to determine Tenant access to Site, DB error", nil)
		}
	}

	// handle retrieving and building status details response
	apiSds, err := handleEntityStatusDetails(ctx, c, gssdh.dbSession, stStrID, logger)
	if err != nil {
		return err
	}

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusOK, apiSds)
}
