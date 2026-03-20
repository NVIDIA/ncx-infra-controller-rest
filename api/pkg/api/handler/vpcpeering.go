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
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	tclient "go.temporal.io/sdk/client"
	tp "go.temporal.io/sdk/temporal"

	"github.com/nvidia/bare-metal-manager-rest/api/internal/config"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/handler/util/common"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/pagination"
	sc "github.com/nvidia/bare-metal-manager-rest/api/pkg/client/site"
	cutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	"github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"
	cdbp "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"
	swe "github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/error"
	"github.com/nvidia/bare-metal-manager-rest/workflow/pkg/queue"

	cwssaws "github.com/nvidia/bare-metal-manager-rest/workflow-schema/schema/site-agent/workflows/v1"
)

// ~~~~~ Create VPC Peering Handler ~~~~~ //

// CreateVpcPeeringHandler is the API Handler for creating new VPC Peering
type CreateVpcPeeringHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewCreateVpcPeeringHandler initializes and returns a new handler for creating VPC peering
func NewCreateVpcPeeringHandler(dbSession *cdb.Session, tc tclient.Client, sc *sc.ClientPool, cfg *config.Config) CreateVpcPeeringHandler {
	return CreateVpcPeeringHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        sc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Create a VPC Peering
// @Description Create a VPC peering between two VPCs on the same site.
// @Tags vpcpeering
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param message body model.APIVpcPeeringCreateRequest true "VPC peering create request"
// @Success 201 {object} model.APIVpcPeering
// @Router /v2/org/{org}/forge/vpc-peering [post]
func (cvph CreateVpcPeeringHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Create", "VpcPeering", c, cvph.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org membership and resolve provider/tenant authorization context.
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, cvph.dbSession, org, dbUser, false, false)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Validate request
	// Bind request data to API model
	apiRequest := model.APIVpcPeeringCreateRequest{}
	err := c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate()
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating VPC peering creation request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating VPC peering creation request data", verr)
	}

	// Retrieve the Site from the DB
	site, err := common.GetSiteFromIDString(ctx, nil, apiRequest.SiteID, cvph.dbSession)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data does not exist", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site specified in request data, DB error", nil)
	}

	// Validate that site is in Registered state
	if site.Status != cdbm.SiteStatusRegistered {
		logger.Warn().Msg("Site specified in request data is not in Registered state")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data is not in Registered state, cannot create VPC Peering", nil)
	}

	// Parse VPC IDs from the request body
	vpc1ID := uuid.MustParse(apiRequest.Vpc1ID)
	vpc2ID := uuid.MustParse(apiRequest.Vpc2ID)

	// Validate both VPCs exist and are accessible
	vpcDAO := cdbm.NewVpcDAO(cvph.dbSession)
	vpc1, err := vpcDAO.GetByID(ctx, nil, vpc1ID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find VPC with ID: %s", vpc1ID.String()), nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC 1 from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve VPC 1 with ID: %s, DB error", vpc1ID.String()), nil)
	}
	vpc2, err := vpcDAO.GetByID(ctx, nil, vpc2ID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find VPC with ID: %s", vpc2ID.String()), nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC 2 from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve VPC 2 with ID: %s, DB error", vpc2ID.String()), nil)
	}

	// Validate VPCs are both on the provided site
	if vpc1.SiteID != site.ID {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("VPC 1: %s does not belong to Site: %s", vpc1ID.String(), site.ID.String()), nil)
	} else if vpc2.SiteID != site.ID {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("VPC 2: %s does not belong to Site: %s", vpc2ID.String(), site.ID.String()), nil)
	}

	// Validate VPCs are in Ready state
	if vpc1.Status != cdbm.VpcStatusReady || vpc2.Status != cdbm.VpcStatusReady {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Both VPCs must be in Ready state to proceed with peering", nil)
	}

	// Check if peering already exists
	vpcPeeringDAO := cdbm.NewVpcPeeringDAO(cvph.dbSession)
	existingPeerings, _, err := vpcPeeringDAO.GetAll(ctx, nil, cdbm.VpcPeeringFilterInput{
		VpcIDs: []uuid.UUID{vpc1ID},
	}, cdbp.PageInput{}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error checking for existing VPC peering")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to check for existing VPC peering, DB error", nil)
	}
	for _, peering := range existingPeerings {
		// One of the VPC IDs must match the VPC ID, so we only need to check if either one equals the peer VPC ID
		if peering.Vpc1ID == vpc2ID || peering.Vpc2ID == vpc2ID {
			return cutil.NewAPIErrorResponse(c, http.StatusConflict, "VPC peering already exists between VPCs specified in request data", nil)
		}
	}

	isMultiTenant := vpc1.TenantID != vpc2.TenantID

	// Tenant Admin only allowed to create single-tenant VPC peerings where both VPCs belong to the them.
	// Provider Admin is allowed to create multi-tenant VPC peerings.
	// If the user is both a Provider Admin and a Tenant Admin, they should be able to create
	// multi-tenant VPC peerings, and any single-tenant VPCs peerings where both VPCs belong to
	// them, but not single tenant VPC peerings where both VPCs belong to another tenant.
	providerAuthorized := false
	if infrastructureProvider != nil {
		// Provider Admin creating multi-tenant peerings requires both tenants to have
		// Tenant Accounts with the Provider and both tenants must have access to the Site.
		if site.InfrastructureProviderID == infrastructureProvider.ID && isMultiTenant {
			taDAO := cdbm.NewTenantAccountDAO(cvph.dbSession)
			_, taCount, err := taDAO.GetAll(ctx, nil, cdbm.TenantAccountFilterInput{
				InfrastructureProviderID: &infrastructureProvider.ID,
				TenantIDs:                []uuid.UUID{vpc1.TenantID, vpc2.TenantID},
			}, cdbp.PageInput{}, nil)
			if err != nil {
				logger.Error().Err(err).Msg("error retrieving Tenant Accounts for tenants of the VPC peering")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to validate Tenant Accounts for tenants of the VPC peering, DB error", nil)
			}
			if taCount == 2 {
				tsDAO := cdbm.NewTenantSiteDAO(cvph.dbSession)
				tenantSites, _, serr := tsDAO.GetAll(
					ctx,
					nil,
					cdbm.TenantSiteFilterInput{
						TenantIDs: []uuid.UUID{vpc1.TenantID, vpc2.TenantID},
						SiteIDs:   []uuid.UUID{site.ID},
					},
					cdbp.PageInput{},
					nil,
				)
				if serr != nil {
					logger.Error().Err(serr).Msg("error retrieving TenantSite for tenants of the VPC peering")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to validate Site access for tenants of the VPC peering, DB error", nil)
				}
				if len(tenantSites) == 2 {
					logger.Info().Msg("Provider is authorized to create multi-tenant VPC peering")
					providerAuthorized = true
				} else {
					logger.Warn().Msg("Not all tenants have access to Site specified in request")
				}
			} else {
				logger.Warn().Msg("Not all tenants have Tenant Accounts with the Provider")
			}
		}
	}

	tenantAuthorized := false
	if tenant != nil {
		// Tenant Admin: tenant must have access to the site.
		tsDAO := cdbm.NewTenantSiteDAO(cvph.dbSession)
		tenantSites, _, err := tsDAO.GetAll(
			ctx,
			nil,
			cdbm.TenantSiteFilterInput{
				TenantIDs: []uuid.UUID{tenant.ID},
				SiteIDs:   []uuid.UUID{site.ID},
			},
			paginator.PageInput{Limit: cdb.GetIntPtr(1)},
			nil,
		)
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving TenantSite from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to validate Site access for Tenant, DB error", nil)
		}

		// Tenant-only users can only create single-tenant peerings where both VPCs belong to them.
		if len(tenantSites) > 0 && vpc1.TenantID == tenant.ID && vpc2.TenantID == tenant.ID {
			logger.Info().Msg("Tenant is authorized to create single-tenant VPC peering")
			tenantAuthorized = true
		} else {
			if vpc1.TenantID != tenant.ID {
				logger.Warn().Str("vpc_id", vpc1.ID.String()).Str("tenant_id", tenant.ID.String()).Msg("VPC 1 does not belong to Tenant associated with current org")
			}
			if vpc2.TenantID != tenant.ID {
				logger.Warn().Str("vpc_id", vpc2.ID.String()).Str("tenant_id", tenant.ID.String()).Msg("VPC 2 does not belong to Tenant associated with current org")
			}
		}
	}

	if !providerAuthorized && !tenantAuthorized {
		logger.Warn().Msg("User does not have access to create the VPC peering")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have access to create the VPC peering", nil)
	}

	var infrastructureProviderID *uuid.UUID
	if providerAuthorized && infrastructureProvider != nil {
		infrastructureProviderID = &infrastructureProvider.ID
	}

	var tenantID *uuid.UUID
	if tenantAuthorized && tenant != nil {
		tenantID = &tenant.ID
	}

	// Start a db tx
	tx, err := cdb.BeginTx(ctx, cvph.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create VPC peering", nil)
	}

	// If false, a rollback will be triggered on any early return.
	// If all goes well, we'll set it to true later on.
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Create VPC peering record in the db
	vpcPeering, err := vpcPeeringDAO.Create(
		ctx,
		tx,
		cdbm.VpcPeeringCreateInput{
			Vpc1ID:                   vpc1ID,
			Vpc2ID:                   vpc2ID,
			SiteID:                   site.ID,
			IsMultiTenant:            isMultiTenant,
			InfrastructureProviderID: infrastructureProviderID,
			TenantID:                 tenantID,
			CreatedByID:              dbUser.ID,
		},
	)
	if err != nil {
		logger.Error().Err(err).Msg("error creating VPC peering in DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create VPC peering, DB error", nil)
	}

	// Create a status detail record for the VPC peering
	sdDAO := cdbm.NewStatusDetailDAO(cvph.dbSession)
	statusDetail, serr := sdDAO.CreateFromParams(ctx, tx, vpcPeering.ID.String(),
		*cdb.GetStrPtr(cdbm.VpcPeeringStatusPending),
		cdb.GetStrPtr("processed vpc peering creation request"))
	if serr != nil {
		logger.Error().Err(serr).Msg("error creating status detail for VPC peering")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Status Detail for VPC peering", nil)
	}
	if statusDetail == nil {
		logger.Error().Msg("Status Detail DB entry not returned from CreateFromParams")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to get new Status Detail for VPC peering", nil)
	}

	// Create the peering directly in Carbide via site agent
	err = vpcPeeringDAO.UpdateStatusByID(ctx, tx, vpcPeering.ID, cdbm.VpcPeeringStatusConfiguring)
	if err != nil {
		logger.Error().Err(err).Msg("error updating VPC peering status to Configuring")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update VPC peering status to Configuring", nil)
	}

	// Create the VPC peering creation request
	createVpcPeeringRequest := &cwssaws.VpcPeeringCreationRequest{
		VpcId:     &cwssaws.VpcId{Value: vpcPeering.Vpc1ID.String()},
		PeerVpcId: &cwssaws.VpcId{Value: vpcPeering.Vpc2ID.String()},
		Id:        &cwssaws.VpcPeeringId{Value: vpcPeering.ID.String()},
	}

	logger.Info().Msg("triggering VPC peering create workflow")

	// Create workflow options
	workflowOptions := tclient.StartWorkflowOptions{
		ID:                       "vpcpeering-create-" + vpcPeering.ID.String(),
		WorkflowExecutionTimeout: cutil.WorkflowExecutionTimeout,
		TaskQueue:                queue.SiteTaskQueue,
	}

	// Get the temporal client for the site we are working with
	stc, err := cvph.scp.GetClientByID(vpcPeering.SiteID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Temporal client for Site", nil)
	}

	// Add context deadline
	workflowCtx, cancel := context.WithTimeout(ctx, cutil.WorkflowContextTimeout)
	defer cancel()

	// Trigger site workflow to create VPC peering
	workflowRun, err := stc.ExecuteWorkflow(workflowCtx, workflowOptions, "CreateVpcPeering", createVpcPeeringRequest)
	if err != nil {
		logger.Error().Err(err).Msg("failed to start VPC peering creation workflow")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to start VPC peering creation workflow", nil)
	}

	workflowId := workflowRun.GetID()

	logger.Info().Str("Workflow ID", workflowId).Msg("started VPC peering creation workflow")

	// Wait for workflow completion synchronously
	err = workflowRun.Get(workflowCtx, nil)
	if err != nil {
		var applicationErr *tp.ApplicationError
		if errors.As(err, &applicationErr) && (applicationErr.Type() == swe.ErrTypeCarbideUnimplemented || applicationErr.Type() == swe.ErrTypeCarbideDenied) {
			logger.Error().Msg("feature not yet implemented on target Site")
			return cutil.NewAPIErrorResponse(c, http.StatusNotImplemented, fmt.Sprintf("Feature not yet implemented on target Site: %s", err), nil)
		}

		var timeoutErr *tp.TimeoutError
		if errors.As(err, &timeoutErr) || err == context.DeadlineExceeded || ctx.Err() != nil {
			return common.TerminateWorkflowOnTimeOut(c, logger, stc, workflowId, err, "VpcPeering", "CreateVpcPeering")
		}

		logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to update CreateVpcPeering")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to execute sync workflow to create VPC Peering on Site: %s", err), nil)
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create VPC peering", nil)
	}
	txCommitted = true

	// Best effort post-commit update: workflow completed, so mark peering as Ready.
	// This is intentionally outside of the transaction so create does not fail if this update fails.
	status := cdbm.VpcPeeringStatusConfiguring
	err = vpcPeeringDAO.UpdateStatusByID(ctx, nil, vpcPeering.ID, cdbm.VpcPeeringStatusReady)
	if err != nil {
		logger.Warn().Err(err).Msg("best-effort update to Ready status failed after workflow completion")
	} else {
		status = cdbm.VpcPeeringStatusReady
	}

	// Update API model with best-known status.
	apiVpcPeering := model.NewAPIVpcPeering(*vpcPeering)
	apiVpcPeering.Status = status
	logger.Info().Str("VPC Peering ID", vpcPeering.ID.String()).Msg("successfully created VPC peering and completed creation workflow")

	return c.JSON(http.StatusCreated, apiVpcPeering)
}

// ~~~~~ Get All VPC Peering Handler ~~~~~ //

// GetAllVpcPeeringHandler is the API Handler for getting all VPC peerings for a VPC
type GetAllVpcPeeringHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewGetAllVpcPeeringHandler initializes and returns a new handler for getting all VPC peerings
func NewGetAllVpcPeeringHandler(dbSession *cdb.Session, tc tclient.Client, cfg *config.Config) GetAllVpcPeeringHandler {
	return GetAllVpcPeeringHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get all VPC peerings
// @Description Get all VPC peerings.
// @Tags vpcpeering
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param siteId query string false "Site ID"
// @Param pageNumber query integer false "Page number of results returned"
// @Param pageSize query integer false "Number of results per page"
// @Param orderBy query string false "Order by field"
// @Success 200 {array} model.APIVpcPeering
// @Router /v2/org/{org}/forge/vpc-peering [get]
func (gavph GetAllVpcPeeringHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("GetAll", "VpcPeering", c, gavph.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org membership and resolve provider/tenant authorization context.
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, gavph.dbSession, org, dbUser, false, false)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	siteIDStr := c.QueryParam("siteId")
	var siteID *uuid.UUID
	var err error
	if siteIDStr != "" {
		providerSiteAuthorized := false
		tenantSiteAuthorized := false

		site, err := common.GetSiteFromIDString(ctx, nil, siteIDStr, gavph.dbSession)
		if err != nil {
			if errors.Is(err, cdb.ErrDoesNotExist) {
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in query does not exist", nil)
			}
			logger.Error().Err(err).Msg("error retrieving Site from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site specified in query, DB error", nil)
		}
		siteID = &site.ID
		gavph.tracerSpan.SetAttribute(handlerSpan, attribute.String("site_id", siteIDStr), logger)

		if infrastructureProvider != nil {
			providerSiteAuthorized = site.InfrastructureProviderID == infrastructureProvider.ID
		}

		if tenant != nil {
			tsDAO := cdbm.NewTenantSiteDAO(gavph.dbSession)
			tenantSites, _, err := tsDAO.GetAll(
				ctx,
				nil,
				cdbm.TenantSiteFilterInput{
					TenantIDs: []uuid.UUID{tenant.ID},
					SiteIDs:   []uuid.UUID{site.ID},
				},
				paginator.PageInput{Limit: cdb.GetIntPtr(1)},
				nil,
			)
			if err != nil {
				logger.Error().Err(err).Msg("error retrieving TenantSite from DB")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to validate Site access for Tenant, DB error", nil)
			}
			tenantSiteAuthorized = len(tenantSites) > 0
		}

		if !providerSiteAuthorized && !tenantSiteAuthorized {
			logger.Warn().Msg("User does not have access to the Site")
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have access to the Site", nil)
		}
	}

	// Validate pagination request early so defaults are initialized before any early returns.
	pageRequest := pagination.PageRequest{}
	err = c.Bind(&pageRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding pagination request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request pagination data", nil)
	}

	// Validate pagination attributes and set defaults.
	err = pageRequest.Validate(cdbm.VpcPeeringOrderByFields)
	if err != nil {
		logger.Warn().Err(err).Msg("error validating pagination request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate pagination request data", err)
	}

	vpcPeeringDAO := cdbm.NewVpcPeeringDAO(gavph.dbSession)
	filterInput := cdbm.VpcPeeringFilterInput{
		InfrastructureProviderID: nil,
		TenantID:                 nil,
	}

	if infrastructureProvider != nil {
		filterInput.InfrastructureProviderID = &infrastructureProvider.ID
	}
	if tenant != nil {
		filterInput.TenantID = &tenant.ID
	}
	if siteID != nil {
		filterInput.SiteIDs = []uuid.UUID{*siteID}
	}

	vpcPeeringPageInput := cdbp.PageInput{
		Limit:   pageRequest.Limit,
		Offset:  pageRequest.Offset,
		OrderBy: pageRequest.OrderBy,
	}
	vpcPeerings, total, err := vpcPeeringDAO.GetAll(ctx, nil, filterInput, vpcPeeringPageInput, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving VPC peerings from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPC peerings", nil)
	}

	// Convert to API models
	apiVpcPeerings := make([]model.APIVpcPeering, len(vpcPeerings))
	for i, vpcPeering := range vpcPeerings {
		apiVpcPeerings[i] = model.NewAPIVpcPeering(vpcPeering)
	}

	logger.Info().Int("Count", len(apiVpcPeerings)).Msg("successfully retrieved VPC peerings")

	// Create pagination response header
	pageResponse := pagination.NewPageResponse(*pageRequest.PageNumber, *pageRequest.PageSize, total, pageRequest.OrderByStr)
	pageHeader, err := json.Marshal(pageResponse)
	if err != nil {
		logger.Error().Err(err).Msg("error marshaling pagination response")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to generate pagination response header", nil)
	}

	c.Response().Header().Set(pagination.ResponseHeaderName, string(pageHeader))

	logger.Info().Msg("finishing GetAllVpcPeeringHandler API handler")

	return c.JSON(http.StatusOK, apiVpcPeerings)
}

// ~~~~~ Get VPC Peering Handler ~~~~~ //

// GetVpcPeeringHandler is the API Handler for getting a VPC peering
type GetVpcPeeringHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewGetVpcPeeringHandler initializes and returns a new handler for getting VPC peering
func NewGetVpcPeeringHandler(dbSession *cdb.Session, tc tclient.Client, cfg *config.Config) GetVpcPeeringHandler {
	return GetVpcPeeringHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get a VPC peering
// @Description Get details of a VPC peering by ID.
// @Tags vpcpeering
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "VPC Peering ID"
// @Success 200 {object} model.APIVpcPeering
// @Router /v2/org/{org}/forge/vpc-peering/{id} [get]
func (gvph GetVpcPeeringHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Get", "VpcPeering", c, gvph.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, gvph.dbSession, org, dbUser, false, false)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	peeringID := c.Param("id")
	logger = logger.With().Str("Peering ID", peeringID).Logger()
	gvph.tracerSpan.SetAttribute(handlerSpan, attribute.String("peering_id", peeringID), logger)

	// Parse and validate peering ID
	peeringUUID, err := uuid.Parse(peeringID)
	if err != nil {
		logger.Warn().Err(err).Msg("error parsing VPC peering ID in URL")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid VPC peering ID in URL parameter", nil)
	}

	// Get VPC peering
	vpcPeeringDAO := cdbm.NewVpcPeeringDAO(gvph.dbSession)
	vpcPeering, err := vpcPeeringDAO.GetByID(ctx, nil, peeringUUID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find VPC peering with ID: %s", peeringUUID.String()), nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC peering from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPC peering", nil)
	}

	// Validate if user is authorized to get the VPC peering
	providerAuthorized := infrastructureProvider != nil && vpcPeering.InfrastructureProviderID != nil && *vpcPeering.InfrastructureProviderID == infrastructureProvider.ID

	// For a tenant to be able to get a VPC peering, at least one of the VPCs of the VPC peering
	// must belong to the tenant
	tenantAuthorized := false
	if tenant != nil {
		// Get two VPCs of the VPC peering
		vpcDAO := cdbm.NewVpcDAO(gvph.dbSession)
		vpc1, err := vpcDAO.GetByID(ctx, nil, vpcPeering.Vpc1ID, nil)
		if err != nil {
			if err == cdb.ErrDoesNotExist {
				return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find VPC with ID: %s", vpcPeering.Vpc1ID.String()), nil)
			}
			logger.Error().Err(err).Msg("error retrieving VPC 1 of VPC peering from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve VPC 1 with ID: %s, DB error", vpcPeering.Vpc1ID.String()), nil)
		}
		vpc2, err := vpcDAO.GetByID(ctx, nil, vpcPeering.Vpc2ID, nil)
		if err != nil {
			if err == cdb.ErrDoesNotExist {
				return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find VPC with ID: %s", vpcPeering.Vpc2ID.String()), nil)
			}
			logger.Error().Err(err).Msg("error retrieving VPC 2 of VPC peering from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve VPC 2 with ID: %s, DB error", vpcPeering.Vpc2ID.String()), nil)
		}

		if vpc1.TenantID == tenant.ID || vpc2.TenantID == tenant.ID {
			logger.Info().Msg("Tenant is authorized to get single-tenant VPC peering")
			tenantAuthorized = true
		} else {
			if vpc1.TenantID != tenant.ID {
				logger.Warn().Str("vpc_id", vpc1.ID.String()).Str("tenant_id", tenant.ID.String()).Msg("VPC 1 does not belong to Tenant associated with current org")
			}
			if vpc2.TenantID != tenant.ID {
				logger.Warn().Str("vpc_id", vpc2.ID.String()).Str("tenant_id", tenant.ID.String()).Msg("VPC 2 does not belong to Tenant associated with current org")
			}
		}
	}

	if !providerAuthorized && !tenantAuthorized {
		logger.Warn().Msg("User does not have access to the VPC peering")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have access to the VPC peering", nil)
	}

	// Convert to API model
	apiVpcPeering := model.NewAPIVpcPeering(*vpcPeering)

	logger.Info().Msg("successfully retrieved VPC peering")

	return c.JSON(http.StatusOK, apiVpcPeering)
}

// ~~~~~ Delete VPC Peering Handler ~~~~~ //

// DeleteVpcPeeringHandler is the API Handler for deleting a VPC peering
type DeleteVpcPeeringHandler struct {
	dbSession  *cdb.Session
	tc         tclient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewDeleteVpcPeeringHandler initializes and returns a new handler for deleting VPC peering
func NewDeleteVpcPeeringHandler(dbSession *cdb.Session, tc tclient.Client, sc *sc.ClientPool, cfg *config.Config) DeleteVpcPeeringHandler {
	return DeleteVpcPeeringHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        sc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Delete a VPC peering
// @Description Delete a VPC peering by ID.
// @Tags vpcpeering
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "VPC Peering ID"
// @Success 204 "No Content"
// @Router /v2/org/{org}/forge/vpc-peering/{id} [delete]
func (dvph DeleteVpcPeeringHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Delete", "VpcPeering", c, dvph.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	// Is DB user missing?
	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org membership and resolve provider/tenant authorization context.
	infrastructureProvider, tenant, apiError := common.IsProviderOrTenant(ctx, logger, dvph.dbSession, org, dbUser, false, false)
	if apiError != nil {
		return cutil.NewAPIErrorResponse(c, apiError.Code, apiError.Message, apiError.Data)
	}

	// Get VPC peering ID from URL param
	vpcPeeringID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid VPC peering ID", nil)
	}
	logger = logger.With().Str("VPC Peering ID", vpcPeeringID.String()).Logger()

	dvph.tracerSpan.SetAttribute(handlerSpan, attribute.String("vpc_peering_id", vpcPeeringID.String()), logger)

	// Get VPC peering from DB
	vpcPeeringDAO := cdbm.NewVpcPeeringDAO(dvph.dbSession)
	vpcPeering, err := vpcPeeringDAO.GetByID(ctx, nil, vpcPeeringID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find VPC peering with specified ID", nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC peering from DB by ID")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPC peering with specified ID", nil)
	}

	// Get two VPCs of the VPC peering
	vpcDAO := cdbm.NewVpcDAO(dvph.dbSession)
	vpc1, err := vpcDAO.GetByID(ctx, nil, vpcPeering.Vpc1ID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find VPC with ID: %s", vpcPeering.Vpc1ID.String()), nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC 1 of VPC peering from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve VPC 1 with ID: %s, DB error", vpcPeering.Vpc1ID.String()), nil)
	}
	vpc2, err := vpcDAO.GetByID(ctx, nil, vpcPeering.Vpc2ID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Could not find VPC with ID: %s", vpcPeering.Vpc2ID.String()), nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC 2 of VPC peering from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve VPC 2 with ID: %s, DB error", vpcPeering.Vpc2ID.String()), nil)
	}

	isMultiTenant := vpc1.TenantID != vpc2.TenantID

	// Validate if user is authorized to delete the VPC peering
	providerAuthorized := false
	if infrastructureProvider != nil {
		// Provider Admin can delete peerings in sites provided by this org.
		// The deletion operation is not gated on TenantAccount/TenantSite checks to avoid
		// blocking cleanup.
		site, err := common.GetSiteFromIDString(ctx, nil, vpcPeering.SiteID.String(), dvph.dbSession)
		if err != nil {
			if errors.Is(err, cdb.ErrDoesNotExist) {
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "VPC peering site not found", nil)
			}
			logger.Error().Err(err).Msg("error retrieving site for VPC peering")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPC peering", nil)
		}

		if site.InfrastructureProviderID == infrastructureProvider.ID && isMultiTenant {
			logger.Info().Msg("Provider is authorized to delete VPC peering in provided site")
			providerAuthorized = true
		}
	}

	tenantAuthorized := false
	if tenant != nil {
		// Tenant-only users can only delete single-tenant peerings where both VPCs belong to them.
		if vpc1.TenantID == tenant.ID && vpc2.TenantID == tenant.ID {
			logger.Info().Msg("Tenant is authorized to delete single-tenant VPC peering")
			tenantAuthorized = true
		} else {
			if vpc1.TenantID != tenant.ID {
				logger.Warn().Str("vpc_id", vpc1.ID.String()).Str("tenant_id", tenant.ID.String()).Msg("VPC 1 does not belong to Tenant associated with current org")
			}
			if vpc2.TenantID != tenant.ID {
				logger.Warn().Str("vpc_id", vpc2.ID.String()).Str("tenant_id", tenant.ID.String()).Msg("VPC 2 does not belong to Tenant associated with current org")
			}
		}
	}

	if !providerAuthorized && !tenantAuthorized {
		logger.Warn().Msg("User does not have access to delete the VPC peering")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have access to delete the VPC peering", nil)
	}

	// Start a db tx for the deletion workflow
	tx, err := cdb.BeginTx(ctx, dvph.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete VPC peering", nil)
	}

	// If false, a rollback will be trigger on any early return.
	// If all goes well, we'll set it to true later on.
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Update status to Deleting first
	err = vpcPeeringDAO.UpdateStatusByID(ctx, tx, vpcPeering.ID, cdbm.VpcPeeringStatusDeleting)
	if err != nil {
		logger.Error().Err(err).Msg("error updating VPC peering status to Deleting")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update VPC peering status to Deleting", nil)
	}

	// Create the VPC peering deletion request
	deleteVpcPeeringRequest := &cwssaws.VpcPeeringDeletionRequest{
		Id: &cwssaws.VpcPeeringId{Value: vpcPeering.ID.String()},
	}

	// Get the site temporal client
	stc, err := dvph.scp.GetClientByID(vpcPeering.SiteID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Temporal client for Site", nil)
	}

	// Setup workflow options
	workflowOptions := tclient.StartWorkflowOptions{
		ID:                       "vpcpeering-delete-" + vpcPeering.ID.String(),
		WorkflowExecutionTimeout: cutil.WorkflowExecutionTimeout,
		TaskQueue:                queue.SiteTaskQueue,
	}

	logger.Info().Msg("triggering VPC peering delete workflow")

	// Add context deadline
	workflowCtx, cancel := context.WithTimeout(ctx, cutil.WorkflowContextTimeout)
	defer cancel()

	// Trigger site workflow to delete VPC peering
	we, err := stc.ExecuteWorkflow(workflowCtx, workflowOptions, "DeleteVpcPeering", deleteVpcPeeringRequest)
	if err != nil {
		logger.Error().Err(err).Msg("failed to start VPC peering deletion workflow")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to start VPC peering deletion workflow", nil)
	}

	wid := we.GetID()
	logger.Info().Str("Workflow ID", wid).Msg("started VPC peering deletion workflow")

	// Wait for workflow completion synchronously
	err = we.Get(workflowCtx, nil)
	if err != nil {
		var applicationErr *tp.ApplicationError
		if errors.As(err, &applicationErr) && (applicationErr.Type() == swe.ErrTypeCarbideUnimplemented || applicationErr.Type() == swe.ErrTypeCarbideDenied) {
			logger.Error().Msg("feature not yet implemented on target Site")
			return cutil.NewAPIErrorResponse(c, http.StatusNotImplemented, fmt.Sprintf("Feature not yet implemented on target Site: %s", err), nil)
		}

		var timeoutErr *tp.TimeoutError
		if errors.As(err, &timeoutErr) || err == context.DeadlineExceeded || ctx.Err() != nil {
			return common.TerminateWorkflowOnTimeOut(c, logger, stc, wid, err, "VpcPeering", "DeleteVpcPeering")
		}

		logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to delete VPC peering")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to execute sync workflow to delete VPC Peering on Site: %s", err), nil)
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete VPC peering", nil)
	}
	txCommitted = true

	// Best effort post-commit cleanup: remove VPC peering from DB.
	// This is intentionally outside of the transaction so delete does not fail if this cleanup fails.
	err = vpcPeeringDAO.Delete(ctx, nil, vpcPeering.ID)
	if err != nil {
		logger.Warn().Err(err).Msg("best-effort delete of VPC peering from DB failed after workflow completion")
	}

	logger.Info().Str("Workflow ID", wid).Msg("successfully completed VPC peering deletion workflow and removed from database")

	return c.NoContent(http.StatusNoContent)
}
