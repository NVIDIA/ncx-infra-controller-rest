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
// @Description Get all iPXE templates propagated from bare-metal-manager-core. The `siteId` query parameter is optional and may be repeated to filter by one or more sites. When omitted, a Provider Admin/Viewer receives templates for all sites owned by their infrastructure provider; a Tenant Admin receives templates for all sites whose provider the tenant has a Tenant Account on.
// @Tags iPXE Template
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param siteId query []string false "Optional site ID(s); may be repeated to filter by multiple sites"
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

	// Validate role (Provider Admin/Viewer or Tenant Admin) and org membership
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, h.dbSession, org, dbUser, true, false)
	if apiError != nil {
		return cerr.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Parse optional siteId query parameters. Multiple values (repeated
	// `?siteId=...&siteId=...`) are supported.
	requestedSiteIDStrs := c.QueryParams()["siteId"]
	requestedSiteIDs := make([]uuid.UUID, 0, len(requestedSiteIDStrs))
	for _, s := range requestedSiteIDStrs {
		if s == "" {
			continue
		}
		parsed, perr := uuid.Parse(s)
		if perr != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid siteId in query parameter: %s", s), nil)
		}
		requestedSiteIDs = append(requestedSiteIDs, parsed)
	}

	// Build the caller's authorized site set, tracking which sites come from the
	// provider path vs the tenant path. A site can be in both sets for a
	// dual-role caller — provider access wins (fewer restrictions).
	//
	// Note on tenant-path scoping: tenant access is established per-site via
	// `TenantSite` associations (a tenant may be associated with some sites of
	// a provider but not others).
	providerSites := map[uuid.UUID]struct{}{}
	tenantSites := map[uuid.UUID]struct{}{}

	if infrastructureProvider != nil {
		siteDAO := cdbm.NewSiteDAO(h.dbSession)
		sites, _, serr := siteDAO.GetAll(ctx, nil,
			cdbm.SiteFilterInput{InfrastructureProviderIDs: []uuid.UUID{infrastructureProvider.ID}},
			cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)},
			nil,
		)
		if serr != nil {
			logger.Error().Err(serr).Msg("error retrieving provider sites from DB")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve provider sites, DB error", nil)
		}
		for i := range sites {
			providerSites[sites[i].ID] = struct{}{}
		}
	}

	if tenant != nil {
		tsDAO := cdbm.NewTenantSiteDAO(h.dbSession)
		tss, _, terr := tsDAO.GetAll(ctx, nil,
			cdbm.TenantSiteFilterInput{TenantIDs: []uuid.UUID{tenant.ID}},
			cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)},
			nil,
		)
		if terr != nil {
			logger.Error().Err(terr).Msg("error retrieving Tenant Site associations from DB")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant Site associations, DB error", nil)
		}
		for i := range tss {
			tenantSites[tss[i].SiteID] = struct{}{}
		}
	}

	isAuthorized := func(id uuid.UUID) bool {
		if _, ok := providerSites[id]; ok {
			return true
		}
		_, ok := tenantSites[id]
		return ok
	}

	// Determine the effective site filter:
	//   - siteId(s) provided: must all be authorized; use them as-is.
	//   - siteId(s) omitted:  use the union of provider and tenant accessible sites.
	var effectiveSiteIDs []uuid.UUID
	if len(requestedSiteIDs) > 0 {
		for _, id := range requestedSiteIDs {
			if !isAuthorized(id) {
				logger.Warn().Str("siteID", id.String()).Msg("org not authorized to access requested Site")
				return cerr.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Current org is not authorized to access Site: %s", id.String()), nil)
			}
		}
		effectiveSiteIDs = requestedSiteIDs
	} else {
		effectiveSiteIDs = make([]uuid.UUID, 0, len(providerSites)+len(tenantSites))
		seen := map[uuid.UUID]struct{}{}
		for id := range providerSites {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				effectiveSiteIDs = append(effectiveSiteIDs, id)
			}
		}
		for id := range tenantSites {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				effectiveSiteIDs = append(effectiveSiteIDs, id)
			}
		}
	}

	// No authorized sites — neither provider-owned nor reachable via a tenant account.
	if len(effectiveSiteIDs) == 0 {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Current org is not associated with any Site", nil)
	}

	// Validate pagination request
	pageRequest := pagination.PageRequest{}
	if err := c.Bind(&pageRequest); err != nil {
		logger.Warn().Err(err).Msg("error binding pagination request data into API model")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request pagination data", nil)
	}
	if err := pageRequest.Validate(cdbm.IpxeTemplateOrderByFields); err != nil {
		logger.Warn().Err(err).Msg("error validating pagination request data")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate pagination request data", err)
	}

	// Get iPXE templates from DB
	templateDAO := cdbm.NewIpxeTemplateDAO(h.dbSession)
	templates, total, err := templateDAO.GetAll(
		ctx,
		nil,
		cdbm.IpxeTemplateFilterInput{SiteIDs: effectiveSiteIDs},
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
// @Description Retrieve an iPXE template by its stable core ID for a given site. Any Tenant Admin or Provider Admin/Viewer can access templates for sites they are associated with.
// @Tags iPXE Template
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "Stable template ID (UUID from core)"
// @Param siteId query string true "ID of Site"
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

	// Validate role (Provider Admin/Viewer or Tenant Admin) — this also validates
	// org membership, so no separate membership check is needed here.
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, h.dbSession, org, dbUser, true, false)
	if apiError != nil {
		return cerr.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Parse template ID from URL (this is the stable core template UUID)
	templateIDStr := c.Param("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid iPXE template ID: %s", templateIDStr), nil)
	}

	logger = logger.With().Str("IpxeTemplate ID", templateIDStr).Logger()
	h.tracerSpan.SetAttribute(handlerSpan, attribute.String("ipxe_template_id", templateIDStr), logger)

	// siteId is required to disambiguate the template across sites
	siteIDStr := c.QueryParam("siteId")
	if siteIDStr == "" {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Site ID must be specified in query parameter 'siteId'", nil)
	}

	site, err := common.GetSiteFromIDString(ctx, nil, siteIDStr, h.dbSession)
	if err != nil {
		if errors.Is(err, common.ErrInvalidID) {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid siteId in request data", nil)
		}
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data does not exist", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site specified in request data, DB error", nil)
	}

	// Get template from DB by (siteID, templateID)
	templateDAO := cdbm.NewIpxeTemplateDAO(h.dbSession)
	tmpl, err := templateDAO.GetBySiteAndTemplateID(ctx, nil, site.ID, templateID)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find iPXE template with ID: %s for site: %s", templateIDStr, siteIDStr), nil)
		}
		logger.Error().Err(err).Msg("error retrieving iPXE template from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve iPXE template, DB error", nil)
	}

	// Validate site ownership — evaluate both roles independently so a mixed-role
	// service account succeeds if either path authorizes the request.
	providerAllowed := infrastructureProvider != nil && site.InfrastructureProviderID == infrastructureProvider.ID

	tenantAllowed := false
	if !providerAllowed && tenant != nil {
		tsDAO := cdbm.NewTenantSiteDAO(h.dbSession)
		_, tsCount, serr := tsDAO.GetAll(ctx, nil, cdbm.TenantSiteFilterInput{
			TenantIDs: []uuid.UUID{tenant.ID},
			SiteIDs:   []uuid.UUID{site.ID},
		}, cdbp.PageInput{}, nil)
		if serr != nil {
			logger.Error().Err(serr).Msg("error retrieving Tenant Site association for Site")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error retrieving Tenant Site association for Site", nil)
		}
		tenantAllowed = tsCount > 0
	}

	if !providerAllowed && !tenantAllowed {
		logger.Warn().Msg("neither provider nor tenant authorization succeeded for Site access")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Current org is not authorized to access the iPXE template's Site", nil)
	}

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, model.NewAPIIpxeTemplate(tmpl))
}
