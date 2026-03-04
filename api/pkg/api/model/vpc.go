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
	"errors"
	"fmt"
	"math"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	validationis "github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/google/uuid"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model/util"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
)

const (
	// VpcMaxLabelCount is the maximum number of Labels allowed per VPC
	VpcMaxLabelCount = 10
)

// APIVpcCreateRequest captures the request data for creating a new VPC
type APIVpcCreateRequest struct {
	// ID is the user-specified UUID of the VPC.
	ID *uuid.UUID `json:"id"`
	// Name is the name of the VPC
	Name string `json:"name"`
	// Description is the description of the VPC
	Description *string `json:"description"`
	// SiteID is the ID of the Site
	SiteID string `json:"siteId"`
	// NetworkVirtualizationType is a VPC virtualization type
	NetworkVirtualizationType *string `json:"networkVirtualizationType"`
	// Labels is a key value objects
	Labels map[string]string `json:"labels"`
	// NetworkSecurityGroupID is the ID if a desired
	// NSG to attach to the VPC
	NetworkSecurityGroupID *string `json:"networkSecurityGroupId"`
	// NVLinkLogicalPartitionID is the ID of the NVLinkLogicalPartition
	NVLinkLogicalPartitionID *string `json:"nvLinkLogicalPartitionId"`
	// Vni is an optional, explicitly requested VPC VNI.
	// The request will be rejected by the site if the VNI
	// is not within a VNI range allowed for explicit requests.
	Vni *int `json:"vni"`
}

// Validate ensure the values passed in create request are acceptable
func (ascr APIVpcCreateRequest) Validate() error {
	err := validation.ValidateStruct(&ascr,
		validation.Field(&ascr.Name,
			validation.Required.Error(validationErrorStringLength),
			validation.By(util.ValidateNameCharacters),
			validation.Length(2, 256).Error(validationErrorStringLength)),
		validation.Field(&ascr.Description,
			validation.When(ascr.Description != nil,
				validation.Length(0, 1024).Error(validationErrorDescriptionStringLength)),
		),
		validation.Field(&ascr.SiteID,
			validation.Required.Error(validationErrorValueRequired),
			validationis.UUID.Error(validationErrorInvalidUUID)),
		validation.Field(&ascr.ID,
			validation.When(ascr.ID != nil, validationis.UUID.Error(validationErrorInvalidUUID))),
	)

	if err != nil {
		return err
	}

	// NetworkVirtualizationType validation
	if ascr.NetworkVirtualizationType != nil {
		if (*ascr.NetworkVirtualizationType != cdbm.VpcEthernetVirtualizer) && (*ascr.NetworkVirtualizationType != cdbm.VpcFNN) {
			return validation.Errors{
				"networkVirtualizationType": errors.New("either ETHERNET_VIRTUALIZER or FNN are currently supported"),
			}
		}
	}

	if ascr.Vni != nil && (*ascr.Vni < 0 || *ascr.Vni > math.MaxUint16) {
		return validation.Errors{
			"labels": fmt.Errorf("VNI must be an integer between 0 and %d", math.MaxUint16),
		}
	}

	// Labels validation
	if ascr.Labels != nil {
		if len(ascr.Labels) > VpcMaxLabelCount {
			return validation.Errors{
				"labels": fmt.Errorf("up to %v key/value pairs can be specified in labels", VpcMaxLabelCount),
			}
		}

		for key, value := range ascr.Labels {
			if key == "" {
				return validation.Errors{
					"labels": errors.New("one or more labels do not have a key specified"),
				}
			}

			// Key validation
			err = validation.Validate(key,
				validation.Match(util.NotAllWhitespaceRegexp).Error("label key consists only of whitespace"),
				validation.Length(1, 255).Error(validationErrorMapKeyLabelStringLength),
			)

			if err != nil {
				return validation.Errors{
					"labels": errors.New(validationErrorMapKeyLabelStringLength),
				}
			}

			// Value validation
			err = validation.Validate(value,
				validation.When(value != "",
					validation.Length(0, 255).Error(validationErrorMapValueLabelStringLength),
				),
			)

			if err != nil {
				return validation.Errors{
					"labels": errors.New(validationErrorMapValueLabelStringLength),
				}
			}
		}
	}

	return err
}

// APIVpcUpdateRequest captures the request data for updating a new VPC
type APIVpcUpdateRequest struct {
	// Name is the name of the VPC
	Name *string `json:"name"`
	// Description is the description of the VPC
	Description *string `json:"description"`
	// Labels is a key value objects
	Labels map[string]string `json:"labels"`
	// NetworkSecurityGroupID is the ID if a desired
	// NSG to attach to the VPC
	NetworkSecurityGroupID *string `json:"networkSecurityGroupId"`
	// NVLinkLogicalPartitionID is the ID of the NVLinkLogicalPartition
	NVLinkLogicalPartitionID *string `json:"nvLinkLogicalPartitionId"`
}

// Validate ensure the values passed in update request are acceptable
func (asur APIVpcUpdateRequest) Validate() error {
	err := validation.ValidateStruct(&asur,
		validation.Field(&asur.Name,
			validation.When(asur.Name != nil, validation.Required.Error(validationErrorStringLength)),
			validation.When(asur.Name != nil, validation.By(util.ValidateNameCharacters)),
			validation.When(asur.Name != nil, validation.Length(2, 256).Error(validationErrorStringLength))),
		validation.Field(&asur.Description,
			validation.When(asur.Description != nil, validation.Length(0, 1024).Error(validationErrorDescriptionStringLength)),
		),
	)

	if err != nil {
		return err
	}

	// Labels validation
	if asur.Labels != nil {
		if len(asur.Labels) > VpcMaxLabelCount {
			return validation.Errors{
				"labels": fmt.Errorf("up to %v key/value pairs can be specified in labels", VpcMaxLabelCount),
			}
		}

		for key, value := range asur.Labels {
			if key == "" {
				return validation.Errors{
					"labels": errors.New("one or more labels do not have a key specified"),
				}
			}

			// Key validation
			err = validation.Validate(key,
				validation.Match(util.NotAllWhitespaceRegexp).Error("label key consists only of whitespace"),
				validation.Length(1, 255).Error(validationErrorMapKeyLabelStringLength),
			)

			if err != nil {
				return validation.Errors{
					"labels": errors.New(validationErrorMapKeyLabelStringLength),
				}
			}

			// Value validation
			err = validation.Validate(value,
				validation.When(value != "",
					validation.Length(0, 255).Error(validationErrorMapValueLabelStringLength),
				),
			)

			if err != nil {
				return validation.Errors{
					"labels": errors.New(validationErrorMapValueLabelStringLength),
				}
			}
		}
	}

	return err
}

// APIVpcVirtualizationUpdateRequest captures the request data for updating virtualization type for a give VPC
type APIVpcVirtualizationUpdateRequest struct {
	// NetworkVirtualizationType is a VPC virtualization type
	NetworkVirtualizationType string `json:"networkVirtualizationType"`
}

// Validate ensure the values passed in update request are acceptable
func (avvur APIVpcVirtualizationUpdateRequest) Validate(existingVpc *cdbm.Vpc) error {
	err := validation.ValidateStruct(&avvur,
		validation.Field(&avvur.NetworkVirtualizationType,
			validation.Required.Error(validationErrorValueRequired),
		),
	)

	if err != nil {
		return err
	}

	// NetworkVirtualizationType validation
	if avvur.NetworkVirtualizationType != cdbm.VpcFNN {
		return validation.Errors{
			"networkVirtualizationType": errors.New("virtualization type can only be updated to FNN"),
		}
	}

	if existingVpc.NetworkVirtualizationType != nil && *existingVpc.NetworkVirtualizationType == cdbm.VpcFNN {
		return validation.Errors{
			"networkVirtualizationType": errors.New("VPC virtualization type is already set to FNN"),
		}
	}

	return nil
}

// APIVpc is a data structure to capture information about VPC at the API layer
type APIVpc struct {
	// ID is the unique UUID v4 identifier of the VPC in Forge Cloud
	ID string `json:"id"`
	// Name is the name of the VPC
	Name string `json:"name"`
	// Description is the description of the VPC
	Description *string `json:"description"`
	// Org is the NGC organization ID of the infrastructure provider and the org the VPC belongs to
	Org string `json:"org"`
	// InfrastructureProviderID is the ID of the infrastructure provider who owns the site
	InfrastructureProviderID *string `json:"infrastructureProviderId"`
	// InfrastructureProvider is the summary of the InfrastructureProvider
	InfrastructureProvider *APIInfrastructureProviderSummary `json:"infrastructureProvider,omitempty"`
	// TenantID is the ID of the Tenant
	TenantID *string `json:"tenantId"`
	// Tenant is the summary of the tenant
	Tenant *APITenantSummary `json:"tenant,omitempty"`
	// SiteID is the ID of the Site
	SiteID *string `json:"siteId"`
	// Site is the summary of the site
	Site *APISiteSummary `json:"site,omitempty"`
	// NetworkVirtualizationType is a VPC virtualization type
	NetworkVirtualizationType *string `json:"networkVirtualizationType"`
	// ControllerVpcID is the ID of the corresponding VPC in Site Controller
	ControllerVpcID *string `json:"controllerVpcId"`
	// Labels is VPC labels specified by user
	Labels map[string]string `json:"labels"`
	// NVLinkLogicalPartitionID is the ID of the NVLinkLogicalPartition
	NVLinkLogicalPartitionID *string `json:"nvLinkLogicalPartitionId"`
	// NVLinkLogicalPartitionSummary is the summary of the NVLinkLogicalPartition
	NVLinkLogicalPartitionSummary *APINVLinkLogicalPartitionSummary `json:"nvLinkLogicalPartitionSummary,omitempty"`
	// NetworkSecurityGroupID is the ID of attached NSG, if any
	NetworkSecurityGroupID *string `json:"networkSecurityGroupId"`
	// NetworkSecurityGroup holds the summary for attached NSG, if requested via includeRelation
	NetworkSecurityGroup *APINetworkSecurityGroupSummary `json:"networkSecurityGroup,omitempty"`
	// NetworkSecurityGroupPropagationDetails is the propagation details for the attched NSG, if any
	NetworkSecurityGroupPropagationDetails *APINetworkSecurityGroupPropagationDetails `json:"networkSecurityGroupPropagationDetails"`
	// Status is the status of the VPC
	Status string `json:"status"`
	// StatusHistory is the status detail records for the VPC over time
	StatusHistory []APIStatusDetail `json:"statusHistory"`
	// CreatedAt indicates the ISO datetime string for when the entity was created
	Created time.Time `json:"created"`
	// Updated indicates the ISO datetime string for when the VPC was last updated
	Updated time.Time `json:"updated"`
	// RequestedVni is the explicitly requested VPC VNI at creation time _if_ one was requested.
	RequestedVni *int
	// Vni is the active/actual VNI of the VPC, regardless of whether it was
	// explicitly requested or auto-allocated.
	Vni *int
}

// NewAPIVpc creates and returns a new APIVpc object
func NewAPIVpc(dbVpc cdbm.Vpc, dbsds []cdbm.StatusDetail) APIVpc {
	apivpc := APIVpc{
		ID:                                     dbVpc.ID.String(),
		Name:                                   dbVpc.Name,
		Description:                            dbVpc.Description,
		Org:                                    dbVpc.Org,
		InfrastructureProviderID:               util.GetUUIDPtrToStrPtr(&dbVpc.InfrastructureProviderID),
		TenantID:                               util.GetUUIDPtrToStrPtr(&dbVpc.TenantID),
		SiteID:                                 util.GetUUIDPtrToStrPtr(&dbVpc.SiteID),
		Labels:                                 dbVpc.Labels,
		Status:                                 dbVpc.Status,
		NetworkSecurityGroupID:                 dbVpc.NetworkSecurityGroupID,
		NetworkSecurityGroupPropagationDetails: NewAPINetworkSecurityGroupPropagationDetails(dbVpc.NetworkSecurityGroupPropagationDetails),
		Created:                                dbVpc.Created,
		Updated:                                dbVpc.Updated,
		RequestedVni:                           dbVpc.Vni,
		Vni:                                    dbVpc.ActiveVni,
	}

	if dbVpc.NetworkVirtualizationType != nil {
		apivpc.NetworkVirtualizationType = dbVpc.NetworkVirtualizationType
	}

	if dbVpc.ControllerVpcID != nil {
		apivpc.ControllerVpcID = util.GetUUIDPtrToStrPtr(dbVpc.ControllerVpcID)
	}

	if dbVpc.NVLinkLogicalPartitionID != nil {
		apivpc.NVLinkLogicalPartitionID = util.GetUUIDPtrToStrPtr(dbVpc.NVLinkLogicalPartitionID)
	}

	if dbVpc.NVLinkLogicalPartition != nil {
		apivpc.NVLinkLogicalPartitionSummary = NewAPINVLinkLogicalPartitionSummary(dbVpc.NVLinkLogicalPartition)
	}

	apivpc.StatusHistory = []APIStatusDetail{}
	for _, dbsd := range dbsds {
		apivpc.StatusHistory = append(apivpc.StatusHistory, NewAPIStatusDetail(dbsd))
	}

	if dbVpc.Site != nil {
		apivpc.Site = NewAPISiteSummary(dbVpc.Site)
	}

	if dbVpc.Tenant != nil {
		apivpc.Tenant = NewAPITenantSummary(dbVpc.Tenant)
	}

	if dbVpc.InfrastructureProvider != nil {
		apivpc.InfrastructureProvider = NewAPIInfrastructureProviderSummary(dbVpc.InfrastructureProvider)
	}

	if dbVpc.NetworkSecurityGroup != nil {
		apivpc.NetworkSecurityGroup = NewAPINetworkSecurityGroupSummary(dbVpc.NetworkSecurityGroup)
	}

	return apivpc
}

// APIVpcStats is a data structure to capture information about VPC stats at the API layer
type APIVpcStats struct {
	// Total is the total number of the VPC object in Forge Cloud
	Total int `json:"total"`
	// Pending is the total number of pending VPC object in Forge Cloud
	Pending int `json:"pending"`
	// Provisioning is the total number of provisioning VPC object in Forge Cloud
	Provisioning int `json:"provisioning"`
	// Ready is the total number of ready VPC object in Forge Cloud
	Ready int `json:"ready"`
	// Deleting is the total number of deleting VPC object in Forge Cloud
	Deleting int `json:"deleting"`
	// Error is the total number of error VPC object in Forge Cloud
	Error int `json:"error"`
}

// APIVpcSummary is the data structure to capture API representation of a Vpc Summary
type APIVpcSummary struct {
	// ID is the unique UUID v4 identifier of the VPC in Forge Cloud
	ID string `json:"id"`
	// Name of the Vpc, only lowercase characters, digits, hyphens and cannot begin/end with hyphen
	Name string `json:"name"`
	// ControllerVpcID is the ID of the corresponding VPC in Site Controller
	ControllerVpcID *string `json:"controllerVpcId"`
	// Network virtualization type is a VPC virtualization type
	NetworkVirtualizationType *string `json:"networkVirtualizationType"`
	// Status is the status of the VPC
	Status string `json:"status"`
}

// NewAPIVpcSummary accepts a DB layer APIVpcSummary object returns an API layer object
func NewAPIVpcSummary(dbVpc *cdbm.Vpc) *APIVpcSummary {
	apiVpcSummary := APIVpcSummary{
		ID:                        dbVpc.ID.String(),
		Name:                      dbVpc.Name,
		NetworkVirtualizationType: dbVpc.NetworkVirtualizationType,
		Status:                    dbVpc.Status,
	}

	if dbVpc.ControllerVpcID != nil {
		apiVpcSummary.ControllerVpcID = util.GetUUIDPtrToStrPtr(dbVpc.ControllerVpcID)
	}

	return &apiVpcSummary
}
