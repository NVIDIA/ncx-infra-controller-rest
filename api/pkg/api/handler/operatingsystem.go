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

	"go.opentelemetry.io/otel/attribute"
	temporalClient "go.temporal.io/sdk/client"
	tp "go.temporal.io/sdk/temporal"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/internal/config"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/handler/util/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/model"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/pagination"
	sc "github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/client/site"
	auth "github.com/NVIDIA/ncx-infra-controller-rest/auth/pkg/authorization"
	cutil "github.com/NVIDIA/ncx-infra-controller-rest/common/pkg/util"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
	cdbp "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
	swe "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/error"
	"github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/queue"

	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

// ~~~~~ Create Handler ~~~~~ //

// CreateOperatingSystemHandler is the API Handler for creating new OperatingSystem
type CreateOperatingSystemHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewCreateOperatingSystemHandler initializes and returns a new handler for creating OperatingSystem
func NewCreateOperatingSystemHandler(dbSession *cdb.Session, tc temporalClient.Client, scp *sc.ClientPool, cfg *config.Config) CreateOperatingSystemHandler {
	return CreateOperatingSystemHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        scp,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Create an OperatingSystem
// @Description Create an OperatingSystem
// @Tags OperatingSystem
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param message body model.APIOperatingSystemCreateRequest true "OperatingSystem creation request"
// @Success 201 {object} model.APIOperatingSystem
// @Router /v2/org/{org}/carbide/operating-system [post]
func (csh CreateOperatingSystemHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("OperatingSystem", "Create", c, csh.tracerSpan)
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

	// Bind request data to API model before role check so we can inspect the OS type.
	apiRequest := model.APIOperatingSystemCreateRequest{}
	err = c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}
	// Infer type of OS from provided parameters:
	osType := model.GetOperatingSystemType(apiRequest.IpxeScript, apiRequest.IpxeTemplateName)

	// Role check: either Provider Admin or Tenant Admin can create OSes.
	// Provider Admin is limited to iPXE Template-based OSes; Tenant Admin may
	// create any type. Both roles are evaluated independently so a dual-role
	// caller is permitted if either role authorizes the operation. When both
	// allow the action, Provider Admin takes priority (provider-owned OS).
	isProviderAdmin := auth.ValidateUserRoles(dbUser, org, nil, auth.ProviderAdminRole)
	isTenantAdmin := auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	allowedByProvider := isProviderAdmin && osType == cdbm.OperatingSystemTypeTemplatedIPXE
	allowedByTenant := isTenantAdmin
	if !allowedByProvider && !allowedByTenant {
		if isProviderAdmin {
			logger.Warn().Msg("provider admin attempted to create non-template OS without tenant admin role")
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Provider Admin can only create iPXE Template-based Operating Systems", nil)
		}
		logger.Warn().Msg("user has neither Provider Admin nor Tenant Admin role, access denied")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Creating Operating Systems requires Provider Admin or Tenant Admin role", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate()
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating Operating System creation request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating Operating System request creation data", verr)
	}

	// Validate and Set UserData
	verr = apiRequest.ValidateAndSetUserData(csh.cfg.GetSitePhoneHomeUrl())
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating user data in Operating System creation request")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating user data in Operating System creation request", verr)
	}

	// Resolve the tenant for this org (needed for tenant-owned OSes and for some lookups).
	tenant, err := common.GetTenantForOrg(ctx, nil, csh.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			logger.Warn().Err(err).Msg("Org does not have a Tenant associated")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Org does not have a Tenant associated", nil)
		}
		logger.Error().Err(err).Msg("unable to retrieve tenant for org")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve tenant for org", nil)
	}
	// If the caller provided an explicit tenantId in the body, validate it matches the org.
	// TODO: tenantId as parameter is deprecated and will need to be removed by 2026-10-01.
	if apiRequest.TenantID != nil {
		apiTenant, terr := common.GetTenantFromIDString(ctx, nil, *apiRequest.TenantID, csh.dbSession)
		if terr != nil {
			logger.Warn().Err(terr).Msg("error retrieving tenant from request")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "TenantID in request is not valid", nil)
		}
		if apiTenant.ID != tenant.ID {
			logger.Warn().Msg("tenant id in request does not match tenant in org")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "TenantID in request does not match tenant in org", nil)
		}
	}

	// Look up InfrastructureProvider now if acting as provider — used for ownership assignment,
	// name-uniqueness check, and site validation throughout the rest of the handler.
	var ip *cdbm.InfrastructureProvider
	if allowedByProvider {
		ip, err = common.GetInfrastructureProviderForOrg(ctx, nil, csh.dbSession, org)
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving Infrastructure Provider for org")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider for org", nil)
		}
	}

	// Check for name uniqueness within the owner's scope.
	osDAO := cdbm.NewOperatingSystemDAO(csh.dbSession)
	var uniquenessFilter cdbm.OperatingSystemFilterInput
	if allowedByProvider && ip != nil {
		uniquenessFilter = cdbm.OperatingSystemFilterInput{
			InfrastructureProviderIDs: []uuid.UUID{ip.ID},
			Names:                     []string{apiRequest.Name},
		}
	} else {
		uniquenessFilter = cdbm.OperatingSystemFilterInput{
			TenantIDs: []uuid.UUID{tenant.ID},
			Names:     []string{apiRequest.Name},
		}
	}
	oss, tot, err := osDAO.GetAll(ctx, nil, uniquenessFilter, cdbp.PageInput{}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("db error checking for name uniqueness of os")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create OperatingSystem due to DB error", nil)
	}
	if tot > 0 {
		logger.Warn().Str("name", apiRequest.Name).Msg("Operating System with same name already exists")
		return cutil.NewAPIErrorResponse(c, http.StatusConflict, "Another Operating System with specified name already exists", validation.Errors{
			"id": errors.New(oss[0].ID.String()),
		})
	}

	// Set the phoneHomeEnabled if provided in request
	phoneHomeEnabled := false
	if apiRequest.PhoneHomeEnabled != nil {
		phoneHomeEnabled = *apiRequest.PhoneHomeEnabled
	}

	// Start a db tx
	tx, err := cdb.BeginTx(ctx, csh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Operating System", nil)
	}

	// This variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Verify or validate site associations.
	// Behaviour depends on osType, scope, and who is acting:
	//   - Image OS (tenant):                siteIds required, validated against TenantSite
	//   - Image OS (provider):               siteIds required, validated against provider's sites
	//   - Templated iPXE + scope "Global":   auto-associate ALL registered provider sites
	//   - Templated iPXE + scope "Limited":  siteIds required, validated against provider sites
	//   - Templated iPXE + scope "Local":    no site associations (bidirectional sync via core)
	//   - raw iPXE:                          no site associations (bidirectional sync via core)
	rdbst := []cdbm.Site{}
	dbossd := []cdbm.StatusDetail{}
	sttsmap := map[uuid.UUID]*cdbm.TenantSite{} // populated only for tenant-owned image-based OS

	isGlobal := osType == cdbm.OperatingSystemTypeTemplatedIPXE && apiRequest.Scope != nil && *apiRequest.Scope == cdbm.OperatingSystemScopeGlobal
	isLimited := osType == cdbm.OperatingSystemTypeTemplatedIPXE && apiRequest.Scope != nil && *apiRequest.Scope == cdbm.OperatingSystemScopeLimited

	if osType == cdbm.OperatingSystemTypeImage || isLimited {
		// Determine how to validate the sites:
		//   - limited-scope iPXE or provider image: sites must belong to the provider
		//   - tenant image:                          sites must be in TenantSite
		isProviderSiteValidation := isLimited || (osType == cdbm.OperatingSystemTypeImage && allowedByProvider)

		var providerSiteID uuid.UUID
		if isProviderSiteValidation {
			if ip == nil {
				// Provider site validation requires the InfrastructureProvider to be resolved.
				ip, err = common.GetInfrastructureProviderForOrg(ctx, nil, csh.dbSession, org)
				if err != nil {
					logger.Error().Err(err).Msg("error retrieving Infrastructure Provider for org")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider for org", nil)
				}
			}
			providerSiteID = ip.ID
		} else {
			// Tenant image: build a TenantSite map for membership validation.
			tsDAO := cdbm.NewTenantSiteDAO(csh.dbSession)
			tss, _, terr := tsDAO.GetAll(
				ctx,
				tx,
				cdbm.TenantSiteFilterInput{TenantIDs: []uuid.UUID{tenant.ID}},
				cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)},
				nil,
			)
			if terr != nil {
				logger.Error().Err(terr).Msg("db error retrieving TenantSite records for Tenant")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site associations for Tenant, DB error", nil)
			}
			for _, ts := range tss {
				cts := ts
				sttsmap[ts.SiteID] = &cts
			}
		}

		for _, stID := range apiRequest.SiteIDs {
			site, serr := common.GetSiteFromIDString(ctx, nil, stID, csh.dbSession)
			if serr != nil {
				if serr == common.ErrInvalidID {
					return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Failed to create Operating System, invalid Site ID: %s", stID), nil)
				}
				if serr == cdb.ErrDoesNotExist {
					return cutil.NewAPIErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Failed to create Operating System, could not find Site with ID: %s ", stID), nil)
				}
				logger.Warn().Err(serr).Str("Site ID", stID).Msg("error retrieving Site from DB by ID")
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Failed to create Operating System, could not retrieve Site with ID: %s, DB error", stID), nil)
			}

			if site.Status != cdbm.SiteStatusRegistered {
				logger.Warn().Str("siteID", site.ID.String()).Msg("Unable to associate Operating System to Site: Site is not in Registered state")
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Failed to create Operating System, Site: %s is not in Registered state", site.ID.String()), nil)
			}

			if isProviderSiteValidation {
				if site.InfrastructureProviderID != providerSiteID {
					return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Unable to associate Operating System with Site: %s, Site does not belong to provider", stID), nil)
				}
			} else {
				// Tenant image: site must belong to the tenant and support image-based OS.
				if _, ok := sttsmap[site.ID]; !ok {
					return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Unable to associate Operating System with Site: %s, Tenant does not have access to Site", stID), nil)
				}
				if site.Config == nil || !site.Config.ImageBasedOperatingSystem {
					logger.Warn().Str("siteId", stID).Msg("Image based Operating System is not supported for Site, ImageBasedOperatingSystem capability is not enabled")
					return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Creation of Image based Operating Systems is not supported. Site must have ImageBasedOperatingSystem capability enabled.", nil)
				}
			}

			rdbst = append(rdbst, *site)
		}
	} else if isGlobal {
		// Global scope: auto-associate with all registered sites belonging to the provider.
		if ip == nil {
			ipVal, iperr := common.GetInfrastructureProviderForOrg(ctx, nil, csh.dbSession, org)
			if iperr != nil {
				logger.Error().Err(iperr).Msg("error retrieving Infrastructure Provider for org")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider for org", nil)
			}
			ip = ipVal
		}
		stDAO := cdbm.NewSiteDAO(csh.dbSession)
		registeredStatus := cdbm.SiteStatusRegistered
		providerSites, _, sterr := stDAO.GetAll(
			ctx, nil,
			cdbm.SiteFilterInput{
				InfrastructureProviderIDs: []uuid.UUID{ip.ID},
				Statuses:                  []string{registeredStatus},
			},
			cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)},
			nil,
		)
		if sterr != nil {
			logger.Error().Err(sterr).Msg("error retrieving provider sites for global-scope OS")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve provider sites, DB error", nil)
		}
		rdbst = append(rdbst, providerSites...)
	}

	// Create status based on OS type
	osStatus := cdbm.OperatingSystemStatusReady
	osStatusMessage := "Operating System is ready for use"
	if osType == cdbm.OperatingSystemTypeImage {
		osStatus = cdbm.OperatingSystemStatusSyncing
		osStatusMessage = "received Operating System creation request, syncing"
	}

	// Assign ownership: provider-owned OSes carry InfrastructureProviderID (tenant_id=nil);
	// tenant-owned OSes carry TenantID (infrastructure_provider_id=nil).
	// This aligns with the sync model where OSes from carbide-core are provider-owned.
	var ownerTenantID *uuid.UUID
	var ownerProviderID *uuid.UUID
	if allowedByProvider {
		if ip == nil {
			// ip may not yet be resolved if the request had no scope/sites requiring it.
			ipVal, iperr := common.GetInfrastructureProviderForOrg(ctx, nil, csh.dbSession, org)
			if iperr != nil {
				logger.Error().Err(iperr).Msg("error retrieving Infrastructure Provider for org during OS create")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider for org", nil)
			}
			ip = ipVal
		}
		ownerProviderID = &ip.ID
	} else {
		ownerTenantID = &tenant.ID
	}

	// Create the db record for Operating System
	osInput := cdbm.OperatingSystemCreateInput{
		Name:                     apiRequest.Name,
		Description:              apiRequest.Description,
		Org:                      org,
		TenantID:                 ownerTenantID,
		InfrastructureProviderID: ownerProviderID,
		OsType:                   osType,
		ImageURL:                 apiRequest.ImageURL,
		ImageSHA:                 apiRequest.ImageSHA,
		ImageAuthType:            apiRequest.ImageAuthType,
		ImageAuthToken:           apiRequest.ImageAuthToken,
		ImageDisk:                apiRequest.ImageDisk,
		RootFsId:                 apiRequest.RootFsID,
		RootFsLabel:              apiRequest.RootFsLabel,
		IpxeScript:               apiRequest.IpxeScript,
		IpxeTemplateName:         apiRequest.IpxeTemplateName,
		IpxeParameters:           apiRequest.IpxeParameters,
		IpxeArtifacts:            stripCachedURLFromArtifacts(apiRequest.IpxeArtifacts),
		UserData:                 apiRequest.UserData,
		IsCloudInit:              apiRequest.IsCloudInit,
		AllowOverride:            apiRequest.AllowOverride,
		EnableBlockStorage:       apiRequest.EnableBlockStorage,
		PhoneHomeEnabled:         phoneHomeEnabled,
		Status:                   osStatus,
		CreatedBy:                dbUser.ID,
		IpxeOsScope:              apiRequest.Scope,
	}
	os, err := osDAO.Create(ctx, tx, osInput)
	if err != nil {
		logger.Error().Err(err).Msg("unable to create Operating System record in DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed creating Operating System record", nil)
	}

	// Create the status detail record for Operating System
	sdDAO := cdbm.NewStatusDetailDAO(csh.dbSession)
	ossd, serr := sdDAO.CreateFromParams(ctx, tx, os.ID.String(), *cdb.GetStrPtr(osStatus),
		cdb.GetStrPtr(osStatusMessage))
	if serr != nil {
		logger.Error().Err(serr).Msg("error creating Status Detail DB entry")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Status Detail for Operating System", nil)
	}

	if ossd == nil {
		logger.Error().Msg("Status Detail DB entry not returned from CreateFromParams")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to get new Status Detail for Operating System", nil)
	}
	dbossd = append(dbossd, *ossd)

	// Create Operating System Site Associations
	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(csh.dbSession)
	for _, st := range rdbst {
		// Create Operating System Site Association
		ossa, serr := ossaDAO.Create(
			ctx,
			tx,
			cdbm.OperatingSystemSiteAssociationCreateInput{
				OperatingSystemID: os.ID,
				SiteID:            st.ID,
				Status:            cdbm.OperatingSystemSiteAssociationStatusSyncing,
				CreatedBy:         dbUser.ID,
			},
		)
		if serr != nil {
			logger.Error().Err(serr).Msg("unable to create the Operating System association record in DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to associate Operating System with one or more Sites, DB error", nil)
		}

		// Create Status details
		_, serr = sdDAO.CreateFromParams(ctx, tx, ossa.ID.String(), *cdb.GetStrPtr(cdbm.OperatingSystemSiteAssociationStatusSyncing),
			cdb.GetStrPtr("received Operating System Association create request, syncing"))
		if serr != nil {
			logger.Error().Err(serr).Msg("error creating Status Detail DB entry")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Status Detail for Operating System Association", nil)
		}

		// Update Operating System Site Association version
		_, err := ossaDAO.GenerateAndUpdateVersion(ctx, tx, ossa.ID)
		if err != nil {
			logger.Error().Err(err).Msg("error updating version for created Operating System Association")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to set version for created Operating System Association, DB error", nil)
		}
	}

	// Retrieve Operating System Associations details
	dbossa, _, err := ossaDAO.GetAll(
		ctx,
		tx,
		cdbm.OperatingSystemSiteAssociationFilterInput{
			OperatingSystemIDs: []uuid.UUID{os.ID},
		},
		cdbp.PageInput{
			Limit: cdb.GetIntPtr(cdbp.TotalLimit),
		},
		[]string{cdbm.SiteRelationName, cdbm.OperatingSystemRelationName},
	)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Operating System Site associations from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Operating System Site associations from DB", nil)
	}

	// Trigger workflows to sync the Operating System with its Sites.
	// Image-based OSes use CreateOsImage; iPXE OSes use CreateOperatingSystem to push the
	// definition to carbide-core (the site controller), using the same UUID as primary key.
	for _, ossa := range dbossa {
		// Get the temporal client for the site we are working with.
		stc, err := csh.scp.GetClientByID(ossa.SiteID)
		if err != nil {
			logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
		}

		var workflowName string
		var workflowInput interface{}
		var workflowIDPrefix string

		if osType == cdbm.OperatingSystemTypeImage {
			workflowName = "CreateOsImage"
			workflowIDPrefix = "image-os-create-"
			workflowInput = &cwssaws.OsImageAttributes{
				Id:                   &cwssaws.UUID{Value: os.ID.String()},
				Name:                 &os.Name,
				TenantOrganizationId: tenant.Org,
				Description:          os.Description,
				SourceUrl:            *os.ImageURL,
				Digest:               *os.ImageSHA,
				CreateVolume:         os.EnableBlockStorage,
				AuthType:             os.ImageAuthType,
				AuthToken:            os.ImageAuthToken,
				RootfsId:             os.RootFsID,
				RootfsLabel:          os.RootFsLabel,
			}
		} else {
			// Templated iPXE OS: verify the referenced template is available at the target site.
			if osType == cdbm.OperatingSystemTypeTemplatedIPXE {
				ipxeTmplDAO := cdbm.NewIpxeTemplateDAO(csh.dbSession)
				_, tmplErr := ipxeTmplDAO.GetBySiteAndName(ctx, nil, ossa.SiteID, *os.IpxeTemplateName)
				if tmplErr != nil {
					if errors.Is(tmplErr, cdb.ErrDoesNotExist) {
						logger.Warn().Str("Site ID", ossa.SiteID.String()).Str("Template", *os.IpxeTemplateName).
							Msg("iPXE template not available at site, skipping sync")
						continue
					}
					logger.Error().Err(tmplErr).Str("Site ID", ossa.SiteID.String()).Msg("error checking iPXE template availability at site")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to verify iPXE template availability at site", nil)
				}
			}

			// iPXE OS: push the definition to carbide-core using the same UUID as primary key.
			workflowName = "CreateOperatingSystem"
			workflowIDPrefix = "templated-ipxe-os-create-"
			workflowInput = &cwssaws.CreateOperatingSystemRequest{
				Id:                   &cwssaws.OperatingSystemId{Value: os.ID.String()},
				Name:                 os.Name,
				Description:          os.Description,
				TenantOrganizationId: tenant.Org,
				IsActive:             os.IsActive,
				AllowOverride:        os.AllowOverride,
				PhoneHomeEnabled:     os.PhoneHomeEnabled,
				UserData:             os.UserData,
				IpxeScript:           os.IpxeScript,
				IpxeTemplateName:     os.IpxeTemplateName,
				IpxeParameters:       dbParamsToProto(os.IpxeParameters),
				IpxeArtifacts:        dbArtifactsToProto(os.IpxeArtifacts),
			}
		}

		workflowOptions := temporalClient.StartWorkflowOptions{
			ID:                       workflowIDPrefix + ossa.SiteID.String() + "-" + os.ID.String() + "-" + *ossa.Version,
			WorkflowExecutionTimeout: cutil.WorkflowExecutionTimeout,
			TaskQueue:                queue.SiteTaskQueue,
		}

		logger.Info().Str("Site ID", ossa.SiteID.String()).Str("Workflow", workflowName).Msg("triggering Operating System create workflow")

		// Add context deadlines
		ctx, cancel := context.WithTimeout(ctx, cutil.WorkflowContextTimeout)
		defer cancel()

		// Trigger Site workflow
		we, err := stc.ExecuteWorkflow(ctx, workflowOptions, workflowName, workflowInput)
		if err != nil {
			logger.Error().Err(err).Msg("failed to synchronously start Temporal workflow to create Operating System")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed start sync workflow to create Operating System on Site: %s", err), nil)
		}

		wid := we.GetID()
		logger.Info().Str("Workflow ID", wid).Msg("executed synchronous create Operating System workflow")

		// Block until the workflow has completed and returned success/error.
		err = we.Get(ctx, nil)
		if err != nil {
			var timeoutErr *tp.TimeoutError
			if errors.As(err, &timeoutErr) || err == context.DeadlineExceeded || ctx.Err() != nil {

				logger.Error().Err(err).Msg("failed to create Operating System, timeout occurred executing workflow on Site.")

				// Create a new context deadlines
				newctx, newcancel := context.WithTimeout(context.Background(), cutil.WorkflowContextNewAfterTimeout)
				defer newcancel()

				// Initiate termination workflow
				serr := stc.TerminateWorkflow(newctx, wid, "", "timeout occurred executing create Operating System workflow")
				if serr != nil {
					logger.Error().Err(serr).Msg("failed to execute terminate Temporal workflow for creating Operating System")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to terminate synchronous Operating System creation workflow after timeout, Cloud and Site data may be de-synced: %s", serr), nil)
				}

				logger.Info().Str("Workflow ID", wid).Msg("initiated terminate synchronous create Operating System workflow successfully")

				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to create Operating System, timeout occurred executing workflow on Site: %s", err), nil)
			}

			code, err := common.UnwrapWorkflowError(err)
			logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to create Operating System")
			return cutil.NewAPIErrorResponse(c, code, fmt.Sprintf("Failed to execute sync workflow to create Operating System on Site: %s", err), nil)
		}
		logger.Info().Str("Workflow ID", wid).Str("Site ID", ossa.SiteID.String()).Msg("completed synchronous create Operating System workflow")
	}

	// commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing Operating System transaction to DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Operating System", nil)
	}
	// set committed so, deferred cleanup functions will do nothing
	txCommitted = true

	// create response
	apiOperatingSystem := model.NewAPIOperatingSystem(os, dbossd, dbossa, sttsmap)
	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusCreated, apiOperatingSystem)
}

// ~~~~~ GetAll Handler ~~~~~ //

// GetAllOperatingSystemHandler is the API Handler for getting all OperatingSystems
type GetAllOperatingSystemHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewGetAllOperatingSystemHandler initializes and returns a new handler for getting all OperatingSystems
func NewGetAllOperatingSystemHandler(dbSession *cdb.Session, tc temporalClient.Client, cfg *config.Config) GetAllOperatingSystemHandler {
	return GetAllOperatingSystemHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get all OperatingSystems
// @Description Get all OperatingSystems
// @Tags OperatingSystem
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param siteId query string true "ID of Site"
// @Param type query string true "type of Operating System" e.g. 'iPXE', 'Image'"
// @Param status query string false "Filter by status" e.g. 'Pending', 'Error'"
// @Param query query string false "Query input for full text search"
// @Param includeRelation query string false "Related entities to include in response e.g. 'InfrastructureProvider', 'Tenant'"
// @Param pageNumber query integer false "Page number of results returned"
// @Param pageSize query integer false "Number of results per page"
// @Param orderBy query string false "Order by field"
// @Success 200 {object} []model.APIOperatingSystem
// @Router /v2/org/{org}/carbide/operating-system [get]
func (gash GetAllOperatingSystemHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("OperatingSystem", "GetAll", c, gash.tracerSpan)
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

	// Validate role: Tenant Admin or Provider Admin may list Operating Systems.
	isTenantAdmin := auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	isProviderAdmin := auth.ValidateUserRoles(dbUser, org, nil, auth.ProviderAdminRole)
	if !isTenantAdmin && !isProviderAdmin {
		logger.Warn().Msg("user does not have Tenant Admin or Provider Admin role, access denied")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin or Provider Admin role with org", nil)
	}

	// Validate pagination request
	pageRequest := pagination.PageRequest{}
	err = c.Bind(&pageRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding pagination request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request pagination data", nil)
	}

	// Validate request attributes
	err = pageRequest.Validate(cdbm.OperatingSystemOrderByFields)
	if err != nil {
		logger.Warn().Err(err).Msg("error validating pagination request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate pagination request data", err)
	}

	// Validate the tenant associated with the org
	tenant, err := common.GetTenantForOrg(ctx, nil, gash.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			logger.Warn().Err(err).Msg("Org does not have a Tenant associated")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Org does not have a Tenant associated", nil)
		}
		logger.Error().Err(err).Msg("unable to retrieve tenant for org")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve tenant for org", nil)
	}

	// Both Tenant Admin and Provider Admin can see OSes belonging to their tenant and to their
	// infrastructure provider. Look up the provider (best-effort: no provider = tenant-only view).
	// We intentionally do NOT filter by Org here: the stored org on synced OSes reflects the
	// carbide-rest tenant org at the time of sync, which is authoritative. TenantIDs already
	// gives the correct per-tenant scope.
	var providerID *uuid.UUID
	if ip, iperr := common.GetInfrastructureProviderForOrg(ctx, nil, gash.dbSession, org); iperr == nil {
		providerID = &ip.ID
	}

	filter := cdbm.OperatingSystemFilterInput{
		TenantIDs: []uuid.UUID{tenant.ID},
	}
	if providerID != nil {
		filter.InfrastructureProviderIDs = []uuid.UUID{*providerID}
	}

	// Get and validate includeRelation params
	qParams := c.QueryParams()
	qIncludeRelations, errMsg := common.GetAndValidateQueryRelations(qParams, cdbm.OperatingSystemRelatedEntities)
	if errMsg != "" {
		logger.Warn().Msg(errMsg)
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, errMsg, nil)
	}

	// now check siteID in query
	tsDAO := cdbm.NewTenantSiteDAO(gash.dbSession)

	qSiteID := qParams["siteId"]
	if len(qSiteID) > 0 {
		for _, siteID := range qSiteID {
			site, err := common.GetSiteFromIDString(ctx, nil, siteID, gash.dbSession)
			if err != nil {
				logger.Warn().Err(err).Msg("error getting Site from query string")
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to retrieve Site specified in query", nil)
			}

			// Determine if caller has access to the requested site.
			// Tenant path: TenantSite association exists.
			// Provider path: site belongs to the caller's infrastructure provider.
			_, tsErr := tsDAO.GetByTenantIDAndSiteID(ctx, nil, tenant.ID, site.ID, nil)
			if tsErr != nil {
				if tsErr != cdb.ErrDoesNotExist {
					logger.Warn().Err(tsErr).Msg("error retrieving Tenant Site association from DB")
					return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to determine if Tenant has access to Site specified in query, DB error", nil)
				}
				if !isProviderAdmin || providerID == nil || site.InfrastructureProviderID != *providerID {
					return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Caller is not associated with Site specified in query", nil)
				}
			}
			filter.SiteIDs = append(filter.SiteIDs, site.ID)
		}
	}

	// Get query type from query param
	if typeQuery := qParams["type"]; len(typeQuery) > 0 {
		gash.tracerSpan.SetAttribute(handlerSpan, attribute.StringSlice("type", typeQuery), logger)
		for _, typeVal := range typeQuery {
			_, ok := cdbm.OperatingSystemsTypeMap[typeVal]
			if !ok {
				logger.Warn().Msg(fmt.Sprintf("Invalid type value in query: %v", typeVal))
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid type value in query", nil)
			}
			filter.OsTypes = append(filter.OsTypes, typeVal)
		}
	}

	// Get query text for full text search from query param
	searchQueryStr := c.QueryParam("query")
	if searchQueryStr != "" {
		filter.SearchQuery = &searchQueryStr
		gash.tracerSpan.SetAttribute(handlerSpan, attribute.String("query", searchQueryStr), logger)
	}

	// Get status from query param
	if statusQuery := qParams["status"]; len(statusQuery) > 0 {
		gash.tracerSpan.SetAttribute(handlerSpan, attribute.StringSlice("status", statusQuery), logger)
		for _, status := range statusQuery {
			_, ok := cdbm.OperatingSystemStatusMap[status]
			if !ok {
				logger.Warn().Msg(fmt.Sprintf("invalid value in status query: %v", status))
				statusError := validation.Errors{
					"status": errors.New(status),
				}
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Status value in query", statusError)
			}
			filter.Statuses = append(filter.Statuses, status)
		}
	}

	// Get all Operating System by Tenant
	osDAO := cdbm.NewOperatingSystemDAO(gash.dbSession)
	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(gash.dbSession)

	// Create response
	oss, total, err := osDAO.GetAll(
		ctx,
		nil,
		filter,
		cdbp.PageInput{
			Offset:  pageRequest.Offset,
			Limit:   pageRequest.Limit,
			OrderBy: pageRequest.OrderBy,
		},
		qIncludeRelations,
	)
	if err != nil {
		logger.Error().Err(err).Msg("error getting os from db")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve OperatingSystems", nil)
	}

	// Get status details
	sdDAO := cdbm.NewStatusDetailDAO(gash.dbSession)

	osIDs := []uuid.UUID{}
	sdEntityIDs := []string{}
	for _, os := range oss {
		sdEntityIDs = append(sdEntityIDs, os.ID.String())
		osIDs = append(osIDs, os.ID)
	}

	ssds, serr := sdDAO.GetRecentByEntityIDs(ctx, nil, sdEntityIDs, common.RECENT_STATUS_DETAIL_COUNT)
	if serr != nil {
		logger.Warn().Err(serr).Msg("error retrieving Status Details for Operating Systems from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to populate status history for Operating Systems", nil)
	}
	ssdMap := map[string][]cdbm.StatusDetail{}
	for _, ssd := range ssds {
		cssd := ssd
		ssdMap[ssd.EntityID] = append(ssdMap[ssd.EntityID], cssd)
	}

	// Get all OperatingSystemSiteAssociations
	var siteIDs []uuid.UUID
	if filter.SiteIDs != nil {
		siteIDs = filter.SiteIDs
	}
	dbossas, _, err := ossaDAO.GetAll(
		ctx,
		nil,
		cdbm.OperatingSystemSiteAssociationFilterInput{
			OperatingSystemIDs: osIDs,
			SiteIDs:            siteIDs,
		},
		cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)},
		[]string{cdbm.SiteRelationName},
	)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Operating System Site associations from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Operating System Site associations from DB", nil)
	}

	// Prepare OperatingSystemSiteAssociation for each OS if it exists
	dbossaMap := map[uuid.UUID][]cdbm.OperatingSystemSiteAssociation{}
	for _, dbossa := range dbossas {
		curVal := dbossa
		dbossaMap[dbossa.OperatingSystemID] = append(dbossaMap[dbossa.OperatingSystemID], curVal)
	}

	// Get all TenantSite records for the Tenant
	sttsmap := map[uuid.UUID]*cdbm.TenantSite{}

	tsDAO = cdbm.NewTenantSiteDAO(gash.dbSession)
	tss, _, err := tsDAO.GetAll(
		ctx,
		nil,
		cdbm.TenantSiteFilterInput{
			TenantIDs: []uuid.UUID{tenant.ID},
			SiteIDs:   siteIDs,
		},
		cdbp.PageInput{
			Limit: cdb.GetIntPtr(cdbp.TotalLimit),
		},
		nil,
	)
	if err != nil {
		logger.Error().Err(err).Msg("db error retrieving TenantSite records for Tenant")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site associations for Tenant, DB error", nil)
	}

	for _, ts := range tss {
		curVal := ts
		sttsmap[ts.SiteID] = &curVal
	}

	// Create response
	apiOperatingSystems := []*model.APIOperatingSystem{}

	for _, os := range oss {
		curVal := os
		apiOperatingSystem := model.NewAPIOperatingSystem(&curVal, ssdMap[os.ID.String()], dbossaMap[os.ID], sttsmap)
		apiOperatingSystems = append(apiOperatingSystems, apiOperatingSystem)
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

	return c.JSON(http.StatusOK, apiOperatingSystems)

}

// ~~~~~ Get Handler ~~~~~ //

// GetOperatingSystemHandler is the API Handler for retrieving OperatingSystem
type GetOperatingSystemHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewGetOperatingSystemHandler initializes and returns a new handler to retrieve OperatingSystem
func NewGetOperatingSystemHandler(dbSession *cdb.Session, tc temporalClient.Client, cfg *config.Config) GetOperatingSystemHandler {
	return GetOperatingSystemHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Retrieve the OperatingSystem
// @Description Retrieve the OperatingSystem
// @Tags OperatingSystem
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of OperatingSystem"
// @Param includeRelation query string false "Related entities to include in response e.g. 'InfrastructureProvider', 'Tenant', 'Site'"
// @Success 200 {object} model.APIOperatingSystem
// @Router /v2/org/{org}/carbide/operating-system/{id} [get]
func (gsh GetOperatingSystemHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("OperatingSystem", "Get", c, gsh.tracerSpan)
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

	// Validate role: Tenant Admin or Provider Admin may retrieve a single Operating System.
	if !auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole) &&
		!auth.ValidateUserRoles(dbUser, org, nil, auth.ProviderAdminRole) {
		logger.Warn().Msg("user does not have Tenant Admin or Provider Admin role, access denied")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin or Provider Admin role with org", nil)
	}

	// Get and validate includeRelation params
	qParams := c.QueryParams()
	qIncludeRelations, errMsg := common.GetAndValidateQueryRelations(qParams, cdbm.OperatingSystemRelatedEntities)
	if errMsg != "" {
		logger.Warn().Msg(errMsg)
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, errMsg, nil)
	}

	// Get os ID from URL param
	osStrID := c.Param("id")

	gsh.tracerSpan.SetAttribute(handlerSpan, attribute.String("operatingsystem_id", osStrID), logger)

	sID, err := uuid.Parse(osStrID)
	if err != nil {
		logger.Warn().Err(err).Msg("error parsing id in url into uuid")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid OperatingSystem ID in URL", nil)
	}

	osDAO := cdbm.NewOperatingSystemDAO(gsh.dbSession)

	// Validate the tenant for which this OperatingSystem is being retrieved
	tenant, err := common.GetTenantForOrg(ctx, nil, gsh.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			logger.Warn().Err(err).Msg("Org does not have a Tenant associated")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Org does not have a Tenant associated", nil)
		}
		logger.Error().Err(err).Msg("unable to retrieve tenant for org")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve tenant for org", nil)
	}

	// Check that operating system exists
	os, err := osDAO.GetByID(ctx, nil, sID, qIncludeRelations)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving OperatingSystem DB entity")
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not retrieve OperatingSystem to update", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Could not retrieve OperatingSystem to update", nil)
	}

	// Verify the caller can see this OS: it must belong to their tenant or their infrastructure provider.
	ownedByTenant := os.TenantID != nil && *os.TenantID == tenant.ID
	ownedByProvider := false
	if os.InfrastructureProviderID != nil {
		ip, iperr := common.GetInfrastructureProviderForOrg(ctx, nil, gsh.dbSession, org)
		if iperr != nil && iperr != common.ErrOrgInstrastructureProviderNotFound {
			logger.Error().Err(iperr).Msg("error retrieving Infrastructure Provider for org")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider for org", nil)
		}
		if iperr == nil {
			ownedByProvider = *os.InfrastructureProviderID == ip.ID
		}
	}
	if !ownedByTenant && !ownedByProvider {
		logger.Warn().Msg("operating system does not belong to the tenant or provider in org")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Operating System does not belong to the tenant or infrastructure provider in org", nil)
	}

	// get status details for the response
	sdDAO := cdbm.NewStatusDetailDAO(gsh.dbSession)
	ssds, err := sdDAO.GetRecentByEntityIDs(ctx, nil, []string{os.ID.String()}, common.RECENT_STATUS_DETAIL_COUNT)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Status Details for operating system from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Status Details for OperatingSystem", nil)
	}

	// Get all OperatingSystemSiteAssociations (both Image and iPXE types may have them).
	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(gsh.dbSession)
	dbossas, _, err := ossaDAO.GetAll(
		ctx,
		nil,
		cdbm.OperatingSystemSiteAssociationFilterInput{
			OperatingSystemIDs: []uuid.UUID{os.ID},
		},
		cdbp.PageInput{
			Limit: cdb.GetIntPtr(cdbp.TotalLimit),
		},
		[]string{cdbm.SiteRelationName},
	)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Operating System Site associations from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Operating System Site associations from DB", nil)
	}

	// Get all TenantSite records for the Tenant
	sttsmap := map[uuid.UUID]*cdbm.TenantSite{}
	tsDAO := cdbm.NewTenantSiteDAO(gsh.dbSession)
	tss, _, err := tsDAO.GetAll(
		ctx,
		nil,
		cdbm.TenantSiteFilterInput{
			TenantIDs: []uuid.UUID{tenant.ID},
		},
		cdbp.PageInput{
			Limit: cdb.GetIntPtr(cdbp.TotalLimit),
		},
		nil,
	)
	if err != nil {
		logger.Error().Err(err).Msg("db error retrieving TenantSite records for Tenant")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site associations for Tenant, DB error", nil)
	}

	for _, ts := range tss {
		cts := ts
		sttsmap[ts.SiteID] = &cts
	}

	// Send response
	apiInstance := model.NewAPIOperatingSystem(os, ssds, dbossas, sttsmap)
	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, apiInstance)
}

// ~~~~~ Update Handler ~~~~~ //

// UpdateOperatingSystemHandler is the API Handler for updating a OperatingSystem
type UpdateOperatingSystemHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewUpdateOperatingSystemHandler initializes and returns a new handler for updating OperatingSystem
func NewUpdateOperatingSystemHandler(dbSession *cdb.Session, tc temporalClient.Client, scp *sc.ClientPool, cfg *config.Config) UpdateOperatingSystemHandler {
	return UpdateOperatingSystemHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        scp,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Update an existing OperatingSystem
// @Description Update an existing OperatingSystem
// @Tags OperatingSystem
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of OperatingSystem"
// @Param message body model.APIOperatingSystemUpdateRequest true "OperatingSystem update request"
// @Success 200 {object} model.APIOperatingSystem
// @Router /v2/org/{org}/carbide/operating-system/{id} [patch]
func (ush UpdateOperatingSystemHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("OperatingSystem", "Update", c, ush.tracerSpan)
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

	// Validate role: Tenant Admin or Provider Admin may update Operating Systems (with different ownership rules).
	isTenantAdmin := auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	isProviderAdmin := auth.ValidateUserRoles(dbUser, org, nil, auth.ProviderAdminRole)
	if !isTenantAdmin && !isProviderAdmin {
		logger.Warn().Msg("user does not have Tenant Admin or Provider Admin role, access denied")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin or Provider Admin role with org", nil)
	}

	// Get os ID from URL param
	osStrID := c.Param("id")

	ush.tracerSpan.SetAttribute(handlerSpan, attribute.String("operatingsystem_id", osStrID), logger)

	osID, err := uuid.Parse(osStrID)
	if err != nil {
		logger.Warn().Err(err).Msg("error parsing id in url into uuid")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid OperatingSystem ID in URL", nil)
	}

	osDAO := cdbm.NewOperatingSystemDAO(ush.dbSession)

	// Validate request
	// Bind request data to API model
	apiRequest := model.APIOperatingSystemUpdateRequest{}
	err = c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	// Check that os exists
	os, err := osDAO.GetByID(ctx, nil, osID, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving OperatingSystem DB entity")
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find Operating System with ID specified in URL", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Could not retrieve OperatingSystem to update", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate(os)
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating Operating System update request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating Operating System update request data", verr)
	}

	// Validate and Set UserData
	verr = apiRequest.ValidateAndSetUserData(ush.cfg.GetSitePhoneHomeUrl(), os)
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating user data in Operating System creation request")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating user data in Operating System creation request", verr)
	}

	// Validate the tenant for which this OperatingSystem is being updated
	tenant, err := common.GetTenantForOrg(ctx, nil, ush.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			logger.Warn().Err(err).Msg("Org does not have a Tenant associated")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Org does not have a Tenant associated", nil)
		}
		logger.Error().Err(err).Msg("unable to retrieve tenant for org")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve tenant for org", nil)
	}

	// Enforce ownership: both roles are evaluated independently so a dual-role
	// caller is permitted if either role authorizes the operation.
	ownedByTenant := os.TenantID != nil && *os.TenantID == tenant.ID && os.InfrastructureProviderID == nil
	ownedByProvider := false
	if isProviderAdmin && os.InfrastructureProviderID != nil {
		ip, iperr := common.GetInfrastructureProviderForOrg(ctx, nil, ush.dbSession, org)
		if iperr != nil {
			logger.Error().Err(iperr).Msg("error retrieving Infrastructure Provider for org")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider for org", nil)
		}
		if *os.InfrastructureProviderID != ip.ID {
			logger.Warn().Msg("provider admin cannot update operating system owned by a different provider")
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Provider Admin can only update Operating Systems owned by their own provider", nil)
		}
		ownedByProvider = true
	}
	allowedByProvider := isProviderAdmin && ownedByProvider
	allowedByTenant := isTenantAdmin && ownedByTenant
	if !allowedByProvider && !allowedByTenant {
		if isProviderAdmin && !isTenantAdmin {
			logger.Warn().Msg("provider admin cannot update tenant-owned operating system")
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Provider Admin can only update provider-owned Operating Systems", nil)
		}
		if isTenantAdmin && !isProviderAdmin {
			logger.Warn().Msg("tenant admin cannot update provider-owned operating system")
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant Admin can only update their own Operating Systems", nil)
		}
		logger.Warn().Msg("user does not have permission to update this operating system")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Operating System does not belong to your tenant or infrastructure provider", nil)
	}

	// Check for name uniqueness within the owner's scope (provider or tenant).
	if apiRequest.Name != nil && *apiRequest.Name != os.Name {
		var uniquenessFilter cdbm.OperatingSystemFilterInput
		if os.InfrastructureProviderID != nil {
			uniquenessFilter = cdbm.OperatingSystemFilterInput{
				InfrastructureProviderIDs: []uuid.UUID{*os.InfrastructureProviderID},
				Names:                     []string{*apiRequest.Name},
			}
		} else {
			uniquenessFilter = cdbm.OperatingSystemFilterInput{
				TenantIDs: []uuid.UUID{tenant.ID},
				Names:     []string{*apiRequest.Name},
			}
		}
		oss, tot, serr := osDAO.GetAll(
			ctx,
			nil,
			uniquenessFilter,
			cdbp.PageInput{},
			nil,
		)
		if serr != nil {
			logger.Error().Err(serr).Msg("db error checking for name uniqueness of os")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update OperatingSystem due to DB error", nil)
		}
		if tot > 0 {
			return cutil.NewAPIErrorResponse(c, http.StatusConflict, "Another Operating System with specified name already exists", validation.Errors{
				"id": errors.New(oss[0].ID.String()),
			})
		}
	}

	dbossas := []cdbm.OperatingSystemSiteAssociation{}
	sttsmap := map[uuid.UUID]*cdbm.TenantSite{}
	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(ush.dbSession)
	tsDAO := cdbm.NewTenantSiteDAO(ush.dbSession)

	osType := os.Type

	// Verify Tenant Site Association
	// Verify if Site is in Registered state
	if osType == cdbm.OperatingSystemTypeImage {
		dbossas, _, err = ossaDAO.GetAll(
			ctx,
			nil,
			cdbm.OperatingSystemSiteAssociationFilterInput{
				OperatingSystemIDs: []uuid.UUID{os.ID},
			},
			cdbp.PageInput{},
			[]string{cdbm.SiteRelationName},
		)
		if err != nil {
			logger.Error().Err(err).Msg("error retrieving Operating System Site associations from DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Operating System Site associations from DB", nil)
		}

		// Get all TenantSite records for the Tenant
		tss, _, err := tsDAO.GetAll(
			ctx,
			nil,
			cdbm.TenantSiteFilterInput{
				TenantIDs: []uuid.UUID{tenant.ID},
			},
			cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)},
			nil,
		)
		if err != nil {
			logger.Error().Err(err).Msg("db error retrieving TenantSite records for Tenant")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site associations for Tenant, DB error", nil)
		}

		for _, ts := range tss {
			cts := ts
			sttsmap[ts.SiteID] = &cts
		}

		// Verify if associated Site is not registered state
		// Verify if current tenant not associated Site
		for _, dbosa := range dbossas {
			if dbosa.Site.Status != cdbm.SiteStatusRegistered {
				logger.Warn().Msg(fmt.Sprintf("unable to update Operating System. Site: %s. Site is not in Registered state", dbosa.Site.Name))
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Failed to update Operating System, Associated Site: %s is not in Registered state", dbosa.Site.Name), nil)
			}

			// Validate the TenantSite exists for current tenant and this site
			_, ok := sttsmap[dbosa.SiteID]
			if !ok {
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Unable to update associate Operating System with Site: %s, Tenant does not have access to Site", dbosa.Site.Name), nil)
			}
		}
	}

	// start a database transaction
	tx, err := cdb.BeginTx(ctx, ush.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("error updating os in DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Operating System", nil)
	}
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// Save update status in DB
	osStatus := db.GetStrPtr(cdbm.OperatingSystemStatusReady)
	osStatusMessage := "Operating System has been updated and ready for use"
	if apiRequest.IsActive != nil && !*apiRequest.IsActive {
		osStatus = db.GetStrPtr(cdbm.OperatingSystemStatusDeactivated)
		osStatusMessage = "Operating System has been deactivated"
		if apiRequest.DeactivationNote != nil && *apiRequest.DeactivationNote != "" {
			osStatusMessage += ". " + *apiRequest.DeactivationNote
		}
	} else {
		if apiRequest.IsActive != nil && *apiRequest.IsActive {
			osStatusMessage = "Operating System has been reactivated and is ready for use"
		}
		if osType == cdbm.OperatingSystemTypeImage {
			osStatus = db.GetStrPtr(cdbm.OperatingSystemStatusSyncing)
			osStatusMessage = "received Operating System update request, syncing"
		}
	}

	// When switching from inactive to active, clear deactivation note
	deactivationNote := apiRequest.DeactivationNote
	if apiRequest.IsActive != nil && *apiRequest.IsActive {
		deactivationNote = nil
		_, err := osDAO.Clear(ctx, tx, cdbm.OperatingSystemClearInput{OperatingSystemId: osID, DeactivationNote: true})
		if err != nil {
			logger.Error().Err(err).Msg("error updating/clearing Operating System in DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update/clear Operating System", nil)
		}
	}
	uos, err := osDAO.Update(ctx, tx, cdbm.OperatingSystemUpdateInput{
		OperatingSystemId: osID,
		Name:              apiRequest.Name,
		Description:       apiRequest.Description,
		ImageURL:          apiRequest.ImageURL,
		ImageSHA:          apiRequest.ImageSHA,
		ImageAuthType:     apiRequest.ImageAuthType,
		ImageAuthToken:    apiRequest.ImageAuthToken,
		ImageDisk:         apiRequest.ImageDisk,
		RootFsId:          apiRequest.RootFsID,
		RootFsLabel:       apiRequest.RootFsLabel,
		IpxeScript:        apiRequest.IpxeScript,
		IpxeTemplateName:  apiRequest.IpxeTemplateName,
		IpxeParameters:    apiRequest.IpxeParameters,
		IpxeArtifacts:     stripCachedURLFromArtifactsPtr(apiRequest.IpxeArtifacts),
		UserData:          apiRequest.UserData,
		IsCloudInit:       apiRequest.IsCloudInit,
		AllowOverride:     apiRequest.AllowOverride,
		PhoneHomeEnabled:  apiRequest.PhoneHomeEnabled,
		IsActive:          apiRequest.IsActive,
		DeactivationNote:  deactivationNote,
		Status:            osStatus,
	})
	if err != nil {
		logger.Error().Err(err).Msg("error updating Operating System in DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Operating System", nil)
	}
	logger.Info().Msg("done updating os in DB")

	sdDAO := cdbm.NewStatusDetailDAO(ush.dbSession)
	_, serr := sdDAO.CreateFromParams(ctx, tx, uos.ID.String(), *osStatus, &osStatusMessage)
	if serr != nil {
		logger.Error().Err(serr).Msg("error creating Status Detail DB entry")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create status detail for Operating System update", nil)
	}

	// get status details for the response
	ssds, _, err := sdDAO.GetAllByEntityID(ctx, tx, uos.ID.String(), nil, cdb.GetIntPtr(pagination.MaxPageSize), nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Status Details for os from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Status Details for Operating System", nil)
	}

	// Trigger workflows to sync the Operating System update with its Sites.
	// Image-based OSes use UpdateOsImage; iPXE OSes use UpdateOperatingSystem to push the
	// updated definition to carbide-core (the site controller).
	if osType == cdbm.OperatingSystemTypeImage {
		for _, dbossa := range dbossas {
			_, err = ossaDAO.Update(
				ctx,
				tx,
				cdbm.OperatingSystemSiteAssociationUpdateInput{
					OperatingSystemSiteAssociationID: dbossa.ID,
					Status:                           cdb.GetStrPtr(cdbm.OperatingSystemSiteAssociationStatusSyncing),
				},
			)
			if err != nil {
				logger.Error().Err(serr).Msg("unable to update the Operating System association record in DB")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Operating System Site Association status, DB error", nil)
			}

			// Create Status details
			_, serr = sdDAO.CreateFromParams(ctx, tx, dbossa.ID.String(), *cdb.GetStrPtr(cdbm.OperatingSystemSiteAssociationStatusSyncing),
				cdb.GetStrPtr("received Operating System Association update request, syncing"))
			if serr != nil {
				logger.Error().Err(serr).Msg("error creating Status Detail DB entry")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Status Detail for Operating System Site Association", nil)
			}

			// Update Operating System Association version
			updatedOssa, err := ossaDAO.GenerateAndUpdateVersion(ctx, tx, dbossa.ID)
			if err != nil {
				logger.Error().Err(err).Msg("error updating version for updated Operating System Association")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to set version for updated Operating System Site Association, DB error", nil)
			}

			// Get the temporal client for the site we are working with.
			stc, err := ush.scp.GetClientByID(dbossa.SiteID)
			if err != nil {
				logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
			}

			updateOsRequest := &cwssaws.OsImageAttributes{
				Id:                   &cwssaws.UUID{Value: uos.ID.String()},
				Name:                 &uos.Name,
				Description:          uos.Description,
				TenantOrganizationId: tenant.Org,
				SourceUrl:            *uos.ImageURL,
				Digest:               *uos.ImageSHA,
				CreateVolume:         uos.EnableBlockStorage,
				AuthType:             uos.ImageAuthType,
				AuthToken:            uos.ImageAuthToken,
				RootfsId:             uos.RootFsID,
				RootfsLabel:          uos.RootFsLabel,
			}

			workflowOptions := temporalClient.StartWorkflowOptions{
				ID:                       "image-os-update-" + updatedOssa.SiteID.String() + "-" + uos.ID.String() + "-" + *updatedOssa.Version,
				WorkflowExecutionTimeout: cutil.WorkflowExecutionTimeout,
				TaskQueue:                queue.SiteTaskQueue,
			}

			logger.Info().Str("Site ID", dbossa.SiteID.String()).Msg("triggering Image based Operating System update workflow")

			// Add context deadlines
			ctx, cancel := context.WithTimeout(ctx, cutil.WorkflowContextTimeout)
			defer cancel()

			// Trigger Site workflow
			we, err := stc.ExecuteWorkflow(ctx, workflowOptions, "UpdateOsImage", updateOsRequest)
			if err != nil {
				logger.Error().Err(err).Msg("failed to synchronously start Temporal workflow to update Operating System")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed start sync workflow to update Operating System on Site: %s", err), nil)
			}

			wid := we.GetID()
			logger.Info().Str("Workflow ID", wid).Msg("executed synchronous update Operating System workflow")

			// Block until the workflow has completed and returned success/error.
			err = we.Get(ctx, nil)
			if err != nil {
				var timeoutErr *tp.TimeoutError
				if errors.As(err, &timeoutErr) || err == context.DeadlineExceeded || ctx.Err() != nil {

					logger.Error().Err(err).Msg("failed to update Operating System, timeout occurred executing workflow on Site.")

					// Create a new context deadlines
					newctx, newcancel := context.WithTimeout(context.Background(), cutil.WorkflowContextNewAfterTimeout)
					defer newcancel()

					// Initiate termination workflow
					serr := stc.TerminateWorkflow(newctx, wid, "", "timeout occurred executing update Operating System workflow")
					if serr != nil {
						logger.Error().Err(serr).Msg("failed to execute terminate Temporal workflow for updating Operating System")
						return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to terminate synchronous Operating System update workflow after timeout, Cloud and Site data may be de-synced: %s", serr), nil)
					}

					logger.Info().Str("Workflow ID", wid).Msg("initiated terminate synchronous update Operating System workflow successfully")

					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to update Operating System, timeout occurred executing workflow on Site: %s", err), nil)
				}
				code, err := common.UnwrapWorkflowError(err)
				logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to update Operating System")
				return cutil.NewAPIErrorResponse(c, code, fmt.Sprintf("Failed to execute sync workflow to update Operating System on Site: %s", err), nil)
			}
			logger.Info().Str("Workflow ID", wid).Str("Site ID", dbossa.SiteID.String()).Msg("completed synchronous update Operating System workflow")
		}
	} else if cdbm.IsIPXEType(osType) {
		// Populate dbossas so the workflow loop below has site associations to
		// iterate over.  The strategy depends on the OS scope.
		if uos.IpxeOsScope != nil && *uos.IpxeOsScope == cdbm.OperatingSystemScopeGlobal {
			// Global scope: ensure associations exist for every registered
			// provider site, creating new ones for sites added after the OS was
			// originally created.
			ip, iperr := common.GetInfrastructureProviderForOrg(ctx, nil, ush.dbSession, org)
			if iperr != nil {
				logger.Error().Err(iperr).Msg("error retrieving Infrastructure Provider for org")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider for org", nil)
			}

			stDAO := cdbm.NewSiteDAO(ush.dbSession)
			providerSites, _, sterr := stDAO.GetAll(
				ctx, nil,
				cdbm.SiteFilterInput{
					InfrastructureProviderIDs: []uuid.UUID{ip.ID},
					Statuses:                  []string{cdbm.SiteStatusRegistered},
				},
				cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)},
				nil,
			)
			if sterr != nil {
				logger.Error().Err(sterr).Msg("error retrieving provider sites for global-scope OS update")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve provider sites, DB error", nil)
			}

			existingOssas, _, eerr := ossaDAO.GetAll(
				ctx, tx,
				cdbm.OperatingSystemSiteAssociationFilterInput{
					OperatingSystemIDs: []uuid.UUID{uos.ID},
				},
				cdbp.PageInput{},
				nil,
			)
			if eerr != nil {
				logger.Error().Err(eerr).Msg("error retrieving existing Operating System Site associations")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Operating System Site associations from DB", nil)
			}

			existingSites := map[uuid.UUID]struct{}{}
			for _, ea := range existingOssas {
				existingSites[ea.SiteID] = struct{}{}
			}

			for _, ps := range providerSites {
				if _, ok := existingSites[ps.ID]; ok {
					continue
				}
				ossa, serr := ossaDAO.Create(ctx, tx, cdbm.OperatingSystemSiteAssociationCreateInput{
					OperatingSystemID: uos.ID,
					SiteID:            ps.ID,
					Status:            cdbm.OperatingSystemSiteAssociationStatusSyncing,
					CreatedBy:         dbUser.ID,
				})
				if serr != nil {
					logger.Error().Err(serr).Msg("unable to create Operating System Site Association for global-scope update")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Operating System Site Association, DB error", nil)
				}
				_, serr = sdDAO.CreateFromParams(ctx, tx, ossa.ID.String(),
					*cdb.GetStrPtr(cdbm.OperatingSystemSiteAssociationStatusSyncing),
					cdb.GetStrPtr("received Operating System Association update request, syncing"))
				if serr != nil {
					logger.Error().Err(serr).Msg("error creating Status Detail DB entry for new association")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Status Detail for Operating System Site Association", nil)
				}
			}

			// Reload all associations (existing + newly created).
			dbossas, _, err = ossaDAO.GetAll(
				ctx, tx,
				cdbm.OperatingSystemSiteAssociationFilterInput{
					OperatingSystemIDs: []uuid.UUID{uos.ID},
				},
				cdbp.PageInput{},
				[]string{cdbm.SiteRelationName},
			)
			if err != nil {
				logger.Error().Err(err).Msg("error retrieving Operating System Site associations from DB")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Operating System Site associations from DB", nil)
			}
		} else {
			// Limited or unscoped: load existing associations from the DB.
			dbossas, _, err = ossaDAO.GetAll(
				ctx, tx,
				cdbm.OperatingSystemSiteAssociationFilterInput{
					OperatingSystemIDs: []uuid.UUID{uos.ID},
				},
				cdbp.PageInput{},
				[]string{cdbm.SiteRelationName},
			)
			if err != nil {
				logger.Error().Err(err).Msg("error retrieving Operating System Site associations from DB")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Operating System Site associations from DB", nil)
			}
		}

		for _, dbossa := range dbossas {
			// Templated iPXE OS: verify the referenced template is available at the target site.
			if osType == cdbm.OperatingSystemTypeTemplatedIPXE {
				ipxeTmplDAO := cdbm.NewIpxeTemplateDAO(ush.dbSession)
				_, tmplErr := ipxeTmplDAO.GetBySiteAndName(ctx, nil, dbossa.SiteID, *uos.IpxeTemplateName)
				if tmplErr != nil {
					if errors.Is(tmplErr, cdb.ErrDoesNotExist) {
						logger.Warn().Str("Site ID", dbossa.SiteID.String()).Str("Template", *uos.IpxeTemplateName).
							Msg("iPXE template not available at site, skipping update sync")
						continue
					}
					logger.Error().Err(tmplErr).Str("Site ID", dbossa.SiteID.String()).Msg("error checking iPXE template availability at site")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to verify iPXE template availability at site", nil)
				}
			}

			_, err = ossaDAO.Update(
				ctx,
				tx,
				cdbm.OperatingSystemSiteAssociationUpdateInput{
					OperatingSystemSiteAssociationID: dbossa.ID,
					Status:                           cdb.GetStrPtr(cdbm.OperatingSystemSiteAssociationStatusSyncing),
				},
			)
			if err != nil {
				logger.Error().Err(err).Msg("unable to update the Operating System association record in DB")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Operating System Site Association status, DB error", nil)
			}

			_, serr = sdDAO.CreateFromParams(ctx, tx, dbossa.ID.String(), *cdb.GetStrPtr(cdbm.OperatingSystemSiteAssociationStatusSyncing),
				cdb.GetStrPtr("received Operating System Association update request, syncing"))
			if serr != nil {
				logger.Error().Err(serr).Msg("error creating Status Detail DB entry")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Status Detail for Operating System Site Association", nil)
			}

			updatedOssa, err := ossaDAO.GenerateAndUpdateVersion(ctx, tx, dbossa.ID)
			if err != nil {
				logger.Error().Err(err).Msg("error updating version for updated Operating System Association")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to set version for updated Operating System Site Association, DB error", nil)
			}

			stc, err := ush.scp.GetClientByID(dbossa.SiteID)
			if err != nil {
				logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
			}

			updateOsRequest := &cwssaws.UpdateOperatingSystemRequest{
				Id:                 &cwssaws.OperatingSystemId{Value: uos.ID.String()},
				Name:               &uos.Name,
				Description:        uos.Description,
				IsActive:           &uos.IsActive,
				AllowOverride:      &uos.AllowOverride,
				PhoneHomeEnabled:   &uos.PhoneHomeEnabled,
				UserData:           uos.UserData,
				IpxeScript:         uos.IpxeScript,
				IpxeTemplateName:   uos.IpxeTemplateName,
				IpxeParameters:     &cwssaws.IpxeScriptParameters{Items: dbParamsToProto(uos.IpxeParameters)},
				IpxeArtifacts:      &cwssaws.IpxeScriptArtifacts{Items: dbArtifactsToProto(uos.IpxeArtifacts)},
				IpxeDefinitionHash: uos.IpxeTemplateDefinitionHash,
			}

			workflowOptions := temporalClient.StartWorkflowOptions{
				ID:                       "ipxe-os-update-" + updatedOssa.SiteID.String() + "-" + uos.ID.String() + "-" + *updatedOssa.Version,
				WorkflowExecutionTimeout: cutil.WorkflowExecutionTimeout,
				TaskQueue:                queue.SiteTaskQueue,
			}

			logger.Info().Str("Site ID", dbossa.SiteID.String()).Msg("triggering iPXE Operating System update workflow")

			ctx, cancel := context.WithTimeout(ctx, cutil.WorkflowContextTimeout)
			defer cancel()

			we, err := stc.ExecuteWorkflow(ctx, workflowOptions, "UpdateOperatingSystem", updateOsRequest)
			if err != nil {
				logger.Error().Err(err).Msg("failed to synchronously start Temporal workflow to update Operating System")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed start sync workflow to update Operating System on Site: %s", err), nil)
			}

			wid := we.GetID()
			err = we.Get(ctx, nil)
			if err != nil {
				var timeoutErr *tp.TimeoutError
				if errors.As(err, &timeoutErr) || err == context.DeadlineExceeded || ctx.Err() != nil {
					newctx, newcancel := context.WithTimeout(context.Background(), cutil.WorkflowContextNewAfterTimeout)
					defer newcancel()
					serr := stc.TerminateWorkflow(newctx, wid, "", "timeout occurred executing update Operating System workflow")
					if serr != nil {
						logger.Error().Err(serr).Msg("failed to terminate Temporal workflow for updating Operating System")
						return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to terminate Operating System update workflow after timeout: %s", serr), nil)
					}
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to update Operating System, timeout occurred executing workflow on Site: %s", err), nil)
				}
				code, err := common.UnwrapWorkflowError(err)
				logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to update Operating System")
				return cutil.NewAPIErrorResponse(c, code, fmt.Sprintf("Failed to execute sync workflow to update Operating System on Site: %s", err), nil)
			}
			logger.Info().Str("Workflow ID", wid).Str("Site ID", dbossa.SiteID.String()).Msg("completed synchronous update Operating System workflow")
		}
	}

	// commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error updating OperatingSystem in DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update OperatingSystem", nil)
	}
	txCommitted = true

	// Send response
	apiOperatingSystem := model.NewAPIOperatingSystem(uos, ssds, dbossas, sttsmap)
	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, apiOperatingSystem)
}

// ~~~~~ Delete Handler ~~~~~ //

// DeleteOperatingSystemHandler is the API Handler for deleting a OperatingSystem
type DeleteOperatingSystemHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewDeleteOperatingSystemHandler initializes and returns a new handler for deleting OperatingSystem
func NewDeleteOperatingSystemHandler(dbSession *cdb.Session, tc temporalClient.Client, scp *sc.ClientPool, cfg *config.Config) DeleteOperatingSystemHandler {
	return DeleteOperatingSystemHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        scp,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Delete an existing OperatingSystem
// @Description Delete an existing OperatingSystem
// @Tags OperatingSystem
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param id path string true "ID of OperatingSystem"
// @Success 202
// @Router /v2/org/{org}/carbide/operating-system/{id} [delete]
func (dsh DeleteOperatingSystemHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("OperatingSystem", "Delete", c, dsh.tracerSpan)
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

	// Validate role: Tenant Admin or Provider Admin may delete Operating Systems (with different ownership rules).
	isTenantAdmin := auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole)
	isProviderAdmin := auth.ValidateUserRoles(dbUser, org, nil, auth.ProviderAdminRole)
	if !isTenantAdmin && !isProviderAdmin {
		logger.Warn().Msg("user does not have Tenant Admin or Provider Admin role, access denied")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin or Provider Admin role with org", nil)
	}

	// Get operating system ID from URL param
	osStrID := c.Param("id")

	dsh.tracerSpan.SetAttribute(handlerSpan, attribute.String("operatingsystem_id", osStrID), logger)

	osID, err := uuid.Parse(osStrID)
	if err != nil {
		logger.Warn().Err(err).Msg("error parsing id in url into uuid")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Operating System ID in URL", nil)
	}

	// Validate the tenant for which this OperatingSystem is being updated
	tenant, err := common.GetTenantForOrg(ctx, nil, dsh.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			logger.Warn().Err(err).Msg("Org does not have a Tenant associated")
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Org does not have a Tenant associated", nil)
		}
		logger.Error().Err(err).Msg("unable to retrieve tenant for org")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve tenant for org", nil)
	}

	// Check that operating system exists
	osDAO := cdbm.NewOperatingSystemDAO(dsh.dbSession)
	os, err := osDAO.GetByID(ctx, nil, osID, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Operating System DB entity")
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not retrieve Operating System to delete", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Could not retrieve Operating System to delete", nil)
	}

	// Enforce ownership: both roles are evaluated independently so a dual-role
	// caller is permitted if either role authorizes the operation.
	ownedByTenantD := os.TenantID != nil && *os.TenantID == tenant.ID && os.InfrastructureProviderID == nil
	ownedByProviderD := false
	if isProviderAdmin && os.InfrastructureProviderID != nil {
		ip, iperr := common.GetInfrastructureProviderForOrg(ctx, nil, dsh.dbSession, org)
		if iperr != nil {
			logger.Error().Err(iperr).Msg("error retrieving Infrastructure Provider for org")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Infrastructure Provider for org", nil)
		}
		if *os.InfrastructureProviderID != ip.ID {
			logger.Warn().Msg("provider admin cannot delete operating system owned by a different provider")
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Provider Admin can only delete Operating Systems owned by their own provider", nil)
		}
		ownedByProviderD = true
	}
	allowedByProvider := isProviderAdmin && ownedByProviderD
	allowedByTenant := isTenantAdmin && ownedByTenantD
	if !allowedByProvider && !allowedByTenant {
		if isProviderAdmin && !isTenantAdmin {
			logger.Warn().Msg("provider admin cannot delete tenant-owned operating system")
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Provider Admin can only delete provider-owned Operating Systems", nil)
		}
		if isTenantAdmin && !isProviderAdmin {
			logger.Warn().Msg("tenant admin cannot delete provider-owned operating system")
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant Admin can only delete their own Operating Systems", nil)
		}
		logger.Warn().Msg("user does not have permission to delete this operating system")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Operating System does not belong to your tenant or infrastructure provider", nil)
	}

	// Retrieve site associations for this Operating System (both Image and iPXE types
	// may have associations that need per-site workflow propagation on delete).
	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(dsh.dbSession)
	ossasToDelete, _, err := ossaDAO.GetAll(
		ctx,
		nil,
		cdbm.OperatingSystemSiteAssociationFilterInput{
			OperatingSystemIDs: []uuid.UUID{os.ID},
		},
		cdbp.PageInput{},
		[]string{cdbm.SiteRelationName},
	)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Operating System Site associations from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Operating System Site associations from DB", nil)
	}

	osType := os.Type

	// For image-based OS, verify all associated sites are in Registered state.
	if osType == cdbm.OperatingSystemTypeImage {
		for _, dbosa := range ossasToDelete {
			if dbosa.Site.Status != cdbm.SiteStatusRegistered {
				logger.Warn().Msg(fmt.Sprintf("unable to delete Operating System. Site: %s. is not in Registered state", dbosa.SiteID.String()))
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Failed to delete Operating System, Associated Site: %s is not in Registered state", dbosa.Site.Name), nil)
			}
		}
	}

	// verify no instances are using the os
	isDAO := cdbm.NewInstanceDAO(dsh.dbSession)

	instances, _, err := isDAO.GetAll(ctx, nil, cdbm.InstanceFilterInput{TenantIDs: []uuid.UUID{tenant.ID}, OperatingSystemIDs: []uuid.UUID{os.ID}}, paginator.PageInput{}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Instances for Operating System from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Instances for deleting operatingsystem", nil)
	}

	if len(instances) > 0 {
		logger.Warn().Msg("Instances exist for Operating System, cannot delete it")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Operating System is being used by one or more Instances and cannot be deleted", nil)
	}

	// Start a db tx
	tx, err := cdb.BeginTx(ctx, dsh.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("unable to start transaction")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error deleting Operating System", nil)
	}
	// this variable is used in cleanup actions to indicate if this transaction committed
	txCommitted := false
	defer common.RollbackTx(ctx, tx, &txCommitted)

	// acquire an advisory lock on the Operating System on which there could be contention
	// this lock is released when the transaction commits or rollsback
	err = tx.TryAcquireAdvisoryLock(ctx, cdb.GetAdvisoryLockIDFromString(os.ID.String()), nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to acquire advisory lock on Operating System")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Operating System, could not acquire data store lock on Operating System", nil)
	}

	// Propagate the delete to associated sites (Image via DeleteOsImage, iPXE via DeleteOperatingSystem).
	if len(ossasToDelete) > 0 {
		// Update Operating System to set status to Deleting
		_, err = osDAO.Update(ctx, tx, cdbm.OperatingSystemUpdateInput{OperatingSystemId: os.ID, Status: cdb.GetStrPtr(cdbm.OperatingSystemStatusDeleting)})
		if err != nil {
			logger.Error().Err(err).Msg("error updating Operating System in DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Operating System", nil)
		}

		sdDAO := cdbm.NewStatusDetailDAO(dsh.dbSession)
		_, err = sdDAO.CreateFromParams(ctx, tx, os.ID.String(), cdbm.OperatingSystemStatusDeleting, cdb.GetStrPtr("received request for deletion, pending processing"))
		if err != nil {
			logger.Error().Err(err).Msg("error creating Status Detail DB entry")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Status Detail for Operating System", nil)
		}

		for _, ossa := range ossasToDelete {
			if ossa.Status != cdbm.OperatingSystemSiteAssociationStatusDeleting {
				_, err = ossaDAO.Update(
					ctx,
					tx,
					cdbm.OperatingSystemSiteAssociationUpdateInput{
						OperatingSystemSiteAssociationID: ossa.ID,
						Status:                           cdb.GetStrPtr(cdbm.OperatingSystemSiteAssociationStatusDeleting),
					},
				)
				if err != nil {
					logger.Error().Err(err).Msg("error updating Operating System Association in DB")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Operating Systems", nil)
				}

				_, err = sdDAO.CreateFromParams(ctx, tx, ossa.ID.String(), cdbm.OperatingSystemSiteAssociationStatusDeleting, cdb.GetStrPtr("received request for deletion, pending processing"))
				if err != nil {
					logger.Error().Err(err).Msg("error creating Status Detail DB entry")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to create Status Detail for Operating System Association", nil)
				}

				stc, err := dsh.scp.GetClientByID(ossa.SiteID)
				if err != nil {
					logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
				}

				var workflowName string
				var deleteOsRequest interface{}
				var workflowID string

				if osType == cdbm.OperatingSystemTypeImage {
					workflowName = "DeleteOsImage"
					workflowID = "image-os-delete-" + ossa.SiteID.String() + "-" + os.ID.String() + "-" + *ossa.Version
					deleteOsRequest = &cwssaws.DeleteOsImageRequest{
						Id:                   &cwssaws.UUID{Value: os.ID.String()},
						TenantOrganizationId: tenant.Org,
					}
				} else {
					workflowName = "DeleteOperatingSystem"
					workflowID = "ipxe-os-delete-" + ossa.SiteID.String() + "-" + os.ID.String()
					deleteOsRequest = &cwssaws.DeleteOperatingSystemRequest{
						Id: &cwssaws.OperatingSystemId{Value: os.ID.String()},
					}
				}

				workflowOptions := temporalClient.StartWorkflowOptions{
					ID:        workflowID,
					TaskQueue: queue.SiteTaskQueue,
				}

				logger.Info().Str("Workflow", workflowName).Msg("triggering Operating System delete workflow")

				we, err := stc.ExecuteWorkflow(ctx, workflowOptions, workflowName, deleteOsRequest)
				if err != nil {
					logger.Error().Err(err).Msg("failed to synchronously start Temporal workflow to delete Operating System")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to start sync workflow to delete Operating System on Site: %s", err), nil)
				}

				wid := we.GetID()
				logger.Info().Str("Workflow ID", wid).Msg("executed synchronous delete Operating System workflow")

				err = we.Get(ctx, nil)
				if err != nil {
					var applicationErr *tp.ApplicationError
					if errors.As(err, &applicationErr) && applicationErr.Type() == swe.ErrTypeCarbideObjectNotFound {
						logger.Warn().Msg(swe.ErrTypeCarbideObjectNotFound + " received from Site")
						err = nil
					}
				}

				if err != nil {
					var timeoutErr *tp.TimeoutError
					if errors.As(err, &timeoutErr) || ctx.Err() != nil {
						logger.Error().Err(err).Msg("failed to delete Operating System, timeout occurred executing workflow on Site.")

						newctx, newcancel := context.WithTimeout(context.Background(), cutil.WorkflowContextNewAfterTimeout)
						defer newcancel()

						serr := stc.TerminateWorkflow(newctx, wid, "", "timeout occurred executing delete Operating System workflow")
						if serr != nil {
							logger.Error().Err(serr).Msg("failed to terminate Temporal workflow for deleting Operating System")
							return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to terminate synchronous Operating System deletion workflow after timeout, Cloud and Site data may be de-synced: %s", serr), nil)
						}

						logger.Info().Str("Workflow ID", wid).Msg("initiated terminate synchronous delete Operating System workflow successfully")

						return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to delete Operating System, timeout occurred executing workflow on Site: %s", err), nil)
					}

					code, err := common.UnwrapWorkflowError(err)
					logger.Error().Err(err).Msg("failed to synchronously execute Temporal workflow to delete Operating System")
					return cutil.NewAPIErrorResponse(c, code, fmt.Sprintf("Failed to execute sync workflow to delete Operating System on Site: %s", err), nil)
				}

				logger.Info().Str("Workflow ID", wid).Msg("completed synchronous delete Operating System workflow")
			}
		}
	}

	// Soft-delete the OS if it has no site associations (legacy iPXE, or image-based with
	// associations already cleaned up by the workflows above).
	if len(ossasToDelete) == 0 {
		err = osDAO.Delete(ctx, tx, os.ID)
		if err != nil {
			logger.Error().Msg("error deleting Operating System record in DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error deleting Operating System record in DB", nil)
		}
	}

	// commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing Operating System transaction to DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to delete Operating System", nil)
	}
	// set committed so, deferred cleanup functions will do nothing
	txCommitted = true

	// Create response
	logger.Info().Msg("finishing API handler")
	return c.String(http.StatusAccepted, "Deletion request was accepted")

}

// dbParamsToProto converts DB model iPXE parameters to the proto representation.
func dbParamsToProto(params []cdbm.OperatingSystemIpxeParameter) []*cwssaws.IpxeScriptParameter {
	result := make([]*cwssaws.IpxeScriptParameter, 0, len(params))
	for _, p := range params {
		result = append(result, &cwssaws.IpxeScriptParameter{Name: p.Name, Value: p.Value})
	}
	return result
}

// dbArtifactsToProto converts DB model iPXE artifacts to the proto representation.
// CacheStrategy is stored as the proto enum's string name (e.g. "CACHE_AS_NEEDED").
func dbArtifactsToProto(artifacts []cdbm.OperatingSystemIpxeArtifact) []*cwssaws.IpxeScriptArtifact {
	result := make([]*cwssaws.IpxeScriptArtifact, 0, len(artifacts))
	for _, a := range artifacts {
		strategy := cwssaws.IpxeScriptArtifactCacheStrategy(cwssaws.IpxeScriptArtifactCacheStrategy_value[a.CacheStrategy])
		result = append(result, &cwssaws.IpxeScriptArtifact{
			Name:          a.Name,
			Url:           a.URL,
			Sha:           a.SHA,
			AuthType:      a.AuthType,
			AuthToken:     a.AuthToken,
			CacheStrategy: strategy,
		})
	}
	return result
}

// stripCachedURLFromArtifacts returns a copy with CachedURL cleared.
// cached_url is a core-only field that must never be stored in REST.
func stripCachedURLFromArtifacts(artifacts []cdbm.OperatingSystemIpxeArtifact) []cdbm.OperatingSystemIpxeArtifact {
	for i := range artifacts {
		artifacts[i].CachedURL = nil
	}
	return artifacts
}

// stripCachedURLFromArtifactsPtr is the pointer-to-slice variant for update requests.
func stripCachedURLFromArtifactsPtr(artifacts *[]cdbm.OperatingSystemIpxeArtifact) *[]cdbm.OperatingSystemIpxeArtifact {
	if artifacts == nil {
		return nil
	}
	stripped := stripCachedURLFromArtifacts(*artifacts)
	return &stripped
}
