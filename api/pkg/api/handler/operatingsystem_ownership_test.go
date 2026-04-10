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

// This file extends operatingsystem_test.go with tests that validate the
// ownership model (TenantID vs InfrastructureProviderID) and role-based
// access-control enforcement introduced alongside the bi-directional sync
// feature.  Each test function is self-contained and uses a fresh schema.
//
// Roles under test
//   - ipUser  — FORGE_PROVIDER_ADMIN only
//   - tnUser  — FORGE_TENANT_ADMIN only
//   - dualUser — both roles (either role may authorize the operation)
//
// Ownership invariants verified
//   - Provider Admin → InfrastructureProviderID set, TenantID nil
//   - Tenant Admin  → TenantID set, InfrastructureProviderID nil
//   - Dual-role     → permitted if either role authorizes the action;
//                      when both allow it, Provider Admin takes priority
//                      for ownership assignment
//
// Cross-ownership visibility (GetAll / GetByID)
//   - Any user whose org has both a Tenant and an InfrastructureProvider
//     can see OSes owned by either entity.
//
// Mutation enforcement (Update / Delete)
//   - Provider Admin can mutate only provider-owned OSes.
//   - Tenant Admin can mutate only tenant-owned OSes.
//   - Dual-role user is permitted if either role authorizes the action.

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/handler/util/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/model"
	sc "github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/client/site"
	"github.com/NVIDIA/ncx-infra-controller-rest/common/pkg/otelecho"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	tmocks "go.temporal.io/sdk/mocks"
)

// ─── shared setup helpers ────────────────────────────────────────────────────

// ownershipTestEnv contains all DB fixtures that the ownership-related tests
// share.  Each test function calls newOwnershipTestEnv and receives a freshly
// reset schema.
type ownershipTestEnv struct {
	dbSession *cdb.Session
	// Shared org that has both an InfrastructureProvider and a Tenant.
	// Users in this org can carry either or both roles.
	sharedOrg string
	ip        *cdbm.InfrastructureProvider
	tenant    *cdbm.Tenant
	site      *cdbm.Site // registered site belonging to ip

	// Users
	ipUser   *cdbm.User // FORGE_PROVIDER_ADMIN only
	tnUser   *cdbm.User // FORGE_TENANT_ADMIN only
	dualUser *cdbm.User // both roles

	// Temporal mocks (permissive — match any workflow invocation)
	tempClient *tmocks.Client
	scp        *sc.ClientPool
}

func newOwnershipTestEnv(t *testing.T) *ownershipTestEnv {
	t.Helper()

	dbSession := testMachineInitDB(t)
	t.Cleanup(func() { dbSession.Close() })
	common.TestSetupSchema(t, dbSession)

	sharedOrg := "shared-org"

	ip := testMachineBuildInfrastructureProvider(t, dbSession, sharedOrg, "shared-ip")
	require.NotNil(t, ip)

	tenant := testMachineBuildTenant(t, dbSession, sharedOrg, "shared-tenant")
	require.NotNil(t, tenant)

	site := testMachineBuildSite(t, dbSession, ip, "shared-site", cdbm.SiteStatusRegistered)
	require.NotNil(t, site)

	// TenantSite so tenant users can reference the site.
	tnu := testMachineBuildUser(t, dbSession, uuid.NewString(),
		[]string{sharedOrg}, []string{"FORGE_TENANT_ADMIN"})
	ts := testBuildTenantSiteAssociation(t, dbSession, sharedOrg, tenant.ID, site.ID, tnu.ID)
	require.NotNil(t, ts)

	ipUser := testMachineBuildUser(t, dbSession, uuid.NewString(),
		[]string{sharedOrg}, []string{"FORGE_PROVIDER_ADMIN"})
	dualUser := testMachineBuildUser(t, dbSession, uuid.NewString(),
		[]string{sharedOrg}, []string{"FORGE_PROVIDER_ADMIN", "FORGE_TENANT_ADMIN"})

	// Permissive Temporal mock: accepts any ExecuteWorkflow call so that tests
	// exercising the success path don't have to enumerate every signature.
	wrun := &tmocks.WorkflowRun{}
	wrun.On("GetID").Return("test-wf-id")
	wrun.On("Get", mock.Anything, mock.Anything).Return(nil)

	tempClient := &tmocks.Client{}
	tempClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).Return(wrun, nil)

	cfg := common.GetTestConfig()
	tcfg, _ := cfg.GetTemporalConfig()
	scp := sc.NewClientPool(tcfg)

	// Per-site Temporal client (permissive).
	siteMock := &tmocks.Client{}
	siteMock.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(wrun, nil)
	siteMock.On("TerminateWorkflow", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(nil)
	scp.IDClientMap[site.ID.String()] = siteMock

	return &ownershipTestEnv{
		dbSession:  dbSession,
		sharedOrg:  sharedOrg,
		ip:         ip,
		tenant:     tenant,
		site:       site,
		ipUser:     ipUser,
		tnUser:     tnu,
		dualUser:   dualUser,
		tempClient: tempClient,
		scp:        scp,
	}
}

// execCreateOS posts a Create request and returns the response recorder.
func (e *ownershipTestEnv) execCreateOS(t *testing.T, user *cdbm.User, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	tracer, traceCtx := otelTraceCtx(t)

	eh := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(rawBody)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	ec := eh.NewContext(req, rec)
	ec.SetParamNames("orgName")
	ec.SetParamValues(e.sharedOrg)
	ec.Set("user", user)
	ec.SetRequest(ec.Request().WithContext(
		context.WithValue(traceCtx, otelecho.TracerKey, tracer),
	))

	cfg := common.GetTestConfig()
	h := CreateOperatingSystemHandler{
		dbSession: e.dbSession,
		tc:        e.tempClient,
		cfg:       cfg,
		scp:       e.scp,
	}
	require.NoError(t, h.Handle(ec))
	return rec
}

// otelTraceCtx returns a no-op tracer and a context for use in handler tests.
func otelTraceCtx(t *testing.T) (interface{}, context.Context) {
	t.Helper()
	tracer, _, ctx := common.TestCommonTraceProviderSetup(t, context.Background())
	return tracer, ctx
}

// ─── Create: ownership assignment ────────────────────────────────────────────

// TestOperatingSystemHandler_Create_ProviderAndTenantOwnership verifies that
// the Create handler assigns ownership correctly based on the caller's role:
//
//   - Provider Admin → InfrastructureProviderID = provider's ID, TenantID = nil
//   - Tenant Admin   → TenantID = tenant's ID, InfrastructureProviderID = nil
//   - Dual-role user → permitted if either role authorizes the action;
//     when both allow it, provider ownership takes priority
//
// The test also covers the "new" iPXE OS type (template-based with parameters
// and artifacts) to ensure those fields round-trip correctly.
func TestOperatingSystemHandler_Create_ProviderAndTenantOwnership(t *testing.T) {
	env := newOwnershipTestEnv(t)

	ipxeScript := "ipxe-script-content"
	templateName := "raw-ipxe"
	scopeGlobal := cdbm.OperatingSystemScopeGlobal

	// template-based request reused for several sub-tests.
	templateBody := model.APIOperatingSystemCreateRequest{
		Name:             "tmpl-os-" + uuid.NewString(),
		IpxeTemplateId: &templateName,
		Scope:            &scopeGlobal,
		IpxeParameters: []cdbm.OperatingSystemIpxeParameter{
			{Name: "kernel_params", Value: "ip=dhcp"},
		},
		IpxeArtifacts: []cdbm.OperatingSystemIpxeArtifact{
			{Name: "kernel", URL: "http://files.example.com/vmlinuz", CacheStrategy: "CACHE_AS_NEEDED"},
		},
	}

	tests := []struct {
		name            string
		user            *cdbm.User
		body            model.APIOperatingSystemCreateRequest
		wantStatus      int
		wantProviderID  *uuid.UUID // nil means we don't assert
		wantTenantID    *uuid.UUID // nil means we don't assert
		wantProviderNil bool
		wantTenantNil   bool
	}{
		{
			name: "provider admin raw iPXE → forbidden (must use template)",
			user: env.ipUser,
			body: model.APIOperatingSystemCreateRequest{
				Name:       "prov-ipxe-" + uuid.NewString(),
				IpxeScript: &ipxeScript,
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:           "provider admin template iPXE → provider-owned",
			user:           env.ipUser,
			body:           templateBody,
			wantStatus:     http.StatusCreated,
			wantProviderID: &env.ip.ID,
			wantTenantNil:  true,
		},
		{
			name: "tenant admin raw iPXE → tenant-owned",
			user: env.tnUser,
			body: model.APIOperatingSystemCreateRequest{
				Name:       "tn-ipxe-" + uuid.NewString(),
				IpxeScript: &ipxeScript,
			},
			wantStatus:      http.StatusCreated,
			wantTenantID:    &env.tenant.ID,
			wantProviderNil: true,
		},
		{
			name: "tenant admin template iPXE → tenant-owned",
			user: env.tnUser,
			body: model.APIOperatingSystemCreateRequest{
				Name:             "tn-tmpl-" + uuid.NewString(),
				IpxeTemplateId: &templateName,
				Scope:            &scopeGlobal,
				IpxeParameters: []cdbm.OperatingSystemIpxeParameter{
					{Name: "kernel_params", Value: "ip=dhcp"},
				},
				IpxeArtifacts: []cdbm.OperatingSystemIpxeArtifact{
					{Name: "kernel", URL: "http://files.example.com/vmlinuz", CacheStrategy: "CACHE_AS_NEEDED"},
				},
			},
			wantStatus:      http.StatusCreated,
			wantTenantID:    &env.tenant.ID,
			wantProviderNil: true,
		},
		{
			name: "dual-role user raw iPXE → tenant-owned (tenant role authorizes)",
			user: env.dualUser,
			body: model.APIOperatingSystemCreateRequest{
				Name:       "dual-ipxe-" + uuid.NewString(),
				IpxeScript: &ipxeScript,
			},
			wantStatus:      http.StatusCreated,
			wantTenantID:    &env.tenant.ID,
			wantProviderNil: true,
		},
		{
			name: "dual-role user template iPXE → provider-owned",
			user: env.dualUser,
			body: model.APIOperatingSystemCreateRequest{
				Name:             "dual-tmpl-" + uuid.NewString(),
				IpxeTemplateId: &templateName,
				Scope:            &scopeGlobal,
				IpxeParameters: []cdbm.OperatingSystemIpxeParameter{
					{Name: "kernel_params", Value: "ip=dhcp"},
				},
				IpxeArtifacts: []cdbm.OperatingSystemIpxeArtifact{
					{Name: "kernel", URL: "http://files.example.com/vmlinuz", CacheStrategy: "CACHE_AS_NEEDED"},
				},
			},
			wantStatus:     http.StatusCreated,
			wantProviderID: &env.ip.ID,
			wantTenantNil:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.execCreateOS(t, tc.user, tc.body)
			assert.Equal(t, tc.wantStatus, rec.Code, "response body: %s", rec.Body.String())
			if rec.Code != http.StatusCreated {
				return
			}

			var rsp model.APIOperatingSystem
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rsp))

			if tc.wantProviderID != nil {
				require.NotNil(t, rsp.InfrastructureProviderID,
					"expected InfrastructureProviderID to be set")
				assert.Equal(t, tc.wantProviderID.String(), *rsp.InfrastructureProviderID)
			}
			if tc.wantProviderNil {
				assert.Nil(t, rsp.InfrastructureProviderID,
					"expected InfrastructureProviderID to be nil")
			}
			if tc.wantTenantID != nil {
				require.NotNil(t, rsp.TenantID, "expected TenantID to be set")
				assert.Equal(t, tc.wantTenantID.String(), *rsp.TenantID)
			}
			if tc.wantTenantNil {
				assert.Nil(t, rsp.TenantID, "expected TenantID to be nil")
			}

			// Verify iPXE parameters and artifacts round-trip for template OS.
			if tc.body.IpxeTemplateId != nil {
				assert.Equal(t, tc.body.IpxeTemplateId, rsp.IpxeTemplateId)
				if len(tc.body.IpxeParameters) > 0 {
					require.Len(t, rsp.IpxeParameters, len(tc.body.IpxeParameters))
					assert.Equal(t, tc.body.IpxeParameters[0].Name, rsp.IpxeParameters[0].Name)
				}
				if len(tc.body.IpxeArtifacts) > 0 {
					require.Len(t, rsp.IpxeArtifacts, len(tc.body.IpxeArtifacts))
					assert.Equal(t, tc.body.IpxeArtifacts[0].Name, rsp.IpxeArtifacts[0].Name)
					assert.Equal(t, tc.body.IpxeArtifacts[0].URL, rsp.IpxeArtifacts[0].URL)
				}
			}
		})
	}
}

// ─── GetAll: cross-ownership visibility ──────────────────────────────────────

// TestOperatingSystemHandler_GetAll_CrossOwnership verifies that any user whose
// org resolves both a Tenant and an InfrastructureProvider can see OSes owned
// by either entity in a single GetAll call.
func TestOperatingSystemHandler_GetAll_CrossOwnership(t *testing.T) {
	env := newOwnershipTestEnv(t)
	ctx := context.Background()

	osDAO := cdbm.NewOperatingSystemDAO(env.dbSession)

	// Seed one provider-owned OS.
	provOS, err := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
		Name:                     "synced-from-core",
		Org:                      env.sharedOrg,
		TenantID:                 nil,
		InfrastructureProviderID: &env.ip.ID,
		OsType:                   cdbm.OperatingSystemTypeIPXE,
		IpxeScript:               cdb.GetStrPtr("#!ipxe\nchain http://boot.example.com"),
		Status:                   cdbm.OperatingSystemStatusReady,
		CreatedBy:                env.ipUser.ID,
	})
	require.NoError(t, err)

	// Seed one tenant-owned OS.
	tnOS, err := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
		Name:       "tenant-os",
		Org:        env.sharedOrg,
		TenantID:   &env.tenant.ID,
		OsType:     cdbm.OperatingSystemTypeIPXE,
		IpxeScript: cdb.GetStrPtr("#!ipxe\nboot"),
		Status:     cdbm.OperatingSystemStatusReady,
		CreatedBy:  env.tnUser.ID,
	})
	require.NoError(t, err)

	_ = provOS
	_ = tnOS

	execGetAll := func(t *testing.T, user *cdbm.User) []model.APIOperatingSystem {
		t.Helper()
		tracer, ctx := otelTraceCtx(t)

		eh := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		ec := eh.NewContext(req, rec)
		ec.SetParamNames("orgName")
		ec.SetParamValues(env.sharedOrg)
		ec.Set("user", user)
		ec.SetRequest(ec.Request().WithContext(
			context.WithValue(ctx, otelecho.TracerKey, tracer),
		))

		cfg := common.GetTestConfig()
		h := GetAllOperatingSystemHandler{
			dbSession: env.dbSession,
			tc:        env.tempClient,
			cfg:       cfg,
		}
		require.NoError(t, h.Handle(ec))
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var rsp []model.APIOperatingSystem
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rsp))
		return rsp
	}

	t.Run("provider admin sees both provider-owned and tenant-owned OSes", func(t *testing.T) {
		oss := execGetAll(t, env.ipUser)
		assert.GreaterOrEqual(t, len(oss), 2, "expected at least the two seeded OSes")
		ids := make([]string, len(oss))
		for i, o := range oss {
			ids[i] = o.ID
		}
		assert.Contains(t, ids, provOS.ID.String())
		assert.Contains(t, ids, tnOS.ID.String())
	})

	t.Run("tenant admin sees both provider-owned and tenant-owned OSes", func(t *testing.T) {
		oss := execGetAll(t, env.tnUser)
		assert.GreaterOrEqual(t, len(oss), 2)
		ids := make([]string, len(oss))
		for i, o := range oss {
			ids[i] = o.ID
		}
		assert.Contains(t, ids, provOS.ID.String())
		assert.Contains(t, ids, tnOS.ID.String())
	})

	t.Run("dual-role user sees both provider-owned and tenant-owned OSes", func(t *testing.T) {
		oss := execGetAll(t, env.dualUser)
		assert.GreaterOrEqual(t, len(oss), 2)
	})
}

// ─── GetByID: cross-ownership visibility ─────────────────────────────────────

// TestOperatingSystemHandler_GetByID_CrossOwnership verifies that any user
// whose org has both a Tenant and a Provider can fetch an individual OS
// regardless of which entity owns it.
func TestOperatingSystemHandler_GetByID_CrossOwnership(t *testing.T) {
	env := newOwnershipTestEnv(t)
	ctx := context.Background()

	osDAO := cdbm.NewOperatingSystemDAO(env.dbSession)

	provOS, err := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
		Name:                     "prov-single",
		Org:                      env.sharedOrg,
		TenantID:                 nil,
		InfrastructureProviderID: &env.ip.ID,
		OsType:                   cdbm.OperatingSystemTypeIPXE,
		IpxeScript:               cdb.GetStrPtr("#!ipxe"),
		Status:                   cdbm.OperatingSystemStatusReady,
		CreatedBy:                env.ipUser.ID,
	})
	require.NoError(t, err)

	tnOS, err := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
		Name:       "tenant-single",
		Org:        env.sharedOrg,
		TenantID:   &env.tenant.ID,
		OsType:     cdbm.OperatingSystemTypeIPXE,
		IpxeScript: cdb.GetStrPtr("#!ipxe"),
		Status:     cdbm.OperatingSystemStatusReady,
		CreatedBy:  env.tnUser.ID,
	})
	require.NoError(t, err)

	execGetByID := func(t *testing.T, user *cdbm.User, osID string) *httptest.ResponseRecorder {
		t.Helper()
		tracer, ctx := otelTraceCtx(t)

		eh := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		ec := eh.NewContext(req, rec)
		ec.SetParamNames("orgName", "id")
		ec.SetParamValues(env.sharedOrg, osID)
		ec.Set("user", user)
		ec.SetRequest(ec.Request().WithContext(
			context.WithValue(ctx, otelecho.TracerKey, tracer),
		))

		cfg := common.GetTestConfig()
		h := GetOperatingSystemHandler{
			dbSession: env.dbSession,
			tc:        env.tempClient,
			cfg:       cfg,
		}
		require.NoError(t, h.Handle(ec))
		return rec
	}

	t.Run("provider admin gets provider-owned OS", func(t *testing.T) {
		rec := execGetByID(t, env.ipUser, provOS.ID.String())
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("tenant admin gets provider-owned OS", func(t *testing.T) {
		rec := execGetByID(t, env.tnUser, provOS.ID.String())
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("provider admin gets tenant-owned OS", func(t *testing.T) {
		rec := execGetByID(t, env.ipUser, tnOS.ID.String())
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("tenant admin gets tenant-owned OS", func(t *testing.T) {
		rec := execGetByID(t, env.tnUser, tnOS.ID.String())
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("dual-role user gets provider-owned OS", func(t *testing.T) {
		rec := execGetByID(t, env.dualUser, provOS.ID.String())
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
}

// ─── Update: role-based ownership enforcement ─────────────────────────────────

// TestOperatingSystemHandler_Update_OwnershipEnforcement exercises the Update
// handler's role-based mutation rules:
//
//   - Provider Admin can update only provider-owned OSes → 200 / 403
//   - Tenant Admin can update only tenant-owned OSes    → 200 / 403
//   - Dual-role user is permitted if either role authorizes the action
func TestOperatingSystemHandler_Update_OwnershipEnforcement(t *testing.T) {
	env := newOwnershipTestEnv(t)
	ctx := context.Background()

	osDAO := cdbm.NewOperatingSystemDAO(env.dbSession)

	// Provider-owned iPXE OS (no site associations → no Temporal calls needed).
	provOS, err := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
		Name:                     "prov-update-target",
		Org:                      env.sharedOrg,
		TenantID:                 nil,
		InfrastructureProviderID: &env.ip.ID,
		OsType:                   cdbm.OperatingSystemTypeIPXE,
		IpxeScript:               cdb.GetStrPtr("#!ipxe\nboot"),
		Status:                   cdbm.OperatingSystemStatusReady,
		CreatedBy:                env.ipUser.ID,
	})
	require.NoError(t, err)

	// Tenant-owned iPXE OS (no site associations).
	tnOS, err := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
		Name:       "tn-update-target",
		Org:        env.sharedOrg,
		TenantID:   &env.tenant.ID,
		OsType:     cdbm.OperatingSystemTypeIPXE,
		IpxeScript: cdb.GetStrPtr("#!ipxe\nboot"),
		Status:     cdbm.OperatingSystemStatusReady,
		CreatedBy:  env.tnUser.ID,
	})
	require.NoError(t, err)

	newScript := "updated-ipxe-script"
	updateBody := model.APIOperatingSystemUpdateRequest{
		IpxeScript: &newScript,
	}
	rawUpdate, err := json.Marshal(updateBody)
	require.NoError(t, err)

	execUpdate := func(t *testing.T, user *cdbm.User, osID string) *httptest.ResponseRecorder {
		t.Helper()
		tracer, ctx := otelTraceCtx(t)

		eh := echo.New()
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(string(rawUpdate)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		ec := eh.NewContext(req, rec)
		ec.SetParamNames("orgName", "id")
		ec.SetParamValues(env.sharedOrg, osID)
		ec.Set("user", user)
		ec.SetRequest(ec.Request().WithContext(
			context.WithValue(ctx, otelecho.TracerKey, tracer),
		))

		cfg := common.GetTestConfig()
		h := UpdateOperatingSystemHandler{
			dbSession: env.dbSession,
			tc:        env.tempClient,
			cfg:       cfg,
			scp:       env.scp,
		}
		require.NoError(t, h.Handle(ec))
		return rec
	}

	t.Run("provider admin updates provider-owned OS → 200", func(t *testing.T) {
		rec := execUpdate(t, env.ipUser, provOS.ID.String())
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("provider admin updates tenant-owned OS → 403", func(t *testing.T) {
		rec := execUpdate(t, env.ipUser, tnOS.ID.String())
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("tenant admin updates tenant-owned OS → 200", func(t *testing.T) {
		rec := execUpdate(t, env.tnUser, tnOS.ID.String())
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("tenant admin updates provider-owned OS → 403", func(t *testing.T) {
		rec := execUpdate(t, env.tnUser, provOS.ID.String())
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("dual-role user updates provider-owned OS → 200 (provider role authorizes)", func(t *testing.T) {
		rec := execUpdate(t, env.dualUser, provOS.ID.String())
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("dual-role user updates tenant-owned OS → 200 (tenant role authorizes)", func(t *testing.T) {
		rec := execUpdate(t, env.dualUser, tnOS.ID.String())
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
}

// ─── Delete: role-based ownership enforcement ─────────────────────────────────

// TestOperatingSystemHandler_Delete_OwnershipEnforcement exercises the Delete
// handler's role-based mutation rules.  iPXE OSes without site associations
// are used so no Temporal workflow is invoked.
//
//   - Provider Admin deletes provider-owned OS → 202
//   - Provider Admin deletes tenant-owned OS   → 403
//   - Tenant Admin deletes tenant-owned OS     → 202
//   - Tenant Admin deletes provider-owned OS   → 403
//   - Dual-role user deletes provider-owned OS → 202 (provider role authorizes)
//   - Dual-role user deletes tenant-owned OS   → 202 (tenant role authorizes)
func TestOperatingSystemHandler_Delete_OwnershipEnforcement(t *testing.T) {
	env := newOwnershipTestEnv(t)
	ctx := context.Background()

	osDAO := cdbm.NewOperatingSystemDAO(env.dbSession)

	// helper: create a fresh provider-owned iPXE OS.
	newProvOS := func(suffix string) *cdbm.OperatingSystem {
		os, err := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
			Name:                     "prov-del-" + suffix,
			Org:                      env.sharedOrg,
			TenantID:                 nil,
			InfrastructureProviderID: &env.ip.ID,
			OsType:                   cdbm.OperatingSystemTypeIPXE,
			IpxeScript:               cdb.GetStrPtr("#!ipxe\nboot"),
			Status:                   cdbm.OperatingSystemStatusReady,
			CreatedBy:                env.ipUser.ID,
		})
		require.NoError(t, err)
		return os
	}

	// helper: create a fresh tenant-owned iPXE OS.
	newTnOS := func(suffix string) *cdbm.OperatingSystem {
		os, err := osDAO.Create(ctx, nil, cdbm.OperatingSystemCreateInput{
			Name:       "tn-del-" + suffix,
			Org:        env.sharedOrg,
			TenantID:   &env.tenant.ID,
			OsType:     cdbm.OperatingSystemTypeIPXE,
			IpxeScript: cdb.GetStrPtr("#!ipxe\nboot"),
			Status:     cdbm.OperatingSystemStatusReady,
			CreatedBy:  env.tnUser.ID,
		})
		require.NoError(t, err)
		return os
	}

	execDelete := func(t *testing.T, user *cdbm.User, osID string) *httptest.ResponseRecorder {
		t.Helper()
		tracer, ctx := otelTraceCtx(t)

		eh := echo.New()
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		rec := httptest.NewRecorder()

		ec := eh.NewContext(req, rec)
		ec.SetParamNames("orgName", "id")
		ec.SetParamValues(env.sharedOrg, osID)
		ec.Set("user", user)
		ec.SetRequest(ec.Request().WithContext(
			context.WithValue(ctx, otelecho.TracerKey, tracer),
		))

		cfg := common.GetTestConfig()
		h := DeleteOperatingSystemHandler{
			dbSession: env.dbSession,
			tc:        env.tempClient,
			cfg:       cfg,
			scp:       env.scp,
		}
		require.NoError(t, h.Handle(ec))
		return rec
	}

	t.Run("provider admin deletes provider-owned OS → 202", func(t *testing.T) {
		os := newProvOS(uuid.NewString())
		rec := execDelete(t, env.ipUser, os.ID.String())
		assert.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	})

	t.Run("provider admin deletes tenant-owned OS → 403", func(t *testing.T) {
		os := newTnOS(uuid.NewString())
		rec := execDelete(t, env.ipUser, os.ID.String())
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("tenant admin deletes tenant-owned OS → 202", func(t *testing.T) {
		os := newTnOS(uuid.NewString())
		rec := execDelete(t, env.tnUser, os.ID.String())
		assert.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	})

	t.Run("tenant admin deletes provider-owned OS → 403", func(t *testing.T) {
		os := newProvOS(uuid.NewString())
		rec := execDelete(t, env.tnUser, os.ID.String())
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("dual-role user deletes provider-owned OS → 202 (provider role authorizes)", func(t *testing.T) {
		os := newProvOS(uuid.NewString())
		rec := execDelete(t, env.dualUser, os.ID.String())
		assert.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	})

	t.Run("dual-role user deletes tenant-owned OS → 202 (tenant role authorizes)", func(t *testing.T) {
		os := newTnOS(uuid.NewString())
		rec := execDelete(t, env.dualUser, os.ID.String())
		assert.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	})
}
