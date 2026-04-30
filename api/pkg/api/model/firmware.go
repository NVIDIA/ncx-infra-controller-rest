/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package model

import (
	"fmt"

	rlav1 "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/rla/protobuf/v1"
)

// ========== Firmware Update Request ==========

// APIUpdateFirmwareRequest is the request body for firmware update operations
type APIUpdateFirmwareRequest struct {
	SiteID  string  `json:"siteId"`
	Version *string `json:"version,omitempty"`
}

// Validate validates the firmware update request
func (r *APIUpdateFirmwareRequest) Validate() error {
	if r.SiteID == "" {
		return fmt.Errorf("siteId is required")
	}
	return nil
}

// ========== Firmware Update Response ==========

// APIUpdateFirmwareResponse is the API response for firmware update operations
type APIUpdateFirmwareResponse struct {
	TaskIDs []string `json:"taskIds"`
}

// FromProto converts an RLA SubmitTaskResponse to an APIUpdateFirmwareResponse
func (r *APIUpdateFirmwareResponse) FromProto(resp *rlav1.SubmitTaskResponse) {
	if resp == nil {
		r.TaskIDs = []string{}
		return
	}
	r.TaskIDs = make([]string, 0, len(resp.GetTaskIds()))
	for _, id := range resp.GetTaskIds() {
		r.TaskIDs = append(r.TaskIDs, id.GetId())
	}
}

// NewAPIUpdateFirmwareResponse creates an APIUpdateFirmwareResponse from an RLA SubmitTaskResponse
func NewAPIUpdateFirmwareResponse(resp *rlav1.SubmitTaskResponse) *APIUpdateFirmwareResponse {
	r := &APIUpdateFirmwareResponse{}
	r.FromProto(resp)
	return r
}

// ========== Batch Rack Firmware Update Request ==========

// APIBatchRackFirmwareUpdateRequest is the JSON body for batch rack firmware update.
type APIBatchRackFirmwareUpdateRequest struct {
	SiteID  string      `json:"siteId"`
	Filter  *RackFilter `json:"filter,omitempty"`
	Version *string     `json:"version,omitempty"`
}

// Validate checks required fields.
func (r *APIBatchRackFirmwareUpdateRequest) Validate() error {
	if r.SiteID == "" {
		return fmt.Errorf("siteId is required")
	}
	return nil
}

// ========== Batch Tray Firmware Update Request ==========

// APIBatchTrayFirmwareUpdateRequest is the JSON body for batch tray firmware update.
type APIBatchTrayFirmwareUpdateRequest struct {
	SiteID  string      `json:"siteId"`
	Filter  *TrayFilter `json:"filter,omitempty"`
	Version *string     `json:"version,omitempty"`
}

// Validate checks required fields and filter constraints.
func (r *APIBatchTrayFirmwareUpdateRequest) Validate() error {
	if r.SiteID == "" {
		return fmt.Errorf("siteId is required")
	}
	return r.Filter.Validate()
}
