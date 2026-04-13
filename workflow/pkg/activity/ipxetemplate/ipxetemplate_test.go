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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	cdbp "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"

	"github.com/NVIDIA/ncx-infra-controller-rest/workflow/internal/config"
	cwu "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/util"

	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

func TestManageIpxeTemplate_Reconcile_CreateUpdateDelete(t *testing.T) {
	ctx := context.Background()
	_ = config.GetTestConfig()

	dbSession := cwu.TestInitDB(t)
	defer dbSession.Close()
	cwu.TestSetupSchema(t, dbSession)

	// Build basic graph: provider, tenant, site
	ipOrg := "test-ip-org"
	ipRoles := []string{"FORGE_PROVIDER_ADMIN"}
	ipu := cwu.TestBuildUser(t, dbSession, uuid.NewString(), []string{ipOrg}, ipRoles)
	ip := cwu.TestBuildInfrastructureProvider(t, dbSession, "test-provider", ipOrg, ipu)
	site := cwu.TestBuildSite(t, dbSession, ip, "test-site", cdbm.SiteStatusRegistered, nil, ipu)
	assert.NotNil(t, site)

	mit := NewManageIpxeTemplate(dbSession, cwu.TestTemporalSiteClientPool(t))
	templateDAO := cdbm.NewIpxeTemplateDAO(dbSession)

	// Stable template IDs (matching core)
	kernelInitrdID := uuid.MustParse("c4b1d4f6-69ba-5f55-90cd-ab2acd002475")
	ubuntuAutoinstallID := uuid.MustParse("a7850943-e3cd-5e9a-93ca-9e12f52939cc")

	// 1) Create: inventory with two PUBLIC templates
	inv1 := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates: []*cwssaws.IpxeTemplate{
			{Id: &cwssaws.IpxeTemplateId{Value: kernelInitrdID.String()}, Name: "kernel-initrd", Scope: cwssaws.IpxeTemplateScope_PUBLIC, RequiredParams: []string{"p1"}, ReservedParams: []string{"r1"}, RequiredArtifacts: []string{"kernel"}},
			{Id: &cwssaws.IpxeTemplateId{Value: ubuntuAutoinstallID.String()}, Name: "ubuntu-autoinstall", Scope: cwssaws.IpxeTemplateScope_PUBLIC, RequiredParams: []string{}, ReservedParams: []string{}, RequiredArtifacts: []string{"iso"}},
		},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site.ID, inv1))

	templates, total, err := templateDAO.GetAll(ctx, nil, cdbm.IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(100)})
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	nameSet := map[string]bool{}
	for _, tmpl := range templates {
		nameSet[tmpl.Name] = true
	}
	assert.True(t, nameSet["kernel-initrd"])
	assert.True(t, nameSet["ubuntu-autoinstall"])

	// 2) Update: change required params of "ubuntu-autoinstall" (still PUBLIC)
	inv2 := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates: []*cwssaws.IpxeTemplate{
			{Id: &cwssaws.IpxeTemplateId{Value: kernelInitrdID.String()}, Name: "kernel-initrd", Scope: cwssaws.IpxeTemplateScope_PUBLIC, RequiredParams: []string{"p1"}, ReservedParams: []string{"r1"}, RequiredArtifacts: []string{"kernel"}},
			{Id: &cwssaws.IpxeTemplateId{Value: ubuntuAutoinstallID.String()}, Name: "ubuntu-autoinstall", Scope: cwssaws.IpxeTemplateScope_PUBLIC, RequiredParams: []string{"new-param"}, ReservedParams: []string{}, RequiredArtifacts: []string{"iso"}},
		},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site.ID, inv2))

	updated, err := templateDAO.GetBySiteAndName(ctx, nil, site.ID, "ubuntu-autoinstall")
	assert.NoError(t, err)
	assert.Equal(t, []string{"new-param"}, updated.RequiredParams)

	// 3) Delete: remove "ubuntu-autoinstall" from inventory
	inv3 := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates: []*cwssaws.IpxeTemplate{
			{Id: &cwssaws.IpxeTemplateId{Value: kernelInitrdID.String()}, Name: "kernel-initrd", Scope: cwssaws.IpxeTemplateScope_PUBLIC, RequiredParams: []string{"p1"}, ReservedParams: []string{"r1"}, RequiredArtifacts: []string{"kernel"}},
		},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site.ID, inv3))

	_, total, err = templateDAO.GetAll(ctx, nil, cdbm.IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(100)})
	assert.NoError(t, err)
	assert.Equal(t, 1, total)

	_, err = templateDAO.GetBySiteAndName(ctx, nil, site.ID, "ubuntu-autoinstall")
	assert.ErrorIs(t, err, cdb.ErrDoesNotExist)
}

func TestManageIpxeTemplate_InternalScopeFiltered(t *testing.T) {
	ctx := context.Background()
	_ = config.GetTestConfig()

	dbSession := cwu.TestInitDB(t)
	defer dbSession.Close()
	cwu.TestSetupSchema(t, dbSession)

	ipOrg := "test-ip-org"
	ipRoles := []string{"FORGE_PROVIDER_ADMIN"}
	ipu := cwu.TestBuildUser(t, dbSession, uuid.NewString(), []string{ipOrg}, ipRoles)
	ip := cwu.TestBuildInfrastructureProvider(t, dbSession, "test-provider", ipOrg, ipu)
	site := cwu.TestBuildSite(t, dbSession, ip, "test-site", cdbm.SiteStatusRegistered, nil, ipu)

	mit := NewManageIpxeTemplate(dbSession, cwu.TestTemporalSiteClientPool(t))
	templateDAO := cdbm.NewIpxeTemplateDAO(dbSession)

	publicID := uuid.MustParse("c4b1d4f6-69ba-5f55-90cd-ab2acd002475")
	internalID := uuid.MustParse("a7850943-e3cd-5e9a-93ca-9e12f52939cc")

	// Inventory with one PUBLIC and one INTERNAL template
	inv := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates: []*cwssaws.IpxeTemplate{
			{Id: &cwssaws.IpxeTemplateId{Value: publicID.String()}, Name: "public-tmpl", Scope: cwssaws.IpxeTemplateScope_PUBLIC},
			{Id: &cwssaws.IpxeTemplateId{Value: internalID.String()}, Name: "internal-tmpl", Scope: cwssaws.IpxeTemplateScope_INTERNAL},
		},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site.ID, inv))

	// Only the PUBLIC template should be stored
	_, total, err := templateDAO.GetAll(ctx, nil, cdbm.IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(100)})
	assert.NoError(t, err)
	assert.Equal(t, 1, total)

	tmpl, err := templateDAO.GetBySiteAndName(ctx, nil, site.ID, "public-tmpl")
	assert.NoError(t, err)
	assert.Equal(t, cdbm.IpxeTemplateScopePublic, tmpl.Scope)

	_, err = templateDAO.GetBySiteAndName(ctx, nil, site.ID, "internal-tmpl")
	assert.ErrorIs(t, err, cdb.ErrDoesNotExist)
}

func TestManageIpxeTemplate_InternalScopeDeletesExistingPublic(t *testing.T) {
	ctx := context.Background()
	_ = config.GetTestConfig()

	dbSession := cwu.TestInitDB(t)
	defer dbSession.Close()
	cwu.TestSetupSchema(t, dbSession)

	ipOrg := "test-ip-org"
	ipRoles := []string{"FORGE_PROVIDER_ADMIN"}
	ipu := cwu.TestBuildUser(t, dbSession, uuid.NewString(), []string{ipOrg}, ipRoles)
	ip := cwu.TestBuildInfrastructureProvider(t, dbSession, "test-provider", ipOrg, ipu)
	site := cwu.TestBuildSite(t, dbSession, ip, "test-site", cdbm.SiteStatusRegistered, nil, ipu)

	mit := NewManageIpxeTemplate(dbSession, cwu.TestTemporalSiteClientPool(t))
	templateDAO := cdbm.NewIpxeTemplateDAO(dbSession)

	templateID := uuid.MustParse("c4b1d4f6-69ba-5f55-90cd-ab2acd002475")

	// First sync: template is PUBLIC
	inv1 := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates: []*cwssaws.IpxeTemplate{
			{Id: &cwssaws.IpxeTemplateId{Value: templateID.String()}, Name: "my-template", Scope: cwssaws.IpxeTemplateScope_PUBLIC},
		},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site.ID, inv1))
	_, err := templateDAO.GetBySiteAndName(ctx, nil, site.ID, "my-template")
	assert.NoError(t, err)

	// Second sync: template changed to INTERNAL — should be removed via reconciliation
	inv2 := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates: []*cwssaws.IpxeTemplate{
			{Id: &cwssaws.IpxeTemplateId{Value: templateID.String()}, Name: "my-template", Scope: cwssaws.IpxeTemplateScope_INTERNAL},
		},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site.ID, inv2))

	// Template should be gone (INTERNAL filtered out, then deletion by absence)
	_, total, err := templateDAO.GetAll(ctx, nil, cdbm.IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(100)})
	assert.NoError(t, err)
	assert.Equal(t, 0, total)
}

func TestManageIpxeTemplate_CrossSiteNameConflict(t *testing.T) {
	ctx := context.Background()
	_ = config.GetTestConfig()

	dbSession := cwu.TestInitDB(t)
	defer dbSession.Close()
	cwu.TestSetupSchema(t, dbSession)

	ipOrg := "test-ip-org"
	ipRoles := []string{"FORGE_PROVIDER_ADMIN"}
	ipu := cwu.TestBuildUser(t, dbSession, uuid.NewString(), []string{ipOrg}, ipRoles)
	ip := cwu.TestBuildInfrastructureProvider(t, dbSession, "test-provider", ipOrg, ipu)
	site1 := cwu.TestBuildSite(t, dbSession, ip, "site-1", cdbm.SiteStatusRegistered, nil, ipu)
	site2 := cwu.TestBuildSite(t, dbSession, ip, "site-2", cdbm.SiteStatusRegistered, nil, ipu)

	mit := NewManageIpxeTemplate(dbSession, cwu.TestTemporalSiteClientPool(t))
	templateDAO := cdbm.NewIpxeTemplateDAO(dbSession)

	sharedTemplateID := uuid.MustParse("c4b1d4f6-69ba-5f55-90cd-ab2acd002475")

	// Site 1 reports template with name "kernel-initrd"
	inv1 := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates: []*cwssaws.IpxeTemplate{
			{Id: &cwssaws.IpxeTemplateId{Value: sharedTemplateID.String()}, Name: "kernel-initrd", Scope: cwssaws.IpxeTemplateScope_PUBLIC},
		},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site1.ID, inv1))

	// Site 2 reports same template ID but different name — should be skipped
	inv2 := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates: []*cwssaws.IpxeTemplate{
			{Id: &cwssaws.IpxeTemplateId{Value: sharedTemplateID.String()}, Name: "wrong-name", Scope: cwssaws.IpxeTemplateScope_PUBLIC},
		},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site2.ID, inv2))

	// Site 2 should have no templates (the conflicting one was skipped)
	_, total, err := templateDAO.GetAll(ctx, nil, cdbm.IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site2.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(100)})
	assert.NoError(t, err)
	assert.Equal(t, 0, total)

	// Site 1 template should still be intact
	tmpl, err := templateDAO.GetBySiteAndName(ctx, nil, site1.ID, "kernel-initrd")
	assert.NoError(t, err)
	assert.Equal(t, sharedTemplateID, tmpl.TemplateID)

	// Site 2 reports same template ID with correct name — should succeed
	inv3 := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates: []*cwssaws.IpxeTemplate{
			{Id: &cwssaws.IpxeTemplateId{Value: sharedTemplateID.String()}, Name: "kernel-initrd", Scope: cwssaws.IpxeTemplateScope_PUBLIC},
		},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site2.ID, inv3))

	_, total, err = templateDAO.GetAll(ctx, nil, cdbm.IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site2.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(100)})
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
}

func TestManageIpxeTemplate_InventoryStatusFailed_Skip(t *testing.T) {
	ctx := context.Background()
	_ = config.GetTestConfig()

	dbSession := cwu.TestInitDB(t)
	defer dbSession.Close()
	cwu.TestSetupSchema(t, dbSession)

	ipOrg := "test-ip-org"
	ipRoles := []string{"FORGE_PROVIDER_ADMIN"}
	ipu := cwu.TestBuildUser(t, dbSession, uuid.NewString(), []string{ipOrg}, ipRoles)
	ip := cwu.TestBuildInfrastructureProvider(t, dbSession, "test-provider", ipOrg, ipu)
	site := cwu.TestBuildSite(t, dbSession, ip, "test-site", cdbm.SiteStatusRegistered, nil, ipu)

	// Seed one template
	templateDAO := cdbm.NewIpxeTemplateDAO(dbSession)
	_, err := templateDAO.Create(ctx, nil, cdbm.IpxeTemplateCreateInput{
		TemplateID: uuid.New(),
		SiteID:     site.ID,
		Name:       "existing-template",
		Scope:      cdbm.IpxeTemplateScopePublic,
	})
	assert.NoError(t, err)

	mit := NewManageIpxeTemplate(dbSession, cwu.TestTemporalSiteClientPool(t))

	// Send a failed inventory — nothing should change
	inv := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_FAILED,
		Templates:       []*cwssaws.IpxeTemplate{{Id: &cwssaws.IpxeTemplateId{Value: uuid.NewString()}, Name: "other-template", Scope: cwssaws.IpxeTemplateScope_PUBLIC}},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site.ID, inv))

	_, total, err := templateDAO.GetAll(ctx, nil, cdbm.IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(100)})
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
}

func TestManageIpxeTemplate_NilInventory(t *testing.T) {
	ctx := context.Background()
	_ = config.GetTestConfig()

	dbSession := cwu.TestInitDB(t)
	defer dbSession.Close()
	cwu.TestSetupSchema(t, dbSession)

	ipOrg := "test-ip-org"
	ipRoles := []string{"FORGE_PROVIDER_ADMIN"}
	ipu := cwu.TestBuildUser(t, dbSession, uuid.NewString(), []string{ipOrg}, ipRoles)
	ip := cwu.TestBuildInfrastructureProvider(t, dbSession, "test-provider", ipOrg, ipu)
	site := cwu.TestBuildSite(t, dbSession, ip, "test-site", cdbm.SiteStatusRegistered, nil, ipu)

	mit := NewManageIpxeTemplate(dbSession, cwu.TestTemporalSiteClientPool(t))

	err := mit.UpdateIpxeTemplatesInDB(ctx, site.ID, nil)
	assert.Error(t, err)
}

func TestManageIpxeTemplate_EmptyInventory_DeletesAll(t *testing.T) {
	ctx := context.Background()
	_ = config.GetTestConfig()

	dbSession := cwu.TestInitDB(t)
	defer dbSession.Close()
	cwu.TestSetupSchema(t, dbSession)

	ipOrg := "test-ip-org"
	ipRoles := []string{"FORGE_PROVIDER_ADMIN"}
	ipu := cwu.TestBuildUser(t, dbSession, uuid.NewString(), []string{ipOrg}, ipRoles)
	ip := cwu.TestBuildInfrastructureProvider(t, dbSession, "test-provider", ipOrg, ipu)
	site := cwu.TestBuildSite(t, dbSession, ip, "test-site", cdbm.SiteStatusRegistered, nil, ipu)

	// Seed templates
	templateDAO := cdbm.NewIpxeTemplateDAO(dbSession)
	for _, name := range []string{"tmpl-a", "tmpl-b"} {
		_, err := templateDAO.Create(ctx, nil, cdbm.IpxeTemplateCreateInput{TemplateID: uuid.New(), SiteID: site.ID, Name: name, Scope: cdbm.IpxeTemplateScopePublic})
		assert.NoError(t, err)
	}

	mit := NewManageIpxeTemplate(dbSession, cwu.TestTemporalSiteClientPool(t))

	// Empty success inventory should delete all
	inv := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates:       []*cwssaws.IpxeTemplate{},
	}
	assert.NoError(t, mit.UpdateIpxeTemplatesInDB(ctx, site.ID, inv))

	_, total, err := templateDAO.GetAll(ctx, nil, cdbm.IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(100)})
	assert.NoError(t, err)
	assert.Equal(t, 0, total)
}

func TestManageIpxeTemplate_UnknownSite(t *testing.T) {
	ctx := context.Background()
	_ = config.GetTestConfig()

	dbSession := cwu.TestInitDB(t)
	defer dbSession.Close()
	cwu.TestSetupSchema(t, dbSession)

	mit := NewManageIpxeTemplate(dbSession, cwu.TestTemporalSiteClientPool(t))

	inv := &cwssaws.IpxeTemplateInventory{
		InventoryStatus: cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		Templates:       []*cwssaws.IpxeTemplate{{Id: &cwssaws.IpxeTemplateId{Value: uuid.NewString()}, Name: "kernel-initrd", Scope: cwssaws.IpxeTemplateScope_PUBLIC}},
	}
	err := mit.UpdateIpxeTemplatesInDB(ctx, uuid.New(), inv)
	assert.Error(t, err)
}
