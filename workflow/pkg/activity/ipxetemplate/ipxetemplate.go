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

package ipxetemplate

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	cdbp "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	sc "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/client/site"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ManageIpxeTemplate is an activity wrapper for managing iPXE template inventory that allows
// injecting DB access
type ManageIpxeTemplate struct {
	dbSession      *cdb.Session
	siteClientPool *sc.ClientPool
}

// UpdateIpxeTemplatesInDB is a Temporal activity that takes a collection of iPXE template data
// pushed by the Site Agent and reconciles the DB
func (mit ManageIpxeTemplate) UpdateIpxeTemplatesInDB(ctx context.Context, siteID uuid.UUID, inventory *cwssaws.IpxeTemplateInventory) error {
	logger := log.With().Str("Activity", "UpdateIpxeTemplatesInDB").Str("Site ID", siteID.String()).Logger()

	logger.Info().Msg("Starting activity")

	if inventory == nil {
		logger.Error().Msg("UpdateIpxeTemplatesInDB called with nil inventory")
		return errors.New("UpdateIpxeTemplatesInDB called with nil inventory")
	}

	if inventory.InventoryStatus == cwssaws.InventoryStatus_INVENTORY_STATUS_FAILED {
		logger.Warn().Msg("Received failed inventory status from Site Agent, skipping inventory processing")
		return nil
	}

	// Ensure site exists
	stDAO := cdbm.NewSiteDAO(mit.dbSession)
	_, err := stDAO.GetByID(ctx, nil, siteID, nil, false)
	if err != nil {
		if errors.Is(err, cdb.ErrDoesNotExist) {
			logger.Warn().Err(err).Msg("Received inventory for unknown or deleted Site")
		} else {
			logger.Error().Err(err).Msg("Failed to retrieve Site from DB")
		}
		return err
	}

	// Initialize DAO
	templateDAO := cdbm.NewIpxeTemplateDAO(mit.dbSession)

	// Fetch all existing iPXE templates for this site
	filterInput := cdbm.IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{siteID}}
	existingTemplates, _, err := templateDAO.GetAll(ctx, nil, filterInput, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get iPXE templates for Site from DB")
		return err
	}

	// Build a map of existing templates keyed by core template ID
	existingByTemplateID := map[uuid.UUID]*cdbm.IpxeTemplate{}
	for i := range existingTemplates {
		existingByTemplateID[existingTemplates[i].TemplateID] = &existingTemplates[i]
	}

	// Track all template IDs reported by this inventory payload
	reportedTemplateIDs := map[uuid.UUID]bool{}

	if inventory.InventoryPage != nil {
		logger.Info().Msgf("Received iPXE template inventory page: %d of %d, page size: %d, total count: %d",
			inventory.InventoryPage.CurrentPage, inventory.InventoryPage.TotalPages,
			inventory.InventoryPage.PageSize, inventory.InventoryPage.TotalItems)

		for _, idStr := range inventory.InventoryPage.ItemIds {
			if tid, perr := uuid.Parse(idStr); perr == nil {
				reportedTemplateIDs[tid] = true
			}
		}
	}

	for _, reported := range inventory.GetTemplates() {
		if reported == nil {
			logger.Error().Msg("Received nil iPXE template entry, skipping")
			continue
		}
		if reported.GetId() == nil || reported.GetId().GetValue() == "" {
			logger.Error().Str("Name", reported.Name).Msg("Received iPXE template with empty id, skipping")
			continue
		}

		templateID, perr := uuid.Parse(reported.GetId().GetValue())
		if perr != nil {
			logger.Error().Err(perr).Str("Name", reported.Name).Msg("Received iPXE template with invalid id, skipping")
			continue
		}

		// Only propagate PUBLIC templates into REST.
		if reported.Scope != cwssaws.IpxeTemplateScope_PUBLIC {
			logger.Debug().Str("Name", reported.Name).Str("Scope", reported.Scope.String()).Msg("Skipping non-public iPXE template")
			continue
		}

		reportedTemplateIDs[templateID] = true
		reportedScope := ipxeScopeToString(reported.Scope)

		cur, found := existingByTemplateID[templateID]
		if !found {
			// Cross-site name consistency check: if any other site already has a
			// template with the same ID but a different name, skip this entry.
			existing, gerr := templateDAO.GetAnyByTemplateID(ctx, nil, templateID)
			if gerr == nil && existing != nil && existing.Name != reported.Name {
				logger.Error().
					Str("TemplateID", templateID.String()).
					Str("ReportedName", reported.Name).
					Str("ExistingName", existing.Name).
					Str("ExistingSiteID", existing.SiteID.String()).
					Msg("Template ID reused with different name across sites, skipping")
				continue
			}

			input := cdbm.IpxeTemplateCreateInput{
				TemplateID:        templateID,
				SiteID:            siteID,
				Name:              reported.Name,
				Template:          reported.Template,
				RequiredParams:    reported.RequiredParams,
				ReservedParams:    reported.ReservedParams,
				RequiredArtifacts: reported.RequiredArtifacts,
				Scope:             reportedScope,
			}
			if _, cerr := templateDAO.Create(ctx, nil, input); cerr != nil {
				logger.Error().Err(cerr).Str("Name", reported.Name).Msg("Failed to create iPXE template in DB")
				return fmt.Errorf("failed to create iPXE template %q: %w", reported.Name, cerr)
			}
			continue
		}

		if cur.Name != reported.Name ||
			cur.Scope != reportedScope ||
			cur.Template != reported.Template ||
			!reflect.DeepEqual(cur.RequiredParams, reported.RequiredParams) ||
			!reflect.DeepEqual(cur.ReservedParams, reported.ReservedParams) ||
			!reflect.DeepEqual(cur.RequiredArtifacts, reported.RequiredArtifacts) {
			input := cdbm.IpxeTemplateUpdateInput{
				ID:                cur.ID,
				Name:              reported.Name,
				Template:          reported.Template,
				RequiredParams:    reported.RequiredParams,
				ReservedParams:    reported.ReservedParams,
				RequiredArtifacts: reported.RequiredArtifacts,
				Scope:             reportedScope,
			}
			if _, uerr := templateDAO.Update(ctx, nil, input); uerr != nil {
				logger.Error().Err(uerr).Str("Name", reported.Name).Msg("Failed to update iPXE template in DB")
				return fmt.Errorf("failed to update iPXE template %q: %w", reported.Name, uerr)
			}
		}
	}

	// Delete any templates present in DB but no longer reported by the Site Controller.
	if inventory.InventoryPage == nil || inventory.InventoryPage.TotalPages == 0 ||
		inventory.InventoryPage.CurrentPage == inventory.InventoryPage.TotalPages {
		for _, existing := range existingTemplates {
			if reportedTemplateIDs[existing.TemplateID] {
				continue
			}
			logger.Info().Str("Name", existing.Name).Str("TemplateID", existing.TemplateID.String()).Msg("Deleting iPXE template from DB since it was no longer reported by Site Controller")
			if derr := templateDAO.Delete(ctx, nil, existing.ID); derr != nil {
				logger.Error().Err(derr).Str("Name", existing.Name).Msg("Failed to delete iPXE template from DB")
				return fmt.Errorf("failed to delete iPXE template %q: %w", existing.Name, derr)
			}
		}
	}

	logger.Info().Msg("Completed activity")
	return nil
}

// NewManageIpxeTemplate returns a new ManageIpxeTemplate activity
func NewManageIpxeTemplate(dbSession *cdb.Session, siteClientPool *sc.ClientPool) ManageIpxeTemplate {
	return ManageIpxeTemplate{
		dbSession:      dbSession,
		siteClientPool: siteClientPool,
	}
}

// ipxeScopeToString converts the IpxeTemplateScope enum from the gRPC proto
// to the lowercase string representation stored in the database.
func ipxeScopeToString(scope cwssaws.IpxeTemplateScope) string {
	if scope == cwssaws.IpxeTemplateScope_PUBLIC {
		return cdbm.IpxeTemplateScopePublic
	}
	return cdbm.IpxeTemplateScopeInternal
}
