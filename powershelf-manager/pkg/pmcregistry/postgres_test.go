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
package pmcregistry

import (
	"context"
	"net"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/credential"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/vendor"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db/migrations"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db/postgres"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db/testutil"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/objects/pmc"
)

// skipIfNoDatabase skips the test if database environment is not configured
func skipIfNoDatabase(t *testing.T) {
	t.Helper()
	if os.Getenv("DB_PORT") == "" {
		log.Warn("Skipping integration test: DB_PORT not set")
		t.Skip("DB_PORT environment variable not set - skipping integration test")
	}
}

// setupTestDB creates a fresh test database with migrations applied
func setupTestDB(t *testing.T) (*postgres.Postgres, func()) {
	t.Helper()
	ctx := context.Background()

	dbConf, err := db.BuildDBConfigFromEnv()
	require.NoError(t, err, "Failed to build DB config from env")

	pg, err := testutil.CreateTestDB(ctx, t, dbConf)
	require.NoError(t, err, "Failed to create test database")

	// Run migrations
	err = migrations.Migrate(ctx, pg)
	require.NoError(t, err, "Failed to run migrations")

	cleanup := func() {
		pg.Close(ctx)
	}

	return pg, cleanup
}

// createTestPMC creates a test PMC object
func createTestPMC(t *testing.T, macStr, ipStr string, v vendor.VendorCode) *pmc.PMC {
	t.Helper()
	mac, err := net.ParseMAC(macStr)
	require.NoError(t, err, "Failed to parse MAC")
	ip := net.ParseIP(ipStr)
	require.NotNil(t, ip, "Failed to parse IP")

	cred := credential.New("admin", "password")
	p, err := pmc.NewFromAddr(mac, ip, v, cred)
	require.NoError(t, err, "Failed to create test PMC")
	return p
}

func TestIntegration_PostgresRegistry_RegisterAndGetPmc(t *testing.T) {
	skipIfNoDatabase(t)

	pg, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create registry using the test database
	registry := &PostgresPmcRegistry{pg: pg}

	// Test data
	testPmc := createTestPMC(t, "00:11:22:33:44:55", "192.168.1.100", vendor.VendorCodeLiteon)

	// Test RegisterPmc
	err := registry.RegisterPmc(ctx, testPmc)
	assert.NoError(t, err, "RegisterPmc should succeed")

	// Test GetPmc by MAC
	retrieved, err := registry.GetPmc(ctx, testPmc.GetMac())
	assert.NoError(t, err, "GetPmc should succeed")
	assert.NotNil(t, retrieved, "Retrieved PMC should not be nil")
	assert.Equal(t, testPmc.GetMac().String(), retrieved.GetMac().String(), "MAC should match")
	assert.Equal(t, testPmc.GetIp().String(), retrieved.GetIp().String(), "IP should match")
	assert.Equal(t, testPmc.GetVendor().Code, retrieved.GetVendor().Code, "Vendor should match")
}

func TestIntegration_PostgresRegistry_IsPmcRegistered(t *testing.T) {
	skipIfNoDatabase(t)

	pg, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	registry := &PostgresPmcRegistry{pg: pg}

	testPmc := createTestPMC(t, "AA:BB:CC:DD:EE:FF", "10.0.0.1", vendor.VendorCodeLiteon)

	// Before registration
	registered, err := registry.IsPmcRegistered(ctx, testPmc.GetMac())
	assert.Error(t, err, "IsPmcRegistered should return error for unregistered PMC")
	assert.False(t, registered, "Should not be registered yet")

	// Register
	err = registry.RegisterPmc(ctx, testPmc)
	assert.NoError(t, err, "RegisterPmc should succeed")

	// After registration
	registered, err = registry.IsPmcRegistered(ctx, testPmc.GetMac())
	assert.NoError(t, err, "IsPmcRegistered should succeed for registered PMC")
	assert.True(t, registered, "Should be registered now")
}

func TestIntegration_PostgresRegistry_GetAllPmcs(t *testing.T) {
	skipIfNoDatabase(t)

	pg, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	registry := &PostgresPmcRegistry{pg: pg}

	// Register multiple PMCs
	pmc1 := createTestPMC(t, "00:11:22:33:44:55", "192.168.1.1", vendor.VendorCodeLiteon)
	pmc2 := createTestPMC(t, "66:77:88:99:AA:BB", "192.168.1.2", vendor.VendorCodeLiteon)
	pmc3 := createTestPMC(t, "CC:DD:EE:FF:00:11", "192.168.1.3", vendor.VendorCodeLiteon)

	require.NoError(t, registry.RegisterPmc(ctx, pmc1))
	require.NoError(t, registry.RegisterPmc(ctx, pmc2))
	require.NoError(t, registry.RegisterPmc(ctx, pmc3))

	// Get all
	allPmcs, err := registry.GetAllPmcs(ctx)
	assert.NoError(t, err, "GetAllPmcs should succeed")
	assert.Len(t, allPmcs, 3, "Should have 3 PMCs")

	// Verify all MACs are present
	macSet := make(map[string]bool)
	for _, p := range allPmcs {
		macSet[p.GetMac().String()] = true
	}
	assert.True(t, macSet["00:11:22:33:44:55"], "PMC 1 should be present")
	assert.True(t, macSet["66:77:88:99:aa:bb"], "PMC 2 should be present")
	assert.True(t, macSet["cc:dd:ee:ff:00:11"], "PMC 3 should be present")
}

func TestIntegration_PostgresRegistry_DuplicateRegistration(t *testing.T) {
	skipIfNoDatabase(t)

	pg, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	registry := &PostgresPmcRegistry{pg: pg}

	testPmc := createTestPMC(t, "00:11:22:33:44:55", "192.168.1.100", vendor.VendorCodeLiteon)

	// First registration should succeed
	err := registry.RegisterPmc(ctx, testPmc)
	assert.NoError(t, err, "First registration should succeed")

	// Second registration should fail (duplicate MAC)
	err = registry.RegisterPmc(ctx, testPmc)
	assert.Error(t, err, "Duplicate registration should fail")
}

func TestIntegration_PostgresRegistry_UniqueIPConstraint(t *testing.T) {
	skipIfNoDatabase(t)

	pg, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	registry := &PostgresPmcRegistry{pg: pg}

	pmc1 := createTestPMC(t, "00:11:22:33:44:55", "192.168.1.100", vendor.VendorCodeLiteon)
	pmc2 := createTestPMC(t, "AA:BB:CC:DD:EE:FF", "192.168.1.100", vendor.VendorCodeLiteon) // Same IP

	// First registration should succeed
	err := registry.RegisterPmc(ctx, pmc1)
	assert.NoError(t, err, "First registration should succeed")

	// Second registration should fail (duplicate IP)
	err = registry.RegisterPmc(ctx, pmc2)
	assert.Error(t, err, "Registration with duplicate IP should fail")
}
