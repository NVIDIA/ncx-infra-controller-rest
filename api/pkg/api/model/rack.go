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

	validation "github.com/go-ozzo/ozzo-validation/v4"
	validationis "github.com/go-ozzo/ozzo-validation/v4/is"

	rlav1 "github.com/nvidia/bare-metal-manager-rest/workflow-schema/rla/protobuf/v1"
)

// ProtoToAPIBMCTypeName maps protobuf BMCType to API-friendly names.
var ProtoToAPIBMCTypeName = map[rlav1.BMCType]string{
	rlav1.BMCType_BMC_TYPE_UNKNOWN: "BmcTypeUnknown",
	rlav1.BMCType_BMC_TYPE_HOST:    "BmcTypeHost",
	rlav1.BMCType_BMC_TYPE_DPU:     "BmcTypeDpu",
}

// ProtoToAPIRackComponentTypeName maps protobuf ComponentType to API-friendly names for rack components.
var ProtoToAPIRackComponentTypeName = map[rlav1.ComponentType]string{
	rlav1.ComponentType_COMPONENT_TYPE_UNKNOWN:    "ComponentTypeUnknown",
	rlav1.ComponentType_COMPONENT_TYPE_COMPUTE:    "ComponentTypeCompute",
	rlav1.ComponentType_COMPONENT_TYPE_NVLSWITCH:  "ComponentTypeNvlswitch",
	rlav1.ComponentType_COMPONENT_TYPE_POWERSHELF: "ComponentTypePowershelf",
	rlav1.ComponentType_COMPONENT_TYPE_TORSWITCH:  "ComponentTypeTorswitch",
	rlav1.ComponentType_COMPONENT_TYPE_UMS:        "ComponentTypeUms",
	rlav1.ComponentType_COMPONENT_TYPE_CDU:        "ComponentTypeCdu",
}

// ProtoToAPIDiffTypeName maps protobuf DiffType to API-friendly names.
var ProtoToAPIDiffTypeName = map[rlav1.DiffType]string{
	rlav1.DiffType_DIFF_TYPE_UNKNOWN:          "DiffTypeUnknown",
	rlav1.DiffType_DIFF_TYPE_ONLY_IN_EXPECTED: "DiffTypeOnlyInExpected",
	rlav1.DiffType_DIFF_TYPE_ONLY_IN_ACTUAL:   "DiffTypeOnlyInActual",
	rlav1.DiffType_DIFF_TYPE_DRIFT:            "DiffTypeDrift",
}

// enumOr returns mapped value or fallback when key is missing from mapping.
func enumOr[K comparable](m map[K]string, key K, fallback string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return fallback
}

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
	ab.Type = enumOr(ProtoToAPIBMCTypeName, protoBMC.GetType(), "BmcTypeUnknown")
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
	arc.Type = enumOr(ProtoToAPIRackComponentTypeName, protoComponent.GetType(), "ComponentTypeUnknown")
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

	d.Type = enumOr(ProtoToAPIDiffTypeName, protoDiff.GetType(), "DiffTypeUnknown")
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

// ========== Create Rack Request/Response ==========

// APICreateRackRequest is the JSON body for POST /rack (CreateExpectedRack)
type APICreateRackRequest struct {
	SiteID       string                           `json:"siteId"`
	Name         string                           `json:"name"`
	Manufacturer string                           `json:"manufacturer"`
	Model        *string                          `json:"model"`
	SerialNumber string                           `json:"serialNumber"`
	Description  *string                          `json:"description"`
	Location     *APIRackLocation                 `json:"location"`
	Components   []*APICreateRackComponentRequest `json:"components"`
}

// Validate validates the create rack request
func (r *APICreateRackRequest) Validate() error {
	err := validation.ValidateStruct(r,
		validation.Field(&r.SiteID,
			validation.Required.Error(validationErrorValueRequired),
			validationis.UUID.Error(validationErrorInvalidUUID)),
		validation.Field(&r.Name,
			validation.Required.Error(validationErrorValueRequired)),
		validation.Field(&r.Manufacturer,
			validation.Required.Error(validationErrorValueRequired)),
		validation.Field(&r.SerialNumber,
			validation.Required.Error(validationErrorValueRequired)),
	)
	if err != nil {
		return err
	}
	errs := validation.Errors{}
	for i, comp := range r.Components {
		if comp == nil {
			errs[fmt.Sprintf("components[%d]", i)] = fmt.Errorf("must not be null")
			continue
		}
		if compErr := comp.Validate(); compErr != nil {
			errs[fmt.Sprintf("components[%d]", i)] = compErr
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ToProtoRack converts the API request to an RLA Rack proto
func (r *APICreateRackRequest) ToProtoRack() *rlav1.Rack {
	rack := &rlav1.Rack{
		Info: &rlav1.DeviceInfo{
			Name:         r.Name,
			Manufacturer: r.Manufacturer,
			SerialNumber: r.SerialNumber,
		},
	}
	if r.Model != nil {
		rack.Info.Model = r.Model
	}
	if r.Description != nil {
		rack.Info.Description = r.Description
	}
	if r.Location != nil {
		rack.Location = r.Location.ToProto()
	}
	for _, comp := range r.Components {
		rack.Components = append(rack.Components, comp.ToProto())
	}
	return rack
}

// APICreateRackComponentRequest is a component in a create rack request
type APICreateRackComponentRequest struct {
	Type            string                     `json:"type"`
	Name            string                     `json:"name"`
	Manufacturer    string                     `json:"manufacturer"`
	Model           *string                    `json:"model"`
	SerialNumber    string                     `json:"serialNumber"`
	Description     *string                    `json:"description"`
	FirmwareVersion *string                    `json:"firmwareVersion"`
	ComponentID     *string                    `json:"componentId"`
	SlotID          *int32                     `json:"slotId"`
	TrayIdx         *int32                     `json:"trayIdx"`
	HostID          *int32                     `json:"hostId"`
	BMCs            []*APICreateRackBMCRequest `json:"bmcs"`
}

// Validate validates a create rack component
func (c *APICreateRackComponentRequest) Validate() error {
	err := validation.ValidateStruct(c,
		validation.Field(&c.Type,
			validation.Required.Error(validationErrorValueRequired)),
		validation.Field(&c.Name,
			validation.Required.Error(validationErrorValueRequired)),
		validation.Field(&c.SerialNumber,
			validation.Required.Error(validationErrorValueRequired)),
	)
	if err != nil {
		return err
	}
	if _, ok := rlav1.ComponentType_value[c.Type]; !ok {
		return fmt.Errorf("invalid component type: %q", c.Type)
	}
	errs := validation.Errors{}
	for i, bmc := range c.BMCs {
		if bmc == nil {
			errs[fmt.Sprintf("bmcs[%d]", i)] = fmt.Errorf("must not be null")
			continue
		}
		if bmcErr := bmc.Validate(); bmcErr != nil {
			errs[fmt.Sprintf("bmcs[%d]", i)] = bmcErr
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ToProto converts to RLA Component proto
func (c *APICreateRackComponentRequest) ToProto() *rlav1.Component {
	comp := &rlav1.Component{
		Type: componentTypeFromString(c.Type),
		Info: &rlav1.DeviceInfo{
			Name:         c.Name,
			Manufacturer: c.Manufacturer,
			SerialNumber: c.SerialNumber,
		},
	}
	if c.SlotID != nil || c.TrayIdx != nil || c.HostID != nil {
		comp.Position = &rlav1.RackPosition{}
		if c.SlotID != nil {
			comp.Position.SlotId = *c.SlotID
		}
		if c.TrayIdx != nil {
			comp.Position.TrayIdx = *c.TrayIdx
		}
		if c.HostID != nil {
			comp.Position.HostId = *c.HostID
		}
	}
	if c.Model != nil {
		comp.Info.Model = c.Model
	}
	if c.Description != nil {
		comp.Info.Description = c.Description
	}
	if c.FirmwareVersion != nil {
		comp.FirmwareVersion = *c.FirmwareVersion
	}
	if c.ComponentID != nil {
		comp.ComponentId = *c.ComponentID
	}
	for _, bmc := range c.BMCs {
		comp.Bmcs = append(comp.Bmcs, bmc.ToProto())
	}
	return comp
}

// APICreateRackBMCRequest is a BMC entry in a create rack request
type APICreateRackBMCRequest struct {
	Type       string  `json:"type"`
	MacAddress string  `json:"macAddress"`
	IPAddress  *string `json:"ipAddress"`
	User       *string `json:"user"`
	Password   *string `json:"password"`
}

// Validate validates a create rack BMC
func (b *APICreateRackBMCRequest) Validate() error {
	err := validation.ValidateStruct(b,
		validation.Field(&b.Type,
			validation.Required.Error(validationErrorValueRequired)),
		validation.Field(&b.MacAddress,
			validation.Required.Error(validationErrorValueRequired)),
	)
	if err != nil {
		return err
	}
	if _, ok := rlav1.BMCType_value[b.Type]; !ok {
		return fmt.Errorf("invalid bmc type: %q", b.Type)
	}
	return nil
}

// ToProto converts to RLA BMCInfo proto
func (b *APICreateRackBMCRequest) ToProto() *rlav1.BMCInfo {
	bmc := &rlav1.BMCInfo{
		Type:       bmcTypeFromString(b.Type),
		MacAddress: b.MacAddress,
	}
	if b.IPAddress != nil {
		bmc.IpAddress = b.IPAddress
	}
	if b.User != nil {
		bmc.User = b.User
	}
	if b.Password != nil {
		bmc.Password = b.Password
	}
	return bmc
}

// ToProto converts APIRackLocation to RLA Location proto
func (arl *APIRackLocation) ToProto() *rlav1.Location {
	return &rlav1.Location{
		Region:     arl.Region,
		Datacenter: arl.Datacenter,
		Room:       arl.Room,
		Position:   arl.Position,
	}
}

// APICreateRackResponse is the API response for POST /rack
type APICreateRackResponse struct {
	ID string `json:"id"`
}

// NewAPICreateRackResponse creates a response from the RLA CreateExpectedRackResponse
func NewAPICreateRackResponse(resp *rlav1.CreateExpectedRackResponse) *APICreateRackResponse {
	if resp == nil {
		return &APICreateRackResponse{}
	}
	return &APICreateRackResponse{
		ID: resp.GetId().GetId(),
	}
}

// ========== Update Rack Request/Response ==========

// APIRackUpdateRequest is the JSON body for PATCH /rack/:id (PatchRack)
type APIRackUpdateRequest struct {
	SiteID       string                 `json:"siteId"`
	Name         *string                `json:"name"`
	Manufacturer *string                `json:"manufacturer"`
	Model        *string                `json:"model"`
	SerialNumber *string                `json:"serialNumber"`
	Description  *string                `json:"description"`
	Location     *APIRackUpdateLocation `json:"location"`
}

// APIRackUpdateLocation is a patch-specific location type with pointer fields
// to distinguish between "not provided" and "set to empty string".
type APIRackUpdateLocation struct {
	Region     *string `json:"region"`
	Datacenter *string `json:"datacenter"`
	Room       *string `json:"room"`
	Position   *string `json:"position"`
}

func (l *APIRackUpdateLocation) ToProto() *rlav1.Location {
	loc := &rlav1.Location{}
	if l.Region != nil {
		loc.Region = *l.Region
	}
	if l.Datacenter != nil {
		loc.Datacenter = *l.Datacenter
	}
	if l.Room != nil {
		loc.Room = *l.Room
	}
	if l.Position != nil {
		loc.Position = *l.Position
	}
	return loc
}

// Validate validates the update rack request
func (r *APIRackUpdateRequest) Validate() error {
	err := validation.ValidateStruct(r,
		validation.Field(&r.SiteID,
			validation.Required.Error(validationErrorValueRequired),
			validationis.UUID.Error(validationErrorInvalidUUID)),
	)
	if err != nil {
		return err
	}
	if r.Name == nil && r.Manufacturer == nil && r.Model == nil &&
		r.SerialNumber == nil && r.Description == nil && r.Location == nil {
		return fmt.Errorf("at least one field to update must be provided")
	}
	return nil
}

// ToProtoRack converts the API update request to an RLA Rack proto.
// The rack ID is set from the URL path parameter.
func (r *APIRackUpdateRequest) ToProtoRack(rackID string) *rlav1.Rack {
	rack := &rlav1.Rack{
		Info: &rlav1.DeviceInfo{
			Id: &rlav1.UUID{Id: rackID},
		},
	}
	if r.Name != nil {
		rack.Info.Name = *r.Name
	}
	if r.Manufacturer != nil {
		rack.Info.Manufacturer = *r.Manufacturer
	}
	if r.Model != nil {
		rack.Info.Model = r.Model
	}
	if r.SerialNumber != nil {
		rack.Info.SerialNumber = *r.SerialNumber
	}
	if r.Description != nil {
		rack.Info.Description = r.Description
	}
	if r.Location != nil {
		rack.Location = r.Location.ToProto()
	}
	return rack
}

// APIRackUpdateResponse is the API response for PATCH /rack/:id
type APIRackUpdateResponse struct {
	Report string `json:"report"`
}

// NewAPIRackUpdateResponse creates a response from the RLA PatchRackResponse
func NewAPIRackUpdateResponse(resp *rlav1.PatchRackResponse) *APIRackUpdateResponse {
	if resp == nil {
		return &APIRackUpdateResponse{}
	}
	return &APIRackUpdateResponse{
		Report: resp.GetReport(),
	}
}

// ========== Proto Enum Helpers ==========

// componentTypeFromString converts a string to RLA ComponentType enum
func componentTypeFromString(s string) rlav1.ComponentType {
	if v, ok := rlav1.ComponentType_value[s]; ok {
		return rlav1.ComponentType(v)
	}
	return rlav1.ComponentType_COMPONENT_TYPE_UNKNOWN
}

// bmcTypeFromString converts a string to RLA BMCType enum
func bmcTypeFromString(s string) rlav1.BMCType {
	if v, ok := rlav1.BMCType_value[s]; ok {
		return rlav1.BMCType(v)
	}
	return rlav1.BMCType_BMC_TYPE_UNKNOWN
}

// ========== Bring Up Request ==========

// APIBringUpRackRequest is the request body for bring up operations on a single rack
type APIBringUpRackRequest struct {
	SiteID      string `json:"siteId"`
	Description string `json:"description,omitempty"`
}

// Validate validates the bring up request
func (r *APIBringUpRackRequest) Validate() error {
	if r.SiteID == "" {
		return fmt.Errorf("siteId is required")
	}
	return nil
}

// ========== Bring Up Response ==========

// APIBringUpRackResponse is the API response for bring up operations
type APIBringUpRackResponse struct {
	TaskIDs []string `json:"taskIds"`
}

// FromProto converts an RLA SubmitTaskResponse to an APIBringUpRackResponse
func (r *APIBringUpRackResponse) FromProto(resp *rlav1.SubmitTaskResponse) {
	if resp == nil {
		r.TaskIDs = []string{}
		return
	}
	r.TaskIDs = make([]string, 0, len(resp.GetTaskIds()))
	for _, id := range resp.GetTaskIds() {
		r.TaskIDs = append(r.TaskIDs, id.GetId())
	}
}

// NewAPIBringUpRackResponse creates an APIBringUpRackResponse from an RLA SubmitTaskResponse
func NewAPIBringUpRackResponse(resp *rlav1.SubmitTaskResponse) *APIBringUpRackResponse {
	r := &APIBringUpRackResponse{}
	r.FromProto(resp)
	return r
}

// ========== Batch Bring Up Rack Request ==========

// APIBatchBringUpRackRequest is the JSON body for batch rack bring up.
type APIBatchBringUpRackRequest struct {
	SiteID      string      `json:"siteId"`
	Filter      *RackFilter `json:"filter,omitempty"`
	Description string      `json:"description,omitempty"`
}

// Validate checks required fields.
func (r *APIBatchBringUpRackRequest) Validate() error {
	if r.SiteID == "" {
		return fmt.Errorf("siteId is required")
	}
	return nil
}
