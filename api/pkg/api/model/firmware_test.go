/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package model

import (
	"testing"

	rlav1 "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/rla/protobuf/v1"
	"github.com/stretchr/testify/assert"
)

func TestAPIUpdateFirmwareRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request APIUpdateFirmwareRequest
		wantErr bool
	}{
		{
			name:    "valid - with siteId and version",
			request: APIUpdateFirmwareRequest{SiteID: "site-1", Version: strPtr("24.11.0")},
			wantErr: false,
		},
		{
			name:    "valid - with siteId only (no version)",
			request: APIUpdateFirmwareRequest{SiteID: "site-1"},
			wantErr: false,
		},
		{
			name:    "invalid - missing siteId",
			request: APIUpdateFirmwareRequest{Version: strPtr("24.11.0")},
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

func TestNewAPIUpdateFirmwareResponse(t *testing.T) {
	tests := []struct {
		name     string
		resp     *rlav1.SubmitTaskResponse
		expected *APIUpdateFirmwareResponse
	}{
		{
			name:     "nil response returns empty task IDs",
			resp:     nil,
			expected: &APIUpdateFirmwareResponse{TaskIDs: []string{}},
		},
		{
			name: "single task ID",
			resp: &rlav1.SubmitTaskResponse{
				TaskIds: []*rlav1.UUID{{Id: "task-1"}},
			},
			expected: &APIUpdateFirmwareResponse{TaskIDs: []string{"task-1"}},
		},
		{
			name: "multiple task IDs",
			resp: &rlav1.SubmitTaskResponse{
				TaskIds: []*rlav1.UUID{{Id: "task-1"}, {Id: "task-2"}},
			},
			expected: &APIUpdateFirmwareResponse{TaskIDs: []string{"task-1", "task-2"}},
		},
		{
			name:     "empty task IDs",
			resp:     &rlav1.SubmitTaskResponse{},
			expected: &APIUpdateFirmwareResponse{TaskIDs: []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAPIUpdateFirmwareResponse(tt.resp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAPIBatchRackFirmwareUpdateRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request APIBatchRackFirmwareUpdateRequest
		wantErr bool
	}{
		{
			name:    "valid - with siteId only",
			request: APIBatchRackFirmwareUpdateRequest{SiteID: "site-1"},
			wantErr: false,
		},
		{
			name: "valid - with filter and version",
			request: APIBatchRackFirmwareUpdateRequest{
				SiteID:  "site-1",
				Filter:  &RackFilter{Names: []string{"rack-1"}},
				Version: strPtr("1.0"),
			},
			wantErr: false,
		},
		{
			name:    "invalid - missing siteId",
			request: APIBatchRackFirmwareUpdateRequest{},
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

func strPtr(s string) *string { return &s }
