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

	mapset "github.com/deckarep/golang-set/v2"
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

// ManageOperatingSystem is an activity wrapper for managing Operating System lifecycle for a Site and allows
// injecting DB access
type ManageOperatingSystem struct {
	dbSession      *cdb.Session
	siteClientPool *sc.ClientPool
}

// Activity functions

// UpdateOsImagesInDB takes information pushed by Site Agent for a collection of image based OSs associated with the Site and updates the DB
func (mos ManageOperatingSystem) UpdateOsImagesInDB(ctx context.Context, siteID uuid.UUID, osImageInventory *cwssaws.OsImageInventory) ([]uuid.UUID, error) {
	logger := log.With().Str("Activity", "UpdateOsImagesInDB").Str("Site ID", siteID.String()).Logger()

	logger.Info().Msg("starting activity")

	stDAO := cdbm.NewSiteDAO(mos.dbSession)

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

	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mos.dbSession)

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
				serr := mos.updateOperatingSystemSiteAssociationStatusInDB(ctx, nil, ossa.ID, cdb.GetStrPtr(ossaStatus), ossaStatusMessage)
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
			serr = mos.UpdateOperatingSystemStatusInDB(ctx, ossa.OperatingSystemID)
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

			serr = mos.updateOperatingSystemSiteAssociationStatusInDB(ctx, nil, ossa.ID, cdb.GetStrPtr(cdbm.OperatingSystemSiteAssociationStatusError), cdb.GetStrPtr("Operating System is missing on Site"))
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
func (mos ManageOperatingSystem) updateOperatingSystemSiteAssociationStatusInDB(ctx context.Context, tx *cdb.Tx, ossaID uuid.UUID, status *string, statusMessage *string) error {
	if status != nil {
		ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mos.dbSession)

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

		statusDetailDAO := cdbm.NewStatusDetailDAO(mos.dbSession)
		_, err = statusDetailDAO.CreateFromParams(ctx, tx, ossaID.String(), *status, statusMessage)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateOperatingSystemStatusInDB is helper function to write Operating System updates to DB
func (mos ManageOperatingSystem) UpdateOperatingSystemStatusInDB(ctx context.Context, osID uuid.UUID) error {
	logger := log.With().Str("Activity", "UpdateOperatingSystemStatusInDB").Str("Operating System ID", osID.String()).Logger()

	logger.Info().Msg("starting activity")

	osDAO := cdbm.NewOperatingSystemDAO(mos.dbSession)

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

	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mos.dbSession)
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
			tx, err := cdb.BeginTx(ctx, mos.dbSession, &sql.TxOptions{})
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

	statusDetailDAO := cdbm.NewStatusDetailDAO(mos.dbSession)
	_, err = statusDetailDAO.CreateFromParams(ctx, nil, osID.String(), *osStatus, osMessage)
	if err != nil {
		return err
	}

	logger.Info().Msg("successfully completed activity")

	return nil
}

// UpdateOperatingSystemsInDB reconciles the operating_system table for a Site based on Operating Systems reported from Site
func (mos ManageOperatingSystem) UpdateOperatingSystemsInDB(ctx context.Context, siteID uuid.UUID, inventory *cwssaws.OperatingSystemInventory) error {
	logger := log.With().Str("Activity", "UpdateOperatingSystemsInDB").Str("Site ID", siteID.String()).Logger()
	logger.Info().Msg("Starting activity")

	if inventory == nil {
		return errors.New("UpdateOperatingSystemsInDB called with nil inventory")
	}

	if inventory.InventoryStatus == cwssaws.InventoryStatus_INVENTORY_STATUS_FAILED {
		logger.Warn().Msg("Received failed inventory status from Site Agent, skipping")
		return nil
	}

	stDAO := cdbm.NewSiteDAO(mos.dbSession)
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
	logger.Debug().Str("InfrastructureProviderID", site.InfrastructureProviderID.String()).Msg("Resolved Infrastructure Provider from Site")

	osDAO := cdbm.NewOperatingSystemDAO(mos.dbSession)
	ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mos.dbSession)
	itsaDAO := cdbm.NewIpxeTemplateSiteAssociationDAO(mos.dbSession)

	// Collect the UUIDs of all reported OS records (active only — the new Find APIs do not
	// return deleted records). Site and REST share the same UUID as PK.
	reportedOSIDs := mapset.NewSet[uuid.UUID]()
	for _, reportedOS := range inventory.GetOperatingSystems() {
		if reportedOS == nil {
			logger.Error().Msg("Received nil OS record in inventory, skipping")
			continue
		}

		controllerOSID := reportedOS.GetId().GetValue()
		if controllerOSID == "" {
			logger.Error().Msg("Received OS record with empty ID, skipping")
			continue
		}

		reportedOSID, parseErr := uuid.Parse(controllerOSID)
		if parseErr != nil {
			logger.Error().Err(parseErr).Str("ControllerOperatingSystemID", controllerOSID).Msg("Received OS record with invalid UUID, skipping")
			continue
		}
		reportedOSIDs.Add(reportedOSID)
	}

	// Fetch DB records matching the reported IDs (including soft-deleted so we can detect
	// the case where REST already deleted an OS that Site still reports active).
	existingOSes, _, err := osDAO.GetAll(ctx, nil, cdbm.OperatingSystemFilterInput{
		OperatingSystemIds: reportedOSIDs.ToSlice(),
		IncludeDeleted:     true,
	}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)

	if err != nil {
		logger.Error().Err(err).Msg("Failed to get Operating Systems from DB")
		return err
	}

	existingOSByID := map[uuid.UUID]*cdbm.OperatingSystem{}
	for i := range existingOSes {
		existingOSByID[existingOSes[i].ID] = &existingOSes[i]
	}

	// Track global/limited OS IDs that need aggregate status recomputation.
	globalOrLimitedOSIDs := map[uuid.UUID]struct{}{}

	// Create or update OSes based on the Site inventory.
	for _, reportedOS := range inventory.GetOperatingSystems() {
		if reportedOS == nil || reportedOS.GetId().GetValue() == "" {
			continue
		}

		reportedOSID, parseErr := uuid.Parse(reportedOS.GetId().GetValue())
		if parseErr != nil {
			continue
		}

		slogger := logger.With().Str("ControllerOperatingSystemID", reportedOSID.String()).Logger()

		coreUpdated, _ := time.Parse(time.RFC3339, reportedOS.Updated)

		ipxeTemplateParams := make([]cdbm.OperatingSystemIpxeParameter, 0, len(reportedOS.IpxeTemplateParameters))
		for _, param := range reportedOS.IpxeTemplateParameters {
			ipxeTemplateParam := cdbm.OperatingSystemIpxeParameter{}
			ipxeTemplateParam.FromProto(param)
			ipxeTemplateParams = append(ipxeTemplateParams, ipxeTemplateParam)
		}

		ipxeTemplateArtifacts := make([]cdbm.OperatingSystemIpxeArtifact, 0, len(reportedOS.IpxeTemplateArtifacts))
		for _, artifact := range reportedOS.IpxeTemplateArtifacts {
			ipxeTemplateArtifact := cdbm.OperatingSystemIpxeArtifact{}
			ipxeTemplateArtifact.FromProto(artifact)
			ipxeTemplateArtifacts = append(ipxeTemplateArtifacts, ipxeTemplateArtifact)
		}

		osType := cdbm.OperatingSystemTypeFromProtoMap[reportedOS.Type]
		if osType == "" {
			slogger.Error().Str("Type", reportedOS.Type.String()).Msg("Received unknown OS type from Site, skipping")
			continue
		}

		existingOS, found := existingOSByID[reportedOSID]
		if !found {
			// Templated iPXE OS: verify the referenced template is available at this site
			// before creating the OS record. Skip silently if not available.
			if osType == cdbm.OperatingSystemTypeTemplatedIPXE && reportedOS.IpxeTemplateId != nil {
				ipxeTemplateID, serr := uuid.Parse(reportedOS.IpxeTemplateId.GetValue())
				if serr != nil {
					slogger.Error().Err(serr).Str("IpxeTemplateID", reportedOS.IpxeTemplateId.GetValue()).Msg("Invalid iPXE template UUID in Operating System, skipping")
					continue
				}

				_, serr = itsaDAO.GetByIpxeTemplateIDAndSiteID(ctx, nil, ipxeTemplateID, siteID, nil)
				if serr != nil {
					if errors.Is(serr, cdb.ErrDoesNotExist) {
						slogger.Warn().Str("IpxeTemplateID", ipxeTemplateID.String()).Msg("iPXE Template Association does not exist for Site, skipping")
						continue
					}
					slogger.Error().Err(serr).Msg("Failed to retrieve IpxeTemplateSiteAssociation, DB error")
					continue
				}
			}

			// New OS from Site: Create it with Site's InfrastructureProviderID.
			// OSes originating in Site are provider-owned (not tenant-owned)
			// ProviderAdmin can update them and all Tenants of the Provider can retrieve them
			// Scope is Local: the definition lives at a single site with bidirectional sync

			status := cdbm.OperatingSystemStatusFromProtoMap[reportedOS.Status]
			if status == "" {
				slogger.Warn().Str("Status", reportedOS.Status.String()).Msg("Received unknown status from Site, using `Syncing` as default")
				status = cdbm.OperatingSystemStatusSyncing
			}

			_, serr := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
				ID:                       reportedOSID,
				Name:                     reportedOS.Name,
				Org:                      site.Org,
				TenantID:                 nil,
				InfrastructureProviderID: &site.InfrastructureProviderID,
				OsType:                   osType,
				Description:              reportedOS.Description,
				UserData:                 reportedOS.UserData,
				IpxeScript:               reportedOS.IpxeScript,
				AllowOverride:            reportedOS.AllowOverride,
				PhoneHomeEnabled:         reportedOS.PhoneHomeEnabled,
				IpxeTemplateId:           cdb.GetStrPtr(reportedOS.IpxeTemplateId.GetValue()),
				IpxeTemplateParameters:   ipxeTemplateParams,
				IpxeTemplateArtifacts:    ipxeTemplateArtifacts,
				IpxeOSHash:               reportedOS.IpxeTemplateDefinitionHash,
				IpxeOsScope:              cdb.GetStrPtr(cdbm.OperatingSystemScopeLocal),
				Status:                   status,
			})
			if serr != nil {
				slogger.Error().Err(serr).Msg("Failed to create Operating System, DB error")
				continue
			}

			if !reportedOS.IsActive {
				// TODO: Allow creation of inactive OSes
				_, serr := osDAO.Update(ctx, nil, cdbm.OperatingSystemUpdateInput{
					OperatingSystemId: reportedOSID,
					IsActive:          cdb.GetBoolPtr(false),
				})
				if serr != nil {
					slogger.Error().Err(serr).Msg("Failed to set Operating System to inactive on creation")
					continue
				}
			}

			// Create site association linking the OS to the reporting site.
			ossaStatus := cdbm.OperatingSystemSiteAssociationStatusFromProtoMap[reportedOS.Status]
			if ossaStatus == "" {
				slogger.Warn().Str("Status", reportedOS.Status.String()).Msg("Received unknown status from Site, using `Syncing` as default")
				ossaStatus = cdbm.OperatingSystemSiteAssociationStatusSyncing
			}

			_, ossaErr := ossaDAO.Create(ctx, nil, cdbm.OperatingSystemSiteAssociationCreateInput{
				OperatingSystemID: reportedOSID,
				SiteID:            siteID,
				Status:            ossaStatus,
			})
			if ossaErr != nil {
				slogger.Error().Err(ossaErr).Msg("Failed to create site association for new OS")
				continue
			}

			// Newly-created OS: definition and per-site association have just been
			// written with the reported state. Skip the existing-OS update path
			// below (it dereferences existingOS which is nil here) and do not add
			// to globalOrLimitedOSIDs because new records are always Local scope.
			continue
		}

		// REST layer has already soft-deleted this OS (user-initiated)
		// Do not restore it even if Site still reports it as active (the delete push to Site may be in-flight)
		if existingOS.Deleted != nil {
			continue
		}

		// Update or create the per-site association for every OS type. For
		// Global/Limited, REST is the source of truth for the definition so we
		// only record the Site's controller state and skip the definition update.
		// For Local (provider-owned, from Site) we also fall through to update
		// the definition below. nil scope is treated as Local for safety
		// (legacy records before the backfill migration)
		isLocalScope := existingOS.IpxeOsScope == nil || *existingOS.IpxeOsScope == cdbm.OperatingSystemScopeLocal
		controllerState := cdbm.OperatingSystemStatusFromProtoMap[reportedOS.Status]
		if controllerState == "" {
			slogger.Warn().Str("Status", reportedOS.Status.String()).Msg("Received unknown status from Site, using `Syncing` as default")
			controllerState = cdbm.OperatingSystemStatusSyncing
		}

		ossaStatus := cdbm.OperatingSystemSiteAssociationStatusFromProtoMap[reportedOS.Status]
		if ossaStatus == "" {
			slogger.Warn().Str("Status", reportedOS.Status.String()).Msg("Received unknown status from Site, using `Syncing` as default")
			ossaStatus = cdbm.OperatingSystemSiteAssociationStatusSyncing
		}

		ossa, serr := ossaDAO.GetByOperatingSystemIDAndSiteID(ctx, nil, reportedOSID, siteID, nil)
		if serr != nil {
			if !errors.Is(serr, cdb.ErrDoesNotExist) {
				slogger.Error().Err(serr).Msg("Failed to retrieve Operating System Site Association, DB error")
				continue
			}

			// Operating System Site Association is missing, create it
			_, serr := ossaDAO.Create(ctx, nil, cdbm.OperatingSystemSiteAssociationCreateInput{
				OperatingSystemID: reportedOSID,
				SiteID:            siteID,
				Status:            ossaStatus,
				ControllerState:   &controllerState,
			})
			if serr != nil {
				slogger.Error().Err(serr).Msg("Failed to create Operating System Site Association")
				continue
			}
		} else {
			// Update existing Operating System Site Association
			_, uerr := ossaDAO.Update(ctx, nil, cdbm.OperatingSystemSiteAssociationUpdateInput{
				OperatingSystemSiteAssociationID: ossa.ID,
				Status:                           &ossaStatus,
				ControllerState:                  &controllerState,
			})
			if uerr != nil {
				slogger.Error().Err(uerr).Msg("Failed to update Operating System Site Association")
				continue
			}
		}

		// TODO: Is this correct?
		if !isLocalScope {
			globalOrLimitedOSIDs[reportedOSID] = struct{}{}
		}

		// Operating System exists in both REST and Site; update the REST record only for
		// Local-scoped OSes (Site is the source of truth for the definition).
		// Global/Limited OSes are REST-owned: skip the definition update and rely solely on
		// the aggregate status recomputation that runs at the end of this function.
		// Backfill: older records may have been created with tenant_id set and no
		// infrastructure_provider_id (before this ownership model was established).
		needsProviderBackfill := isLocalScope && existingOS.InfrastructureProviderID == nil
		needsOrgBackfill := isLocalScope && existingOS.Org == "" && site.Org != ""
		needsIsActiveCorrection := isLocalScope && existingOS.IsActive != reportedOS.IsActive
		needsTenantClear := isLocalScope && existingOS.TenantID != nil

		if isLocalScope && (coreUpdated.After(existingOS.Updated) || needsProviderBackfill || needsOrgBackfill || needsIsActiveCorrection || needsTenantClear) {
			controllerState := cdbm.OperatingSystemStatusFromProtoMap[reportedOS.Status]
			if controllerState == "" {
				slogger.Warn().Str("Status", reportedOS.Status.String()).Msg("Received unknown status from Site, using `Syncing` as default")
				controllerState = cdbm.OperatingSystemStatusSyncing
			}

			var ipxeTemplateID *string
			if reportedOS.IpxeTemplateId != nil {
				ipxeTemplateID = cdb.GetStrPtr(reportedOS.IpxeTemplateId.GetValue())
			}

			updateInput := cdbm.OperatingSystemUpdateInput{
				OperatingSystemId:        existingOS.ID,
				Name:                     &reportedOS.Name,
				Org:                      &site.Org,
				TenantID:                 nil,
				InfrastructureProviderID: &site.InfrastructureProviderID,
				OsType:                   &osType,
				Description:              reportedOS.Description,
				UserData:                 reportedOS.UserData,
				IpxeScript:               reportedOS.IpxeScript,
				AllowOverride:            &reportedOS.AllowOverride,
				PhoneHomeEnabled:         &reportedOS.PhoneHomeEnabled,
				IsActive:                 &reportedOS.IsActive,
				IpxeTemplateId:           ipxeTemplateID,
				IpxeTemplateParameters:   &ipxeTemplateParams,
				IpxeTemplateArtifacts:    &ipxeTemplateArtifacts,
				IpxeOSHash:               reportedOS.IpxeTemplateDefinitionHash,
				Status:                   &controllerState,
			}
			if _, uerr := osDAO.Update(ctx, nil, updateInput); uerr != nil {
				slogger.Error().Err(uerr).Msg("Failed to update Operating System, DB error")
				continue
			}
			// Backfill: if the record previously had a tenant_id (old ownership model), clear it.
			// Provider-owned OSes must not have tenant_id set.
			if existingOS.TenantID != nil {
				_, cerr := osDAO.Clear(ctx, nil, cdbm.OperatingSystemClearInput{
					OperatingSystemId: existingOS.ID,
					TenantID:          true,
				})
				if cerr != nil {
					slogger.Error().Err(cerr).Msg("Failed to clear Tenant ID from Provider owned Operating System, DB error")
					continue
				}
			}
		}
	}

	// Deletion propagation: Site's Find APIs return only active records, so any iPXE OS
	// in our DB that is NOT in this inventory was deleted in carbide-core. Soft-delete it here.
	// Image-based OSes are not managed by this inventory, so we restrict to iPXE types only.
	// Exception: global- and limited-scoped OSes are owned by REST and must not be
	// deleted based on Site's inventory (Site is not their source of truth)
	allIpxeOSes, _, err := osDAO.GetAll(ctx, nil, cdbm.OperatingSystemFilterInput{
		OsTypes:                  []string{cdbm.OperatingSystemTypeIPXE, cdbm.OperatingSystemTypeTemplatedIPXE},
		InfrastructureProviderID: &site.InfrastructureProviderID,
	}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch iPXE Operating Systems from DB for deletion reconciliation")
		return err
	}

	for _, ipxeOS := range allIpxeOSes {
		if ipxeOS.IpxeOsScope != nil && *ipxeOS.IpxeOsScope != cdbm.OperatingSystemScopeLocal {
			continue
		}

		slogger := logger.With().Str("OperatingSystemID", ipxeOS.ID.String()).Logger()

		if !reportedOSIDs.Contains(ipxeOS.ID) {
			slogger.Info().Msg("Soft-deleting iPXE OS absent from Site inventory")
			serr := osDAO.Delete(ctx, nil, ipxeOS.ID)
			if serr != nil {
				slogger.Error().Err(serr).Msg("Failed to soft-delete OS, DB error")
				continue
			}
		}
	}

	// Aggregate status for global/limited OSes from their per-site core statuses.
	// Rule: If all Site Associations have `Ready` status then the Operating System is `Ready`. Otherwise, it is `Syncing`.
	if len(globalOrLimitedOSIDs) > 0 {
		ossaDAO := cdbm.NewOperatingSystemSiteAssociationDAO(mos.dbSession)
		for osID := range globalOrLimitedOSIDs {
			slogger := logger.With().Str("OperatingSystemID", osID.String()).Logger()

			ossas, _, serr := ossaDAO.GetAll(ctx, nil, cdbm.OperatingSystemSiteAssociationFilterInput{
				OperatingSystemIDs: []uuid.UUID{osID},
			}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)

			if serr != nil {
				slogger.Error().Err(serr).Msg("Failed to fetch Site Associations to determine Operating System status, DB error")
				continue
			}

			allReady := true
			for _, ossa := range ossas {
				if ossa.Status != cdbm.OperatingSystemSiteAssociationStatusSynced {
					allReady = false
					break
				}
			}

			aggregatedStatus := cdbm.OperatingSystemStatusSyncing
			if allReady && len(ossas) > 0 {
				aggregatedStatus = cdbm.OperatingSystemStatusReady
			}

			_, serr = osDAO.Update(ctx, nil, cdbm.OperatingSystemUpdateInput{
				OperatingSystemId: osID,
				Status:            &aggregatedStatus,
			})
			if serr != nil {
				slogger.Error().Err(serr).Msg("Failed to update aggregate OS status, DB error")
			}
		}
	}

	logger.Info().Msg("Completed activity")

	return nil
}

// NewManageOperatingSystem returns a new ManageOperatingSystem activity
func NewManageOperatingSystem(dbSession *cdb.Session, siteClientPool *sc.ClientPool) ManageOperatingSystem {
	return ManageOperatingSystem{
		dbSession:      dbSession,
		siteClientPool: siteClientPool,
	}
}
