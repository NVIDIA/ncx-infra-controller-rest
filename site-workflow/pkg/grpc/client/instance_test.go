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

package client

import (
	"context"
	"testing"

	"github.com/gogo/status"
	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

func TestInstance_DeleteInstance(t *testing.T) {
	mockCarbide := NewMockCarbideClient()

	type fields struct {
		CarbideClient *CarbideClient
	}
	type args struct {
		ctx     context.Context
		request *wflows.DeleteInstanceRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test delete instance success",
			fields: fields{
				CarbideClient: mockCarbide,
			},
			args: args{
				ctx: context.Background(),
				request: &wflows.DeleteInstanceRequest{
					InstanceId: &wflows.UUID{Value: uuid.New().String()},
				},
			},
			wantErr: false,
		},
		{
			name: "test delete instance failed, NotFound",
			fields: fields{
				CarbideClient: mockCarbide,
			},
			args: args{
				ctx: context.WithValue(context.Background(), "wantError", status.Error(codes.NotFound, "instance not found: ")),
				request: &wflows.DeleteInstanceRequest{
					InstanceId: &wflows.UUID{Value: uuid.New().String()},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := &compute{
				carbide: tt.fields.CarbideClient.carbide,
			}
			_, err := cc.DeleteInstance(tt.args.ctx, tt.args.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInstance_CreateInstance(t *testing.T) {
	mockCarbide := NewMockCarbideClient()

	type fields struct {
		CarbideClient *CarbideClient
	}
	type args struct {
		ctx     context.Context
		request *wflows.CreateInstanceRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test create instance success",
			fields: fields{
				CarbideClient: mockCarbide,
			},
			args: args{
				ctx: context.Background(),
				request: &wflows.CreateInstanceRequest{
					InstanceId:       &wflows.UUID{Value: uuid.New().String()},
					MachineId:        &wflows.MachineId{Id: uuid.New().String()},
					TenantOrg:        "testOrg",
					PhoneHomeEnabled: true,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := &compute{
				carbide: tt.fields.CarbideClient.carbide,
			}
			_, err := cc.CreateInstance(tt.args.ctx, tt.args.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
