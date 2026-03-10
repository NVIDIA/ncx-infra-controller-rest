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

package activity

import (
	"context"
	"testing"

	"github.com/google/uuid"
	cClient "github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/grpc/client"
	cwssaws "github.com/nvidia/bare-metal-manager-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	tmocks "go.temporal.io/sdk/mocks"
)

func TestManageExpectedPowerShelfInventory_DiscoverExpectedPowerShelfInventory(t *testing.T) {
	mockCarbide := cClient.NewMockCarbideClient()

	carbideAtomicClient := cClient.NewCarbideAtomicClient(&cClient.CarbideClientConfig{})
	carbideAtomicClient.SwapClient(mockCarbide)

	wid := "test-workflow-id"
	wrun := &tmocks.WorkflowRun{}
	wrun.On("GetID").Return(wid)

	type fields struct {
		siteID               uuid.UUID
		carbideAtomicClient  *cClient.CarbideAtomicClient
		temporalPublishQueue string
		sitePageSize         int
		cloudPageSize        int
	}
	type args struct {
		wantTotalItems int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test collecting and publishing expected power shelf inventory, empty inventory",
			fields: fields{
				siteID:               uuid.New(),
				carbideAtomicClient:  carbideAtomicClient,
				temporalPublishQueue: "test-queue",
				sitePageSize:         100,
				cloudPageSize:        25,
			},
			args: args{
				wantTotalItems: 0,
			},
		},
		{
			name: "test collecting and publishing expected power shelf inventory, normal inventory",
			fields: fields{
				siteID:               uuid.New(),
				carbideAtomicClient:  carbideAtomicClient,
				temporalPublishQueue: "test-queue",
				sitePageSize:         100,
				cloudPageSize:        25,
			},
			args: args{
				wantTotalItems: 195,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &tmocks.Client{}
			tc.Mock.On("ExecuteWorkflow", mock.Anything, mock.AnythingOfType("internal.StartWorkflowOptions"),
				mock.AnythingOfType("string"), mock.AnythingOfType("uuid.UUID"), mock.Anything).Return(wrun, nil)
			tc.AssertNumberOfCalls(t, "ExecuteWorkflow", 0)

			manageInstance := NewManageExpectedPowerShelfInventory(
				tt.fields.siteID,
				tt.fields.carbideAtomicClient,
				tc,
				tt.fields.temporalPublishQueue,
				tt.fields.cloudPageSize,
			)

			ctx := context.Background()
			ctx = context.WithValue(ctx, "wantCount", tt.args.wantTotalItems)

			totalPages := tt.args.wantTotalItems / tt.fields.cloudPageSize
			if tt.args.wantTotalItems%tt.fields.cloudPageSize > 0 {
				totalPages++
			}

			err := manageInstance.DiscoverExpectedPowerShelfInventory(ctx)
			assert.NoError(t, err)

			if tt.args.wantTotalItems == 0 {
				tc.AssertNumberOfCalls(t, "ExecuteWorkflow", 1)
			} else {
				tc.AssertNumberOfCalls(t, "ExecuteWorkflow", totalPages)
			}

			inventory, ok := tc.Calls[0].Arguments[4].(*cwssaws.ExpectedPowerShelfInventory)
			assert.True(t, ok)

			if tt.args.wantTotalItems == 0 {
				assert.Equal(t, 0, len(inventory.ExpectedPowerShelves))
			} else {
				assert.Equal(t, tt.fields.cloudPageSize, len(inventory.ExpectedPowerShelves))
			}

			assert.Equal(t, cwssaws.InventoryStatus_INVENTORY_STATUS_SUCCESS, inventory.InventoryStatus)
			assert.Equal(t, totalPages, int(inventory.InventoryPage.TotalPages))
			assert.Equal(t, 1, int(inventory.InventoryPage.CurrentPage))
			assert.Equal(t, tt.fields.cloudPageSize, int(inventory.InventoryPage.PageSize))
			assert.Equal(t, tt.args.wantTotalItems, int(inventory.InventoryPage.TotalItems))
			assert.Equal(t, tt.args.wantTotalItems, len(inventory.InventoryPage.ItemIds))
		})
	}
}

func TestManageExpectedPowerShelf_CreateExpectedPowerShelfOnSite(t *testing.T) {
	mockCarbide := cClient.NewMockCarbideClient()

	carbideAtomicClient := cClient.NewCarbideAtomicClient(&cClient.CarbideClientConfig{})
	carbideAtomicClient.SwapClient(mockCarbide)

	type fields struct {
		CarbideAtomicClient *cClient.CarbideAtomicClient
	}
	type args struct {
		ctx     context.Context
		request *cwssaws.ExpectedPowerShelf
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test create expected power shelf success",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                &cwssaws.UUID{Value: "test-powershelf-001"},
					BmcMacAddress:     "00:11:22:33:44:55",
					ShelfSerialNumber: "SHELF-123456789",
				},
			},
			wantErr: false,
		},
		{
			name: "test create expected power shelf fail on missing MAC address",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                &cwssaws.UUID{Value: "test-powershelf-002"},
					BmcMacAddress:     "",
					ShelfSerialNumber: "SHELF-123456789",
				},
			},
			wantErr: true,
		},
		{
			name: "test create expected power shelf fail on missing serial number",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                &cwssaws.UUID{Value: "test-powershelf-003"},
					BmcMacAddress:     "00:11:22:33:44:55",
					ShelfSerialNumber: "",
				},
			},
			wantErr: true,
		},
		{
			name: "test create expected power shelf fail on missing id",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                nil,
					BmcMacAddress:     "00:11:22:33:44:55",
					ShelfSerialNumber: "SHELF-123456789",
				},
			},
			wantErr: true,
		},
		{
			name: "test create expected power shelf fail on missing identifying information",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                &cwssaws.UUID{Value: "test-powershelf-004"},
					BmcMacAddress:     "",
					ShelfSerialNumber: "",
				},
			},
			wantErr: true,
		},
		{
			name: "test create expected power shelf fail on missing request",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx:     context.Background(),
				request: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm := NewManageExpectedPowerShelf(tt.fields.CarbideAtomicClient)
			err := mm.CreateExpectedPowerShelfOnSite(tt.args.ctx, tt.args.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManageExpectedPowerShelf_UpdateExpectedPowerShelfOnSite(t *testing.T) {
	mockCarbide := cClient.NewMockCarbideClient()

	carbideAtomicClient := cClient.NewCarbideAtomicClient(&cClient.CarbideClientConfig{})
	carbideAtomicClient.SwapClient(mockCarbide)

	type fields struct {
		CarbideAtomicClient *cClient.CarbideAtomicClient
	}
	type args struct {
		ctx     context.Context
		request *cwssaws.ExpectedPowerShelf
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test update expected power shelf success",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                &cwssaws.UUID{Value: "test-update-001"},
					BmcMacAddress:     "00:11:22:33:44:55",
					ShelfSerialNumber: "SHELF-123456789",
				},
			},
			wantErr: false,
		},
		{
			name: "test update expected power shelf fail on missing id",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                nil,
					BmcMacAddress:     "00:11:22:33:44:55",
					ShelfSerialNumber: "SHELF-123456789",
				},
			},
			wantErr: true,
		},
		{
			name: "test update expected power shelf fail on missing MAC address",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                &cwssaws.UUID{Value: "test-update-002"},
					BmcMacAddress:     "",
					ShelfSerialNumber: "SHELF-123456789",
				},
			},
			wantErr: true,
		},
		{
			name: "test update expected power shelf fail on missing serial number",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                &cwssaws.UUID{Value: "test-update-003"},
					BmcMacAddress:     "00:11:22:33:44:55",
					ShelfSerialNumber: "",
				},
			},
			wantErr: true,
		},
		{
			name: "test update expected power shelf fail on missing both MAC and serial",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelf{
					Id:                &cwssaws.UUID{Value: "test-update-004"},
					BmcMacAddress:     "",
					ShelfSerialNumber: "",
				},
			},
			wantErr: true,
		},
		{
			name: "test update expected power shelf fail on missing request",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx:     context.Background(),
				request: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm := NewManageExpectedPowerShelf(tt.fields.CarbideAtomicClient)
			err := mm.UpdateExpectedPowerShelfOnSite(tt.args.ctx, tt.args.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManageExpectedPowerShelf_DeleteExpectedPowerShelfOnSite(t *testing.T) {
	mockCarbide := cClient.NewMockCarbideClient()

	carbideAtomicClient := cClient.NewCarbideAtomicClient(&cClient.CarbideClientConfig{})
	carbideAtomicClient.SwapClient(mockCarbide)

	type fields struct {
		CarbideAtomicClient *cClient.CarbideAtomicClient
	}
	type args struct {
		ctx     context.Context
		request *cwssaws.ExpectedPowerShelfRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test delete expected power shelf success",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelfRequest{
					Id:            &cwssaws.UUID{Value: "test-delete-001"},
					BmcMacAddress: "00:11:22:33:44:55",
				},
			},
			wantErr: false,
		},
		{
			name: "test delete expected power shelf fail on missing id",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelfRequest{
					Id:            nil,
					BmcMacAddress: "00:11:22:33:44:55",
				},
			},
			wantErr: true,
		},
		{
			name: "test delete expected power shelf success with missing BMC MAC address",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx: context.Background(),
				request: &cwssaws.ExpectedPowerShelfRequest{
					Id:            &cwssaws.UUID{Value: "test-delete-002"},
					BmcMacAddress: "",
				},
			},
			wantErr: false,
		},
		{
			name: "test delete expected power shelf fail on missing request",
			fields: fields{
				CarbideAtomicClient: carbideAtomicClient,
			},
			args: args{
				ctx:     context.Background(),
				request: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm := NewManageExpectedPowerShelf(tt.fields.CarbideAtomicClient)
			err := mm.DeleteExpectedPowerShelfOnSite(tt.args.ctx, tt.args.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
