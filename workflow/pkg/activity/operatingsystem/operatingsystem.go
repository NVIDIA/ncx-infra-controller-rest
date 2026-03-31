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

package operatingsystem

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	cdbp "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"

	sc "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/client/site"

	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"

	cwutil "github.com/NVIDIA/ncx-infra-controller-rest/common/pkg/util"
)

const (
	MsgOsImageSynced = "Operating System successfully synced to Site"
)

var (
	// ControllerOsImageStatusMap is a list of valid status for the Controller Os Image
	ControllerOsImageStatusMap = map[cwssaws.OsImageStatus]bool{
		cwssaws.OsImageStatus_ImageInProgress:    true,
		cwssaws.OsImageStatus_ImageUninitialized: true,
		cwssaws.OsImageStatus_ImageDisabled:      true,
		cwssaws.OsImageStatus_ImageReady:         true,
		cwssaws.OsImageStatus_ImageFailed:        true,
	}
)

// ManageOsImage is an activity wrapper for managing Image based OS lifecycle for a Site and allows
// injecting DB access
type ManageOsImage struct {
	dbSession      *cdb.Session
	siteClientPool *sc.ClientPool
}

// Activity functions

// UpdateOsImagesInDB takes information pushed by Site Agent for a collection of image based OSs associated with the Site and updates the DB
func (mskg ManageOsImage) UpdateOsImagesInDB(ctx context.Context, siteID uuid.UUID, osImageInventory *cwssaws.OsImageInventory) ([]uuid.UUID, error) {
	logger := log.With().Str("Activity", "UpdateOsImagesInDB").Str("Site ID", siteID.String()).Logger()

	logger.Info().Msg("starting activity")

	stDAO := cdbm.NewSiteDAO(mskg.dbSession)

	site, err := stDAO.GetByID(ctx, nil, siteID, nil, false)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			logger.Warn().Err(err).Msg("received Os Image inventory for unknown or deleted Site")
		} else {
			logger.Error().Err(err).Msg("failed to retrieve Site from DB")
		}
		return nil, err
	}

	if osImageInventory.InventoryStatus == cwssaws.InventoryStatus_INVENTORY_STATUS_FAILED {
		logger.Warn().Msg("received failed inventory status from Site Agent, skipping inventory processing")
		return nil, nil
	}

	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mskg.dbSession)

	existingOssas, _, err := ossaDAO.GetAll(
		ctx,
		nil,
		cdbm.OperatingSystemSiteAssociationFilterInput{
			SiteIDs: []uuid.UUID{site.ID},
		},
		cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)},
		[]string{cdbm.OperatingSystemRelationName},
	)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get OS Image Site Associations for Site from DB")
		return nil, err
	}

	// Construct a map ID of Operating System Site Association to Operating System
	existingOsImageMap := make(map[string]*cdbm.OperatingSystemSiteAssociation)
	for _, ossa := range existingOssas {
		curossa := ossa
		existingOsImageMap[ossa.OperatingSystemID.String()] = &curossa
	}

	reportedOsImageIDMap := map[uuid.UUID]bool{}

	if osImageInventory.InventoryPage != nil {
		logger.Info().Msgf("Received OS Image inventory page: %d of %d, page size: %d, total count: %d",
			osImageInventory.InventoryPage.CurrentPage, osImageInventory.InventoryPage.TotalPages,
			osImageInventory.InventoryPage.PageSize, osImageInventory.InventoryPage.TotalItems)

		for _, strId := range osImageInventory.InventoryPage.ItemIds {
			id, serr := uuid.Parse(strId)
			if serr != nil {
				logger.Error().Err(serr).Str("ID", strId).Msg("failed to parse OS Image ID from inventory page")
				continue
			}
			reportedOsImageIDMap[id] = true
		}
	}

	updatedOperatingSystemMap := map[uuid.UUID]bool{}

	// Iterate through OS Image Inventory and update DB
	for _, controllerOsImage := range osImageInventory.OsImages {
		if controllerOsImage != nil && controllerOsImage.Attributes != nil {

			osImageIDStr := controllerOsImage.Attributes.Id.GetValue()
			slogger := logger.With().Str("OS Image ID", osImageIDStr).Logger()

			ossa, ok := existingOsImageMap[osImageIDStr]
			if !ok {
				slogger.Error().Str("OS Image ID", controllerOsImage.Attributes.Id.Value).Msg("OS Image Site Association does not have a record in DB, possibly created directly on Site")
				continue
			}

			reportedOsImageIDMap[ossa.OperatingSystemID] = true

			// Reset missing flag if necessary
			if ossa.IsMissingOnSite {
				// Update Operating System Site Association missing flag as it is now found on Site
				_, serr := ossaDAO.Update(
					ctx,
					nil,
					cdbm.OperatingSystemSiteAssociationUpdateInput{
						OperatingSystemSiteAssociationID: ossa.ID,
						IsMissingOnSite:                  cdb.GetBoolPtr(false),
					},
				)
				if serr != nil {
					slogger.Error().Err(serr).Msg("failed to update OS Image Site Association missing flag in DB")
					continue
				}
			}

			if ossa.Status == cdbm.OperatingSystemSiteAssociationStatusDeleting {
				continue
			}

			// Update Operating System Site Association status if necessary
			ossaStatus := cdbm.OperatingSystemSiteAssociationStatusSyncing
			ossaStatusMessage := controllerOsImage.StatusMessage

			ok = ControllerOsImageStatusMap[controllerOsImage.Status]
			if !ok {
				slogger.Error().Str("OS Image ID", controllerOsImage.Attributes.Id.Value).Str("OS Image Status", controllerOsImage.Status.String()).Msg("received unknown OS Image status from Site Agent")
			}

			switch controllerOsImage.Status {
			case cwssaws.OsImageStatus_ImageInProgress, cwssaws.OsImageStatus_ImageUninitialized, cwssaws.OsImageStatus_ImageDisabled:
				ossaStatusMessage = cdb.GetStrPtr("OS Image is still syncing")
			case cwssaws.OsImageStatus_ImageReady:
				ossaStatus = cdbm.OperatingSystemSiteAssociationStatusSynced
				ossaStatusMessage = cdb.GetStrPtr("OS Image is ready to use")
			case cwssaws.OsImageStatus_ImageFailed:
				ossaStatus = cdbm.OperatingSystemSiteAssociationStatusError
				if ossaStatusMessage == nil || *ossaStatusMessage == "" {
					ossaStatusMessage = cdb.GetStrPtr("OS Image failed to sync on Site")
				}
			}

			// if determined status is different that current
			// only that case update
			if ossaStatus != ossa.Status {
				serr := mskg.updateOperatingSystemSiteAssociationStatusInDB(ctx, nil, ossa.ID, cdb.GetStrPtr(ossaStatus), ossaStatusMessage)
				if serr != nil {
					slogger.Error().Err(err).Msg("failed to update OS Image Site Association status detail in DB")
				}
				updatedOperatingSystemMap[ossa.OperatingSystemID] = true
			}
		}
	}

	// Populate list of ossas that were not found
	ossasToDelete := []*cdbm.OperatingSystemSiteAssociation{}

	// If inventory paging is enabled, we only need to do this once and we do it on the last page
	if osImageInventory.InventoryPage == nil || osImageInventory.InventoryPage.TotalPages == 0 || (osImageInventory.InventoryPage.CurrentPage == osImageInventory.InventoryPage.TotalPages) {
		for _, ossa := range existingOsImageMap {
			found := false
			_, found = reportedOsImageIDMap[ossa.OperatingSystemID]
			if !found || ossa.Status == cdbm.OperatingSystemSiteAssociationStatusDeleting {
				// The OS Image was not found in the Os Image Inventory, so add it to list of OS Image to potentially delete
				ossasToDelete = append(ossasToDelete, ossa)
			}
		}
	}

	// Process all Operating Site Associations in DB
	for _, ossa := range ossasToDelete {
		slogger := logger.With().Str("OS Image Site Association ID", ossa.ID.String()).Logger()

		// Operating System was not found on Site
		if ossa.Status == cdbm.OperatingSystemSiteAssociationStatusDeleting {
			// If the OperatingSystemSiteAssociation was being deleted, we can proceed with removing it from the DB
			serr := ossaDAO.Delete(ctx, nil, ossa.ID)
			if serr != nil {
				slogger.Error().Err(serr).Msg("failed to delete Operating System Site Association from DB")
				continue
			}
			// Trigger re-evaluation of Operating System status (delete if no association exists)
			serr = mskg.UpdateOperatingSystemStatusInDB(ctx, ossa.OperatingSystemID)
			if serr != nil {
				slogger.Error().Err(err).Msg("failed to trigger Operating System status update in DB")
			}
		} else {
			// Was this created within inventory receipt interval? If so, we may be processing an older inventory
			if time.Since(ossa.Created) < cwutil.InventoryReceiptInterval {
				continue
			}

			// Set isMissingOnSite flag to true and update status, user can decide on deletion
			_, serr := ossaDAO.Update(
				ctx,
				nil,
				cdbm.OperatingSystemSiteAssociationUpdateInput{
					OperatingSystemSiteAssociationID: ossa.ID,
					IsMissingOnSite:                  cdb.GetBoolPtr(true),
				},
			)
			if serr != nil {
				slogger.Error().Err(serr).Msg("failed to set missing on Site flag in DB for Operating System Site Association")
				continue
			}

			serr = mskg.updateOperatingSystemSiteAssociationStatusInDB(ctx, nil, ossa.ID, cdb.GetStrPtr(cdbm.OperatingSystemSiteAssociationStatusError), cdb.GetStrPtr("Operating System is missing on Site"))
			if serr != nil {
				slogger.Error().Err(err).Msg("failed to update Operating System Site Association status detail in DB")
			}

			updatedOperatingSystemMap[ossa.OperatingSystemID] = true
		}
	}

	updatedOsIDs := []uuid.UUID{}
	for osID := range updatedOperatingSystemMap {
		updatedOsIDs = append(updatedOsIDs, osID)
	}

	return updatedOsIDs, nil
}

// updateOperatingSystemSiteAssociationStatusInDB is helper function to write OperatingSystemSiteAssociation updates to DB
func (mskg ManageOsImage) updateOperatingSystemSiteAssociationStatusInDB(ctx context.Context, tx *cdb.Tx, ossaID uuid.UUID, status *string, statusMessage *string) error {
	if status != nil {
		ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mskg.dbSession)

		_, err := ossaDAO.Update(
			ctx,
			tx,
			cdbm.OperatingSystemSiteAssociationUpdateInput{
				OperatingSystemSiteAssociationID: ossaID,
				Status:                           status,
			},
		)
		if err != nil {
			return err
		}

		statusDetailDAO := cdbm.NewStatusDetailDAO(mskg.dbSession)
		_, err = statusDetailDAO.CreateFromParams(ctx, tx, ossaID.String(), *status, statusMessage)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateOperatingSystemStatusInDB is helper function to write Operating System updates to DB
func (mskg ManageOsImage) UpdateOperatingSystemStatusInDB(ctx context.Context, osID uuid.UUID) error {
	logger := log.With().Str("Activity", "UpdateOperatingSystemStatusInDB").Str("Operating System ID", osID.String()).Logger()

	logger.Info().Msg("starting activity")

	osDAO := cdbm.NewOperatingSystemDAO(mskg.dbSession)

	os, err := osDAO.GetByID(ctx, nil, osID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			logger.Warn().Err(err).Msg("received request for unknown or deleted Operating System")
		} else {
			logger.Error().Err(err).Msg("failed to retrieve Operating System from DB")
		}
		return nil
	}

	logger.Info().Msg("retrieved Operating System from DB")

	var osStatus *string
	var osMessage *string

	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mskg.dbSession)
	ossas, ossaTotal, err := ossaDAO.GetAll(
		ctx,
		nil,
		cdbm.OperatingSystemSiteAssociationFilterInput{
			OperatingSystemIDs: []uuid.UUID{osID},
		},
		cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)},
		nil,
	)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get Operating System Site Associations from DB for Operating System")
		return err
	}

	// Operating System is in deleting state
	if os.Status == cdbm.OperatingSystemStatusDeleting {
		if ossaTotal == 0 {
			// Start a db tx
			tx, err := cdb.BeginTx(ctx, mskg.dbSession, &sql.TxOptions{})
			if err != nil {
				logger.Error().Err(err).Msg("failed to start transaction")
				return err
			}

			// No more associations left, we can delete the Operating System
			serr := osDAO.Delete(ctx, tx, osID)
			if serr != nil {
				logger.Error().Err(serr).Msg("failed to delete Operating System from DB")
				terr := tx.Rollback()
				if terr != nil {
					logger.Error().Err(terr).Msg("failed to rollback transaction")
				}
				return serr
			}

			// Commit transaction
			err = tx.Commit()
			if err != nil {
				logger.Error().Err(err).Msg("error committing transaction to DB")
				return err
			}
		}

		// One or more associations left to delete from Sites
		return nil
	}

	if ossaTotal == 0 {
		if os.Status == cdbm.OperatingSystemStatusReady {
			return nil
		}
		osStatus = cdb.GetStrPtr(cdbm.OperatingSystemStatusReady)
		osMessage = cdb.GetStrPtr("Operating System successfully synced to all Sites")
	} else {
		statusCountMap := map[string]int{}
		for _, dbossa := range ossas {
			statusCountMap[dbossa.Status]++
		}

		if statusCountMap[cdbm.OperatingSystemSiteAssociationStatusError] > 0 {
			if os.Status == cdbm.OperatingSystemStatusError {
				return nil
			}
			osStatus = cdb.GetStrPtr(cdbm.OperatingSystemStatusError)
			osMessage = cdb.GetStrPtr("Failed to sync Operating System to one or more Sites")
		} else if statusCountMap[cdbm.OperatingSystemSiteAssociationStatusSyncing] > 0 {
			if os.Status == cdbm.OperatingSystemStatusSyncing {
				return nil
			}
			osStatus = cdb.GetStrPtr(cdbm.OperatingSystemStatusSyncing)
			osMessage = cdb.GetStrPtr("Operating System syncing to one or more Sites")
		} else {
			if os.Status == cdbm.OperatingSystemStatusReady {
				return nil
			}
			osStatus = cdb.GetStrPtr(cdbm.OperatingSystemStatusReady)
			osMessage = cdb.GetStrPtr("Operating System successfully synced to all Sites")
		}
	}

	// Update status
	_, err = osDAO.Update(
		ctx,
		nil,
		cdbm.OperatingSystemUpdateInput{
			OperatingSystemId: osID,
			Status:            osStatus,
		},
	)
	if err != nil {
		return err
	}

	statusDetailDAO := cdbm.NewStatusDetailDAO(mskg.dbSession)
	_, err = statusDetailDAO.CreateFromParams(ctx, nil, osID.String(), *osStatus, osMessage)
	if err != nil {
		return err
	}

	logger.Info().Msg("successfully completed activity")

	return nil
}

// NewManageOsImage returns a new ManageOsImage activity
func NewManageOsImage(dbSession *cdb.Session, siteClientPool *sc.ClientPool) ManageOsImage {
	return ManageOsImage{
		dbSession:      dbSession,
		siteClientPool: siteClientPool,
	}
}

// ManageOperatingSystemSync is an activity wrapper for managing carbide-core Operating System inventory.
// carbide-core and carbide-rest share the same UUID as the primary key (os.id = carbide-core id).
// Conflict resolution is timestamp-based: the side with the most recent updated (or deleted)
// timestamp is the source of truth.
type ManageOperatingSystemSync struct {
	dbSession      *cdb.Session
	siteClientPool *sc.ClientPool
}

// UpdateOperatingSystemsInDB reconciles the operating_system table for a Site based on Operating Systems
// reported by carbide-core via the inventory workflow.
func (mod ManageOperatingSystemSync) UpdateOperatingSystemsInDB(ctx context.Context, siteID uuid.UUID, inventory *cwssaws.OperatingSystemInventory) error {
	logger := log.With().Str("Activity", "UpdateOperatingSystemsInDB").Str("Site ID", siteID.String()).Logger()
	logger.Info().Msg("Starting activity")

	if inventory == nil {
		return errors.New("UpdateOperatingSystemsInDB called with nil inventory")
	}
	if inventory.InventoryStatus == cwssaws.InventoryStatus_INVENTORY_STATUS_FAILED {
		logger.Warn().Msg("Received failed inventory status from Site Agent, skipping")
		return nil
	}

	stDAO := cdbm.NewSiteDAO(mod.dbSession)
	site, err := stDAO.GetByID(ctx, nil, siteID, nil, false)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			logger.Warn().Err(err).Msg("Received inventory for unknown or deleted Site")
		} else {
			logger.Error().Err(err).Msg("Failed to retrieve Site from DB")
		}
		return err
	}

	// OSes that originate in carbide-core are owned by the infrastructure provider, not by
	// any individual tenant. We tag them with the site's InfrastructureProviderID so that
	// ProviderAdmin can update them and all tenants of that provider can read them.
	siteProviderID := site.InfrastructureProviderID
	logger.Debug().Str("infrastructure_provider_id", siteProviderID.String()).Msg("Resolved infrastructure provider from site")

	// Resolve the carbide-rest org for this site (best-effort, used for display only).
	// carbide-core's TenantOrganizationId may use a different name than carbide-rest, so we
	// look up the TenantSite association instead of trusting the reported org field.
	tsDAO := cdbm.NewTenantSiteDAO(mod.dbSession)
	tenantSites, _, tsErr := tsDAO.GetAll(ctx, nil, cdbm.TenantSiteFilterInput{
		SiteIDs: []uuid.UUID{siteID},
	}, cdbp.PageInput{Limit: cdb.GetIntPtr(1)}, nil)

	var localOrg string
	if tsErr == nil && len(tenantSites) > 0 {
		localOrg = tenantSites[0].TenantOrg
		logger.Debug().Str("org", localOrg).Msg("Resolved org from TenantSite")
	}

	osDAO := cdbm.NewOperatingSystemDAO(mod.dbSession)

	// Collect the UUIDs of all reported OS records (active only — the new Find APIs do not
	// return deleted records). carbide-core and carbide-rest share the same UUID as PK.
	reportedIDSet := make(map[uuid.UUID]struct{}, len(inventory.GetOperatingSystems()))
	reportedIDList := make([]uuid.UUID, 0, len(inventory.GetOperatingSystems()))
	for _, reported := range inventory.GetOperatingSystems() {
		if reported == nil || reported.GetId().GetValue() == "" {
			continue
		}
		id, parseErr := uuid.Parse(reported.GetId().GetValue())
		if parseErr != nil {
			logger.Error().Err(parseErr).Str("ID", reported.GetId().GetValue()).Msg("Received OS with invalid UUID, skipping")
			continue
		}
		if _, dup := reportedIDSet[id]; !dup {
			reportedIDSet[id] = struct{}{}
			reportedIDList = append(reportedIDList, id)
		}
	}

	// Fetch DB records matching the reported IDs (including soft-deleted so we can detect
	// the case where carbide-rest already deleted an OS that carbide-core still reports active).
	existingOSes, _, err := osDAO.GetAll(ctx, nil, cdbm.OperatingSystemFilterInput{
		OperatingSystemIds: reportedIDList,
		IncludeDeleted:     true,
	}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get Operating Systems from DB")
		return err
	}

	existingByID := map[uuid.UUID]*cdbm.OperatingSystem{}
	for i := range existingOSes {
		existingByID[existingOSes[i].ID] = &existingOSes[i]
	}

	// Track global/limited OS IDs that need aggregate status recomputation.
	globalLimitedOSIDs := map[uuid.UUID]struct{}{}

	// Create or update OSes based on the carbide-core inventory.
	for _, reported := range inventory.GetOperatingSystems() {
		if reported == nil || reported.GetId().GetValue() == "" {
			continue
		}

		osID, parseErr := uuid.Parse(reported.GetId().GetValue())
		if parseErr != nil {
			continue
		}

		coreUpdated, _ := time.Parse(time.RFC3339, reported.Updated)
		params := protoParamsToModel(reported.IpxeParameters)
		artifacts := protoArtifactsToModel(reported.IpxeArtifacts)
		osType := osTypeToString(reported.Type)

		cur, found := existingByID[osID]
		if !found {
			// New OS from carbide-core: create it with the site's InfrastructureProviderID.
			// OSes originating in carbide-core are provider-owned (not tenant-owned): ProviderAdmin
			// can update them and all tenants of the provider can read them via the OR filter.
			if _, cerr := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
				ID:                       osID,
				Name:                     reported.Name,
				Org:                      localOrg,
				TenantID:                 nil,
				InfrastructureProviderID: &siteProviderID,
				OsType:                   osType,
				Description:              reported.Description,
				UserData:                 reported.UserData,
				IpxeScript:               reported.IpxeScript,
				AllowOverride:            reported.AllowOverride,
				PhoneHomeEnabled:         reported.PhoneHomeEnabled,
				IpxeTemplateName:         reported.IpxeTemplateName,
				IpxeParameters:           params,
				IpxeArtifacts:            artifacts,
				DefinitionHash:           reported.IpxeDefinitionHash,
				Status:                   tenantStateToRestStatus(reported.Status),
			}); cerr != nil {
				logger.Error().Err(cerr).Str("ID", reported.GetId().GetValue()).Msg("Failed to create Operating System in DB")
			} else if !reported.IsActive {
				// bun ORM hardcodes is_active=true on INSERT; correct it with an update.
				inactive := false
				if _, uerr := osDAO.Update(ctx, nil, cdbm.OperatingSystemUpdateInput{
					OperatingSystemId: osID,
					IsActive:          &inactive,
				}); uerr != nil {
					logger.Error().Err(uerr).Str("ID", reported.GetId().GetValue()).Msg("Failed to set is_active=false after create")
				}
			}
			continue
		}

		// carbide-rest has already soft-deleted this OS (user-initiated via subscriber workflow).
		// Do not restore it even if carbide-core still reports it as active (the delete push
		// to carbide-core may be in-flight).
		if cur.Deleted != nil {
			continue
		}

		// For global- or limited-scoped OSes, carbide-rest is the source of truth
		// for the OS definition. However, we still need to record the per-site
		// operational status (CoreStatus) on the OSSA and aggregate the overall
		// OS status from all site statuses.
		if cur.Scope != nil && *cur.Scope != cdbm.OperatingSystemScopeLocal {
			coreStatus := tenantStateToRestStatus(reported.Status)
			ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mod.dbSession)
			ossa, ossaErr := ossaDAO.GetByOperatingSystemIDAndSiteID(ctx, nil, osID, siteID, nil)
			if ossaErr == nil && ossa != nil {
				if _, uerr := ossaDAO.Update(ctx, nil, cdbm.OperatingSystemSiteAssociationUpdateInput{
					OperatingSystemSiteAssociationID: ossa.ID,
					CoreStatus:                       &coreStatus,
				}); uerr != nil {
					logger.Error().Err(uerr).Str("osID", osID.String()).Msg("Failed to update OSSA core_status")
				} else {
					globalLimitedOSIDs[osID] = struct{}{}
				}
			}
			continue
		}

		// Both sides are active — update carbide-rest only when carbide-core is newer.
		// Backfill: older records may have been created with tenant_id set and no
		// infrastructure_provider_id (before this ownership model was established).
		needsProviderBackfill := cur.InfrastructureProviderID == nil
		needsOrgBackfill := cur.Org == "" && localOrg != ""
		if coreUpdated.After(cur.Updated) || needsProviderBackfill || needsOrgBackfill {
			coreStatus := tenantStateToRestStatus(reported.Status)
			updateInput := cdbm.OperatingSystemUpdateInput{
				OperatingSystemId:        cur.ID,
				Name:                     &reported.Name,
				Org:                      &localOrg,
				TenantID:                 nil,
				InfrastructureProviderID: &siteProviderID,
				OsType:                   &osType,
				Description:              reported.Description,
				UserData:                 reported.UserData,
				IpxeScript:               reported.IpxeScript,
				AllowOverride:            &reported.AllowOverride,
				PhoneHomeEnabled:         &reported.PhoneHomeEnabled,
				IsActive:                 &reported.IsActive,
				IpxeTemplateName:         reported.IpxeTemplateName,
				IpxeParameters:           &params,
				IpxeArtifacts:            &artifacts,
				DefinitionHash:           reported.IpxeDefinitionHash,
				Status:                   &coreStatus,
			}
			if _, uerr := osDAO.Update(ctx, nil, updateInput); uerr != nil {
				logger.Error().Err(uerr).Str("ID", reported.GetId().GetValue()).Msg("Failed to update Operating System in DB")
				continue
			}
			// Backfill: if the record previously had a tenant_id (old ownership model), clear it.
			// Provider-owned OSes must not have tenant_id set.
			if cur.TenantID != nil {
				if _, cerr := osDAO.Clear(ctx, nil, cdbm.OperatingSystemClearInput{
					OperatingSystemId: cur.ID,
					TenantID:          true,
				}); cerr != nil {
					logger.Error().Err(cerr).Str("ID", reported.GetId().GetValue()).Msg("Failed to clear tenant_id from provider-owned OS")
				}
			}
		}
	}

	// Deletion propagation: carbide-core's Find APIs return only active records, so any iPXE OS
	// in our DB that is NOT in this inventory was deleted in carbide-core. Soft-delete it here.
	// Image-based OSes are not managed by this inventory, so we restrict to iPXE types only.
	// Exception: global- and limited-scoped OSes are owned by carbide-rest and must not be
	// deleted based on carbide-core's inventory (carbide-core is not their source of truth).
	allIpxeOSes, _, err := osDAO.GetAll(ctx, nil, cdbm.OperatingSystemFilterInput{
		OsTypes:                  []string{cdbm.OperatingSystemTypeIPXE},
		InfrastructureProviderID: &siteProviderID,
	}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch iPXE Operating Systems from DB for deletion reconciliation")
		return err
	}
	for _, dbOS := range allIpxeOSes {
		if dbOS.Scope != nil && *dbOS.Scope != cdbm.OperatingSystemScopeLocal {
			continue
		}
		if _, inCore := reportedIDSet[dbOS.ID]; !inCore {
			logger.Info().Str("ID", dbOS.ID.String()).Msg("Soft-deleting iPXE OS absent from carbide-core inventory")
			if derr := osDAO.Delete(ctx, nil, dbOS.ID); derr != nil {
				logger.Error().Err(derr).Str("ID", dbOS.ID.String()).Msg("Failed to soft-delete OS")
			}
		}
	}

	// Aggregate status for global/limited OSes from their per-site core statuses.
	// Rule: all sites READY -> OS READY; any site not READY -> OS PROVISIONING.
	if len(globalLimitedOSIDs) > 0 {
		ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mod.dbSession)
		for osID := range globalLimitedOSIDs {
			ossas, _, gerr := ossaDAO.GetAll(ctx, nil, cdbm.OperatingSystemSiteAssociationFilterInput{
				OperatingSystemIDs: []uuid.UUID{osID},
			}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
			if gerr != nil {
				logger.Error().Err(gerr).Str("osID", osID.String()).Msg("Failed to fetch OSSAs for aggregate status")
				continue
			}
			allReady := true
			for _, o := range ossas {
				if o.CoreStatus == nil || *o.CoreStatus != cdbm.OperatingSystemStatusReady {
					allReady = false
					break
				}
			}
			aggregatedStatus := cdbm.OperatingSystemStatusProvisioning
			if allReady && len(ossas) > 0 {
				aggregatedStatus = cdbm.OperatingSystemStatusReady
			}
			if _, uerr := osDAO.Update(ctx, nil, cdbm.OperatingSystemUpdateInput{
				OperatingSystemId: osID,
				Status:            &aggregatedStatus,
			}); uerr != nil {
				logger.Error().Err(uerr).Str("osID", osID.String()).Msg("Failed to update aggregate OS status")
			}
		}
	}

	logger.Info().Msg("Completed activity")
	return nil
}

// NewManageOperatingSystemSync returns a new ManageOperatingSystemSync activity
func NewManageOperatingSystemSync(dbSession *cdb.Session, siteClientPool *sc.ClientPool) ManageOperatingSystemSync {
	return ManageOperatingSystemSync{
		dbSession:      dbSession,
		siteClientPool: siteClientPool,
	}
}

// tenantStateToRestStatus maps carbide-core's TenantState to the REST OperatingSystem status string.
func tenantStateToRestStatus(s cwssaws.TenantState) string {
	switch s {
	case cwssaws.TenantState_READY:
		return cdbm.OperatingSystemStatusReady
	case cwssaws.TenantState_FAILED:
		return cdbm.OperatingSystemStatusError
	default:
		return cdbm.OperatingSystemStatusProvisioning
	}
}

// osTypeToString maps carbide-core's OperatingSystemType enum to the DB type string.
// Both OS_TYPE_IPXE and OS_TYPE_IPXE_OS_DEFINITION map to the "iPXE" DB type; the
// distinction between raw-script and template-based is carried by IpxeTemplateName.
func osTypeToString(t cwssaws.OperatingSystemType) string {
	switch t {
	case cwssaws.OperatingSystemType_OS_TYPE_IPXE, cwssaws.OperatingSystemType_OS_TYPE_IPXE_OS_DEFINITION:
		return cdbm.OperatingSystemTypeIPXE
	default:
		return cdbm.OperatingSystemTypeIPXE
	}
}

// protoParamsToModel converts proto IpxeOsParameter slice to DB model slice
func protoParamsToModel(params []*cwssaws.IpxeOsParameter) []cdbm.OperatingSystemIpxeParameter {
	result := make([]cdbm.OperatingSystemIpxeParameter, 0, len(params))
	for _, p := range params {
		if p == nil {
			continue
		}
		result = append(result, cdbm.OperatingSystemIpxeParameter{Name: p.Name, Value: p.Value})
	}
	return result
}

// protoArtifactsToModel converts proto IpxeOsArtifact slice to DB model slice
func protoArtifactsToModel(artifacts []*cwssaws.IpxeOsArtifact) []cdbm.OperatingSystemIpxeArtifact {
	result := make([]cdbm.OperatingSystemIpxeArtifact, 0, len(artifacts))
	for _, a := range artifacts {
		if a == nil {
			continue
		}
		result = append(result, cdbm.OperatingSystemIpxeArtifact{
			Name:          a.Name,
			URL:           a.Url,
			SHA:           a.Sha,
			AuthType:      a.AuthType,
			AuthToken:     a.AuthToken,
			CacheStrategy: a.GetCacheStrategy().String(),
		})
	}
	return result
}
