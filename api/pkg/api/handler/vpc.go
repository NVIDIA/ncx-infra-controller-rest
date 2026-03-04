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

	"github.com/labstack/echo/v4"
	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	"github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"
	cdbp "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"
	swe "github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/error"
	wutil "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/util"

	"go.opentelemetry.io/otel/attribute"
	temporalClient "go.temporal.io/sdk/client"
	tp "go.temporal.io/sdk/temporal"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nvidia/bare-metal-manager-rest/api/internal/config"
	common "github.com/nvidia/bare-metal-manager-rest/api/pkg/api/handler/util/common"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/pagination"
	sc "github.com/nvidia/bare-metal-manager-rest/api/pkg/client/site"
	auth "github.com/nvidia/bare-metal-manager-rest/auth/pkg/authorization"
	cerr "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	cwutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	sutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"

	cwssaws "github.com/nvidia/bare-metal-manager-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/nvidia/bare-metal-manager-rest/workflow/pkg/queue"
)

// ~~~~~ Create Handler ~~~~~ //

// CreateVPCHandler is the API Handler for creating new VPC
type CreateVPCHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewCreateVPCHandler initializes and returns a new handler for creating Tenant
func NewCreateVPCHandler(dbSession *cdb.Session, tc temporalClient.Client, sc *sc.ClientPool, cfg *config.Config) CreateVPCHandler {
	return CreateVPCHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        sc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Create a VPC
// @Description Create a VPC for the org.
// @Tags vpc
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param message body model.APIVpcCreateRequest true "VPC create request"
// @Success 201 {object} model.APIVpc
// @Router /v2/org/{org}/carbide/vpc [post]
func (cvh CreateVPCHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "VPC").Str("Handler", "Create").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := cvh.tracerSpan.CreateChildInContext(ctx, "CreateVPCHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()

		cvh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, cvh.tracerSpan, handlerSpan)
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

	// Validate role, only Tenant Admins are allowed to interact with VPC endpoints
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Tenant Admin role with org, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}

	// Validate request
	// Bind request data to API model
	apiRequest := model.APIVpcCreateRequest{}
	err = c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate()
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating VPC creation request data")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating VPC creation request data", verr)
	}

	// Validate the site for which this VPC is being created
	site, err := common.GetSiteFromIDString(ctx, nil, apiRequest.SiteID, cvh.dbSession)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Could not find Site with ID specified in request data", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Site from DB by ID")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID in request data", nil)
	}

	// Verify if site is ready
	if site.Status != cdbm.SiteStatusRegistered {
		logger.Warn().Msg(fmt.Sprintf("Site: %v specified in request data must be in Registered state in order to proceed", site.ID))
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data must be in Registered state in order to proceed", nil)
	}

	// Get Tenant for this org
	tenant, err := common.GetTenantForOrg(ctx, nil, cvh.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Org does not have a Tenant associated", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Tenant for this org")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant", nil)
	}

	// Ensure that Tenant has an Allocation with specified Site
	aDAO := cdbm.NewAllocationDAO(cvh.dbSession)
	allocationFilter := cdbm.AllocationFilterInput{TenantIDs: []uuid.UUID{tenant.ID}, SiteIDs: []uuid.UUID{site.ID}}
	aCount, err := aDAO.GetCount(ctx, nil, allocationFilter)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Allocations count from DB for Tenant and Site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site Allocations count for Tenant", nil)
	}

	if aCount == 0 {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden,
			"Tenant does not have any Allocations with Site specified in request data", nil)
	}

	vpcDAO := cdbm.NewVpcDAO(cvh.dbSession)
	if apiRequest.ID != nil {
		_, total, err := vpcDAO.GetAll(ctx, nil, cdbm.VpcFilterInput{VpcIDs: []uuid.UUID{*apiRequest.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(paginator.DefaultLimit)}, nil)
		if err != nil {
			logger.Error().Err(err).Msg("db error checking for ID uniqueness of tenant vpc")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Vpc due to DB error", nil)
		}
		if total > 0 {
			logger.Warn().Str("tenantId", tenant.ID.String()).Str("name", apiRequest.ID.String()).Msg("vpc with same ID already exists")
			return cerr.NewAPIErrorResponse(c, http.StatusConflict, "A Vpc with specified ID already exists", validation.Errors{
				"id": errors.New(apiRequest.ID.String()),
			})
		}
	}

	// check for name uniqueness for the tenant, ie, tenant cannot have another vpc with same name at the site
	// TODO consider doing this with an advisory lock for correctness
	vpcs, tot, err := vpcDAO.GetAll(ctx, nil, cdbm.VpcFilterInput{Name: &apiRequest.Name, InfrastructureProviderID: cdb.GetUUIDPtr(site.InfrastructureProviderID), TenantIDs: []uuid.UUID{tenant.ID}, SiteIDs: []uuid.UUID{site.ID}}, cdbp.PageInput{}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("db error checking for name uniqueness of tenant vpc")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Vpc due to DB error", nil)
	}
	if tot > 0 {
		logger.Warn().Str("tenantId", tenant.ID.String()).Str("name", apiRequest.Name).Msg("vpc with same name already exists for tenant")
		return cerr.NewAPIErrorResponse(c, http.StatusConflict, "A Vpc with specified name already exists for Tenant", validation.Errors{
			"id": errors.New(vpcs[0].ID.String()),
		})
	}

	// If an NSG was requested, validate it.
	if apiRequest.NetworkSecurityGroupID != nil {
		nsgDAO := cdbm.NewNetworkSecurityGroupDAO(cvh.dbSession)

		nsg, err := nsgDAO.GetByID(ctx, nil, *apiRequest.NetworkSecurityGroupID, nil)
		if err != nil {
			if err == cdb.ErrDoesNotExist {
				logger.Error().Err(err).Msg("could not find NetworkSecurityGroup with ID specified in request data")
				// Should probably be using StatusPreconditionFailed here, and maybe for all of these NSG errors.
				return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Could not find NetworkSecurityGroup with ID specified in request data", nil)
			}

			logger.Error().Err(err).Msg("error retrieving NetworkSecurityGroup with ID specified in request data")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve NetworkSecurityGroup with ID specified in request data", nil)
		}

		if nsg.SiteID != site.ID {
			logger.Error().Msg("NetworkSecurityGroup in request does not belong to Site")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NetworkSecurityGroup with ID specified in request data does not belong to Site", nil)
		}

		if nsg.TenantID != tenant.ID {
			logger.Error().Msg("NetworkSecurityGroup in request does not belong to Tenant")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NetworkSecurityGroup with ID specified in request data does not belong to Tenant", nil)
		}
	}

	siteConfig := &cdbm.SiteConfig{}
	if site.Config != nil {
		siteConfig = site.Config
	}

	// Network Virtualization type support
	networkVirtualizationType := apiRequest.NetworkVirtualizationType
	if networkVirtualizationType == nil {
		// Default to `EthernetVirtualizer`
		networkVirtualizationType = cdb.GetStrPtr(cdbm.VpcEthernetVirtualizer)

		// If site has native networking enabled, use FNN
		if siteConfig.NativeNetworking {
			networkVirtualizationType = cdb.GetStrPtr(cdbm.VpcFNN)
		}
	}

	// Verify if site has been enabled for FNN type
	if *networkVirtualizationType == cdbm.VpcFNN {
		if !siteConfig.NativeNetworking {
			logger.Warn().Msg(fmt.Sprintf("Site: %v specified in request data must have native networking enabled in order to create FNN VPCs", site.ID))
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data must have native networking enabled in order to create FNN VPCs", nil)
		}
	}

	var defaultNvllPartitionId *uuid.UUID
	if apiRequest.NVLinkLogicalPartitionID != nil {
		if !siteConfig.NVLinkPartition {
			logger.Warn().Msg(fmt.Sprintf("Site: %v specified in request data must have NVLink Partition enabled in order to create VPC with default NVLink Partition", site.ID))
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request data must have NVLink Partition enabled in order to create VPC with default NVLink Partition", nil)
		}

		nvllpDAO := cdbm.NewNVLinkLogicalPartitionDAO(cvh.dbSession)
		nvllpID, err := uuid.Parse(*apiRequest.NVLinkLogicalPartitionID)
		if err != nil {
			logger.Error().Err(err).Msg("error parsing NVLink Logical Partition ID specified in request data")
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid NVLink Logical Partition ID specified in request data", nil)
		}

		nvllPartition, err := nvllpDAO.GetByID(ctx, nil, nvllpID, nil)
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving NVLink Logical Partition with ID specified in request data")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve NVLink Logical Partition with ID specified in request data", nil)
		}

		if nvllPartition.SiteID != site.ID {
			logger.Error().Msg("NVLink Logical Partition in request does not belong to Site")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NVLink Logical Partition with ID specified in request data does not belong to Site", nil)
		}

		if nvllPartition.Status != cdbm.NVLinkLogicalPartitionStatusReady {
			logger.Error().Msg("NVLink Logical Partition in request is not in Ready state")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NVLink Logical Partition with ID specified in request data is not in Ready state", nil)
		}

		// Verify that the NVLink Logical Partition is associated with the VPC's Tenant
		if nvllPartition.TenantID != tenant.ID {
			logger.Error().Msg("NVLink Logical Partition in request does not belong to Tenant")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NVLink Logical Partition with ID specified in request data does not belong to Tenant", nil)
		}
		defaultNvllPartitionId = &nvllpID
	}

	// Labels support
	var labels map[string]string
	if apiRequest.Labels != nil {
		labels = apiRequest.Labels
	}

	// Start a database transaction
	tx, err := cdb.BeginTx(ctx, cvh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error creating vpc", nil)
	}
	// This variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Create VPC
	vpcInput := cdbm.VpcCreateInput{
		ID:                        apiRequest.ID,
		Name:                      apiRequest.Name,
		Description:               apiRequest.Description,
		Org:                       org,
		InfrastructureProviderID:  site.InfrastructureProviderID,
		NetworkSecurityGroupID:    apiRequest.NetworkSecurityGroupID,
		TenantID:                  tenant.ID,
		SiteID:                    site.ID,
		NetworkVirtualizationType: networkVirtualizationType,
		NVLinkLogicalPartitionID:  defaultNvllPartitionId,
		Labels:                    labels,
		Status:                    cdbm.VpcStatusReady,
		CreatedBy:                 *dbUser,
		Vni:                       apiRequest.Vni,
	}

	vpc, err := vpcDAO.Create(ctx, tx, vpcInput)
	if err != nil {
		logger.Error().Err(err).Msg("error creating VPC DB entry")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed creating new VPC record, DB error", nil)
	}

	// Update the controller ID
	// We need this to match the VPC ID.  This was previously handled
	// by the async cloud workflow after successful creation on site.
	uvpcInput := cdbm.VpcUpdateInput{
		VpcID:           vpc.ID,
		ControllerVpcID: cdb.GetUUIDPtr(vpc.ID),
	}
	vpc, err = vpcDAO.Update(ctx, tx, uvpcInput)
	if err != nil {
		logger.Error().Err(err).Msg("error creating VPC DB entry")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed updating new VPC record, DB error", nil)
	}

	// Create status detail
	sdDAO := cdbm.NewStatusDetailDAO(cvh.dbSession)
	ssd, err := sdDAO.CreateFromParams(ctx, tx, vpc.ID.String(), *cdb.GetStrPtr(cdbm.VpcStatusReady),
		cdb.GetStrPtr("VPC successfully provisioned on Site"))
	if err != nil {
		logger.Error().Err(err).Msg("error creating Status Detail DB entry")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Status Detail for VPC", nil)
	}
	if ssd == nil {
		logger.Error().Msg("Status Detail DB entry not returned from CreateFromParams")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to get new Status Detail for VPC", nil)
	}

	// Get the temporal client for the site we are working with.
	stc, err := cvh.scp.GetClientByID(vpc.SiteID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	// Get network virtulization type
	nwvt := cwssaws.VpcVirtualizationType_ETHERNET_VIRTUALIZER
	if *vpc.NetworkVirtualizationType == cwssaws.VpcVirtualizationType_FNN.String() {
		nwvt = cwssaws.VpcVirtualizationType_FNN
	}

	vni, err := wutil.GetIntPtrToUint32Ptr(apiRequest.Vni)
	// We already validate the VNI value in the model validate call, so no err
	// is ever expected by this point, but we still need to handle it.
	if err != nil {
		logger.Error().Err(err).Msg("failed to convert vni to uint32 pointer after validated passed")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "VNI value conversion failed despite passed validation", nil)
	}

	createVpcRequest := &cwssaws.VpcCreationRequest{
		Id:                        &cwssaws.VpcId{Value: common.GetSiteVpcID(vpc).String()},
		Name:                      vpc.Name,
		TenantOrganizationId:      tenant.Org,
		NetworkVirtualizationType: &nwvt,
		NetworkSecurityGroupId:    vpc.NetworkSecurityGroupID,
		Vni:                       vni,
	}

	// Add default NVLinkLogicalPartition ID if it is present
	if vpc.NVLinkLogicalPartitionID != nil {
		createVpcRequest.DefaultNvlinkLogicalPartitionId = &cwssaws.NVLinkLogicalPartitionId{Value: vpc.NVLinkLogicalPartitionID.String()}
	}

	workflowOptions := temporalClient.StartWorkflowOptions{
		ID:                       "vpc-create-" + vpc.ID.String(),
		TaskQueue:                queue.SiteTaskQueue,
		WorkflowExecutionTimeout: cwutil.WorkflowExecutionTimeout,
	}

	// Vpc metadata info
	metadata := &cwssaws.Metadata{
		Name:        vpc.Name,
		Description: "",
	}

	// Include descripotion if it is present
	if vpc.Description != nil {
		metadata.Description = *vpc.Description
	}

	// Prepare labels for site controller
	if len(vpc.Labels) > 0 {
		var labels []*cwssaws.Label
		for key, value := range vpc.Labels {
			curVal := value
			localLable := &cwssaws.Label{
				Key:   key,
				Value: &curVal,
			}
			labels = append(labels, localLable)
		}
		metadata.Labels = labels
	}
	createVpcRequest.Metadata = metadata

	logger.Info().Msg("triggering VPC create workflow")

	// Add context deadlines
	ctx, cancel := context.WithTimeout(ctx, cwutil.WorkflowContextTimeout)
	defer cancel()

	// Trigger Site workflow
	we, err := stc.ExecuteWorkflow(ctx, workflowOptions, "CreateVPCV2", createVpcRequest)
	if err != nil {
		logger.Error().Err(err).Msg("failed to synchronously start Temporal workflow to create VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed start sync workflow to create VPC on Site: %s", err), nil)
	}

	wid := we.GetID()
	logger.Info().Str("Workflow ID", wid).Msg("executed synchronous create VPC workflow")

	// Block until the workflow has completed and returned success/error.
	err = we.Get(ctx, nil)
	if err != nil {
		var timeoutErr *tp.TimeoutError
		if errors.As(err, &timeoutErr) || err == context.DeadlineExceeded || ctx.Err() != nil {

			logger.Error().Err(err).Msg("failed to create VPC, timeout occurred executing workflow on Site.")

			// Create a new context deadlines
			newctx, newcancel := context.WithTimeout(context.Background(), cwutil.WorkflowContextNewAfterTimeout)
			defer newcancel()

			// Initiate termination workflow
			serr := stc.TerminateWorkflow(newctx, wid, "", "timeout occurred executing create VPC workflow")
			if serr != nil {
				logger.Error().Err(serr).Msg("failed to execute terminate Temporal workflow for creating VPC")
				return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to terminate synchronous VPC creation workflow after timeout, Cloud and Site data may be de-synced: %s", serr), nil)
			}

			logger.Info().Str("Workflow ID", wid).Msg("initiated terminate synchronous create VPC workflow successfully")

			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to create VPC, timeout occurred executing workflow on Site: %s", err), nil)
		}

		code, err := common.UnwrapWorkflowError(err)
		logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to create VPC")
		return cerr.NewAPIErrorResponse(c, code, fmt.Sprintf("Failed to execute sync workflow to create VPC on Site: %s", err), nil)
	}

	logger.Info().Str("Workflow ID", wid).Msg("completed synchronous create VPC workflow")

	// commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create VPC, DB transaction error", nil)
	}
	// set committed so, deferred cleanup functions will do nothing
	txCommitted = true
	// Create response
	apiVpc := model.NewAPIVpc(*vpc, []cdbm.StatusDetail{*ssd})

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusCreated, apiVpc)
}

// ~~~~~ Update Handler ~~~~~ //

// UpdateVPCHandler is the API Handler for updating a VPC
type UpdateVPCHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewUpdateVPCHandler initializes and returns a new handler for updating VPC
func NewUpdateVPCHandler(dbSession *cdb.Session, tc temporalClient.Client, sc *sc.ClientPool, cfg *config.Config) UpdateVPCHandler {
	return UpdateVPCHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        sc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Update an existing VPC
// @Description Update an existing VPC for the org
// @Tags vpc
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Vpc"
// @Param message body model.APIVpcUpdateRequest true "VPC update request"
// @Success 200 {object} model.APIVpc
// @Router /v2/org/{org}/carbide/vpc/{id} [patch]
func (uvh UpdateVPCHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "VPC").Str("Handler", "Update").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := uvh.tracerSpan.CreateChildInContext(ctx, "UpdateVPCHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()
		uvh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, uvh.tracerSpan, handlerSpan)
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

	// Validate role, only Tenant Admins are allowed to interact with VPC endpoints
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Tenant Admin role with org, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}

	// Get vpc instance ID from URL param
	vpcStrID := c.Param("id")

	uvh.tracerSpan.SetAttribute(handlerSpan, attribute.String("vpc_id", vpcStrID), logger)

	// Validate request
	// Bind request data to API model
	apiRequest := model.APIVpcUpdateRequest{}
	err = c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate()
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating VPC update request data")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating VPC update request data", verr)
	}

	// Check that VPC exists
	vpc, err := common.GetVpcFromIDString(ctx, nil, vpcStrID, []string{cdbm.SiteRelationName}, uvh.dbSession)
	if err != nil {
		// Check if it's a UUID parsing error (happens before DB call)
		if err == common.ErrInvalidID {
			logger.Warn().Err(err).Msg("invalid VPC ID in request")
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid VPC ID in request", nil)
		}
		if err == cdb.ErrDoesNotExist {
			logger.Warn().Err(err).Msg("VPC not found")
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound, "Could not retrieve VPC to update", nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC DB entity")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPC", nil)
	}

	// Get Tenant for this org
	tenant, err := common.GetTenantForOrg(ctx, nil, uvh.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Org does not have a Tenant associated", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Tenant for this org")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant", nil)
	}

	// Check that VPC belongs to the Tenant
	if vpc.TenantID != tenant.ID {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "VPC does not belong to current Tenant", nil)
	}

	// Ensure that Tenant has an Allocation with specified Site
	aDAO := cdbm.NewAllocationDAO(uvh.dbSession)
	allocationFilter := cdbm.AllocationFilterInput{TenantIDs: []uuid.UUID{tenant.ID}, SiteIDs: []uuid.UUID{vpc.SiteID}}
	aCount, err := aDAO.GetCount(ctx, nil, allocationFilter)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Allocations count from DB for Tenant and Site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site Allocations count for Tenant", nil)
	}

	if aCount == 0 {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden,
			"Tenant does not have any Allocations with Site specified in request data", nil)
	}

	vpcDAO := cdbm.NewVpcDAO(uvh.dbSession)
	// check for name uniqueness for the tenant, ie, tenant cannot have another vpc with same name at the site
	if apiRequest.Name != nil && *apiRequest.Name != vpc.Name {
		vpcs, tot, err := vpcDAO.GetAll(ctx, nil, cdbm.VpcFilterInput{Name: apiRequest.Name, InfrastructureProviderID: &vpc.InfrastructureProviderID, TenantIDs: []uuid.UUID{tenant.ID}, SiteIDs: []uuid.UUID{vpc.SiteID}}, cdbp.PageInput{}, nil)
		if err != nil {
			logger.Error().Err(err).Msg("db error checking for name uniqueness of tenant vpc")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Vpc due to DB error", nil)
		}
		if tot > 0 {
			return cerr.NewAPIErrorResponse(c, http.StatusConflict, "Another VPC with specified name already exists for Tenant", validation.Errors{
				"id": errors.New(vpcs[0].ID.String()),
			})
		}
	}

	var nsgID *string
	// If an NSG was requested, validate it.
	if apiRequest.NetworkSecurityGroupID != nil && *apiRequest.NetworkSecurityGroupID != "" {
		nsgDAO := cdbm.NewNetworkSecurityGroupDAO(uvh.dbSession)

		nsg, err := nsgDAO.GetByID(ctx, nil, *apiRequest.NetworkSecurityGroupID, nil)
		if err != nil {
			if err == cdb.ErrDoesNotExist {
				logger.Error().Err(err).Msg("could not find NetworkSecurityGroup with ID specified in request data")
				return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Could not find NetworkSecurityGroup with ID specified in request data", nil)
			}

			logger.Error().Err(err).Msg("error retrieving NetworkSecurityGroup with ID specified in request data")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve NetworkSecurityGroup with ID specified in request data", nil)
		}

		if nsg.SiteID != vpc.SiteID {
			logger.Error().Msg("NetworkSecurityGroup in request does not belong to Site")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NetworkSecurityGroup with ID specified in request data does not belong to Site", nil)
		}

		if nsg.TenantID != tenant.ID {
			logger.Error().Msg("NetworkSecurityGroup in request does not belong to Tenant")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NetworkSecurityGroup with ID specified in request data does not belong to Tenant", nil)
		}

		nsgID = cdb.GetStrPtr(nsg.ID)
	}

	// Labels support
	var labels map[string]string
	if apiRequest.Labels != nil {
		labels = apiRequest.Labels
	}

	siteConfig := &cdbm.SiteConfig{}
	if vpc.Site != nil && vpc.Site.Config != nil {
		siteConfig = vpc.Site.Config
	}

	var defaultNvllPartitionId *uuid.UUID
	if apiRequest.NVLinkLogicalPartitionID != nil {
		if !siteConfig.NVLinkPartition {
			logger.Warn().Msg(fmt.Sprintf("Site: %v specified in request data must have NVLink Partition enabled in order to update VPC with default NVLink Partition", vpc.SiteID.String()))
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Site: %v specified in request data must have NVLink Partition enabled in order to update VPC with default NVLink Partition", vpc.SiteID.String()), nil)
		}

		// Verify that the existing default NVLink Logical Partition is not being used by any Instance from the VPC
		instanceDAO := cdbm.NewInstanceDAO(uvh.dbSession)
		instances, _, err := instanceDAO.GetAll(ctx, nil, cdbm.InstanceFilterInput{VpcIDs: []uuid.UUID{vpc.ID}}, cdbp.PageInput{}, nil)
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving Instances from DB for VPC")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Instances for VPC", nil)
		}

		nvlIfcDAO := cdbm.NewNVLinkInterfaceDAO(uvh.dbSession)
		for _, instance := range instances {
			// Get NVLink Interfaces for the Instance
			nvlIfcs, _, err := nvlIfcDAO.GetAll(ctx, nil, cdbm.NVLinkInterfaceFilterInput{InstanceIDs: []uuid.UUID{instance.ID}}, cdbp.PageInput{}, nil)
			if err != nil {
				logger.Error().Err(err).Msg("error retrieving NVLink Interfaces from DB for Instance")
				return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve NVLink Interfaces for Instance", nil)
			}

			for _, nvlIfc := range nvlIfcs {
				if nvlIfc.NVLinkLogicalPartitionID == *vpc.NVLinkLogicalPartitionID {
					logger.Error().Msg("Existing default NVLink Logical Partition is already being used by an Instance from the VPC")
					return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Existing default NVLink Logical Partition is already being used by an Instance from the VPC", nil)
				}
			}
		}

		nvllpDAO := cdbm.NewNVLinkLogicalPartitionDAO(uvh.dbSession)
		nvllpID, err := uuid.Parse(*apiRequest.NVLinkLogicalPartitionID)
		if err != nil {
			logger.Error().Err(err).Msg("error parsing NVLink Logical Partition ID specified in request data")
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid NVLink Logical Partition ID specified in request data", nil)
		}

		nvllPartition, err := nvllpDAO.GetByID(ctx, nil, nvllpID, nil)
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving NVLink Logical Partition with ID specified in request data")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve NVLink Logical Partition with ID specified in request data", nil)
		}

		// Verify that the NVLink Logical Partition is associated with the VPC's Site
		if nvllPartition.SiteID != vpc.SiteID {
			logger.Error().Msg("NVLink Logical Partition in request does not belong to Site")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NVLink Logical Partition with ID specified in request data does not belong to Site", nil)
		}

		// Verify that the NVLink Logical Partition is in the Ready state
		if nvllPartition.Status != cdbm.NVLinkLogicalPartitionStatusReady {
			logger.Error().Msg("NVLink Logical Partition in request is not in Ready state")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NVLink Logical Partition with ID specified in request data is not in Ready state", nil)
		}

		// Verify that the NVLink Logical Partition is associated with the VPC's Tenant
		if nvllPartition.TenantID != tenant.ID {
			logger.Error().Msg("NVLink Logical Partition in request does not belong to Tenant")
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "NVLink Logical Partition with ID specified in request data does not belong to Tenant", nil)
		}

		defaultNvllPartitionId = &nvllpID
	}

	// Start a database transaction
	tx, err := cdb.BeginTx(ctx, uvh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error creating vpc", nil)
	}
	// This variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Update VPC
	uvpcInput := cdbm.VpcUpdateInput{
		VpcID:                  vpc.ID,
		Name:                   apiRequest.Name,
		Description:            apiRequest.Description,
		Labels:                 labels,
		NetworkSecurityGroupID: nsgID,
	}

	if defaultNvllPartitionId != nil {
		uvpcInput.NVLinkLogicalPartitionID = defaultNvllPartitionId
	}

	vpc, err = vpcDAO.Update(ctx, tx, uvpcInput)
	if err != nil {
		logger.Error().Err(err).Msg("error updating VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update VPC", nil)
	}

	clearInput := cdbm.VpcClearInput{VpcID: vpc.ID}
	shouldClear := false
	// If this request is attempting to clear the OS for the instance, set it.
	if apiRequest.NetworkSecurityGroupID != nil && *apiRequest.NetworkSecurityGroupID == "" {
		clearInput.NetworkSecurityGroupID = true
		shouldClear = true
	}

	// If this request is attempting to clear NSG for the VPC, set it.
	if apiRequest.NetworkSecurityGroupID != nil {
		if *apiRequest.NetworkSecurityGroupID == "" {
			clearInput.NetworkSecurityGroupID = true
		}

		// We should always clear details for any NSG change so that users don't see stale
		// status.
		clearInput.NetworkSecurityGroupPropagationDetails = true
		shouldClear = true
	}

	// Clear it in the db if something should be cleared.
	if shouldClear {
		vpc, err = vpcDAO.Clear(ctx, tx, clearInput)
		if err != nil {
			logger.Error().Err(err).Msg("error clearing requested VPC properties")
			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to clear requested VPC properties", nil)
		}
	}

	// Get status details
	sdDAO := cdbm.NewStatusDetailDAO(uvh.dbSession)

	ssds, _, err := sdDAO.GetAllByEntityID(ctx, tx, vpc.ID.String(), nil, cdb.GetIntPtr(pagination.MaxPageSize), nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Status Details for VPC from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Status Details for VPC", nil)
	}

	// Get the temporal client for the site we are working with.
	stc, err := uvh.scp.GetClientByID(vpc.SiteID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	updateVpcRequest := &cwssaws.VpcUpdateRequest{
		Id:                     &cwssaws.VpcId{Value: common.GetSiteVpcID(vpc).String()},
		NetworkSecurityGroupId: vpc.NetworkSecurityGroupID,
	}

	// Add default NVLink Logical Partition ID if it is present
	if defaultNvllPartitionId != nil {
		updateVpcRequest.DefaultNvlinkLogicalPartitionId = &cwssaws.NVLinkLogicalPartitionId{Value: defaultNvllPartitionId.String()}
	}

	workflowOptions := temporalClient.StartWorkflowOptions{
		ID:                       "vpc-update-" + vpc.ID.String(),
		TaskQueue:                queue.SiteTaskQueue,
		WorkflowExecutionTimeout: cwutil.WorkflowExecutionTimeout,
	}

	// Vpc metadata info
	metadata := &cwssaws.Metadata{
		Name:        vpc.Name,
		Description: "",
	}

	// Include description
	if vpc.Description != nil {
		metadata.Description = *vpc.Description
	}

	// Prepare the labels
	clabels := []*cwssaws.Label{}
	for key, value := range vpc.Labels {
		curVal := value
		localLable := &cwssaws.Label{
			Key:   key,
			Value: &curVal,
		}
		clabels = append(clabels, localLable)
	}

	metadata.Labels = clabels
	updateVpcRequest.Metadata = metadata

	logger.Info().Msg("triggering VPC update workflow")

	// Add context deadlines
	ctx, cancel := context.WithTimeout(ctx, cwutil.WorkflowContextTimeout)
	defer cancel()

	// Trigger Site workflow
	we, err := stc.ExecuteWorkflow(ctx, workflowOptions, "UpdateVPC", updateVpcRequest)
	if err != nil {
		logger.Error().Err(err).Msg("failed to synchronously start Temporal workflow to update VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed start sync workflow to update VPC on Site: %s", err), nil)
	}

	wid := we.GetID()
	logger.Info().Str("Workflow ID", wid).Msg("executed synchronous update VPC workflow")

	// Block until the workflow has completed and returned success/error.
	err = we.Get(ctx, nil)
	if err != nil {
		var timeoutErr *tp.TimeoutError
		if errors.As(err, &timeoutErr) || ctx.Err() != nil {

			logger.Error().Err(err).Msg("failed to update VPC, timeout occurred executing workflow on Site.")

			// Create a new context deadlines
			newctx, newcancel := context.WithTimeout(context.Background(), cwutil.WorkflowContextNewAfterTimeout)
			defer newcancel()

			// Initiate termination workflow
			serr := stc.TerminateWorkflow(newctx, wid, "", "timeout occurred executing update VPC workflow")
			if serr != nil {
				logger.Error().Err(serr).Msg("failed to execute terminate Temporal workflow for updating VPC")
				return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to terminate synchronous VPC updating workflow after timeout, Cloud and Site data may be de-synced: %s", serr), nil)
			}

			logger.Info().Str("Workflow ID", wid).Msg("initiated terminate synchronous update VPC workflow successfully")

			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to update VPC, timeout occurred executing workflow on Site: %s", err), nil)
		}

		code, err := common.UnwrapWorkflowError(err)
		logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to update VPC")
		return cerr.NewAPIErrorResponse(c, code, fmt.Sprintf("Failed to execute sync workflow to update VPC on Site: %s", err), nil)
	}

	logger.Info().Str("Workflow ID", wid).Msg("completed synchronous update VPC workflow")

	// commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update VPC", nil)
	}
	txCommitted = true

	// Create response
	apiVpc := model.NewAPIVpc(*vpc, ssds)

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, apiVpc)
}

// ~~~~~ Update Virtualization Handler ~~~~~ //

// UpdateVPCVirtualizationHandler is the API Handler for updating virtualization of a VPC
type UpdateVPCVirtualizationHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewUpdateVPCVirtualizationHandler initializes and returns a new handler for updating virtualization of a VPC
func NewUpdateVPCVirtualizationHandler(dbSession *cdb.Session, tc temporalClient.Client, sc *sc.ClientPool, cfg *config.Config) UpdateVPCVirtualizationHandler {
	return UpdateVPCVirtualizationHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        sc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Update virtualization of a VPC
// @Description Update the network virtualization of a VPC
// @Tags vpc
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Vpc"
// @Param message body model.APIVpcVirtualizationUpdateRequest true "VPC virtualization update request"
// @Success 200 {object} model.APIVpc
// @Router /v2/org/{org}/carbide/vpc/{id}/virtualization [patch]
func (uvvh UpdateVPCVirtualizationHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "VPC").Str("Handler", "Update Virtualization").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := uvvh.tracerSpan.CreateChildInContext(ctx, "UpdateVPCHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()
		uvvh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, uvvh.tracerSpan, handlerSpan)
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

	// Validate role, only Tenant Admins are allowed to interact with VPC endpoints
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Tenant Admin role with org, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}

	// Get vpc instance ID from URL param
	vpcStrID := c.Param("id")

	uvvh.tracerSpan.SetAttribute(handlerSpan, attribute.String("vpc_id", vpcStrID), logger)

	// Validate request
	// Bind request data to API model
	apiRequest := model.APIVpcVirtualizationUpdateRequest{}
	err = c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	// Check that VPC exists
	vpc, err := common.GetVpcFromIDString(ctx, nil, vpcStrID, nil, uvvh.dbSession)
	if err != nil {
		// Check if it's a UUID parsing error (happens before DB call)
		if err == common.ErrInvalidID {
			logger.Warn().Err(err).Msg("invalid VPC ID in request")
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid VPC ID in request", nil)
		}
		if err == cdb.ErrDoesNotExist {
			logger.Warn().Err(err).Msg("VPC not found")
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound, "Could not retrieve VPC to update", nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC DB entity")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPC", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate(vpc)
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating VPC update request data")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating VPC virtualization update request data", verr)
	}

	// Get Tenant for this org
	tenant, err := common.GetTenantForOrg(ctx, nil, uvvh.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Org does not have a Tenant associated", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Tenant for this org")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant", nil)
	}

	// Check that VPC belongs to the Tenant
	if vpc.TenantID != tenant.ID {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "VPC does not belong to current Tenant", nil)
	}

	// Ensure that Tenant has access to Site
	tsDAO := cdbm.NewTenantSiteDAO(uvvh.dbSession)
	_, err = tsDAO.GetByTenantIDAndSiteID(ctx, nil, tenant.ID, vpc.SiteID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant does not have access to Site, VPC cannot be updated", nil)
		}

		logger.Error().Err(err).Msg("error retrieving Tenant Site association")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant Site association", nil)
	}

	// Start a database transaction
	tx, err := cdb.BeginTx(ctx, uvvh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error creating vpc", nil)
	}
	// This variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Update VPC
	vpcDAO := cdbm.NewVpcDAO(uvvh.dbSession)
	uvpcInput := cdbm.VpcUpdateInput{
		VpcID:                     vpc.ID,
		NetworkVirtualizationType: &apiRequest.NetworkVirtualizationType,
	}
	uv, err := vpcDAO.Update(ctx, tx, uvpcInput)
	if err != nil {
		logger.Error().Err(err).Msg("error updating VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update VPC virtualization, DB error", nil)
	}

	// Get status details
	sdDAO := cdbm.NewStatusDetailDAO(uvvh.dbSession)

	ssds, _, err := sdDAO.GetAllByEntityID(ctx, tx, uv.ID.String(), nil, cdb.GetIntPtr(pagination.MaxPageSize), nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Status Details for VPC from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve status history for VPC", nil)
	}

	// Get the temporal client for the site we are working with.
	stc, err := uvvh.scp.GetClientByID(vpc.SiteID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	// VPC virtualization type can only be updated to FNN, the request validator guarantees that
	siteVirtualizationType := cwssaws.VpcVirtualizationType_FNN
	siteRequest := &cwssaws.VpcUpdateVirtualizationRequest{
		Id:                        &cwssaws.VpcId{Value: common.GetSiteVpcID(vpc).String()},
		NetworkVirtualizationType: &siteVirtualizationType,
	}

	workflowOptions := temporalClient.StartWorkflowOptions{
		ID:                       "vpc-update-virtualzation-" + uv.ID.String(),
		TaskQueue:                queue.SiteTaskQueue,
		WorkflowExecutionTimeout: cwutil.WorkflowExecutionTimeout,
	}

	logger.Info().Msg("triggering VPC virtualization update workflow")

	// Add context deadlines
	ctx, cancel := context.WithTimeout(ctx, cwutil.WorkflowContextTimeout)
	defer cancel()

	// Trigger Site workflow
	we, err := stc.ExecuteWorkflow(ctx, workflowOptions, "UpdateVPCVirtualization", siteRequest)
	if err != nil {
		logger.Error().Err(err).Msg("failed to synchronously start Temporal workflow to update VPC virtualization")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed start sync workflow to update VPC on Site: %s", err), nil)
	}

	wid := we.GetID()
	logger.Info().Str("Workflow ID", wid).Msg("executed synchronous update VPC virtualization workflow")

	// Block until the workflow has completed and returned success/error.
	err = we.Get(ctx, nil)

	if err != nil {
		var timeoutErr *tp.TimeoutError
		if errors.As(err, &timeoutErr) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			return common.TerminateWorkflowOnTimeOut(c, logger, stc, wid, err, "VPC", "UpdateVirtualization")
		}
		code, err := common.UnwrapWorkflowError(err)
		logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to update VPC virtualization")
		return cerr.NewAPIErrorResponse(c, code, fmt.Sprintf("Failed to execute sync workflow to update VPC virtualization on Site: %s", err), nil)
	}

	logger.Info().Str("Workflow ID", wid).Msg("completed synchronous update VPC workflow")

	// commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update VPC", nil)
	}
	txCommitted = true

	// Create response
	apiVpc := model.NewAPIVpc(*uv, ssds)

	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, apiVpc)
}

// ~~~~~ Get Handler ~~~~~ //

// GetVPCHandler is the API Handler for getting a VPC
type GetVPCHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetVPCHandler initializes and returns a new handler for getting VPC
func NewGetVPCHandler(dbSession *cdb.Session, tc temporalClient.Client, cfg *config.Config) GetVPCHandler {
	return GetVPCHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get a VPC
// @Description Get a VPC for the org
// @Tags vpc
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of Vpc"
// @Param includeRelation query string false "Related entities to include in response e.g. 'InfrastructureProvider', 'Site', 'Tenant'"
// @Success 200 {object} model.APIVpc
// @Router /v2/org/{org}/carbide/vpc/{id} [get]
func (gvh GetVPCHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "VPC").Str("Handler", "Get").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := gvh.tracerSpan.CreateChildInContext(ctx, "GetVPCHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()

		gvh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, gvh.tracerSpan, handlerSpan)
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

	// Validate role, only Tenant Admins are allowed to interact with VPC endpoints
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Tenant Admin role with org, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}

	// Get and validate includeRelation params
	qParams := c.QueryParams()
	qIncludeRelations, errMsg := common.GetAndValidateQueryRelations(qParams, cdbm.VpcRelatedEntities)
	if errMsg != "" {
		logger.Warn().Msg(errMsg)
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, errMsg, nil)
	}

	// Get VPC ID from URL param
	vpcIDStr := c.Param("id")

	gvh.tracerSpan.SetAttribute(handlerSpan, attribute.String("vpc_id", vpcIDStr), logger)

	// Get VPC
	vpcDAO := cdbm.NewVpcDAO(gvh.dbSession)

	vpcID, err := uuid.Parse(vpcIDStr)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid VPC ID in URL", nil)
	}

	vpc, err := vpcDAO.GetByID(ctx, nil, vpcID, qIncludeRelations)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find VPC with specified ID", nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPC", nil)
	}

	// Get Tenant for this org
	tenant, err := common.GetTenantForOrg(ctx, nil, gvh.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Org does not have a Tenant associated", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Tenant for this org")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant", nil)
	}

	// Check if VPC belongs to Tenant
	if vpc.TenantID != tenant.ID {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "VPC does not belong to current Tenant", nil)
	}

	// Get status details
	sdDAO := cdbm.NewStatusDetailDAO(gvh.dbSession)

	ssds, err := sdDAO.GetRecentByEntityIDs(ctx, nil, []string{vpcID.String()}, common.RECENT_STATUS_DETAIL_COUNT)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Status Details for VPC from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Status Details for VPC", nil)
	}

	// Create response
	vc := model.NewAPIVpc(*vpc, ssds)

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusOK, vc)
}

// ~~~~~ GetAll Handler ~~~~~ //

// GetAllVPCHandler is the API Handler for retrieving all VPCs
type GetAllVPCHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewGetAllVPCHandler initializes and returns a new handler for retreiving all VPCs
func NewGetAllVPCHandler(dbSession *cdb.Session, tc temporalClient.Client, cfg *config.Config) GetAllVPCHandler {
	return GetAllVPCHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get all VPCs
// @Description Get all VPCs for the org
// @Tags vpc
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param siteId query string false "Site ID"
// @Param status query string false "Filter by status" e.g. 'Pending', 'Error'"
// @Param query query string false "Query input for full text search"
// @Param includeRelation query string false "Related entities to include in response e.g. 'InfrastructureProvider', 'Site', 'Tenant'"
// @Param pageNumber query integer false "Page number of results returned"
// @Param pageSize query integer false "Number of results per page"
// @Param orderBy query string false "Order by field"
// @Success 200 {array} []model.APIVpc
// @Router /v2/org/{org}/carbide/vpc [get]
func (gavh GetAllVPCHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "VPC").Str("Handler", "GetAll").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := gavh.tracerSpan.CreateChildInContext(ctx, "GetAllVPCHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()

		gavh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, gavh.tracerSpan, handlerSpan)
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

	// Validate role, only Tenant Admins are allowed to interact with VPC endpoints
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Tenant Admin role with org, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}

	// Validate pagination request
	pageRequest := pagination.PageRequest{}
	err = c.Bind(&pageRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding pagination request data into API model")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request pagination data", nil)
	}

	// Validate request attributes
	err = pageRequest.Validate(cdbm.VpcOrderByFields)
	if err != nil {
		logger.Warn().Err(err).Msg("error validating pagination request data")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate pagination request data", err)
	}

	// Get and validate includeRelation params
	qParams := c.QueryParams()
	qIncludeRelations, errMsg := common.GetAndValidateQueryRelations(qParams, cdbm.VpcRelatedEntities)
	if errMsg != "" {
		logger.Warn().Msg(errMsg)
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, errMsg, nil)
	}

	// Get infrastructure provider ID from query param
	var infrastructureProviderID *uuid.UUID
	qInfrastructureProviderID := c.QueryParam("infrastructureProviderId")
	if qInfrastructureProviderID != "" {
		id, serr := uuid.Parse(qInfrastructureProviderID)
		if serr != nil {
			logger.Warn().Err(serr).Msg("error parsing infrastructureProviderId in query into uuid")
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Infrastructure Provider ID in query", nil)
		}
		infrastructureProviderID = &id

		// Check for IP existence
		ipDAO := cdbm.NewInfrastructureProviderDAO(gavh.dbSession)
		_, verr := ipDAO.GetByID(ctx, nil, *infrastructureProviderID, nil)
		if verr != nil {
			logger.Warn().Err(verr).Msg("error retrieving InfrastructureProvider from DB by ID")
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Could not retrieve InfrastructureProvider with ID specified in query", nil)
		}
	}

	// Get site ID from query param
	var siteID *uuid.UUID

	siteIDStr := c.QueryParam("siteId")
	if siteIDStr != "" {
		parsedID, serr := uuid.Parse(siteIDStr)
		if serr != nil {
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID in query", nil)
		}
		siteID = &parsedID

		// Check for Site existence
		stDAO := cdbm.NewSiteDAO(gavh.dbSession)
		_, verr := stDAO.GetByID(ctx, nil, *siteID, nil, false)
		if verr != nil {
			logger.Warn().Err(verr).Msg("error retrieving Site from DB by ID")
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Could not retrieve Site with ID specified in query", nil)
		}
	}

	// Get query text for full text search from query param
	var searchQuery *string

	searchQueryStr := c.QueryParam("query")
	if searchQueryStr != "" {
		searchQuery = &searchQueryStr
		gavh.tracerSpan.SetAttribute(handlerSpan, attribute.String("query", searchQueryStr), logger)
	}

	// Get status from query param
	var status *string

	statusQuery := c.QueryParam("status")
	if statusQuery != "" {
		_, ok := cdbm.VpcStatusMap[statusQuery]
		if !ok {
			logger.Warn().Msg(fmt.Sprintf("invalid value in status query: %v", statusQuery))
			return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Status value in query", nil)
		}
		status = &statusQuery
		gavh.tracerSpan.SetAttribute(handlerSpan, attribute.String("status", statusQuery), logger)
	}

	// Get Tenant for this org
	tnDAO := cdbm.NewTenantDAO(gavh.dbSession)

	tenants, err := tnDAO.GetAllByOrg(ctx, nil, org, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Tenant for this org")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant for org", nil)
	}

	if len(tenants) == 0 {
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "Org does not have a Tenant associated", nil)
	}
	tenant := tenants[0]

	// Get all VPCs by Tenant, and Site, if specified
	vpcDAO := cdbm.NewVpcDAO(gavh.dbSession)

	vpcFilter := cdbm.VpcFilterInput{
		Org:                      &org,
		InfrastructureProviderID: infrastructureProviderID,
		SearchQuery:              searchQuery,
		TenantIDs:                []uuid.UUID{tenant.ID},
	}

	if siteID != nil {
		vpcFilter.SiteIDs = []uuid.UUID{*siteID}
	}

	if status != nil {
		vpcFilter.Statuses = []string{*status}
	}

	// Get network security group IDs from query param
	if len(qParams["networkSecurityGroupId"]) > 0 {
		vpcFilter.NetworkSecurityGroupIDs = qParams["networkSecurityGroupId"]
	}

	vpcPageInput := cdbp.PageInput{
		Limit:   pageRequest.Limit,
		Offset:  pageRequest.Offset,
		OrderBy: pageRequest.OrderBy,
	}

	vpcs, total, serr := vpcDAO.GetAll(
		ctx,
		nil,
		vpcFilter,
		vpcPageInput,
		qIncludeRelations,
	)

	if serr != nil {
		logger.Error().Err(serr).Msg("error retrieving VPCs for this Site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPCs for Site", nil)
	}

	// Get status details
	sdDAO := cdbm.NewStatusDetailDAO(gavh.dbSession)

	sdEntityIDs := []string{}
	for _, vpc := range vpcs {
		sdEntityIDs = append(sdEntityIDs, vpc.ID.String())
	}
	ssds, serr := sdDAO.GetRecentByEntityIDs(ctx, nil, sdEntityIDs, common.RECENT_STATUS_DETAIL_COUNT)
	if serr != nil {
		logger.Warn().Err(serr).Msg("error retrieving Status Details for VPCs from DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to populate status history for VPCs", nil)
	}
	ssdMap := map[string][]cdbm.StatusDetail{}
	for _, ssd := range ssds {
		cssd := ssd
		ssdMap[ssd.EntityID] = append(ssdMap[ssd.EntityID], cssd)
	}

	// Create response
	apiVpcs := []model.APIVpc{}

	for _, vpc := range vpcs {
		apiVpc := model.NewAPIVpc(vpc, ssdMap[vpc.ID.String()])
		apiVpcs = append(apiVpcs, apiVpc)
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

	return c.JSON(http.StatusOK, apiVpcs)
}

// ~~~~~ Delete Handler ~~~~~ //

// DeleteVPCHandler is the API Handler for deleting a VPC
type DeleteVPCHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *sutil.TracerSpan
}

// NewDeleteVPCHandler initializes and returns a new handler for deleting VPC
func NewDeleteVPCHandler(dbSession *cdb.Session, tc temporalClient.Client, scp *sc.ClientPool, cfg *config.Config) DeleteVPCHandler {
	return DeleteVPCHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		scp:        scp,
		tracerSpan: sutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Delete a VPC
// @Description Delete a VPC fro the org
// @Tags vpc
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of VPC"
// @Success 202
// @Router /v2/org/{org}/carbide/vpc/{id} [delete]
func (dvh DeleteVPCHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "VPC").Str("Handler", "Delete").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := dvh.tracerSpan.CreateChildInContext(ctx, "DeleteVPCHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()

		dvh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, dvh.tracerSpan, handlerSpan)
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

	// Validate role, only Tenant Admins are allowed to interact with VPC endpoints
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Tenant Admin role with org, access denied")
		return cerr.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}

	// Get VPC ID from URL param
	vpcStrID := c.Param("id")
	vpcID, err := uuid.Parse(vpcStrID)
	if err != nil {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid VPC ID in URL", nil)
	}

	dvh.tracerSpan.SetAttribute(handlerSpan, attribute.String("vpc_id", vpcStrID), logger)

	// Get VPC from DB
	vpcDAO := cdbm.NewVpcDAO(dvh.dbSession)
	vpc, err := vpcDAO.GetByID(ctx, nil, vpcID, []string{
		cdbm.SiteRelationName,
		cdbm.TenantRelationName,
	})
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cerr.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find VPC with specified ID", nil)
		}
		logger.Error().Err(err).Msg("error retrieving VPC from DB by ID")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPC with specified ID", nil)
	}

	if vpc.Tenant == nil {
		logger.Warn().Err(err).Msg("failed to retrieve Tenant details")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant details", nil)
	}

	// Validate the tenant for which this VPC is being deleted
	if vpc.Tenant.Org != org {
		logger.Warn().Msg("org specified in request does not match org of Tenant associated with VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Org specified in request does not match org of Tenant associated with VPC", nil)
	}

	// Verify that the VPC is associated with a site and then that the site is
	// in a valid state.
	if vpc.Site == nil {
		logger.Error().Msg("failed to pull site data for VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site details for VPC", nil)
	}

	// Verify if site is ready
	if vpc.Site.Status != cdbm.SiteStatusRegistered {
		logger.Warn().Str("Site ID", vpc.SiteID.String()).Msg("Site associated with VPC must be in Registered state in order to proceed")
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Site associated with VPC must be in Registered state in order to proceed", nil)
	}

	// Check if VPC has any resources subnet attached
	// Check if VPC has subnet attached
	sbDAO := cdbm.NewSubnetDAO(dvh.dbSession)
	subnets, _, err := sbDAO.GetAll(ctx, nil, cdbm.SubnetFilterInput{TenantIDs: []uuid.UUID{vpc.TenantID}, VpcIDs: []uuid.UUID{vpc.ID}}, cdbp.PageInput{}, []string{})
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Subnet for this VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Subnets for this VPC", nil)
	}
	if len(subnets) > 0 {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Cannot delete VPC, one or more Subnets exist for this VPC", nil)
	}

	// Check if VPC has VPC prefix attached
	vpcPrefixDAO := cdbm.NewVpcPrefixDAO(dvh.dbSession)
	vpcPrefixes, _, err := vpcPrefixDAO.GetAll(ctx, nil, cdbm.VpcPrefixFilterInput{TenantIDs: []uuid.UUID{vpc.TenantID}, VpcIDs: []uuid.UUID{vpc.ID}}, cdbp.PageInput{}, []string{})
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving VPC prefixes for this VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve VPC prefixes for this VPC", nil)
	}
	if len(vpcPrefixes) > 0 {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Cannot delete VPC, one or more VPC prefixes exist for this VPC", nil)
	}

	// Check if VPC has instance
	insDAO := cdbm.NewInstanceDAO(dvh.dbSession)
	instances, _, err := insDAO.GetAll(ctx, nil, cdbm.InstanceFilterInput{TenantIDs: []uuid.UUID{vpc.TenantID}, VpcIDs: []uuid.UUID{vpc.ID}}, cdbp.PageInput{}, []string{})
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving instances for this VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve instances for this VPC", nil)
	}
	if len(instances) > 0 {
		return cerr.NewAPIErrorResponse(c, http.StatusBadRequest, "Cannot delete VPC, one or more instances for this VPC", nil)
	}

	// Start a DB transaction
	tx, err := cdb.BeginTx(ctx, dvh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete VPC", nil)
	}

	// This variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Update VPC to set status to Deleting
	uvpcInput := cdbm.VpcUpdateInput{
		VpcID:  vpc.ID,
		Status: cdb.GetStrPtr(cdbm.VpcStatusDeleting),
	}
	_, err = vpcDAO.Update(ctx, tx, uvpcInput)
	if err != nil {
		logger.Error().Err(err).Msg("error updating VPC in DB")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete VPC", nil)
	}

	// Create status detail
	sdDAO := cdbm.NewStatusDetailDAO(dvh.dbSession)
	_, err = sdDAO.CreateFromParams(ctx, tx, vpc.ID.String(), *cdb.GetStrPtr(cdbm.VpcStatusDeleting),
		cdb.GetStrPtr("received request for deletion, pending processing"))
	if err != nil {
		logger.Error().Err(err).Msg("error creating Status Detail DB entry")
	}

	// Get the temporal client for the site we are working with.
	stc, err := dvh.scp.GetClientByID(vpc.SiteID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	deleteVpcRequest := &cwssaws.VpcDeletionRequest{
		Id: &cwssaws.VpcId{Value: common.GetSiteVpcID(vpc).String()},
	}

	workflowOptions := temporalClient.StartWorkflowOptions{
		ID:                       "vpc-delete-" + vpc.ID.String(),
		TaskQueue:                queue.SiteTaskQueue,
		WorkflowExecutionTimeout: cwutil.WorkflowExecutionTimeout,
	}

	logger.Info().Msg("triggering VPC delete workflow")

	// Add context deadlines
	ctx, cancel := context.WithTimeout(ctx, cwutil.WorkflowContextTimeout)
	defer cancel()

	// Trigger Site workflow
	we, err := stc.ExecuteWorkflow(ctx, workflowOptions, "DeleteVPCV2", deleteVpcRequest)
	if err != nil {
		logger.Error().Err(err).Msg("failed to synchronously start Temporal workflow to delete VPC")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed start sync workflow to delete VPC on Site: %s", err), nil)
	}

	wid := we.GetID()
	logger.Info().Str("Workflow ID", wid).Msg("executed synchronous delete VPC workflow")

	// Execute the workflow synchronously
	err = we.Get(ctx, nil)
	// Handle skippable errors
	if err != nil {
		// If this was a 404 back from Carbide, we can treat the object as already having been deleted and allow things to proceed.
		var applicationErr *tp.ApplicationError
		if errors.As(err, &applicationErr) && applicationErr.Type() == swe.ErrTypeCarbideObjectNotFound {
			logger.Warn().Msg(swe.ErrTypeCarbideObjectNotFound + " received from Site")
			// Reset error to nil
			err = nil
		}
	}

	if err != nil {
		var timeoutErr *tp.TimeoutError
		if errors.As(err, &timeoutErr) || err == context.DeadlineExceeded || ctx.Err() != nil {

			logger.Error().Err(err).Msg("failed to delete VPC, timeout occurred executing workflow on Site.")

			// Create a new context deadlines
			newctx, newcancel := context.WithTimeout(context.Background(), cwutil.WorkflowContextNewAfterTimeout)
			defer newcancel()

			// Initiate termination workflow
			serr := stc.TerminateWorkflow(newctx, wid, "", "timeout occurred executing delete VPC workflow")
			if serr != nil {
				logger.Error().Err(serr).Msg("failed to execute terminate Temporal workflow for creating VPC")
				return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to terminate synchronous VPC deletion workflow after timeout, Cloud and Site data may be de-synced: %s", serr), nil)
			}

			logger.Info().Str("Workflow ID", wid).Msg("initiated terminate synchronous delete VPC workflow successfully")

			return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to delete VPC, timeout occurred executing workflow on Site: %s", err), nil)
		}

		code, err := common.UnwrapWorkflowError(err)
		logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to delete VPC")
		return cerr.NewAPIErrorResponse(c, code, fmt.Sprintf("Failed to execute sync workflow to delete VPC on Site: %s", err), nil)
	}

	logger.Info().Str("Workflow ID", wid).Msg("completed synchronous delete VPC workflow")

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing transaction")
		return cerr.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete VPC", nil)
	}
	txCommitted = true

	// Return response
	logger.Info().Msg("finishing API handler")

	return c.String(http.StatusAccepted, "Deletion request was accepted")
}
