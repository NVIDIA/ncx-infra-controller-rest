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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/internal/config"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/model"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	cdbu "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/util"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/extra/bundebug"
)

func testIpxeTemplateInitDB(t *testing.T) *cdb.Session {
	dbSession := cdbu.GetTestDBSession(t, false)
	dbSession.DB.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(false),
		bundebug.FromEnv("BUNDEBUG"),
	))
	return dbSession
}

func testIpxeTemplateHandlerSetupSchema(t *testing.T, dbSession *cdb.Session) {
	ctx := context.Background()

	assert.Nil(t, dbSession.DB.ResetModel(ctx, (*cdbm.TenantAccount)(nil)))
	assert.Nil(t, dbSession.DB.ResetModel(ctx, (*cdbm.IpxeTemplate)(nil)))
	assert.Nil(t, dbSession.DB.ResetModel(ctx, (*cdbm.Tenant)(nil)))
	assert.Nil(t, dbSession.DB.ResetModel(ctx, (*cdbm.Site)(nil)))
	assert.Nil(t, dbSession.DB.ResetModel(ctx, (*cdbm.InfrastructureProvider)(nil)))
}

type ipxeTemplateTestFixture struct {
	ip    *cdbm.InfrastructureProvider
	site  *cdbm.Site
	tmpl1 *cdbm.IpxeTemplate
	tmpl2 *cdbm.IpxeTemplate
}

func testIpxeTemplateSetupTestData(t *testing.T, dbSession *cdb.Session, org string) *ipxeTemplateTestFixture {
	ctx := context.Background()

	ip := &cdbm.InfrastructureProvider{
		ID:   uuid.New(),
		Name: "test-provider",
		Org:  org,
	}
	_, err := dbSession.DB.NewInsert().Model(ip).Exec(ctx)
	assert.Nil(t, err)

	site := &cdbm.Site{
		ID:                       uuid.New(),
		Name:                     "test-site",
		Org:                      org,
		InfrastructureProviderID: ip.ID,
		Status:                   cdbm.SiteStatusRegistered,
	}
	_, err = dbSession.DB.NewInsert().Model(site).Exec(ctx)
	assert.Nil(t, err)

	dao := cdbm.NewIpxeTemplateDAO(dbSession)

	tmpl1, err := dao.Create(ctx, nil, cdbm.IpxeTemplateCreateInput{
		SiteID: site.ID, Name: "kernel-initrd", Scope: cdbm.IpxeTemplateScopePublic,
		RequiredParams: []string{"kernel_params"}, ReservedParams: []string{"base_url"}, RequiredArtifacts: []string{"kernel"},
	})
	assert.Nil(t, err)

	tmpl2, err := dao.Create(ctx, nil, cdbm.IpxeTemplateCreateInput{
		SiteID: site.ID, Name: "ubuntu-autoinstall", Scope: cdbm.IpxeTemplateScopePublic,
		RequiredParams: []string{}, ReservedParams: []string{}, RequiredArtifacts: []string{"iso"},
	})
	assert.Nil(t, err)

	return &ipxeTemplateTestFixture{ip: ip, site: site, tmpl1: tmpl1, tmpl2: tmpl2}
}

func createIpxeTemplateMockUser(org string) *cdbm.User {
	return &cdbm.User{
		StarfleetID: cdb.GetStrPtr("test-user"),
		OrgData: cdbm.OrgData{
			org: cdbm.Org{
				ID:          123,
				Name:        org,
				DisplayName: org,
				OrgType:     "ENTERPRISE",
				Roles:       []string{"FORGE_PROVIDER_VIEWER"},
			},
		},
	}
}

func createIpxeTemplateTenantMockUser(org string) *cdbm.User {
	return &cdbm.User{
		StarfleetID: cdb.GetStrPtr("test-tenant-user"),
		OrgData: cdbm.OrgData{
			org: cdbm.Org{
				ID:          456,
				Name:        org,
				DisplayName: org,
				OrgType:     "ENTERPRISE",
				Roles:       []string{"FORGE_TENANT_ADMIN"},
			},
		},
	}
}

func createIpxeTemplateMixedRoleMockUser(org string) *cdbm.User {
	return &cdbm.User{
		StarfleetID: cdb.GetStrPtr("test-mixed-user"),
		OrgData: cdbm.OrgData{
			org: cdbm.Org{
				ID:          789,
				Name:        org,
				DisplayName: org,
				OrgType:     "ENTERPRISE",
				Roles:       []string{"FORGE_PROVIDER_VIEWER", "FORGE_TENANT_ADMIN"},
			},
		},
	}
}

func TestGetAllIpxeTemplateHandler_Handle(t *testing.T) {
	e := echo.New()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()

	testIpxeTemplateHandlerSetupSchema(t, dbSession)

	ctx := context.Background()
	cfg := &config.Config{}
	handler := NewGetAllIpxeTemplateHandler(dbSession, nil, cfg)

	org := "test-org"
	fix := testIpxeTemplateSetupTestData(t, dbSession, org)

	// Unmanaged site in a different org
	unmanagedIP := &cdbm.InfrastructureProvider{ID: uuid.New(), Name: "unmanaged-provider", Org: "other-org"}
	_, err := dbSession.DB.NewInsert().Model(unmanagedIP).Exec(ctx)
	assert.Nil(t, err)

	unmanagedSite := &cdbm.Site{ID: uuid.New(), Name: "unmanaged-site", Org: "other-org", InfrastructureProviderID: unmanagedIP.ID, Status: cdbm.SiteStatusRegistered}
	_, err = dbSession.DB.NewInsert().Model(unmanagedSite).Exec(ctx)
	assert.Nil(t, err)

	// Tenant with TargetedInstanceCreation capability and a TenantAccount
	tenantOrg := "test-tenant-org"
	tenantWithCapability := &cdbm.Tenant{ID: uuid.New(), Name: "test-tenant", Org: tenantOrg, Config: &cdbm.TenantConfig{TargetedInstanceCreation: true}}
	_, err = dbSession.DB.NewInsert().Model(tenantWithCapability).Exec(ctx)
	assert.Nil(t, err)

	tenantAccount := &cdbm.TenantAccount{ID: uuid.New(), AccountNumber: "ta-123", TenantID: &tenantWithCapability.ID, TenantOrg: tenantOrg, InfrastructureProviderID: fix.ip.ID, Status: "active"}
	_, err = dbSession.DB.NewInsert().Model(tenantAccount).Exec(ctx)
	assert.Nil(t, err)

	// Tenant without capability
	tenantOrgNoCapability := "test-tenant-no-capability"
	tenantWithoutCapability := &cdbm.Tenant{ID: uuid.New(), Name: "no-cap-tenant", Org: tenantOrgNoCapability, Config: &cdbm.TenantConfig{TargetedInstanceCreation: false}}
	_, err = dbSession.DB.NewInsert().Model(tenantWithoutCapability).Exec(ctx)
	assert.Nil(t, err)

	// Tenant with capability but no TenantAccount
	tenantOrgNoAccount := "test-tenant-no-account"
	tenantWithoutAccount := &cdbm.Tenant{ID: uuid.New(), Name: "no-account-tenant", Org: tenantOrgNoAccount, Config: &cdbm.TenantConfig{TargetedInstanceCreation: true}}
	_, err = dbSession.DB.NewInsert().Model(tenantWithoutAccount).Exec(ctx)
	assert.Nil(t, err)

	// Mixed-role org: has both a provider (different from fix.ip) and a privileged tenant
	// with a TenantAccount linked to fix.ip. A mixed-role user in this org should fail
	// the provider check (site belongs to fix.ip, not mixedIP) but pass via the tenant path.
	mixedOrg := "mixed-role-org"
	mixedIP := &cdbm.InfrastructureProvider{ID: uuid.New(), Name: "mixed-provider", Org: mixedOrg}
	_, err = dbSession.DB.NewInsert().Model(mixedIP).Exec(ctx)
	assert.Nil(t, err)

	mixedTenant := &cdbm.Tenant{ID: uuid.New(), Name: "mixed-tenant", Org: mixedOrg, Config: &cdbm.TenantConfig{TargetedInstanceCreation: true}}
	_, err = dbSession.DB.NewInsert().Model(mixedTenant).Exec(ctx)
	assert.Nil(t, err)

	mixedTA := &cdbm.TenantAccount{ID: uuid.New(), AccountNumber: "ta-mixed", TenantID: &mixedTenant.ID, TenantOrg: mixedOrg, InfrastructureProviderID: fix.ip.ID, Status: "active"}
	_, err = dbSession.DB.NewInsert().Model(mixedTA).Exec(ctx)
	assert.Nil(t, err)

	_ = fix.tmpl1
	_ = fix.tmpl2
	_ = tenantAccount
	_ = tenantWithoutCapability
	_ = tenantWithoutAccount

	tests := []struct {
		name                 string
		siteID               string
		scope                string
		setupContext         func(c echo.Context)
		expectedStatus       int
		checkResponseContent func(t *testing.T, body []byte)
	}{
		{
			name:   "missing siteId returns bad request",
			siteID: "",
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMockUser(org))
				c.SetParamNames("orgName")
				c.SetParamValues(org)
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "successful GetAll with valid siteId",
			siteID: fix.site.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMockUser(org))
				c.SetParamNames("orgName")
				c.SetParamValues(org)
			},
			expectedStatus: http.StatusOK,
			checkResponseContent: func(t *testing.T, body []byte) {
				var response []*model.APIIpxeTemplate
				assert.Nil(t, json.Unmarshal(body, &response))
				assert.Len(t, response, 2)
				for _, tmpl := range response {
					assert.Equal(t, fix.site.ID.String(), tmpl.SiteID)
				}
			},
		},
		{
			name:   "filter by scope returns subset",
			siteID: fix.site.ID.String(),
			scope:  cdbm.IpxeTemplateScopePublic,
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMockUser(org))
				c.SetParamNames("orgName")
				c.SetParamValues(org)
			},
			expectedStatus: http.StatusOK,
			checkResponseContent: func(t *testing.T, body []byte) {
				var response []*model.APIIpxeTemplate
				assert.Nil(t, json.Unmarshal(body, &response))
				assert.Len(t, response, 2)
			},
		},
		{
			name:   "cannot retrieve from unmanaged site",
			siteID: unmanagedSite.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMockUser(org))
				c.SetParamNames("orgName")
				c.SetParamValues(org)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "missing user context returns 500",
			siteID: "",
			setupContext: func(c echo.Context) {
				c.SetParamNames("orgName")
				c.SetParamValues(org)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "tenant with TargetedInstanceCreation can retrieve templates",
			siteID: fix.site.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateTenantMockUser(tenantOrg))
				c.SetParamNames("orgName")
				c.SetParamValues(tenantOrg)
			},
			expectedStatus: http.StatusOK,
			checkResponseContent: func(t *testing.T, body []byte) {
				var response []*model.APIIpxeTemplate
				assert.Nil(t, json.Unmarshal(body, &response))
				assert.Len(t, response, 2)
			},
		},
		{
			name:   "tenant without TargetedInstanceCreation is denied",
			siteID: fix.site.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateTenantMockUser(tenantOrgNoCapability))
				c.SetParamNames("orgName")
				c.SetParamValues(tenantOrgNoCapability)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "tenant without TenantAccount with Provider is denied",
			siteID: fix.site.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateTenantMockUser(tenantOrgNoAccount))
				c.SetParamNames("orgName")
				c.SetParamValues(tenantOrgNoAccount)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "mixed-role user fails provider check but passes tenant authorization",
			siteID: fix.site.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMixedRoleMockUser(mixedOrg))
				c.SetParamNames("orgName")
				c.SetParamValues(mixedOrg)
			},
			expectedStatus: http.StatusOK,
			checkResponseContent: func(t *testing.T, body []byte) {
				var response []*model.APIIpxeTemplate
				assert.Nil(t, json.Unmarshal(body, &response))
				assert.Len(t, response, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/v2/org/" + org + "/carbide/ipxe-template"
			if tt.siteID != "" {
				url += "?siteId=" + tt.siteID
			}
			if tt.scope != "" {
				if tt.siteID != "" {
					url += "&scope=" + tt.scope
				} else {
					url += "?scope=" + tt.scope
				}
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req = req.WithContext(context.Background())
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			tt.setupContext(c)

			err := handler.Handle(c)
			assert.Nil(t, err)
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedStatus != rec.Code {
				t.Errorf("Response: %v", rec.Body.String())
			}
			if tt.checkResponseContent != nil && rec.Code == http.StatusOK {
				tt.checkResponseContent(t, rec.Body.Bytes())
			}
		})
	}
}

func TestGetIpxeTemplateHandler_Handle(t *testing.T) {
	e := echo.New()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()

	testIpxeTemplateHandlerSetupSchema(t, dbSession)

	ctx := context.Background()
	cfg := &config.Config{}
	handler := NewGetIpxeTemplateHandler(dbSession, nil, cfg)

	org := "test-org"
	fix := testIpxeTemplateSetupTestData(t, dbSession, org)

	// Unmanaged site in a different org
	unmanagedIP := &cdbm.InfrastructureProvider{ID: uuid.New(), Name: "unmanaged-provider-get", Org: "other-org"}
	_, err := dbSession.DB.NewInsert().Model(unmanagedIP).Exec(ctx)
	assert.Nil(t, err)

	unmanagedSite := &cdbm.Site{ID: uuid.New(), Name: "unmanaged-site-get", Org: "other-org", InfrastructureProviderID: unmanagedIP.ID, Status: cdbm.SiteStatusRegistered}
	_, err = dbSession.DB.NewInsert().Model(unmanagedSite).Exec(ctx)
	assert.Nil(t, err)

	unmanagedTmpl, err := cdbm.NewIpxeTemplateDAO(dbSession).Create(ctx, nil, cdbm.IpxeTemplateCreateInput{
		SiteID: unmanagedSite.ID, Name: "unmanaged-tmpl", Scope: cdbm.IpxeTemplateScopePublic,
	})
	assert.Nil(t, err)

	// Tenant with capability and account
	tenantOrg := "test-tenant-org"
	tenantWithCapability := &cdbm.Tenant{ID: uuid.New(), Name: "test-tenant", Org: tenantOrg, Config: &cdbm.TenantConfig{TargetedInstanceCreation: true}}
	_, err = dbSession.DB.NewInsert().Model(tenantWithCapability).Exec(ctx)
	assert.Nil(t, err)

	tenantAccount := &cdbm.TenantAccount{ID: uuid.New(), AccountNumber: "ta-456", TenantID: &tenantWithCapability.ID, TenantOrg: tenantOrg, InfrastructureProviderID: fix.ip.ID, Status: "active"}
	_, err = dbSession.DB.NewInsert().Model(tenantAccount).Exec(ctx)
	assert.Nil(t, err)

	// Tenant without capability
	tenantOrgNoCapability := "test-tenant-no-capability-get"
	tenantWithoutCapability := &cdbm.Tenant{ID: uuid.New(), Name: "no-cap-tenant-get", Org: tenantOrgNoCapability, Config: &cdbm.TenantConfig{TargetedInstanceCreation: false}}
	_, err = dbSession.DB.NewInsert().Model(tenantWithoutCapability).Exec(ctx)
	assert.Nil(t, err)

	// Tenant with capability but no account
	tenantOrgNoAccount := "test-tenant-no-account-get"
	tenantWithoutAccount := &cdbm.Tenant{ID: uuid.New(), Name: "no-account-tenant-get", Org: tenantOrgNoAccount, Config: &cdbm.TenantConfig{TargetedInstanceCreation: true}}
	_, err = dbSession.DB.NewInsert().Model(tenantWithoutAccount).Exec(ctx)
	assert.Nil(t, err)

	// Mixed-role org: provider check fails (site belongs to fix.ip), tenant path succeeds
	mixedOrg := "mixed-role-org-get"
	mixedIP := &cdbm.InfrastructureProvider{ID: uuid.New(), Name: "mixed-provider-get", Org: mixedOrg}
	_, err = dbSession.DB.NewInsert().Model(mixedIP).Exec(ctx)
	assert.Nil(t, err)

	mixedTenant := &cdbm.Tenant{ID: uuid.New(), Name: "mixed-tenant-get", Org: mixedOrg, Config: &cdbm.TenantConfig{TargetedInstanceCreation: true}}
	_, err = dbSession.DB.NewInsert().Model(mixedTenant).Exec(ctx)
	assert.Nil(t, err)

	mixedTA := &cdbm.TenantAccount{ID: uuid.New(), AccountNumber: "ta-mixed-get", TenantID: &mixedTenant.ID, TenantOrg: mixedOrg, InfrastructureProviderID: fix.ip.ID, Status: "active"}
	_, err = dbSession.DB.NewInsert().Model(mixedTA).Exec(ctx)
	assert.Nil(t, err)

	_ = tenantAccount
	_ = tenantWithoutCapability
	_ = tenantWithoutAccount

	tests := []struct {
		name                 string
		templateID           string
		setupContext         func(c echo.Context)
		expectedStatus       int
		checkResponseContent func(t *testing.T, body []byte)
	}{
		{
			name:       "successful retrieval",
			templateID: fix.tmpl1.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMockUser(org))
				c.SetParamNames("orgName", "id")
				c.SetParamValues(org, fix.tmpl1.ID.String())
			},
			expectedStatus: http.StatusOK,
			checkResponseContent: func(t *testing.T, body []byte) {
				var response model.APIIpxeTemplate
				assert.Nil(t, json.Unmarshal(body, &response))
				assert.Equal(t, fix.tmpl1.ID.String(), response.ID)
				assert.Equal(t, fix.site.ID.String(), response.SiteID)
				assert.Equal(t, "kernel-initrd", response.Name)
			},
		},
		{
			name:       "template not found",
			templateID: uuid.New().String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMockUser(org))
				c.SetParamNames("orgName", "id")
				c.SetParamValues(org, uuid.New().String())
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:       "invalid uuid returns bad request",
			templateID: "not-a-uuid",
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMockUser(org))
				c.SetParamNames("orgName", "id")
				c.SetParamValues(org, "not-a-uuid")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "cannot retrieve from unmanaged site",
			templateID: unmanagedTmpl.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMockUser(org))
				c.SetParamNames("orgName", "id")
				c.SetParamValues(org, unmanagedTmpl.ID.String())
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:       "missing user context returns 500",
			templateID: fix.tmpl1.ID.String(),
			setupContext: func(c echo.Context) {
				c.SetParamNames("orgName", "id")
				c.SetParamValues(org, fix.tmpl1.ID.String())
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:       "tenant with TargetedInstanceCreation can retrieve template",
			templateID: fix.tmpl1.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateTenantMockUser(tenantOrg))
				c.SetParamNames("orgName", "id")
				c.SetParamValues(tenantOrg, fix.tmpl1.ID.String())
			},
			expectedStatus: http.StatusOK,
			checkResponseContent: func(t *testing.T, body []byte) {
				var response model.APIIpxeTemplate
				assert.Nil(t, json.Unmarshal(body, &response))
				assert.Equal(t, fix.tmpl1.ID.String(), response.ID)
			},
		},
		{
			name:       "tenant without TargetedInstanceCreation is denied",
			templateID: fix.tmpl1.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateTenantMockUser(tenantOrgNoCapability))
				c.SetParamNames("orgName", "id")
				c.SetParamValues(tenantOrgNoCapability, fix.tmpl1.ID.String())
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:       "tenant without TenantAccount with Provider is denied",
			templateID: fix.tmpl1.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateTenantMockUser(tenantOrgNoAccount))
				c.SetParamNames("orgName", "id")
				c.SetParamValues(tenantOrgNoAccount, fix.tmpl1.ID.String())
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:       "mixed-role user fails provider check but passes tenant authorization",
			templateID: fix.tmpl1.ID.String(),
			setupContext: func(c echo.Context) {
				c.Set("user", createIpxeTemplateMixedRoleMockUser(mixedOrg))
				c.SetParamNames("orgName", "id")
				c.SetParamValues(mixedOrg, fix.tmpl1.ID.String())
			},
			expectedStatus: http.StatusOK,
			checkResponseContent: func(t *testing.T, body []byte) {
				var response model.APIIpxeTemplate
				assert.Nil(t, json.Unmarshal(body, &response))
				assert.Equal(t, fix.tmpl1.ID.String(), response.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/v2/org/" + org + "/carbide/ipxe-template/" + tt.templateID
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req = req.WithContext(context.Background())
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			tt.setupContext(c)

			err := handler.Handle(c)
			assert.Nil(t, err)
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedStatus != rec.Code {
				t.Errorf("Response: %v", rec.Body.String())
			}
			if tt.checkResponseContent != nil && rec.Code == http.StatusOK {
				tt.checkResponseContent(t, rec.Body.Bytes())
			}
		})
	}
}
