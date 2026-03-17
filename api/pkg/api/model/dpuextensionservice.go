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
	"time"

	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/model/util"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	validationis "github.com/go-ozzo/ozzo-validation/v4/is"
)

const (
	// DpuExtensionServiceTypeKubernetesPod is the service type for Kubernetes Pod
	DpuExtensionServiceTypeKubernetesPod = "KubernetesPod"
	// DpuExtensionServiceTimeFormat is the time format used on Site for version info creation time
	DpuExtensionServiceTimeFormat = "2006-01-02 15:04:05.000000 UTC"
)

// APIDpuExtensionServiceCreateRequest is the data structure to capture user request to create a new DpuExtensionService
type APIDpuExtensionServiceCreateRequest struct {
	// Name is the name of the DpuExtensionService
	Name string `json:"name"`
	// Description is the description of the DpuExtensionService
	Description *string `json:"description"`
	// ServiceType is the type of service
	ServiceType string `json:"serviceType"`
	// SiteID is the ID of the Site
	SiteID string `json:"siteId"`
	// Data is the deployment spec for the DPU Extension Service
	Data string `json:"data"`
	// Credentials are the credentials to download resources
	Credentials *APIDpuExtensionServiceCredentials `json:"credentials"`
}

// Validate ensures that the values passed in request are acceptable
func (descr APIDpuExtensionServiceCreateRequest) Validate() error {
	err := validation.ValidateStruct(&descr,
		validation.Field(&descr.Name,
			validation.Required.Error(validationErrorStringLength),
			validation.By(util.ValidateNameCharacters),
			validation.Length(2, 256).Error(validationErrorStringLength)),
		validation.Field(&descr.ServiceType,
			validation.Required.Error(validationErrorValueRequired),
			validation.In(DpuExtensionServiceTypeKubernetesPod).Error("must be 'KubernetesPod'")),
		validation.Field(&descr.SiteID,
			validation.Required.Error(validationErrorValueRequired),
			validationis.UUID.Error(validationErrorInvalidUUID)),
		validation.Field(&descr.Data,
			validation.Required.Error(validationErrorValueRequired)),
	)
	if err != nil {
		return err
	}

	// Validate credentials if provided
	if descr.Credentials != nil {
		err = descr.Credentials.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// APIDpuExtensionServiceUpdateRequest is the data structure to capture user request to update a DpuExtensionService
type APIDpuExtensionServiceUpdateRequest struct {
	// Name is the name of the DpuExtensionService
	Name *string `json:"name"`
	// Description is the description of the DpuExtensionService
	Description *string `json:"description"`
	// Data is the deployment spec for the DPU Extension Service
	Data *string `json:"data"`
	// Credentials are the credentials to download resources
	Credentials *APIDpuExtensionServiceCredentials `json:"credentials"`
}

// Validate ensures that the values passed in request are acceptable
func (desur APIDpuExtensionServiceUpdateRequest) Validate() error {
	err := validation.ValidateStruct(&desur,
		validation.Field(&desur.Name,
			validation.When(desur.Name != nil, validation.Required.Error(validationErrorStringLength)),
			validation.When(desur.Name != nil, validation.By(util.ValidateNameCharacters)),
			validation.When(desur.Name != nil, validation.Length(2, 256).Error(validationErrorStringLength))),
	)
	if err != nil {
		return err
	}

	// Validate credentials if provided
	if desur.Credentials != nil {
		err = desur.Credentials.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// APIDpuExtensionServiceCredentials is the data structure for registry credentials
type APIDpuExtensionServiceCredentials struct {
	// RegistryURL is the URL for the registry
	RegistryURL string `json:"registryUrl"`
	// Username for the registry
	Username *string `json:"username"`
	// Password for the registry
	Password *string `json:"password"`
}

// Validate ensures that the credentials are valid
func (desc APIDpuExtensionServiceCredentials) Validate() error {
	return validation.ValidateStruct(&desc,
		validation.Field(&desc.RegistryURL,
			validation.Required.Error(validationErrorValueRequired),
			validationis.URL.Error("must be a valid URL")),
		validation.Field(&desc.Username,
			validation.When(desc.RegistryURL != "", validation.Required.Error("`username` must be specified if `registryUrl` is specified"))),
		validation.Field(&desc.Password,
			validation.When(desc.RegistryURL != "", validation.Required.Error("`password` must be specified if `registryUrl` is specified"))),
	)
}

// APIDpuExtensionService is the data structure to capture API representation of a DpuExtensionService
type APIDpuExtensionService struct {
	// ID is the unique UUID v4 identifier for the DpuExtensionService
	ID string `json:"id"`
	// Name is the name of the DpuExtensionService
	Name string `json:"name"`
	// Description is the description of the DpuExtensionService
	Description *string `json:"description"`
	// ServiceType is the type of service
	ServiceType string `json:"serviceType"`
	// SiteID is the ID of the Site
	SiteID string `json:"siteId"`
	// Site is the summary of the site
	Site *APISiteSummary `json:"site,omitempty"`
	// TenantID is the ID of the Tenant
	TenantID string `json:"tenantId"`
	// Tenant is the summary of the tenant
	Tenant *APITenantSummary `json:"tenant,omitempty"`
	// Version is the latest version of the DPU Extension Service
	Version *string `json:"version"`
	// VersionInfo holds the details for the latest version
	VersionInfo *APIDpuExtensionServiceVersionInfo `json:"versionInfo"`
	// ActiveVersions is a list of active versions available for deployment
	ActiveVersions []string `json:"activeVersions"`
	// Status is the status of the DpuExtensionService
	Status string `json:"status"`
	// StatusHistory is the status detail records for the DpuExtensionService over time
	StatusHistory []APIStatusDetail `json:"statusHistory"`
	// Created indicates the ISO datetime string for when the DpuExtensionService was created
	Created time.Time `json:"created"`
	// Updated indicates the ISO datetime string for when the DpuExtensionService was last updated
	Updated time.Time `json:"updated"`
}

// NewAPIDpuExtensionService creates and returns a new APIDpuExtensionService object
func NewAPIDpuExtensionService(dbdes *cdbm.DpuExtensionService, dbdesds []cdbm.StatusDetail) *APIDpuExtensionService {
	apiDpuExtensionService := &APIDpuExtensionService{
		ID:             dbdes.ID.String(),
		Name:           dbdes.Name,
		Description:    dbdes.Description,
		ServiceType:    dbdes.ServiceType,
		SiteID:         dbdes.SiteID.String(),
		TenantID:       dbdes.TenantID.String(),
		Version:        dbdes.Version,
		ActiveVersions: dbdes.ActiveVersions,
		Status:         dbdes.Status,
		StatusHistory:  []APIStatusDetail{},
		Created:        dbdes.Created,
		Updated:        dbdes.Updated,
	}

	if dbdes.VersionInfo != nil {
		apiDpuExtensionService.VersionInfo = NewAPIDpuExtensionServiceVersionInfo(dbdes.VersionInfo)
	}

	if dbdes.Site != nil {
		apiDpuExtensionService.Site = NewAPISiteSummary(dbdes.Site)
	}

	if dbdes.Tenant != nil {
		apiDpuExtensionService.Tenant = NewAPITenantSummary(dbdes.Tenant)
	}

	apiDpuExtensionService.StatusHistory = []APIStatusDetail{}
	for _, dbsd := range dbdesds {
		apiDpuExtensionService.StatusHistory = append(apiDpuExtensionService.StatusHistory, NewAPIStatusDetail(dbsd))
	}

	return apiDpuExtensionService
}

// APIDpuExtensionServiceSummary is the data structure to capture API summary of a DpuExtensionService
type APIDpuExtensionServiceSummary struct {
	// ID is the unique UUID v4 identifier for the DpuExtensionService
	ID string `json:"id"`
	// Name is the name of the DpuExtensionService
	Name string `json:"name"`
	// ServiceType is the type of service
	ServiceType string `json:"serviceType"`
	// LatestVersion is the latest version of the DPU Extension Service
	LatestVersion *string `json:"latestVersion"`
	// Status is the status of the DpuExtensionService
	Status string `json:"status"`
}

// NewAPIDpuExtensionServiceSummary creates and returns a new APIDpuExtensionServiceSummary object
func NewAPIDpuExtensionServiceSummary(dbdes *cdbm.DpuExtensionService) *APIDpuExtensionServiceSummary {
	return &APIDpuExtensionServiceSummary{
		ID:            dbdes.ID.String(),
		Name:          dbdes.Name,
		ServiceType:   dbdes.ServiceType,
		LatestVersion: dbdes.Version,
		Status:        dbdes.Status,
	}
}

// APIDpuExtensionServiceVersionInfo is the data structure for version information
type APIDpuExtensionServiceVersionInfo struct {
	// Version is the version identifier
	Version string `json:"version"`
	// Data is the deployment spec
	Data string `json:"data"`
	// HasCredentials indicates if this version has credentials
	HasCredentials bool `json:"hasCredentials"`
	// Created indicates when this version was created
	Created time.Time `json:"created"`
}

// NewAPIDpuExtensionServiceVersionInfo creates and returns a new APIDpuExtensionServiceVersionInfo object
func NewAPIDpuExtensionServiceVersionInfo(dbv *cdbm.DpuExtensionServiceVersionInfo) *APIDpuExtensionServiceVersionInfo {
	return &APIDpuExtensionServiceVersionInfo{
		Version:        dbv.Version,
		Data:           dbv.Data,
		HasCredentials: dbv.HasCredentials,
		Created:        dbv.Created,
	}
}
