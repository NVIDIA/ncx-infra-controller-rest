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

package ipam

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	cdbutil "github.com/nvidia/bare-metal-manager-rest/db/pkg/util"
	cipam "github.com/nvidia/bare-metal-manager-rest/ipam"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/extra/bundebug"
)

// ~~~~~ For Testing IPAM ~~~~~ //

var (
	testIpamDB cipam.Storage
)

// getTestIpamer returns the test ipamer
func getTestIpamer(t *testing.T, ipamDB cipam.Storage) cipam.Ipamer {
	return cipam.NewWithStorage(ipamDB)
}

// getTestIpamDB returns the test ipam DB
func getTestIpamDB(t *testing.T, dbSession *db.Session, reset bool) cipam.Storage {
	if testIpamDB != nil {
		if reset {
			testIpamDB.DeleteAllPrefixes(context.Background(), "")
		}
		return testIpamDB
	}

	storage := cipam.NewBunStorage(dbSession.DB, nil)

	// ensure the ipam schema is applied in test db
	storage.ApplyDbSchema()

	testIpamDB := NewIpamStorage(dbSession.DB, nil)
	if reset {
		testIpamDB.DeleteAllPrefixes(context.Background(), "")
	}
	return testIpamDB
}

// reset the tables needed for IPBlock tests
func testIpamSetupSchema(t *testing.T, dbSession *db.Session) {
	// create Infrastructure Provider table
	err := dbSession.DB.ResetModel(context.Background(), (*cdbm.InfrastructureProvider)(nil))
	assert.Nil(t, err)
	// create Site table
	err = dbSession.DB.ResetModel(context.Background(), (*cdbm.Site)(nil))
	assert.Nil(t, err)
	// create tenant table
	err = dbSession.DB.ResetModel(context.Background(), (*cdbm.Tenant)(nil))
	assert.Nil(t, err)
	// create IPBlock table
	err = dbSession.DB.ResetModel(context.Background(), (*cdbm.IPBlock)(nil))
	assert.Nil(t, err)
}

func testIpamBuildSite(t *testing.T, dbSession *db.Session, ip *cdbm.InfrastructureProvider, name string) *cdbm.Site {
	st := &cdbm.Site{
		ID:                          uuid.New(),
		Name:                        name,
		DisplayName:                 db.GetStrPtr("Test"),
		Org:                         "test",
		InfrastructureProviderID:    ip.ID,
		SiteControllerVersion:       db.GetStrPtr("1.0.0"),
		SiteAgentVersion:            db.GetStrPtr("1.0.0"),
		RegistrationToken:           db.GetStrPtr("1234-5678-9012-3456"),
		RegistrationTokenExpiration: db.GetTimePtr(db.GetCurTime()),
		Status:                      cdbm.SiteStatusPending,
		CreatedBy:                   uuid.New(),
	}
	_, err := dbSession.DB.NewInsert().Model(st).Exec(context.Background())
	assert.Nil(t, err)
	return st
}

func testIpamBuildInfrastructureProvider(t *testing.T, dbSession *db.Session, name string) *cdbm.InfrastructureProvider {
	ip := &cdbm.InfrastructureProvider{
		ID:          uuid.New(),
		Name:        name,
		DisplayName: db.GetStrPtr("TestInfraProvider"),
		Org:         "test",
	}
	_, err := dbSession.DB.NewInsert().Model(ip).Exec(context.Background())
	assert.Nil(t, err)
	return ip
}

func testIpamBuildTenant(t *testing.T, dbSession *db.Session, name string) *cdbm.Tenant {
	tenant := &cdbm.Tenant{
		ID:   uuid.New(),
		Name: name,
		Org:  "test",
	}
	_, err := dbSession.DB.NewInsert().Model(tenant).Exec(context.Background())
	assert.Nil(t, err)
	return tenant
}

func testIpamBuildIPBlock(t *testing.T, dbSession *db.Session, ipBlock *cdbm.IPBlock) *cdbm.IPBlock {
	_, err := dbSession.DB.NewInsert().Model(ipBlock).Exec(context.Background())
	assert.Nil(t, err)
	return ipBlock
}

func TestCreateIpamEntryForIPBlock(t *testing.T) {
	dbSession := cdbutil.GetTestDBSession(t, false)
	defer dbSession.Close()

	ipamDB := getTestIpamDB(t, dbSession, true)
	ctx := context.Background()
	tests := []struct {
		name                     string
		prefix                   string
		PrefixLength             int
		routingType              string
		infrastructureProviderID string
		siteID                   string
		expectedErr              bool
	}{
		{
			name:                     "success for one entry",
			prefix:                   "192.168.1.0",
			PrefixLength:             24,
			routingType:              "DatacenterOnly",
			infrastructureProviderID: "testIP",
			siteID:                   "testSite",
			expectedErr:              false,
		},
		{
			name:                     "error when cidr already exists",
			prefix:                   "192.168.1.0",
			PrefixLength:             24,
			routingType:              "DatacenterOnly",
			infrastructureProviderID: "testIP",
			siteID:                   "testSite",
			expectedErr:              true,
		},
		{
			name:                     "error when cidr clash",
			prefix:                   "192.168.1.2",
			PrefixLength:             24,
			routingType:              "DatacenterOnly",
			infrastructureProviderID: "testIP",
			siteID:                   "testSite",
			expectedErr:              true,
		},
		{
			name:                     "success when cidr clash but different namespace",
			prefix:                   "192.168.1.0",
			PrefixLength:             24,
			routingType:              "DatacenterOnly",
			infrastructureProviderID: "testIP2",
			siteID:                   "testSite",
			expectedErr:              false,
		},
		{
			name:                     "success when cidr dont clash",
			prefix:                   "192.168.2.0",
			PrefixLength:             24,
			routingType:              "DatacenterOnly",
			infrastructureProviderID: "testIP",
			siteID:                   "testSite",
			expectedErr:              false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pref, err := CreateIpamEntryForIPBlock(ctx, ipamDB, tc.prefix, tc.PrefixLength, tc.routingType, tc.infrastructureProviderID, tc.siteID)
			assert.Equal(t, tc.expectedErr, err != nil)
			if err == nil {
				assert.NotNil(t, pref)
				fmt.Println(pref.Namespace, pref.String())
			}
		})
	}
}

func TestDeleteIpamEntryForIPBlock(t *testing.T) {
	dbSession := cdbutil.GetTestDBSession(t, false)
	defer dbSession.Close()
	ipamDB := getTestIpamDB(t, dbSession, true)
	ctx := context.Background()
	_, err := CreateIpamEntryForIPBlock(ctx, ipamDB, "192.168.0.0", 24, "DatacenterOnly", "testIPDel", "testSite")
	assert.Nil(t, err)
	_, err = CreateIpamEntryForIPBlock(ctx, ipamDB, "192.168.1.0", 24, "DatacenterOnly", "testIP2Del", "testSite")
	assert.Nil(t, err)
	tests := []struct {
		name                     string
		prefix                   string
		PrefixLength             int
		routingType              string
		infrastructureProviderID string
		siteID                   string
		expectedErr              bool
	}{
		{
			name:                     "success deleting one entry",
			prefix:                   "192.168.0.0",
			PrefixLength:             24,
			routingType:              "DatacenterOnly",
			infrastructureProviderID: "testIPDel",
			siteID:                   "testSite",
			expectedErr:              false,
		},
		{
			name:                     "success when deleting entry doesnt exist",
			prefix:                   "192.168.0.0",
			PrefixLength:             24,
			routingType:              "DatacenterOnly",
			infrastructureProviderID: "testIPDel",
			siteID:                   "testSite",
			expectedErr:              false,
		},
		{
			name:                     "success deleting another entry",
			prefix:                   "192.168.1.0",
			PrefixLength:             24,
			routingType:              "DatacenterOnly",
			infrastructureProviderID: "testIP2Del",
			siteID:                   "testSite",
			expectedErr:              false,
		},
		{
			name:                     "success when deleting entry namespace doesnt exist",
			prefix:                   "192.168.0.0",
			PrefixLength:             24,
			routingType:              "DatacenterOnly",
			infrastructureProviderID: "testIPNone",
			siteID:                   "testSite",
			expectedErr:              false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := DeleteIpamEntryForIPBlock(ctx, ipamDB, tc.prefix, tc.PrefixLength, tc.routingType, tc.infrastructureProviderID, tc.siteID)
			assert.Equal(t, tc.expectedErr, err != nil)
		})
	}
}

func TestGetIpamUsageForIPBlock(t *testing.T) {
	dbSession := cdbutil.GetTestDBSession(t, false)
	defer dbSession.Close()

	ipamDB := getTestIpamDB(t, dbSession, true)
	ctx := context.Background()

	testIpamSetupSchema(t, dbSession)

	ip := testIpamBuildInfrastructureProvider(t, dbSession, "testip")
	site := testIpamBuildSite(t, dbSession, ip, "testsite")

	ipID := ip.ID
	siteID := site.ID

	ipBlock1 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: ipID,
		SiteID:                   siteID,
		Prefix:                   "192.168.0.0",
		PrefixLength:             16,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}

	ipBlock2 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: ipID,
		SiteID:                   siteID,
		Prefix:                   "192.169.1.0",
		PrefixLength:             28,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
		FullGrant:                true,
	}

	ipBlock3 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: ipID,
		SiteID:                   siteID,
		Prefix:                   "192.162.1.0",
		PrefixLength:             16,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}

	ipbPrefix1, err := CreateIpamEntryForIPBlock(ctx, ipamDB, "192.168.0.0", 16, "DatacenterOnly", ipID.String(), siteID.String())
	assert.Nil(t, err)

	ipbPrefix2, err := CreateIpamEntryForIPBlock(ctx, ipamDB, "192.169.1.0", 28, "DatacenterOnly", ipID.String(), siteID.String())
	assert.Nil(t, err)

	tests := []struct {
		name             string
		inputIPB         *cdbm.IPBlock
		inputPrefix      *cipam.Prefix
		expectedErr      bool
		exptectFullGrant bool
	}{
		{
			name:        "success nil IPBlock",
			inputIPB:    nil,
			inputPrefix: nil,
			expectedErr: true,
		},
		{
			name:        "success no prefix found for IPBlock",
			inputIPB:    ipBlock3,
			inputPrefix: nil,
			expectedErr: true,
		},
		{
			name:        "success get usage for IPBlock 1",
			inputIPB:    ipBlock1,
			inputPrefix: ipbPrefix1,
			expectedErr: false,
		},
		{
			name:             "success get usage for full grant IPBlock 2",
			inputIPB:         ipBlock2,
			inputPrefix:      ipbPrefix2,
			expectedErr:      false,
			exptectFullGrant: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resUsage, err := GetIpamUsageForIPBlock(ctx, ipamDB, tc.inputIPB)
			assert.Equal(t, tc.expectedErr, err != nil)
			if !tc.expectedErr {
				assert.NotNil(t, resUsage)
				if tc.exptectFullGrant {
					assert.Equal(t, int(resUsage.AcquiredIPs), 0)
					assert.Equal(t, int(resUsage.AcquiredPrefixes), 1)
					assert.Equal(t, len(resUsage.AvailablePrefixes), 0)
					assert.Equal(t, int(resUsage.AvailableSmallestPrefixes), 0)

				} else {
					assert.Equal(t, int(resUsage.AcquiredIPs), int(tc.inputPrefix.Usage().AcquiredIPs))
					assert.Equal(t, int(resUsage.AcquiredPrefixes), int(tc.inputPrefix.Usage().AcquiredPrefixes))
					assert.Equal(t, len(resUsage.AvailablePrefixes), len(tc.inputPrefix.Usage().AvailablePrefixes))
					assert.Equal(t, int(resUsage.AvailableSmallestPrefixes), int(tc.inputPrefix.Usage().AvailableSmallestPrefixes))
				}
			}
		})
	}
}

func TestCreateChildIpamEntryForIPBlock(t *testing.T) {
	dbSession := cdbutil.GetTestDBSession(t, false)
	defer dbSession.Close()
	dbSession.DB.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(false),
		bundebug.FromEnv("BUNDEBUG"),
	))
	ipamDB := getTestIpamDB(t, dbSession, true)
	ctx := context.Background()
	testIpamSetupSchema(t, dbSession)

	ip := testIpamBuildInfrastructureProvider(t, dbSession, "testip")
	site := testIpamBuildSite(t, dbSession, ip, "testsite")
	tenant := testIpamBuildTenant(t, dbSession, "testtenant")

	ipID := ip.ID
	siteID := site.ID
	tenantID := tenant.ID

	ipBlock1 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: ipID,
		SiteID:                   siteID,
		Prefix:                   "192.168.0.0",
		PrefixLength:             16,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}
	ipBlock2 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: ipID,
		SiteID:                   siteID,
		Prefix:                   "192.169.1.0",
		PrefixLength:             28,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}
	ipBlock3 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: uuid.New(),
		SiteID:                   uuid.New(),
		Prefix:                   "192.169.0.0",
		PrefixLength:             16,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}
	ipBlock4 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: uuid.New(),
		SiteID:                   uuid.New(),
		Prefix:                   "192.169.0.0",
		PrefixLength:             16,
		FullGrant:                true,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}
	ipBlock5 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: ipID,
		SiteID:                   siteID,
		TenantID:                 &tenantID,
		Prefix:                   "192.170.0.0",
		PrefixLength:             16,
		FullGrant:                false,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}
	ipBlock6 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: ipID,
		SiteID:                   siteID,
		TenantID:                 &tenantID,
		Prefix:                   "192.171.0.0",
		PrefixLength:             16,
		FullGrant:                false,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}

	ipamer := cipam.NewWithStorage(ipamDB)
	ipamer.SetNamespace(GetIpamNamespaceForIPBlock(ctx, cdbm.IPBlockRoutingTypeDatacenterOnly, ipID.String(), siteID.String()))
	prefix, err := ipamer.NewPrefix(ctx, "192.168.0.0/16")
	assert.Nil(t, err)
	assert.Equal(t, "192.168.0.0/16", prefix.Cidr)
	prefix, err = ipamer.NewPrefix(ctx, "192.169.1.0/28")
	assert.Nil(t, err)
	assert.Equal(t, "192.169.1.0/28", prefix.Cidr)
	prefix, err = ipamer.NewPrefix(ctx, "192.170.0.0/16")
	assert.Nil(t, err)
	assert.Equal(t, "192.170.0.0/16", prefix.Cidr)
	childPrefix, err := ipamer.AcquireChildPrefix(ctx, "192.170.0.0/16", uint8(24))
	assert.Nil(t, err)
	assert.NotNil(t, childPrefix)

	ipb6 := testIpamBuildIPBlock(t, dbSession, ipBlock6)
	assert.NotNil(t, ipb6)
	ipbDAO := cdbm.NewIPBlockDAO(dbSession)
	prefix, err = ipamer.NewPrefix(ctx, "192.171.0.0/16")
	assert.Nil(t, err)
	assert.Equal(t, "192.171.0.0/16", prefix.Cidr)

	tests := []struct {
		name              string
		parentIPBlock     *cdbm.IPBlock
		tx                *cdb.Tx
		childCount        int
		childPrefixLength int
		expectedErr       bool
		checkFullGrant    bool
	}{

		{
			name:              "success creating one child entry",
			parentIPBlock:     ipBlock1,
			childCount:        1,
			childPrefixLength: 24,
			expectedErr:       false,
		},
		{
			name:              "success creating multiple child entries",
			parentIPBlock:     ipBlock1,
			childCount:        5,
			childPrefixLength: 24,
			expectedErr:       false,
		},
		{
			name:              "failure when there is no space for child block",
			parentIPBlock:     ipBlock2,
			childCount:        1,
			childPrefixLength: 24,
			expectedErr:       true,
		},
		{
			name:              "failure when parent doesnt exist",
			parentIPBlock:     ipBlock3,
			childCount:        1,
			childPrefixLength: 24,
			expectedErr:       true,
		},
		{
			name:          "failure when parent is nil",
			parentIPBlock: nil,
			expectedErr:   true,
			childCount:    1,
		},
		{
			name:              "error in full grant when parent prefix does not exist",
			parentIPBlock:     ipBlock3,
			childCount:        1,
			childPrefixLength: 16,
			expectedErr:       true,
		},
		{
			name:          "failure when parent is fully granted already",
			parentIPBlock: ipBlock4,
			expectedErr:   true,
			childCount:    1,
		},
		{
			name:              "failure when full-grant is required but parent IPBlock has acquired prefix already",
			parentIPBlock:     ipBlock5,
			expectedErr:       true,
			childPrefixLength: 16,
			childCount:        1,
		},
		{
			name:              "success when full-grant is required",
			parentIPBlock:     ipb6,
			expectedErr:       false,
			childPrefixLength: 16,
			childCount:        1,
			checkFullGrant:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < tc.childCount; i++ {
				pref, err := CreateChildIpamEntryForIPBlock(ctx, nil, dbSession, ipamDB, tc.parentIPBlock, tc.childPrefixLength)
				assert.Equal(t, tc.expectedErr, err != nil)
				if !tc.expectedErr {
					assert.NotNil(t, pref)
					fmt.Println(pref.Cidr)
				} else {
					fmt.Println(err)
				}
				if tc.checkFullGrant {
					assert.Equal(t, true, tc.parentIPBlock.FullGrant)
					ipb, err := ipbDAO.GetByID(ctx, nil, tc.parentIPBlock.ID, nil)
					assert.Nil(t, err)
					assert.Equal(t, true, ipb.FullGrant)
					assert.Equal(t, pref.Cidr, GetCidrForIPBlock(ctx, tc.parentIPBlock.Prefix, tc.parentIPBlock.PrefixLength))
				}
			}
		})
	}
}

func TestParseCidrIntoPrefixAndPrefixLength(t *testing.T) {
	tests := []struct {
		name           string
		cidr           string
		expectedErr    bool
		expectedPrefix string
		expectedBits   int
	}{
		{
			name:           "success ipv4",
			cidr:           "192.168.1.1/24",
			expectedErr:    false,
			expectedPrefix: "192.168.1.0",
			expectedBits:   24,
		},
		{
			name:           "success ipv6",
			cidr:           "2001:aabb::/48",
			expectedErr:    false,
			expectedPrefix: "2001:aabb::",
			expectedBits:   48,
		},
		{
			name:           "error ipv4",
			cidr:           "192.168.0.330/24",
			expectedErr:    true,
			expectedPrefix: "",
			expectedBits:   0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, bits, err := ParseCidrIntoPrefixAndBlockSize(tc.cidr)
			assert.Equal(t, tc.expectedErr, err != nil)
			assert.Equal(t, tc.expectedPrefix, p)
			assert.Equal(t, tc.expectedBits, bits)
		})
	}
}

func TestDeleteChildIpamEntryFromCidr(t *testing.T) {
	dbSession := cdbutil.GetTestDBSession(t, false)
	defer dbSession.Close()
	dbSession.DB.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(false),
		bundebug.FromEnv("BUNDEBUG"),
	))
	ipamDB := getTestIpamDB(t, dbSession, true)
	ctx := context.Background()
	testIpamSetupSchema(t, dbSession)

	ip := testIpamBuildInfrastructureProvider(t, dbSession, "testip")
	site := testIpamBuildSite(t, dbSession, ip, "testsite")
	tenant := testIpamBuildTenant(t, dbSession, "testtenant")

	ipID := ip.ID
	siteID := site.ID
	tenantID := tenant.ID

	ipBlock1 := &cdbm.IPBlock{
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: ipID,
		SiteID:                   siteID,
		Prefix:                   "192.168.0.0",
		PrefixLength:             16,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}
	ipamer := cipam.NewWithStorage(ipamDB)
	ipamer.SetNamespace(GetIpamNamespaceForIPBlock(ctx, cdbm.IPBlockRoutingTypeDatacenterOnly, ipID.String(), siteID.String()))
	prefix, err := ipamer.NewPrefix(ctx, "192.168.0.0/16")
	assert.Nil(t, err)
	assert.Equal(t, "192.168.0.0/16", prefix.Cidr)
	childPrefix, err := CreateChildIpamEntryForIPBlock(ctx, nil, dbSession, ipamDB, ipBlock1, 24)
	assert.Nil(t, err)

	ipBlock6 := &cdbm.IPBlock{
		ID:                       uuid.New(),
		RoutingType:              cdbm.IPBlockRoutingTypeDatacenterOnly,
		InfrastructureProviderID: ipID,
		SiteID:                   siteID,
		TenantID:                 &tenantID,
		Prefix:                   "192.170.0.0",
		PrefixLength:             16,
		FullGrant:                true,
		ProtocolVersion:          cdbm.IPBlockProtocolVersionV4,
	}
	ipb6 := testIpamBuildIPBlock(t, dbSession, ipBlock6)
	assert.NotNil(t, ipb6)
	ipbDAO := cdbm.NewIPBlockDAO(dbSession)

	tests := []struct {
		name           string
		parentIPBlock  *cdbm.IPBlock
		tx             *cdb.Tx
		childCidr      string
		expectedErr    bool
		checkFullGrant bool
	}{

		{
			name:          "success when cidr exists",
			parentIPBlock: ipBlock1,
			childCidr:     childPrefix.Cidr,
			expectedErr:   false,
		},
		{
			name:          "failure when cidr doesnt exist",
			parentIPBlock: ipBlock1,
			childCidr:     "172.10.10.10/28",
			expectedErr:   true,
		},
		{
			name:          "failure when parent is nil",
			parentIPBlock: nil,
			expectedErr:   true,
		},
		{
			name:           "failure when full-grant but child cidr doesnt match",
			parentIPBlock:  ipb6,
			expectedErr:    true,
			childCidr:      "192.170.0.0/24",
			checkFullGrant: false,
		},
		{
			name:           "success when full-grant",
			parentIPBlock:  ipb6,
			expectedErr:    false,
			childCidr:      "192.170.0.0/16",
			checkFullGrant: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := DeleteChildIpamEntryFromCidr(ctx, nil, dbSession, ipamDB, tc.parentIPBlock, tc.childCidr)
			assert.Equal(t, tc.expectedErr, err != nil)
			if !tc.expectedErr {
				// verify that prefix was indeed deleted
				p := ipamer.PrefixFrom(ctx, tc.childCidr)
				assert.Nil(t, p)
			} else {
				fmt.Println(err)
			}
			if tc.checkFullGrant {
				assert.Equal(t, false, tc.parentIPBlock.FullGrant)
				ipb, err := ipbDAO.GetByID(ctx, nil, tc.parentIPBlock.ID, nil)
				assert.Nil(t, err)
				assert.Equal(t, false, ipb.FullGrant)
			}
		})
	}
}

// Generic IPAM library tests from the api
func TestIpamer_NewPrefix(t *testing.T) {
	// test ipam operations from api
	ctx := context.Background()
	dbSession := cdbutil.GetTestDBSession(t, false)
	defer dbSession.Close()
	ipamDB := getTestIpamDB(t, dbSession, true)
	ipamer := cipam.NewWithStorage(ipamDB)
	ipamer.SetNamespace("TestNewPrefix")

	prefix, err := ipamer.NewPrefix(ctx, "192.168.0.0/24")
	assert.Nil(t, err)
	ip1, err := ipamer.AcquireIP(ctx, prefix.Cidr)
	assert.Nil(t, err)
	ip2, err := ipamer.AcquireIP(ctx, prefix.Cidr)
	assert.Nil(t, err)

	assert.Equal(t, "192.168.0.0/24", prefix.String())
	assert.Equal(t, "192.168.0.1", ip1.IP.String())

	assert.Equal(t, "192.168.0.0/24", ip1.ParentPrefix)
	assert.Equal(t, "192.168.0.2", ip2.IP.String())
	assert.Equal(t, "192.168.0.0/24", ip2.ParentPrefix)

	_, err = ipamer.ReleaseIP(ctx, ip2)
	assert.Nil(t, err)

	_, err = ipamer.ReleaseIP(ctx, ip1)
	assert.Nil(t, err)

	_, err = ipamer.DeletePrefix(ctx, prefix.Cidr)
	assert.Nil(t, err)
}

func TestIpamer_AcquireChildPrefixV6(t *testing.T) {
	// test ipam operations from api
	ctx := context.Background()
	dbSession := cdbutil.GetTestDBSession(t, false)
	defer dbSession.Close()
	ipamDB := getTestIpamDB(t, dbSession, true)
	ipamer := cipam.NewWithStorage(ipamDB)
	ipamer.SetNamespace("TestAcquireChildPrefix")

	prefix, err := ipamer.NewPrefix(ctx, "2001:aabb::/48")
	assert.Nil(t, err)
	cp1, err := ipamer.AcquireChildPrefix(ctx, prefix.Cidr, 64)
	assert.Nil(t, err)
	cp2, err := ipamer.AcquireChildPrefix(ctx, prefix.Cidr, 72)
	assert.Nil(t, err)
	ip21, err := ipamer.AcquireIP(ctx, cp2.Cidr)
	assert.Nil(t, err)
	prefix = ipamer.PrefixFrom(ctx, prefix.Cidr)

	assert.Equal(t, "2001:aabb::/48", prefix.String())
	assert.Equal(t, "2001:aabb::/64", cp1.String())
	assert.Equal(t, "2001:aabb:0:1::/72", cp2.String())
	assert.Equal(t, "2001:aabb:0:1::1", ip21.IP.String())

	assert.Nil(t, ipamer.ReleaseChildPrefix(ctx, cp1))
	_, err = ipamer.ReleaseIP(ctx, ip21)
	assert.Nil(t, err)
	err = ipamer.ReleaseChildPrefix(ctx, cp2)
	assert.Nil(t, err)
	_, err = ipamer.DeletePrefix(ctx, prefix.Cidr)
	assert.Nil(t, err)
}

func TestIpamer_AcquireChildPrefixV4(t *testing.T) {
	// test ipam operations from api
	ctx := context.Background()
	dbSession := cdbutil.GetTestDBSession(t, false)
	defer dbSession.Close()
	ipamDB := getTestIpamDB(t, dbSession, true)
	ipamer := cipam.NewWithStorage(ipamDB)
	ipamer.SetNamespace("TestAcquireChildPrefixV4")

	prefix, err := ipamer.NewPrefix(ctx, "192.168.0.0/16")
	assert.Nil(t, err)
	cp1, err := ipamer.AcquireChildPrefix(ctx, prefix.Cidr, 24)
	assert.Nil(t, err)
	fmt.Println(cp1.Cidr)
	cp2, err := ipamer.AcquireChildPrefix(ctx, prefix.Cidr, 24)
	assert.Nil(t, err)
	ip21, err := ipamer.AcquireIP(ctx, cp2.Cidr)
	assert.Nil(t, err)
	fmt.Println(cp2.Cidr)
	prefix = ipamer.PrefixFrom(ctx, prefix.Cidr)
	assert.Nil(t, ipamer.ReleaseChildPrefix(ctx, cp1))
	_, err = ipamer.ReleaseIP(ctx, ip21)
	assert.Nil(t, err)
	err = ipamer.ReleaseChildPrefix(ctx, cp2)
	assert.Nil(t, err)
	_, err = ipamer.DeletePrefix(ctx, prefix.Cidr)
	assert.Nil(t, err)
}

func TestGetFirstIPFromCidr(t *testing.T) {
	tests := []struct {
		name        string
		cidr        string
		expectedErr bool
		expectedIP  string
	}{
		{
			name:        "success",
			cidr:        "192.168.1.0/24",
			expectedErr: false,
			expectedIP:  "192.168.1.1",
		},
		{
			name:        "fail",
			cidr:        "badcidr",
			expectedErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip, err := GetFirstIPFromCidr(tc.cidr)
			assert.Equal(t, tc.expectedErr, err != nil)
			if err == nil {
				assert.Equal(t, tc.expectedIP, ip)
			}
		})
	}
}

func TestValidateIPAddresses(t *testing.T) {
	tests := []struct {
		name             string
		ipAddresses      []string
		cidr             string
		gateway          string
		expectedValid    []string
		expectedRejected []string
	}{
		{
			name:             "all valid IPs in /24",
			ipAddresses:      []string{"192.168.1.10", "192.168.1.20"},
			cidr:             "192.168.1.0/24",
			gateway:          "192.168.1.1",
			expectedValid:    []string{"192.168.1.10", "192.168.1.20"},
			expectedRejected: nil,
		},
		{
			name:             "reject network address",
			ipAddresses:      []string{"192.168.1.0", "192.168.1.10"},
			cidr:             "192.168.1.0/24",
			gateway:          "",
			expectedValid:    []string{"192.168.1.10"},
			expectedRejected: []string{"192.168.1.0 (network address of 192.168.1.0/24)"},
		},
		{
			name:             "reject broadcast address",
			ipAddresses:      []string{"192.168.1.255", "192.168.1.10"},
			cidr:             "192.168.1.0/24",
			gateway:          "",
			expectedValid:    []string{"192.168.1.10"},
			expectedRejected: []string{"192.168.1.255 (broadcast address of 192.168.1.0/24)"},
		},
		{
			name:             "reject gateway address",
			ipAddresses:      []string{"192.168.1.1", "192.168.1.10"},
			cidr:             "192.168.1.0/24",
			gateway:          "192.168.1.1",
			expectedValid:    []string{"192.168.1.10"},
			expectedRejected: []string{"192.168.1.1 (gateway address of 192.168.1.0/24)"},
		},
		{
			name:             "reject all reserved addresses at once",
			ipAddresses:      []string{"192.168.1.0", "192.168.1.1", "192.168.1.255", "192.168.1.100"},
			cidr:             "192.168.1.0/24",
			gateway:          "192.168.1.1",
			expectedValid:    []string{"192.168.1.100"},
			expectedRejected: []string{"192.168.1.0 (network address of 192.168.1.0/24)", "192.168.1.1 (gateway address of 192.168.1.0/24)", "192.168.1.255 (broadcast address of 192.168.1.0/24)"},
		},
		{
			name:             "reject IP outside prefix",
			ipAddresses:      []string{"10.0.0.1", "192.168.1.10"},
			cidr:             "192.168.1.0/24",
			gateway:          "",
			expectedValid:    []string{"192.168.1.10"},
			expectedRejected: []string{"10.0.0.1 (not in prefix 192.168.1.0/24)"},
		},
		{
			name:             "reject invalid IP string",
			ipAddresses:      []string{"not-an-ip", "192.168.1.10"},
			cidr:             "192.168.1.0/24",
			gateway:          "",
			expectedValid:    []string{"192.168.1.10"},
			expectedRejected: []string{"not-an-ip (invalid IP)"},
		},
		{
			name:             "small /30 prefix - reject network and broadcast",
			ipAddresses:      []string{"10.0.0.0", "10.0.0.1", "10.0.0.2", "10.0.0.3"},
			cidr:             "10.0.0.0/30",
			gateway:          "10.0.0.1",
			expectedValid:    []string{"10.0.0.2"},
			expectedRejected: []string{"10.0.0.0 (network address of 10.0.0.0/30)", "10.0.0.1 (gateway address of 10.0.0.0/30)", "10.0.0.3 (broadcast address of 10.0.0.0/30)"},
		},
		{
			name:             "IPv6 - reject network address only (no broadcast in IPv6)",
			ipAddresses:      []string{"2001:db8::", "2001:db8::1", "2001:db8::f"},
			cidr:             "2001:db8::/124",
			gateway:          "",
			expectedValid:    []string{"2001:db8::1", "2001:db8::f"},
			expectedRejected: []string{"2001:db8:: (network address of 2001:db8::/124)"},
		},
		{
			name:             "empty IP list",
			ipAddresses:      []string{},
			cidr:             "192.168.1.0/24",
			gateway:          "",
			expectedValid:    []string{},
			expectedRejected: nil,
		},
		{
			name:             "invalid CIDR passes through",
			ipAddresses:      []string{"192.168.1.10"},
			cidr:             "bad-cidr",
			gateway:          "",
			expectedValid:    []string{"192.168.1.10"},
			expectedRejected: nil,
		},
		{
			name:             "no gateway - only network and broadcast rejected",
			ipAddresses:      []string{"192.168.1.0", "192.168.1.1", "192.168.1.255"},
			cidr:             "192.168.1.0/24",
			gateway:          "",
			expectedValid:    []string{"192.168.1.1"},
			expectedRejected: []string{"192.168.1.0 (network address of 192.168.1.0/24)", "192.168.1.255 (broadcast address of 192.168.1.0/24)"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid, rejected := ValidateIPAddresses(tc.ipAddresses, tc.cidr, tc.gateway)
			assert.Equal(t, tc.expectedValid, valid)
			assert.Equal(t, tc.expectedRejected, rejected)
		})
	}
}

func TestGetInterfaceCIDRAndGateway(t *testing.T) {
	ipv4Prefix := "192.168.1.0"
	ipv4Gateway := "192.168.1.1"
	ipv6Prefix := "2001:db8::"
	ipv6Gateway := "2001:db8::1"

	t.Run("subnet interface with IPv4", func(t *testing.T) {
		ifc := &cdbm.Interface{
			Subnet: &cdbm.Subnet{
				IPv4Prefix:   &ipv4Prefix,
				PrefixLength: 24,
				IPv4Gateway:  &ipv4Gateway,
			},
		}
		cidr, gw := GetInterfaceCIDRAndGateway(ifc)
		assert.Equal(t, "192.168.1.0/24", cidr)
		assert.Equal(t, "192.168.1.1", gw)
	})

	t.Run("subnet interface with IPv6 only", func(t *testing.T) {
		ifc := &cdbm.Interface{
			Subnet: &cdbm.Subnet{
				IPv6Prefix:   &ipv6Prefix,
				PrefixLength: 64,
				IPv6Gateway:  &ipv6Gateway,
			},
		}
		cidr, gw := GetInterfaceCIDRAndGateway(ifc)
		assert.Equal(t, "2001:db8::/64", cidr)
		assert.Equal(t, "2001:db8::1", gw)
	})

	t.Run("vpc prefix interface with CIDR format", func(t *testing.T) {
		// In production, VpcPrefix.Prefix is set from childPrefix.Cidr which
		// includes the prefix length (e.g. "10.0.0.0/16")
		ifc := &cdbm.Interface{
			VpcPrefix: &cdbm.VpcPrefix{
				Prefix:       "10.0.0.0/16",
				PrefixLength: 16,
			},
		}
		cidr, gw := GetInterfaceCIDRAndGateway(ifc)
		assert.Equal(t, "10.0.0.0/16", cidr)
		assert.Equal(t, "", gw)
	})

	t.Run("vpc prefix interface with bare IP format", func(t *testing.T) {
		// Fallback: if Prefix is just an IP without prefix length
		ifc := &cdbm.Interface{
			VpcPrefix: &cdbm.VpcPrefix{
				Prefix:       "10.0.0.0",
				PrefixLength: 16,
			},
		}
		cidr, gw := GetInterfaceCIDRAndGateway(ifc)
		assert.Equal(t, "10.0.0.0/16", cidr)
		assert.Equal(t, "", gw)
	})

	t.Run("no subnet or vpc prefix", func(t *testing.T) {
		ifc := &cdbm.Interface{}
		cidr, gw := GetInterfaceCIDRAndGateway(ifc)
		assert.Equal(t, "", cidr)
		assert.Equal(t, "", gw)
	})
}
