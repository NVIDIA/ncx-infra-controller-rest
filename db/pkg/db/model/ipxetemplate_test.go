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

package model

import (
	"context"
	"testing"

	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/util"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testIpxeTemplateSetupSchema(t *testing.T, dbSession *db.Session) {
	ctx := context.Background()
	require.Nil(t, dbSession.DB.ResetModel(ctx, (*User)(nil)))
	require.Nil(t, dbSession.DB.ResetModel(ctx, (*InfrastructureProvider)(nil)))
	require.Nil(t, dbSession.DB.ResetModel(ctx, (*Site)(nil)))
	require.Nil(t, dbSession.DB.ResetModel(ctx, (*IpxeTemplate)(nil)))

	// Add unique constraint on (site_id, name). This is applied by migration 20260306120000_ipxe_template.go
	// in production; tests use ResetModel so we add it here to match.
	_, err := dbSession.DB.Exec("ALTER TABLE ipxe_template DROP CONSTRAINT IF EXISTS ipxe_template_site_id_name_key")
	require.Nil(t, err)
	_, err = dbSession.DB.Exec("ALTER TABLE ipxe_template ADD CONSTRAINT ipxe_template_site_id_name_key UNIQUE (site_id, name)")
	require.Nil(t, err)
}

func testIpxeTemplateInitDB(t *testing.T) *db.Session {
	return util.GetTestDBSession(t, false)
}

func testIpxeTemplateCreate(ctx context.Context, t *testing.T, dao IpxeTemplateDAO, siteID uuid.UUID, name, scope string) *IpxeTemplate {
	tmpl, err := dao.Create(ctx, nil, IpxeTemplateCreateInput{
		SiteID:            siteID,
		Name:              name,
		RequiredParams:    []string{"kernel_params"},
		ReservedParams:    []string{"base_url", "console"},
		RequiredArtifacts: []string{"kernel", "initrd"},
		Scope:             scope,
	})
	require.NoError(t, err)
	require.NotNil(t, tmpl)
	return tmpl
}

func TestIpxeTemplateSQLDAO_Create(t *testing.T) {
	ctx := context.Background()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()
	testIpxeTemplateSetupSchema(t, dbSession)

	user := TestBuildUser(t, dbSession, "test-user", "test-org", []string{"admin"})
	ip := TestBuildInfrastructureProvider(t, dbSession, "test-provider", "test-org", user)
	site := TestBuildSite(t, dbSession, ip, "test-site", user)

	dao := NewIpxeTemplateDAO(dbSession)

	tests := []struct {
		desc        string
		input       IpxeTemplateCreateInput
		expectError bool
	}{
		{
			desc: "create public template",
			input: IpxeTemplateCreateInput{
				SiteID:            site.ID,
				Name:              "kernel-initrd",
				RequiredParams:    []string{"kernel_params"},
				ReservedParams:    []string{"base_url", "console"},
				RequiredArtifacts: []string{"kernel", "initrd"},
				Scope:             IpxeTemplateScopePublic,
			},
		},
		{
			desc: "create internal template",
			input: IpxeTemplateCreateInput{
				SiteID:            site.ID,
				Name:              "discovery-scout-x86_64",
				RequiredParams:    []string{"mac", "cli_cmd", "machine_id", "server_uri"},
				ReservedParams:    []string{"base_url"},
				RequiredArtifacts: []string{},
				Scope:             IpxeTemplateScopeInternal,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			tmpl, err := dao.Create(ctx, nil, tc.input)
			assert.Equal(t, tc.expectError, err != nil)
			if !tc.expectError {
				require.NotNil(t, tmpl)
				assert.NotEqual(t, uuid.Nil, tmpl.ID)
				assert.Equal(t, tc.input.SiteID, tmpl.SiteID)
				assert.Equal(t, tc.input.Name, tmpl.Name)
				assert.Equal(t, tc.input.Scope, tmpl.Scope)
				assert.Equal(t, tc.input.RequiredParams, tmpl.RequiredParams)
				assert.Equal(t, tc.input.ReservedParams, tmpl.ReservedParams)
				assert.Equal(t, tc.input.RequiredArtifacts, tmpl.RequiredArtifacts)
			}
		})
	}
}

func TestIpxeTemplateSQLDAO_Get(t *testing.T) {
	ctx := context.Background()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()
	testIpxeTemplateSetupSchema(t, dbSession)

	user := TestBuildUser(t, dbSession, "test-user", "test-org", []string{"admin"})
	ip := TestBuildInfrastructureProvider(t, dbSession, "test-provider", "test-org", user)
	site := TestBuildSite(t, dbSession, ip, "test-site", user)

	dao := NewIpxeTemplateDAO(dbSession)
	created := testIpxeTemplateCreate(ctx, t, dao, site.ID, "kernel-initrd", IpxeTemplateScopePublic)

	tests := []struct {
		desc        string
		id          uuid.UUID
		expectError bool
	}{
		{desc: "existing template", id: created.ID},
		{desc: "not found", id: uuid.New(), expectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := dao.Get(ctx, nil, tc.id)
			assert.Equal(t, tc.expectError, err != nil)
			if !tc.expectError {
				require.NotNil(t, got)
				assert.Equal(t, tc.id, got.ID)
				assert.Equal(t, "kernel-initrd", got.Name)
			}
		})
	}
}

func TestIpxeTemplateSQLDAO_GetBySiteAndName(t *testing.T) {
	ctx := context.Background()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()
	testIpxeTemplateSetupSchema(t, dbSession)

	user := TestBuildUser(t, dbSession, "test-user", "test-org", []string{"admin"})
	ip := TestBuildInfrastructureProvider(t, dbSession, "test-provider", "test-org", user)
	site := TestBuildSite(t, dbSession, ip, "test-site", user)

	dao := NewIpxeTemplateDAO(dbSession)
	testIpxeTemplateCreate(ctx, t, dao, site.ID, "ubuntu-autoinstall", IpxeTemplateScopePublic)

	tests := []struct {
		desc        string
		siteID      uuid.UUID
		name        string
		expectError bool
	}{
		{desc: "existing template", siteID: site.ID, name: "ubuntu-autoinstall"},
		{desc: "wrong name", siteID: site.ID, name: "does-not-exist", expectError: true},
		{desc: "wrong site", siteID: uuid.New(), name: "ubuntu-autoinstall", expectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := dao.GetBySiteAndName(ctx, nil, tc.siteID, tc.name)
			assert.Equal(t, tc.expectError, err != nil)
			if !tc.expectError {
				require.NotNil(t, got)
				assert.Equal(t, tc.name, got.Name)
				assert.Equal(t, tc.siteID, got.SiteID)
			}
		})
	}
}

func TestIpxeTemplateSQLDAO_GetAll(t *testing.T) {
	ctx := context.Background()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()
	testIpxeTemplateSetupSchema(t, dbSession)

	user := TestBuildUser(t, dbSession, "test-user", "test-org", []string{"admin"})
	ip := TestBuildInfrastructureProvider(t, dbSession, "test-provider", "test-org", user)
	site := TestBuildSite(t, dbSession, ip, "test-site", user)

	dao := NewIpxeTemplateDAO(dbSession)
	testIpxeTemplateCreate(ctx, t, dao, site.ID, "kernel-initrd", IpxeTemplateScopePublic)
	testIpxeTemplateCreate(ctx, t, dao, site.ID, "ubuntu-autoinstall", IpxeTemplateScopePublic)
	testIpxeTemplateCreate(ctx, t, dao, site.ID, "discovery-scout-x86_64", IpxeTemplateScopeInternal)

	tests := []struct {
		desc          string
		filter        IpxeTemplateFilterInput
		page          paginator.PageInput
		expectedCount int
		expectedTotal *int
	}{
		{desc: "no filter returns all", expectedCount: 3, expectedTotal: db.GetIntPtr(3)},
		{desc: "filter by site", filter: IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site.ID}}, expectedCount: 3},
		{desc: "filter by name", filter: IpxeTemplateFilterInput{Names: []string{"kernel-initrd"}}, expectedCount: 1},
		{desc: "filter by scope public", filter: IpxeTemplateFilterInput{Scopes: []string{IpxeTemplateScopePublic}}, expectedCount: 2},
		{desc: "filter by scope internal", filter: IpxeTemplateFilterInput{Scopes: []string{IpxeTemplateScopeInternal}}, expectedCount: 1},
		{desc: "limit applies", page: paginator.PageInput{Offset: db.GetIntPtr(0), Limit: db.GetIntPtr(2)}, expectedCount: 2, expectedTotal: db.GetIntPtr(3)},
		{desc: "offset applies", page: paginator.PageInput{Offset: db.GetIntPtr(1)}, expectedCount: 2, expectedTotal: db.GetIntPtr(3)},
		{desc: "unknown site returns empty", filter: IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{uuid.New()}}, expectedCount: 0},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, total, err := dao.GetAll(ctx, nil, tc.filter, tc.page)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCount, len(got))
			if tc.expectedTotal != nil {
				assert.Equal(t, *tc.expectedTotal, total)
			}
		})
	}
}

func TestIpxeTemplateSQLDAO_Update(t *testing.T) {
	ctx := context.Background()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()
	testIpxeTemplateSetupSchema(t, dbSession)

	user := TestBuildUser(t, dbSession, "test-user", "test-org", []string{"admin"})
	ip := TestBuildInfrastructureProvider(t, dbSession, "test-provider", "test-org", user)
	site := TestBuildSite(t, dbSession, ip, "test-site", user)

	dao := NewIpxeTemplateDAO(dbSession)
	created := testIpxeTemplateCreate(ctx, t, dao, site.ID, "kernel-initrd", IpxeTemplateScopeInternal)

	// Update scope and params
	updated, err := dao.Update(ctx, nil, IpxeTemplateUpdateInput{
		ID:                created.ID,
		RequiredParams:    []string{"kernel_params", "extra_option"},
		ReservedParams:    []string{"base_url"},
		RequiredArtifacts: []string{"kernel"},
		Scope:             IpxeTemplateScopePublic,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, IpxeTemplateScopePublic, updated.Scope)
	assert.Equal(t, []string{"kernel_params", "extra_option"}, updated.RequiredParams)
	assert.Equal(t, []string{"base_url"}, updated.ReservedParams)
	assert.Equal(t, []string{"kernel"}, updated.RequiredArtifacts)
	// Name and SiteID should be unchanged
	assert.Equal(t, "kernel-initrd", updated.Name)
	assert.Equal(t, site.ID, updated.SiteID)
}

func TestIpxeTemplateSQLDAO_Delete(t *testing.T) {
	ctx := context.Background()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()
	testIpxeTemplateSetupSchema(t, dbSession)

	user := TestBuildUser(t, dbSession, "test-user", "test-org", []string{"admin"})
	ip := TestBuildInfrastructureProvider(t, dbSession, "test-provider", "test-org", user)
	site := TestBuildSite(t, dbSession, ip, "test-site", user)

	dao := NewIpxeTemplateDAO(dbSession)
	t1 := testIpxeTemplateCreate(ctx, t, dao, site.ID, "kernel-initrd", IpxeTemplateScopePublic)
	testIpxeTemplateCreate(ctx, t, dao, site.ID, "ubuntu-autoinstall", IpxeTemplateScopePublic)

	// Delete existing
	err := dao.Delete(ctx, nil, t1.ID)
	require.NoError(t, err)

	// Verify it's gone
	_, err = dao.Get(ctx, nil, t1.ID)
	assert.ErrorIs(t, err, db.ErrDoesNotExist)

	// The other template is unaffected
	remaining, total, err := dao.GetAll(ctx, nil, IpxeTemplateFilterInput{SiteIDs: []uuid.UUID{site.ID}}, paginator.PageInput{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "ubuntu-autoinstall", remaining[0].Name)

	// Delete non-existing ID is a no-op (no error)
	err = dao.Delete(ctx, nil, uuid.New())
	assert.NoError(t, err)
}

func TestIpxeTemplateSQLDAO_UniqueConstraint(t *testing.T) {
	ctx := context.Background()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()
	testIpxeTemplateSetupSchema(t, dbSession)

	user := TestBuildUser(t, dbSession, "test-user", "test-org", []string{"admin"})
	ip := TestBuildInfrastructureProvider(t, dbSession, "test-provider", "test-org", user)
	site := TestBuildSite(t, dbSession, ip, "test-site", user)

	dao := NewIpxeTemplateDAO(dbSession)
	testIpxeTemplateCreate(ctx, t, dao, site.ID, "kernel-initrd", IpxeTemplateScopePublic)

	// Creating the same (name, site) again should fail
	_, err := dao.Create(ctx, nil, IpxeTemplateCreateInput{
		SiteID: site.ID,
		Name:   "kernel-initrd",
		Scope:  IpxeTemplateScopePublic,
	})
	assert.Error(t, err)

	// Same name for a different site is allowed
	site2 := TestBuildSite(t, dbSession, ip, "test-site-2", user)
	_, err = dao.Create(ctx, nil, IpxeTemplateCreateInput{
		SiteID: site2.ID,
		Name:   "kernel-initrd",
		Scope:  IpxeTemplateScopePublic,
	})
	assert.NoError(t, err)
}

func TestIpxeTemplateSQLDAO_DefaultArrayFields(t *testing.T) {
	ctx := context.Background()
	dbSession := testIpxeTemplateInitDB(t)
	defer dbSession.Close()
	testIpxeTemplateSetupSchema(t, dbSession)

	user := TestBuildUser(t, dbSession, "test-user", "test-org", []string{"admin"})
	ip := TestBuildInfrastructureProvider(t, dbSession, "test-provider", "test-org", user)
	site := TestBuildSite(t, dbSession, ip, "test-site", user)

	dao := NewIpxeTemplateDAO(dbSession)

	// Create with no params/artifacts
	created, err := dao.Create(ctx, nil, IpxeTemplateCreateInput{
		SiteID: site.ID,
		Name:   "ipxe-shell",
		Scope:  IpxeTemplateScopeInternal,
	})
	require.NoError(t, err)

	// Retrieve and verify empty arrays are not nil
	retrieved, err := dao.Get(ctx, nil, created.ID)
	require.NoError(t, err)
	assert.NotNil(t, retrieved.RequiredParams)
	assert.NotNil(t, retrieved.ReservedParams)
	assert.NotNil(t, retrieved.RequiredArtifacts)
}
