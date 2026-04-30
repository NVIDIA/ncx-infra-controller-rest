/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package model

import (
	"reflect"
	"testing"
	"time"

	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/google/uuid"
)

func TestNewAPIMachineInstanceType(t *testing.T) {
	type args struct {
		dbmit *cdbm.MachineInstanceType
	}

	dbmit := &cdbm.MachineInstanceType{
		ID:             uuid.New(),
		MachineID:      uuid.NewString(),
		InstanceTypeID: uuid.New(),
		Created:        time.Now(),
		Updated:        time.Now(),
	}

	tests := []struct {
		name string
		args args
		want *APIMachineInstanceType
	}{
		{
			name: "test new API Machine Instance Type initializer",
			args: args{
				dbmit: dbmit,
			},
			want: func() *APIMachineInstanceType {
				expected := &APIMachineInstanceType{
					ID:             dbmit.ID.String(),
					MachineID:      dbmit.MachineID,
					InstanceTypeID: dbmit.InstanceTypeID.String(),
					Created:        dbmit.Created,
					Updated:        dbmit.Updated,
				}

				for _, deprecation := range machineInstanceTypeDeprecations {
					expected.Deprecations = append(expected.Deprecations, NewAPIDeprecation(deprecation))
				}

				return expected
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewAPIMachineInstanceType(tt.args.dbmit); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAPIMachineInstanceType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIMachineInstanceTypeCreateRequest_Validate(t *testing.T) {
	type fields struct {
		MachineIDs []string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "test valid Machine Instance Type request",
			fields: fields{
				MachineIDs: []string{"test-machine-id", uuid.NewString()},
			},
			wantErr: false,
		},
		{
			name: "test invalid Machine Instance Type request, empty MachineIDs",
			fields: fields{
				MachineIDs: []string{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mitcr := APIMachineInstanceTypeCreateRequest{
				MachineIDs: tt.fields.MachineIDs,
			}
			if err := mitcr.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("APIMachineInstanceTypeCreateRequest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
