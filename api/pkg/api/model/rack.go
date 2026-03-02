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
	"fmt"
	"net/url"

	rlav1 "github.com/nvidia/bare-metal-manager-rest/workflow-schema/rla/protobuf/v1"
)

// ========== Rack Query Fields ==========

// RackFilterFieldMap maps API field names to RLA protobuf filter enum
var RackFilterFieldMap = map[string]rlav1.RackFilterField{
	"name":         rlav1.RackFilterField_RACK_FILTER_FIELD_NAME,
	"manufacturer": rlav1.RackFilterField_RACK_FILTER_FIELD_MANUFACTURER,
	"model":        rlav1.RackFilterField_RACK_FILTER_FIELD_MODEL,
}

// RackOrderByFieldMap maps API field names to RLA protobuf order by enum
var RackOrderByFieldMap = map[string]rlav1.RackOrderByField{
	"name":         rlav1.RackOrderByField_RACK_ORDER_BY_FIELD_NAME,
	"manufacturer": rlav1.RackOrderByField_RACK_ORDER_BY_FIELD_MANUFACTURER,
	"model":        rlav1.RackOrderByField_RACK_ORDER_BY_FIELD_MODEL,
}

// GetProtoRackFilter creates an RLA protobuf filter for the given rack field and patterns.
// Multiple patterns are OR'd together.
func GetProtoRackFilter(fieldName string, patterns []string) *rlav1.Filter {
	field, ok := RackFilterFieldMap[fieldName]
	if !ok || len(patterns) == 0 {
		return nil
	}
	return &rlav1.Filter{
		Field: &rlav1.Filter_RackField{
			RackField: field,
		},
		QueryInfo: &rlav1.StringQueryInfo{
			Patterns:   patterns,
			IsWildcard: false,
			UseOr:      len(patterns) > 1,
		},
	}
}

// GetProtoRackOrderByFromQueryParam creates an RLA protobuf OrderBy from API query parameters
func GetProtoRackOrderByFromQueryParam(fieldName, direction string) *rlav1.OrderBy {
	field, ok := RackOrderByFieldMap[fieldName]
	if !ok {
		return nil
	}
	return &rlav1.OrderBy{
		Field: &rlav1.OrderBy_RackField{
			RackField: field,
		},
		Direction: direction,
	}
}

// ========== Rack Request Models ==========

// APIRackGetRequest captures query parameters for getting a single rack.
type APIRackGetRequest struct {
	SiteID            string `query:"siteId"`
	IncludeComponents bool   `query:"includeComponents"`
}

func (r *APIRackGetRequest) Validate() error {
	if r.SiteID == "" {
		return fmt.Errorf("siteId query parameter is required")
	}
	return nil
}

// ========== Rack Filter (for batch operations) ==========

// RackFilter specifies which racks to target in a batch operation.
// If nil or empty, the operation targets all racks in the site.
type RackFilter struct {
	Names []string `json:"names,omitempty"`
}

// ToTargetSpec converts the filter to an RLA OperationTargetSpec.
// Handles nil receiver gracefully (targets all racks).
func (f *RackFilter) ToTargetSpec() *rlav1.OperationTargetSpec {
	var rackTargets []*rlav1.RackTarget

	if f != nil {
		for _, name := range f.Names {
			rackTargets = append(rackTargets, &rlav1.RackTarget{
				Identifier: &rlav1.RackTarget_Name{
					Name: name,
				},
			})
		}
	}

	if len(rackTargets) == 0 {
		rackTargets = append(rackTargets, &rlav1.RackTarget{})
	}

	return &rlav1.OperationTargetSpec{
		Targets: &rlav1.OperationTargetSpec_Racks{
			Racks: &rlav1.RackTargets{
				Targets: rackTargets,
			},
		},
	}
}

// APIRackGetAllRequest captures query parameters for listing racks.
type APIRackGetAllRequest struct {
	SiteID            string   `query:"siteId"`
	IncludeComponents bool     `query:"includeComponents"`
	Name              []string `query:"name"`
	Manufacturer      []string `query:"manufacturer"`
	PageNumber        string   `query:"pageNumber"`
	PageSize          string   `query:"pageSize"`
	OrderBy           string   `query:"orderBy"`
}

func (r *APIRackGetAllRequest) Validate() error {
	if r.SiteID == "" {
		return fmt.Errorf("siteId query parameter is required")
	}
	return nil
}

// ToFilters converts the request's filter fields to RLA protobuf filters.
func (r *APIRackGetAllRequest) ToFilters() []*rlav1.Filter {
	var filters []*rlav1.Filter
	if f := GetProtoRackFilter("name", r.Name); f != nil {
		filters = append(filters, f)
	}
	if f := GetProtoRackFilter("manufacturer", r.Manufacturer); f != nil {
		filters = append(filters, f)
	}
	return filters
}

// QueryValues returns only the known query parameters as url.Values,
// suitable for deterministic workflow ID hashing without unknown param interference.
func (r *APIRackGetAllRequest) QueryValues() url.Values {
	v := url.Values{}
	v.Set("siteId", r.SiteID)
	if r.IncludeComponents {
		v.Set("includeComponents", "true")
	}
	for _, n := range r.Name {
		v.Add("name", n)
	}
	for _, m := range r.Manufacturer {
		v.Add("manufacturer", m)
	}
	if r.PageNumber != "" {
		v.Set("pageNumber", r.PageNumber)
	}
	if r.PageSize != "" {
		v.Set("pageSize", r.PageSize)
	}
	if r.OrderBy != "" {
		v.Set("orderBy", r.OrderBy)
	}
	return v
}

// APIRackValidateAllRequest captures query parameters for validating racks.
type APIRackValidateAllRequest struct {
	SiteID       string   `query:"siteId"`
	Name         []string `query:"name"`
	Manufacturer []string `query:"manufacturer"`
}

func (r *APIRackValidateAllRequest) Validate() error {
	if r.SiteID == "" {
		return fmt.Errorf("siteId query parameter is required")
	}
	return nil
}

// ToFilters converts the request's filter fields to RLA protobuf filters.
func (r *APIRackValidateAllRequest) ToFilters() []*rlav1.Filter {
	var filters []*rlav1.Filter
	if f := GetProtoRackFilter("name", r.Name); f != nil {
		filters = append(filters, f)
	}
	if f := GetProtoRackFilter("manufacturer", r.Manufacturer); f != nil {
		filters = append(filters, f)
	}
	return filters
}

// QueryValues returns only the known query parameters as url.Values.
func (r *APIRackValidateAllRequest) QueryValues() url.Values {
	v := url.Values{}
	v.Set("siteId", r.SiteID)
	for _, n := range r.Name {
		v.Add("name", n)
	}
	for _, m := range r.Manufacturer {
		v.Add("manufacturer", m)
	}
	return v
}

// ========== Rack API Models ==========

// APIRack is the API representation of a Rack from RLA
type APIRack struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Manufacturer string              `json:"manufacturer"`
	Model        string              `json:"model"`
	SerialNumber string              `json:"serialNumber"`
	Description  string              `json:"description"`
	Location     *APIRackLocation    `json:"location,omitempty"`
	Components   []*APIRackComponent `json:"components,omitempty"`
}

// FromProto converts an RLA protobuf Rack to an APIRack
func (ar *APIRack) FromProto(protoRack *rlav1.Rack, includeComponents bool) {
	if protoRack == nil {
		return
	}

	// Get info from DeviceInfo
	if protoRack.GetInfo() != nil {
		info := protoRack.GetInfo()
		if info.GetId() != nil {
			ar.ID = info.GetId().GetId()
		}
		ar.Name = info.GetName()
		ar.Manufacturer = info.GetManufacturer()
		if info.Model != nil {
			ar.Model = *info.Model
		}
		ar.SerialNumber = info.GetSerialNumber()
		if info.Description != nil {
			ar.Description = *info.Description
		}
	}

	// Get location
	if protoRack.GetLocation() != nil {
		ar.Location = &APIRackLocation{}
		ar.Location.FromProto(protoRack.GetLocation())
	}

	// Get components
	if includeComponents && len(protoRack.GetComponents()) > 0 {
		ar.Components = make([]*APIRackComponent, 0, len(protoRack.GetComponents()))
		for _, comp := range protoRack.GetComponents() {
			apiComp := &APIRackComponent{}
			apiComp.FromProto(comp)
			ar.Components = append(ar.Components, apiComp)
		}
	}
}

// NewAPIRack creates an APIRack from the RLA protobuf Rack
func NewAPIRack(protoRack *rlav1.Rack, includeComponents bool) *APIRack {
	if protoRack == nil {
		return nil
	}
	apiRack := &APIRack{}
	apiRack.FromProto(protoRack, includeComponents)
	return apiRack
}

// APIRackLocation represents the location of a rack
type APIRackLocation struct {
	Region     string `json:"region"`
	Datacenter string `json:"datacenter"`
	Room       string `json:"room"`
	Position   string `json:"position"`
}

// FromProto converts a proto Location to an APIRackLocation
func (arl *APIRackLocation) FromProto(protoLocation *rlav1.Location) {
	if protoLocation == nil {
		return
	}
	arl.Region = protoLocation.GetRegion()
	arl.Datacenter = protoLocation.GetDatacenter()
	arl.Room = protoLocation.GetRoom()
	arl.Position = protoLocation.GetPosition()
}

// APIBMC represents a BMC (Baseboard Management Controller) entry
type APIBMC struct {
	Type       string `json:"type"`
	MacAddress string `json:"macAddress"`
	IPAddress  string `json:"ipAddress"`
}

// FromProto converts a proto BMC to an APIBMC
func (ab *APIBMC) FromProto(protoBMC *rlav1.BMCInfo) {
	if protoBMC == nil {
		return
	}
	ab.Type = protoBMC.GetType().String()
	ab.MacAddress = protoBMC.GetMacAddress()
	ab.IPAddress = protoBMC.GetIpAddress()
}

// APIRackComponent represents a component within a rack
type APIRackComponent struct {
	ID              string    `json:"id"`
	ComponentID     string    `json:"componentId"`
	RackID          string    `json:"rackId"`
	Type            string    `json:"type"`
	Name            string    `json:"name"`
	SerialNumber    string    `json:"serialNumber"`
	Manufacturer    string    `json:"manufacturer"`
	Model           string    `json:"model"`
	Description     string    `json:"description"`
	FirmwareVersion string    `json:"firmwareVersion"`
	SlotID          int32     `json:"slotId"`
	TrayIdx         int32     `json:"trayIdx"`
	HostID          int32     `json:"hostId"`
	BMCs            []*APIBMC `json:"bmcs"`
	PowerState      string    `json:"powerState"`
}

// FromProto converts a proto Component to an APIRackComponent
func (arc *APIRackComponent) FromProto(protoComponent *rlav1.Component) {
	if protoComponent == nil {
		return
	}
	arc.Type = protoComponent.GetType().String()
	arc.FirmwareVersion = protoComponent.GetFirmwareVersion()
	arc.ComponentID = protoComponent.GetComponentId()
	arc.PowerState = protoComponent.GetPowerState()

	// Get rack ID
	if protoComponent.GetRackId() != nil {
		arc.RackID = protoComponent.GetRackId().GetId()
	}

	// Get component info
	if protoComponent.GetInfo() != nil {
		compInfo := protoComponent.GetInfo()
		if compInfo.GetId() != nil {
			arc.ID = compInfo.GetId().GetId()
		}
		arc.Name = compInfo.GetName()
		arc.SerialNumber = compInfo.GetSerialNumber()
		arc.Manufacturer = compInfo.GetManufacturer()
		arc.Model = compInfo.GetModel()
		arc.Description = compInfo.GetDescription()
	}

	// Get position
	if protoComponent.GetPosition() != nil {
		arc.SlotID = protoComponent.GetPosition().GetSlotId()
		arc.TrayIdx = protoComponent.GetPosition().GetTrayIdx()
		arc.HostID = protoComponent.GetPosition().GetHostId()
	}

	// Get BMCs
	if len(protoComponent.GetBmcs()) > 0 {
		arc.BMCs = make([]*APIBMC, 0, len(protoComponent.GetBmcs()))
		for _, bmc := range protoComponent.GetBmcs() {
			apiBMC := &APIBMC{}
			apiBMC.FromProto(bmc)
			arc.BMCs = append(arc.BMCs, apiBMC)
		}
	}
}

// ========== Rack Validation API Models ==========

// APIFieldDiff represents a single field difference
type APIFieldDiff struct {
	FieldName     string `json:"fieldName"`
	ExpectedValue string `json:"expectedValue"`
	ActualValue   string `json:"actualValue"`
}

// FromProto converts an RLA protobuf FieldDiff to an APIFieldDiff
func (f *APIFieldDiff) FromProto(protoFieldDiff *rlav1.FieldDiff) {
	if protoFieldDiff == nil {
		return
	}
	f.FieldName = protoFieldDiff.GetFieldName()
	f.ExpectedValue = protoFieldDiff.GetExpectedValue()
	f.ActualValue = protoFieldDiff.GetActualValue()
}

// APIComponentDiff represents a single component difference found during validation
type APIComponentDiff struct {
	Type        string            `json:"type"`
	ComponentID string            `json:"componentId"`
	Expected    *APIRackComponent `json:"expected,omitempty"`
	Actual      *APIRackComponent `json:"actual,omitempty"`
	FieldDiffs  []*APIFieldDiff   `json:"fieldDiffs,omitempty"`
}

// FromProto converts an RLA protobuf ComponentDiff to an APIComponentDiff
func (d *APIComponentDiff) FromProto(protoDiff *rlav1.ComponentDiff) {
	if protoDiff == nil {
		return
	}

	d.Type = protoDiff.GetType().String()
	d.ComponentID = protoDiff.GetComponentId()

	if protoDiff.GetExpected() != nil {
		d.Expected = &APIRackComponent{}
		d.Expected.FromProto(protoDiff.GetExpected())
	}

	if protoDiff.GetActual() != nil {
		d.Actual = &APIRackComponent{}
		d.Actual.FromProto(protoDiff.GetActual())
	}

	if len(protoDiff.GetFieldDiffs()) > 0 {
		d.FieldDiffs = make([]*APIFieldDiff, 0, len(protoDiff.GetFieldDiffs()))
		for _, fd := range protoDiff.GetFieldDiffs() {
			apiFieldDiff := &APIFieldDiff{}
			apiFieldDiff.FromProto(fd)
			d.FieldDiffs = append(d.FieldDiffs, apiFieldDiff)
		}
	}
}

// APIRackValidationResult is the API representation of a rack validation result
type APIRackValidationResult struct {
	Diffs               []*APIComponentDiff `json:"diffs"`
	TotalDiffs          int32               `json:"totalDiffs"`
	OnlyInExpectedCount int32               `json:"onlyInExpectedCount"`
	OnlyInActualCount   int32               `json:"onlyInActualCount"`
	DriftCount          int32               `json:"driftCount"`
	MatchCount          int32               `json:"matchCount"`
}

// FromProto converts an RLA protobuf ValidateComponentsResponse to an APIRackValidationResult
func (r *APIRackValidationResult) FromProto(protoResp *rlav1.ValidateComponentsResponse) {
	if protoResp == nil {
		return
	}

	r.TotalDiffs = protoResp.GetTotalDiffs()
	r.OnlyInExpectedCount = protoResp.GetOnlyInExpectedCount()
	r.OnlyInActualCount = protoResp.GetOnlyInActualCount()
	r.DriftCount = protoResp.GetDriftCount()
	r.MatchCount = protoResp.GetMatchCount()

	r.Diffs = make([]*APIComponentDiff, 0, len(protoResp.GetDiffs()))
	for _, diff := range protoResp.GetDiffs() {
		apiDiff := &APIComponentDiff{}
		apiDiff.FromProto(diff)
		r.Diffs = append(r.Diffs, apiDiff)
	}
}

// NewAPIRackValidationResult creates an APIRackValidationResult from the RLA protobuf response
func NewAPIRackValidationResult(protoResp *rlav1.ValidateComponentsResponse) *APIRackValidationResult {
	if protoResp == nil {
		return nil
	}
	result := &APIRackValidationResult{}
	result.FromProto(protoResp)
	return result
}
