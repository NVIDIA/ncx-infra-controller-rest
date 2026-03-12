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
	"testing"

	rlav1 "github.com/nvidia/bare-metal-manager-rest/workflow-schema/rla/protobuf/v1"
	"github.com/stretchr/testify/assert"
)

func TestNewAPIRack(t *testing.T) {
	description := "Test rack description"
	model := "NVL72"

	tests := []struct {
		name           string
		rack           *rlav1.Rack
		withComponents bool
		want           *APIRack
	}{
		{
			name:           "nil rack returns nil",
			rack:           nil,
			withComponents: false,
			want:           nil,
		},
		{
			name: "basic rack without components",
			rack: &rlav1.Rack{
				Info: &rlav1.DeviceInfo{
					Id:           &rlav1.UUID{Id: "test-rack-id"},
					Name:         "test-rack",
					Manufacturer: "NVIDIA",
					Model:        &model,
					SerialNumber: "SN12345",
					Description:  &description,
				},
				Location: &rlav1.Location{
					Region:     "us-west-2",
					Datacenter: "DC1",
					Room:       "Room-A",
					Position:   "Row-1-Pos-5",
				},
			},
			withComponents: false,
			want: &APIRack{
				ID:           "test-rack-id",
				Name:         "test-rack",
				Manufacturer: "NVIDIA",
				Model:        "NVL72",
				SerialNumber: "SN12345",
				Description:  "Test rack description",
				Location: &APIRackLocation{
					Region:     "us-west-2",
					Datacenter: "DC1",
					Room:       "Room-A",
					Position:   "Row-1-Pos-5",
				},
				Components: nil,
			},
		},
		{
			name: "rack with components",
			rack: &rlav1.Rack{
				Info: &rlav1.DeviceInfo{
					Id:   &rlav1.UUID{Id: "rack-with-components"},
					Name: "rack-1",
				},
				Components: []*rlav1.Component{
					{
						Type: rlav1.ComponentType_COMPONENT_TYPE_COMPUTE,
						Info: &rlav1.DeviceInfo{
							Id:           &rlav1.UUID{Id: "comp-1"},
							Name:         "compute-node-1",
							SerialNumber: "CSN001",
							Manufacturer: "NVIDIA",
						},
						FirmwareVersion: "1.0.0",
						Position: &rlav1.RackPosition{
							SlotId: 1,
						},
						ComponentId: "carbide-machine-123",
					},
					{
						Type: rlav1.ComponentType_COMPONENT_TYPE_TORSWITCH,
						Info: &rlav1.DeviceInfo{
							Id:   &rlav1.UUID{Id: "comp-2"},
							Name: "switch-1",
						},
						Position: &rlav1.RackPosition{
							SlotId: 48,
						},
					},
				},
			},
			withComponents: true,
			want: &APIRack{
				ID:   "rack-with-components",
				Name: "rack-1",
				Components: []*APIRackComponent{
					{
						ID:              "comp-1",
						ComponentID:     "carbide-machine-123",
						Type:            "COMPONENT_TYPE_COMPUTE",
						Name:            "compute-node-1",
						SerialNumber:    "CSN001",
						Manufacturer:    "NVIDIA",
						FirmwareVersion: "1.0.0",
						SlotID:          1,
					},
					{
						ID:     "comp-2",
						Type:   "COMPONENT_TYPE_TORSWITCH",
						Name:   "switch-1",
						SlotID: 48,
					},
				},
			},
		},
		{
			name: "rack with components but withComponents=false",
			rack: &rlav1.Rack{
				Info: &rlav1.DeviceInfo{
					Id:   &rlav1.UUID{Id: "rack-id"},
					Name: "rack-name",
				},
				Components: []*rlav1.Component{
					{
						Type: rlav1.ComponentType_COMPONENT_TYPE_COMPUTE,
						Info: &rlav1.DeviceInfo{
							Id:   &rlav1.UUID{Id: "comp-1"},
							Name: "compute-node-1",
						},
					},
				},
			},
			withComponents: false,
			want: &APIRack{
				ID:         "rack-id",
				Name:       "rack-name",
				Components: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAPIRack(tt.rack, tt.withComponents)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			assert.NotNil(t, got)
			assert.Equal(t, tt.want.ID, got.ID)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.Manufacturer, got.Manufacturer)
			assert.Equal(t, tt.want.Model, got.Model)
			assert.Equal(t, tt.want.SerialNumber, got.SerialNumber)
			assert.Equal(t, tt.want.Description, got.Description)

			if tt.want.Location != nil {
				assert.NotNil(t, got.Location)
				assert.Equal(t, tt.want.Location.Region, got.Location.Region)
				assert.Equal(t, tt.want.Location.Datacenter, got.Location.Datacenter)
				assert.Equal(t, tt.want.Location.Room, got.Location.Room)
				assert.Equal(t, tt.want.Location.Position, got.Location.Position)
			}

			if tt.want.Components != nil {
				assert.NotNil(t, got.Components)
				assert.Equal(t, len(tt.want.Components), len(got.Components))
				for i, wantComp := range tt.want.Components {
					gotComp := got.Components[i]
					assert.Equal(t, wantComp.ID, gotComp.ID)
					assert.Equal(t, wantComp.ComponentID, gotComp.ComponentID)
					assert.Equal(t, wantComp.Type, gotComp.Type)
					assert.Equal(t, wantComp.Name, gotComp.Name)
					assert.Equal(t, wantComp.SerialNumber, gotComp.SerialNumber)
					assert.Equal(t, wantComp.Manufacturer, gotComp.Manufacturer)
					assert.Equal(t, wantComp.FirmwareVersion, gotComp.FirmwareVersion)
					assert.Equal(t, wantComp.SlotID, gotComp.SlotID)
				}
			} else {
				assert.Nil(t, got.Components)
			}
		})
	}
}

func TestAPICreateRackRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request APICreateRackRequest
		wantErr bool
	}{
		{
			name: "valid - all required fields",
			request: APICreateRackRequest{
				SiteID:       "site-1",
				Name:         "Rack-01",
				Manufacturer: "NVIDIA",
				SerialNumber: "SN-001",
			},
			wantErr: false,
		},
		{
			name: "valid - with optional fields",
			request: APICreateRackRequest{
				SiteID:       "site-1",
				Name:         "Rack-01",
				Manufacturer: "NVIDIA",
				SerialNumber: "SN-001",
				Model:        "NVL72",
				Description:  "Test rack",
				Location:     &APIRackLocation{Region: "us-east-1"},
			},
			wantErr: false,
		},
		{
			name:    "invalid - missing siteId",
			request: APICreateRackRequest{Name: "Rack-01", Manufacturer: "NVIDIA", SerialNumber: "SN-001"},
			wantErr: true,
		},
		{
			name:    "invalid - missing name",
			request: APICreateRackRequest{SiteID: "site-1", Manufacturer: "NVIDIA", SerialNumber: "SN-001"},
			wantErr: true,
		},
		{
			name:    "invalid - missing manufacturer",
			request: APICreateRackRequest{SiteID: "site-1", Name: "Rack-01", SerialNumber: "SN-001"},
			wantErr: true,
		},
		{
			name:    "invalid - missing serialNumber",
			request: APICreateRackRequest{SiteID: "site-1", Name: "Rack-01", Manufacturer: "NVIDIA"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPICreateRackRequest_ToProtoRack(t *testing.T) {
	model := "NVL72"
	desc := "Test rack"
	req := APICreateRackRequest{
		Name:         "Rack-01",
		Manufacturer: "NVIDIA",
		SerialNumber: "SN-001",
		Model:        model,
		Description:  desc,
		Location: &APIRackLocation{
			Region:     "us-east-1",
			Datacenter: "DC1",
			Room:       "A",
			Position:   "1",
		},
	}

	proto := req.ToProtoRack()
	assert.NotNil(t, proto)
	assert.NotNil(t, proto.Info)
	assert.Equal(t, "Rack-01", proto.Info.Name)
	assert.Equal(t, "NVIDIA", proto.Info.Manufacturer)
	assert.Equal(t, "SN-001", proto.Info.SerialNumber)
	assert.Equal(t, &model, proto.Info.Model)
	assert.Equal(t, &desc, proto.Info.Description)
	assert.NotNil(t, proto.Location)
	assert.Equal(t, "us-east-1", proto.Location.Region)
	assert.Equal(t, "DC1", proto.Location.Datacenter)
}

func TestAPIPatchRackRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request APIPatchRackRequest
		wantErr bool
	}{
		{
			name:    "valid - with siteId",
			request: APIPatchRackRequest{SiteID: "site-1"},
			wantErr: false,
		},
		{
			name:    "invalid - missing siteId",
			request: APIPatchRackRequest{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIPatchRackRequest_ToProtoRack(t *testing.T) {
	name := "Updated-Rack"
	manufacturer := "Dell"
	req := APIPatchRackRequest{
		SiteID:       "site-1",
		Name:         &name,
		Manufacturer: &manufacturer,
		Location: &APIRackLocation{
			Region: "us-west-2",
		},
	}

	proto := req.ToProtoRack("test-rack-id")
	assert.NotNil(t, proto)
	assert.NotNil(t, proto.Info)
	assert.Equal(t, "test-rack-id", proto.Info.Id.Id)
	assert.Equal(t, "Updated-Rack", proto.Info.Name)
	assert.Equal(t, "Dell", proto.Info.Manufacturer)
	assert.NotNil(t, proto.Location)
	assert.Equal(t, "us-west-2", proto.Location.Region)
}

func TestNewAPICreateRackResponse(t *testing.T) {
	tests := []struct {
		name     string
		resp     *rlav1.CreateExpectedRackResponse
		expected *APICreateRackResponse
	}{
		{
			name:     "nil response returns empty",
			resp:     nil,
			expected: &APICreateRackResponse{},
		},
		{
			name: "valid response",
			resp: &rlav1.CreateExpectedRackResponse{
				Id: &rlav1.UUID{Id: "new-rack-id"},
			},
			expected: &APICreateRackResponse{ID: "new-rack-id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAPICreateRackResponse(tt.resp)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected.ID, result.ID)
		})
	}
}

func TestNewAPIPatchRackResponse(t *testing.T) {
	tests := []struct {
		name     string
		resp     *rlav1.PatchRackResponse
		expected *APIPatchRackResponse
	}{
		{
			name:     "nil response returns empty",
			resp:     nil,
			expected: &APIPatchRackResponse{},
		},
		{
			name: "valid response",
			resp: &rlav1.PatchRackResponse{
				Report: "Rack updated successfully",
			},
			expected: &APIPatchRackResponse{Report: "Rack updated successfully"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAPIPatchRackResponse(tt.resp)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected.Report, result.Report)
		})
	}
}

func TestAPIBringUpRackRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request APIBringUpRackRequest
		wantErr bool
	}{
		{
			name:    "valid - with siteId",
			request: APIBringUpRackRequest{SiteID: "site-1"},
			wantErr: false,
		},
		{
			name:    "valid - with siteId and description",
			request: APIBringUpRackRequest{SiteID: "site-1", Description: "bring up rack"},
			wantErr: false,
		},
		{
			name:    "invalid - missing siteId",
			request: APIBringUpRackRequest{},
			wantErr: true,
		},
		{
			name:    "invalid - empty siteId",
			request: APIBringUpRackRequest{SiteID: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewAPIBringUpRackResponse(t *testing.T) {
	tests := []struct {
		name     string
		resp     *rlav1.SubmitTaskResponse
		expected *APIBringUpRackResponse
	}{
		{
			name:     "nil response returns empty task IDs",
			resp:     nil,
			expected: &APIBringUpRackResponse{TaskIDs: []string{}},
		},
		{
			name: "single task ID",
			resp: &rlav1.SubmitTaskResponse{
				TaskIds: []*rlav1.UUID{{Id: "task-1"}},
			},
			expected: &APIBringUpRackResponse{TaskIDs: []string{"task-1"}},
		},
		{
			name: "multiple task IDs",
			resp: &rlav1.SubmitTaskResponse{
				TaskIds: []*rlav1.UUID{{Id: "task-1"}, {Id: "task-2"}},
			},
			expected: &APIBringUpRackResponse{TaskIDs: []string{"task-1", "task-2"}},
		},
		{
			name:     "empty task IDs",
			resp:     &rlav1.SubmitTaskResponse{},
			expected: &APIBringUpRackResponse{TaskIDs: []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAPIBringUpRackResponse(tt.resp)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected.TaskIDs, result.TaskIDs)
		})
	}
}

func TestAPIBatchBringUpRackRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request APIBatchBringUpRackRequest
		wantErr bool
	}{
		{
			name:    "valid - with siteId only",
			request: APIBatchBringUpRackRequest{SiteID: "site-1"},
			wantErr: false,
		},
		{
			name: "valid - with filter",
			request: APIBatchBringUpRackRequest{
				SiteID: "site-1",
				Filter: &RackFilter{Names: []string{"Rack-001"}},
			},
			wantErr: false,
		},
		{
			name: "valid - with description",
			request: APIBatchBringUpRackRequest{
				SiteID:      "site-1",
				Description: "batch bring up",
			},
			wantErr: false,
		},
		{
			name:    "invalid - missing siteId",
			request: APIBatchBringUpRackRequest{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
