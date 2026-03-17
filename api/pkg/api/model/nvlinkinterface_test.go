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
	"reflect"
	"testing"
	"time"

	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewAPINVLinkInterface(t *testing.T) {
	type args struct {
		dbnvli *cdbm.NVLinkInterface
	}

	dbnvli := &cdbm.NVLinkInterface{
		ID:                       uuid.New(),
		InstanceID:               uuid.New(),
		SiteID:                   uuid.New(),
		NVLinkLogicalPartitionID: uuid.New(),
		DeviceInstance:           1,
		Status:                   cdbm.NVLinkInterfaceStatusReady,
		Created:                  time.Now(),
		Updated:                  time.Now(),
	}

	tests := []struct {
		name string
		args args
		want *APINVLinkInterface
	}{
		{
			name: "test new API NVLink Interface initializer",
			args: args{
				dbnvli: dbnvli,
			},
			want: &APINVLinkInterface{
				ID:                       dbnvli.ID.String(),
				InstanceID:               dbnvli.InstanceID.String(),
				NVLinkLogicalPartitionID: dbnvli.NVLinkLogicalPartitionID.String(),
				DeviceInstance:           dbnvli.DeviceInstance,
				Status:                   dbnvli.Status,
				Created:                  dbnvli.Created,
				Updated:                  dbnvli.Updated,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewAPINVLinkInterface(tt.args.dbnvli); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAPINVLinkInterface() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPINVLinkInterfaceCreateOrUpdateRequest_Validate(t *testing.T) {
	type fields struct {
		nvLinkLogicalPartitionID string
		deviceInstance           int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "test validation success",
			fields: fields{
				nvLinkLogicalPartitionID: uuid.New().String(),
				deviceInstance:           0,
			},
			wantErr: false,
		},
		{
			name: "test validation failure, invalid NVLink Logical Partition ID",
			fields: fields{
				nvLinkLogicalPartitionID: "badid",
				deviceInstance:           1,
			},
			wantErr: true,
		},
		{
			name: "test validation failure, GPU Index not supported",
			fields: fields{
				nvLinkLogicalPartitionID: uuid.New().String(),
				deviceInstance:           4,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nvlicr := APINVLinkInterfaceCreateOrUpdateRequest{
				NVLinkLogicalPartitionID: tt.fields.nvLinkLogicalPartitionID,
				DeviceInstance:           tt.fields.deviceInstance,
			}
			err := nvlicr.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
