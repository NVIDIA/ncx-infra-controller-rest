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
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	temporalClient "go.temporal.io/sdk/client"

	"github.com/google/uuid"

	"github.com/labstack/echo/v4"

	cutil "github.com/NVIDIA/ncx-infra-controller-rest/common/pkg/util"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/ipam"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/internal/config"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/handler/util/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/model"
	auth "github.com/NVIDIA/ncx-infra-controller-rest/auth/pkg/authorization"
)

// ~~~~~ Update Handler ~~~~~ //

// UpdateAllocationConstraintHandler is the API Handler for updating a Allocation Constraint
type UpdateAllocationConstraintHandler struct {
	dbSession  *cdb.Session
	tc         temporalClient.Client
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewUpdateAllocationConstraintHandler initializes and returns a new handler for updating Allocation Constraint
func NewUpdateAllocationConstraintHandler(dbSession *cdb.Session, tc temporalClient.Client, cfg *config.Config) UpdateAllocationConstraintHandler {
	return UpdateAllocationConstraintHandler{
		dbSession:  dbSession,
		tc:         tc,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Update an existing Allocation Constraint
// @Description Update an existing Allocation Constraint
// @Tags Allocation
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param allocation_id path string true "ID of Allocation"
// @Param id path string true "ID of Allocation Constraint"
// @Param message body model.APIAllocationConstraintUpdateRequest true "Allocation Constraint update request"
// @Success 200 {object} model.APIAllocationConstraint
// @Router /v2/org/{org}/carbide/allocation/{allocation_id}/constraint/{id} [patch]
func (uach UpdateAllocationConstraintHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("AllocationConstraint", "Update", c, uach.tracerSpan)
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

	// Validate role, currently only Provider Admins can update an Allocation
	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.ProviderAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Provider Admin role, access denied")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Provider Admin role with org", nil)
	}

	// Get allocation ID from URL param
	aStrID := c.Param("allocationId")
	aID, err := uuid.Parse(aStrID)
	if err != nil {
		logger.Warn().Err(err).Msg("error parsing allocation id in url into uuid")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Allocation ID in URL", nil)
	}

	// Get allocationconstraint ID from URL param
	acStrID := c.Param("id")
	acID, err := uuid.Parse(acStrID)
	if err != nil {
		logger.Warn().Err(err).Msg("error parsing id in url into uuid")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid AllocationConstraint ID in URL", nil)
	}

	uach.tracerSpan.SetAttribute(handlerSpan, attribute.String("allocationconstraint_id", acStrID), logger)

	// Validate request
	// Bind request data to API model
	apiRequest := model.APIAllocationConstraintUpdateRequest{}
	err = c.Bind(&apiRequest)
	if err != nil {
		logger.Warn().Err(err).Msg("error binding request data into API model")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data, potentially invalid structure", nil)
	}

	// Validate request attributes
	verr := apiRequest.Validate()
	if verr != nil {
		logger.Warn().Err(verr).Msg("error validating Allocation Constraint update request data")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating Allocation Constraint update request data", verr)
	}

	// Check that AllocationConstraint exists
	acDAO := cdbm.NewAllocationConstraintDAO(uach.dbSession)
	ac, err := acDAO.GetByID(ctx, nil, acID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not retrieve AllocationConstraint to update", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Allocation Constraint DB entity")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Could not retrieve AllocationConstraint to update", nil)
	}

	if ac.AllocationID != aID {
		logger.Warn().Msg("Allocation constraint does not belong to Allocation specified in request")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest,
			"Allocation Constraint does not belong to Allocation specified in request", nil)
	}

	// Check that Allocation exists
	aDAO := cdbm.NewAllocationDAO(uach.dbSession)
	a, err := aDAO.GetByID(ctx, nil, aID, []string{cdbm.SiteRelationName, cdbm.TenantRelationName})
	if err != nil {
		logger.Error().Err(err).Msg("error retrieving Allocation DB entity")
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find Allocation with ID specified in request", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error retrieving Allocation with ID specified in request", nil)
	}

	// Check that the org's infrastructureProvider matches infrastructure provider in allocation
	ip, err := common.GetInfrastructureProviderForOrg(ctx, nil, uach.dbSession, org)
	if err != nil {
		logger.Warn().Err(err).Msg("error retrieving Infrastructure Provider for org")
		return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Error retrieving Infrastructure Provider for org", nil)
	}

	if a.InfrastructureProviderID != ip.ID {
		logger.Warn().Msg("Allocation does not belong to org's Infrastructure Provider")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest,
			"Allocation does not belong to org's Infrastructure Provider", nil)
	}

	updatedac := ac

	var dbit *cdbm.InstanceType
	var dbParentIPBlock *cdbm.IPBlock
	var serr error
	// Check if the new value is same as the existing value
	if ac.ConstraintValue != apiRequest.ConstraintValue {
		// start a database transaction
		tx, err := cdb.BeginTx(ctx, uach.dbSession, &sql.TxOptions{})
		if err != nil {
			logger.Error().Err(err).Msg("failed to start transaction")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update AllocationConstraint", nil)
		}
		txCommitted := false
		defer common.RollbackTx(ctx, tx, &txCommitted)

		ipamStorage := ipam.NewIpamStorage(uach.dbSession.DB, tx.GetBunTx())

		// validate constraint for respective resourcetype have been assigned
		switch ac.ResourceType {
		// validate if tenant has instance based on the allocation
		case cdbm.AllocationResourceTypeInstanceType:
			// Validating Instance type
			dbit, serr = common.GetInstanceTypeFromIDString(ctx, tx, ac.ResourceTypeID.String(), uach.dbSession)
			if serr != nil {
				logger.Warn().Err(serr).Str("Resource ID", ac.ResourceTypeID.String()).Msg("error getting Instance type for Allocation Constraint")
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Error retrieving Instance Type in Allocation Constraint in request", nil)
			}

			// Acquire the shared quota lock for this tenant/site/instance-type pool.
			// This lock is released when the transaction commits or rolls back.
			// We start by coordinating around instance type and tenant.
			// If the constraint update is a decrease, this is all we need to ensure we coordinate around the same
			// lock as Instance creation.
			// If the constraint update is an increase, we'll "upgrade" to coordination around
			// only instance type to match allocation creation and machine/type dissociation.
			serr = common.AcquireInstanceTypeQuotaLock(ctx, tx, a.TenantID, dbit.ID)
			if serr != nil {
				logger.Error().Err(serr).Msg("Failed to acquire advisory lock on Instance Type quota pool")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Allocation Constraint, DB error", nil)
			}

			// Get the current tenant's allocation IDs for the allocation site.
			// We'll use them to scope the aggregate capacity calculation.
			allocationIDs, serr := common.GetAllocationIDsForTenantAtSite(ctx, tx, uach.dbSession, a.InfrastructureProviderID, a.TenantID, a.SiteID)
			if serr != nil {
				logger.Error().Err(serr).Msg("error getting Allocation IDs for Tenant at Site")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Allocation IDs for Tenant at Site", nil)
			}

			// Get all matching constraints for the tenant/site aggregate pool.
			allocConstraints, _, serr := acDAO.GetAll(
				ctx,
				tx,
				allocationIDs,
				cdb.GetStrPtr(cdbm.AllocationResourceTypeInstanceType),
				[]uuid.UUID{dbit.ID},
				cdb.GetStrPtr(cdbm.AllocationConstraintTypeReserved),
				nil,
				nil,
				nil,
				cdb.GetIntPtr(paginator.TotalLimit),
				nil,
			)
			if serr != nil {
				logger.Error().Err(serr).Msg("error getting AllocationConstraints from db for InstanceType")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve AllocationConstraints for InstanceType", nil)
			}

			// Sum up all the constraints so we can see what effect changes to the current
			// allocation constraint will have on the total pool.
			sumConstraints := apiRequest.ConstraintValue
			for _, constraint := range allocConstraints {
				// We exclude the constraint of the request because that's
				// what's being changed, so the current value in the DB isn't
				// relevant, and we've already added the requested new value
				// into the sum.
				if constraint.ID != ac.ID {
					sumConstraints += constraint.ConstraintValue
				}
			}

			// If the request is reducing the constraint value, make sure the total
			// pool does not fall below what has already been allocated.
			if apiRequest.ConstraintValue < ac.ConstraintValue {
				// Validate if any instances exist for this instance type.
				inDAO := cdbm.NewInstanceDAO(uach.dbSession)
				_, instanceCount, serr := inDAO.GetAll(ctx, tx, cdbm.InstanceFilterInput{
					TenantIDs:       []uuid.UUID{a.TenantID},
					SiteIDs:         []uuid.UUID{a.SiteID},
					InstanceTypeIDs: []uuid.UUID{dbit.ID},
				}, paginator.PageInput{}, nil)
				if serr != nil {
					logger.Error().Err(serr).Msg("error getting Instances from db for AllocationConstraint")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Instances for AllocationConstraint", nil)
				}

				if instanceCount > sumConstraints {
					logger.Warn().Msg("updating this Allocation Constraint as requested would leave the tenant pool below the active instance count for the instance type")
					return cutil.NewAPIErrorResponse(
						c,
						http.StatusBadRequest,
						fmt.Sprintf(
							"Updating this Allocation Constraint as specified would result in %d total Machines for Instance Type: %s allocated to Tenant, less than Tenant's active Instance count: %d for the Instance Type",
							sumConstraints,
							dbit.Name,
							instanceCount,
						),
						nil,
					)
				}
			} else if ac.ConstraintType == cdbm.AllocationConstraintTypeReserved && apiRequest.ConstraintValue > ac.ConstraintValue {
				// If the new value being requested is greater than the current one,
				// check whether there are enough machines to support the increased pool size.
				// We need to "upgrade" our lock to coordinate around only InstanceType here.
				serr = tx.TryAcquireAdvisoryLock(ctx, cdb.GetAdvisoryLockIDFromString(dbit.ID.String()), nil)
				if serr != nil {
					logger.Error().Err(serr).Msg("Failed to acquire advisory lock on Instance Type quota pool")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update Allocation Constraint, DB error", nil)
				}

				ok, serr := common.CheckMachinesForInstanceTypeAllocation(ctx, tx, uach.dbSession, logger, dbit.ID, apiRequest.ConstraintValue-ac.ConstraintValue)
				if serr != nil {
					logger.Error().Err(serr).Str("resourceId", ac.ResourceTypeID.String()).Msg("error checking available machines for instance type allocation")
					return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error checking Machine availability for the Instance Type Allocation", nil)
				}
				if !ok {
					logger.Warn().Str("resourceId", ac.ResourceTypeID.String()).Msg("machines unavailable for instance type allocation")
					return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "New constraint value cannot be satisfied due to Machine availability", nil)
				}
			}

		// validate if tenant has subnet based on IPBlock
		case cdbm.AllocationResourceTypeIPBlock:
			if ac.DerivedResourceID == nil {
				logger.Error().Msg("allocation constraint does not have a derived resource id")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "IP Block constraint is missing derived resource ID, potentially inconsistent data", nil)
			}

			// get parent IPBlock
			ipbDAO := cdbm.NewIPBlockDAO(uach.dbSession)
			dbParentIPBlock, serr := ipbDAO.GetByID(ctx, tx, ac.ResourceTypeID, nil)
			if serr != nil {
				logger.Error().Err(serr).Msg("error getting ipblock corresponding to allocation constraint")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error retrieving IP Block for Allocation Constraint", nil)
			}

			if apiRequest.ConstraintValue < dbParentIPBlock.PrefixLength {
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "New constraint value cannot be less that source IP Block size", nil)
			}

			// get childIPBlock
			existingChildIPBlock, serr := ipbDAO.GetByID(ctx, tx, *ac.DerivedResourceID, nil)
			if serr != nil {
				logger.Error().Err(serr).Msg("error getting child ipblock corresponding to allocation constraint")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error retrieving IP Block for Allocation Constraint", nil)
			}

			subnetFilter := cdbm.SubnetFilterInput{
				TenantIDs: []uuid.UUID{a.TenantID},
			}

			var ipv4IPBlockID *uuid.UUID
			var ipv6IPBlockID *uuid.UUID
			switch existingChildIPBlock.ProtocolVersion {
			case cdbm.IPBlockProtocolVersionV4:
				ipv4IPBlockID = &existingChildIPBlock.ID
				subnetFilter.IPv4BlockIDs = []uuid.UUID{*ipv4IPBlockID}
			case cdbm.IPBlockProtocolVersionV6:
				ipv6IPBlockID = &existingChildIPBlock.ID
				subnetFilter.IPv6BlockIDs = []uuid.UUID{*ipv6IPBlockID}
			}

			// acquire an advisory lock on the Tenant and DerivedIPBlock ID on which subnets are being created
			// this lock is released when the transaction commits or rollsback
			serr = tx.TryAcquireAdvisoryLock(ctx, cdb.GetAdvisoryLockIDFromString(fmt.Sprintf("%s-%s", a.TenantID.String(), existingChildIPBlock.ID.String())), nil)
			if serr != nil {
				logger.Error().Err(serr).Msg("Failed to acquire advisory lock on IP Block")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error udating allocation constraint", nil)
			}

			// check if the tenant has subnets using this ipblock
			sDAO := cdbm.NewSubnetDAO(uach.dbSession)
			_, sCount, serr := sDAO.GetAll(ctx, tx, subnetFilter, paginator.PageInput{}, []string{})
			if serr != nil {
				logger.Error().Err(serr).Msg("error getting subnets corresponding to allocation constraint")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Error retrieving Subnets for allocation constraint", nil)
			}
			if sCount > 0 {
				logger.Warn().Msg("subnets present for allocation constraint, cannot update allocation constraint")
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Subnets exist for allocation constraint in allocation", nil)
			}

			// We must delete or cleanup the existing child prefix IPAM entry when we successfully update the constraint value by
			// creating new child prefix entry in IPAM
			existingChildCidr := ipam.GetCidrForIPBlock(ctx, existingChildIPBlock.Prefix, existingChildIPBlock.PrefixLength)
			err = ipam.DeleteChildIpamEntryFromCidr(ctx, tx, uach.dbSession, ipamStorage, dbParentIPBlock, existingChildCidr)
			if err != nil {
				logger.Error().Err(err).Msg("unable to delete child ipam entry for updated allocation constraint")
				if !errors.Is(err, ipam.ErrPrefixDoesNotExistForIPBlock) {
					return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Could not delete child IPAM entry from IPBlock for updated Allocation Constraint. Details: %s", err.Error()), nil)
				}
			}

			// Allocate a child prefix in ipam for updated constraint value
			newChildPrefix, serr := ipam.CreateChildIpamEntryForIPBlock(ctx, tx, uach.dbSession, ipamStorage, dbParentIPBlock, apiRequest.ConstraintValue)
			if serr != nil {
				// printing parent prefix usage to debug the child prefix failure
				parentPrefix, sserr := ipamStorage.ReadPrefix(ctx, dbParentIPBlock.Prefix, ipam.GetIpamNamespaceForIPBlock(ctx, dbParentIPBlock.RoutingType, dbParentIPBlock.InfrastructureProviderID.String(), dbParentIPBlock.SiteID.String()))
				if sserr == nil {
					logger.Info().Str("IP Block ID", dbParentIPBlock.ID.String()).Str("IP Block Prefix", dbParentIPBlock.Prefix).Msgf("%+v\n", parentPrefix.Usage())
				}

				logger.Warn().Err(serr).Msg("unable to create child ipam entry for updated allocation constraint")
				return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Could not create child IPAM entry for updated Allocation Constraint. Details: %s", serr.Error()), nil)
			}
			logger.Info().Str("Child CIDR", newChildPrefix.Cidr).Msg("created child CIDR")

			// Create an IP Block corresponding to the child prefix
			newprefix, newblockSize, serr := ipam.ParseCidrIntoPrefixAndBlockSize(newChildPrefix.Cidr)
			if serr != nil {
				logger.Error().Err(serr).Msg("unable to create child ipam entry for updated allocation constraint")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Could not create IPAM entry for updated Allocation Constraint. Details: %s", serr.Error()), nil)
			}

			// Update existind IP Block with new prefix, block size
			_, serr = ipbDAO.Update(
				ctx,
				tx,
				cdbm.IPBlockUpdateInput{
					IPBlockID:    existingChildIPBlock.ID,
					Prefix:       cdb.GetStrPtr(newprefix),
					PrefixLength: cdb.GetIntPtr(newblockSize),
				},
			)
			if serr != nil {
				logger.Error().Err(serr).Msg("unable to update child ip block entry for allocation constraint")
				return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed updating ipblock entry for Allocation Constraint", nil)
			}
		}

		updatedac, err = acDAO.UpdateFromParams(ctx, tx, ac.ID, nil, nil, nil, nil, cdb.GetIntPtr(apiRequest.ConstraintValue), nil)
		if err != nil {
			logger.Error().Err(err).Msg("error updating AllocationConstraint in DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update AllocationConstraint", nil)
		}

		err = tx.Commit()
		if err != nil {
			logger.Error().Err(err).Msg("error updating AllocationConstraint in DB")
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to update AllocationConstraint", nil)
		}
		txCommitted = true
	}

	// Create response
	apiac := model.NewAPIAllocationConstraint(updatedac, dbit, dbParentIPBlock)
	logger.Info().Msg("finishing API handler")
	return c.JSON(http.StatusOK, apiac)
}
