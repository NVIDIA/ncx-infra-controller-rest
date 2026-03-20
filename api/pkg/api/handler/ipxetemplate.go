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
	"errors"
	"fmt"
	"net/http"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/internal/config"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/handler/util/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/model"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/pagination"
	cerr "github.com/NVIDIA/ncx-infra-controller-rest/common/pkg/util"
	sutil "github.com/NVIDIA/ncx-infra-controller-rest/common/pkg/util"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
	cdbp "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	tclient "go.temporal.io/sdk/client"
)

// ~~~~~ GetAll Handler ~~~~~ //

// GetAllIpxeTemplateHandler is the API Handler for getting all iPXE templates
type GetAllIpxeTemplateHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetAllIpxeTemplateHandler initializes and returns a new handler for getting all iPXE templates
func NewGetAllIpxeTemplateHandler(dbSession *cdb.Session, tc tclient.Client, cfg *config.Config) GetAllIpxeTemplateHandler {
	return GetAllIpxeTemplateHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get all iPXE templates
// @Description Get all iPXE templates propagated from bare-metal-manager-core for a given site
// @Tags iPXE Template
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param siteId query string true "ID of Site"
// @Param scope query string false "Filter by scope ('internal' or 'public')"
// @Param pageNumber query integer false "Page number of results returned"
// @Param pageSize query integer false "Number of results per page"
// @Param orderBy query string false "Order by field"
// @Success 200 {object} []model.APIIpxeTemplate
// @Router /v2/org/{org}/carbide/ipxe-template [get]
func (h GetAllIpxeTemplateHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("GetAll", "IpxeTemplate", c, h.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org membership
	if _, err := dbUser.OrgData.GetOrgByName(org); err != nil {
		logger.Warn().Msg("could not validate org membership for user, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	// Validate role: only Provider Admins/Viewers or Tenant Admins with TargetedInstanceCreation capability
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, h.dbSession, org, dbUser, true, true)
	if apiError != nil {
		return cerr.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// siteId is required
	siteIDStr := c.QueryParam("siteId")
	if siteIDStr == "" {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Site ID must be specified in query parameter 'siteId'", nil)
	}

	site, err := common.GetSiteFromIDString(ctx, nil, siteIDStr, h.dbSession)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data does not exist", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site specified in request data, DB error", nil)
	}

	// Validate site ownership
	if infrastructureProvider != nil {
		if site.InfrastructureProviderID != infrastructureProvider.ID {
			logger.Warn().Msg("Site specified in request data does not belong to current org's Infrastructure Provider")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Site specified in request data does not belong to current org", nil)
		}
	} else if tenant != nil {
		if !tenant.Config.TargetedInstanceCreation {
			logger.Warn().Msg("Tenant doesn't have targeted Instance creation capability, access denied")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant must have targeted Instance creation capability in order to retrieve iPXE templates", nil)
		}

		taDAO := cdbm.NewTenantAccountDAO(h.dbSession)
		_, taCount, serr := taDAO.GetAll(ctx, nil, cdbm.TenantAccountFilterInput{
			InfrastructureProviderID: &site.InfrastructureProviderID,
			TenantIDs:                []uuid.UUID{tenant.ID},
		}, cdbp.PageInput{}, []string{})
		if serr != nil {
			logger.Error().Err(serr).Msg("error retrieving Tenant Account for Site")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error retrieving Tenant Account for Site", nil)
		}
		if taCount == 0 {
			logger.Error().Msg("privileged Tenant doesn't have an account with Infrastructure Provider")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Privileged Tenant must have an account with Provider of Site specified in query", nil)
		}
	}

	filterInput := cdbm.IpxeTemplateFilterInput{
		SiteIDs: []uuid.UUID{site.ID},
	}

	// Optional scope filter
	if scopeStr := c.QueryParam("scope"); scopeStr != "" {
		filterInput.Scopes = []string{scopeStr}
	}

	// Validate pagination request
	pageRequest := pagination.PageRequest{}
	if err = c.Bind(&pageRequest); err != nil {
		logger.Warn().Err(err).Msg("error binding pagination request data into API model")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request pagination data", nil)
	}
	if err = pageRequest.Validate(cdbm.IpxeTemplateOrderByFields); err != nil {
		logger.Warn().Err(err).Msg("error validating pagination request data")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate pagination request data", err)
	}

	// Get iPXE templates from DB
	templateDAO := cdbm.NewIpxeTemplateDAO(h.dbSession)
	templates, total, err := templateDAO.GetAll(
		ctx,
		nil,
		filterInput,
		paginator.PageInput{
			Offset:  pageRequest.Offset,
			Limit:   pageRequest.Limit,
			OrderBy: pageRequest.OrderBy,
		},
	)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving iPXE templates from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve iPXE templates, DB error", nil)
	}

	// Build response
	apiTemplates := []*model.APIIpxeTemplate{}
	for i := range templates {
		apiTemplates = append(apiTemplates, model.NewAPIIpxeTemplate(&templates[i]))
	}

	// Build pagination response header
	pageResponse := pagination.NewPageResponse(*pageRequest.PageNumber, *pageRequest.PageSize, total, pageRequest.OrderByStr)
	pageHeader, err := json.Marshal(pageResponse)
	if err != nil {
		logger.Error().Err(err).Msg("error marshaling pagination response")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to generate pagination response header", nil)
	}
	c.Response().Header().Set(pagination.ResponseHeaderName, string(pageHeader))

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, apiTemplates)
}

// ~~~~~ Get Handler ~~~~~ //

// GetIpxeTemplateHandler is the API Handler for retrieving a single iPXE template
type GetIpxeTemplateHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetIpxeTemplateHandler initializes and returns a new handler to retrieve an iPXE template
func NewGetIpxeTemplateHandler(dbSession *cdb.Session, tc tclient.Client, cfg *config.Config) GetIpxeTemplateHandler {
	return GetIpxeTemplateHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Retrieve an iPXE template
// @Description Retrieve an iPXE template by ID
// @Tags iPXE Template
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of iPXE template"
// @Success 200 {object} model.APIIpxeTemplate
// @Router /v2/org/{org}/carbide/ipxe-template/{id} [get]
func (h GetIpxeTemplateHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Get", "IpxeTemplate", c, h.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org membership
	if _, err := dbUser.OrgData.GetOrgByName(org); err != nil {
		logger.Warn().Msg("could not validate org membership for user, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	// Validate role
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, h.dbSession, org, dbUser, true, true)
	if apiError != nil {
		return cerr.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Parse template ID from URL
	templateIDStr := c.Param("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid iPXE template ID: %s", templateIDStr), nil)
	}

	logger = logger.With().Str("IpxeTemplate ID", templateIDStr).Logger()
	h.tracerSpan.SetAttribute(handlerSpan, attribute.String("ipxe_template_id", templateIDStr), logger)

	// Get template from DB
	templateDAO := cdbm.NewIpxeTemplateDAO(h.dbSession)
	tmpl, err := templateDAO.Get(ctx, nil, templateID)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find iPXE template with ID: %s", templateIDStr), nil)
		}
		logger.Error().Err(err).Msg("error retrieving iPXE template from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve iPXE template, DB error", nil)
	}

	// Get Site to validate access
	siteDAO := cdbm.NewSiteDAO(h.dbSession)
	site, err := siteDAO.GetByID(ctx, nil, tmpl.SiteID, nil, false)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site details for iPXE template, DB error", nil)
	}

	// Validate site ownership
	if infrastructureProvider != nil {
		if site.InfrastructureProviderID != infrastructureProvider.ID {
			logger.Warn().Msg("iPXE template does not belong to a Site owned by org's Infrastructure Provider")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "iPXE template does not belong to a Site owned by current org", nil)
		}
	} else if tenant != nil {
		if !tenant.Config.TargetedInstanceCreation {
			logger.Warn().Msg("Tenant doesn't have targeted Instance creation capability, access denied")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant must have targeted Instance creation capability in order to retrieve iPXE template", nil)
		}

		taDAO := cdbm.NewTenantAccountDAO(h.dbSession)
		_, taCount, serr := taDAO.GetAll(ctx, nil, cdbm.TenantAccountFilterInput{
			InfrastructureProviderID: &site.InfrastructureProviderID,
			TenantIDs:                []uuid.UUID{tenant.ID},
		}, cdbp.PageInput{}, []string{})
		if serr != nil {
			logger.Error().Err(serr).Msg("error retrieving Tenant Account for Site")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error retrieving Tenant Account for Site", nil)
		}
		if taCount == 0 {
			logger.Error().Msg("privileged Tenant doesn't have an account with Infrastructure Provider")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Privileged Tenant must have an account with Provider of iPXE template's Site", nil)
		}
	}

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, model.NewAPIIpxeTemplate(tmpl))
}
