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
	"time"

	"github.com/google/uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	validationis "github.com/go-ozzo/ozzo-validation/v4/is"

	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model/util"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
)

// APIExpectedSwitchCreateRequest is the data structure to capture request to create a new ExpectedSwitch
type APIExpectedSwitchCreateRequest struct {
	// SiteID is the ID of the Site
	SiteID string `json:"siteId"`
	// BmcMacAddress is the MAC address of the expected switch's BMC
	BmcMacAddress string `json:"bmcMacAddress"`
	// DefaultBmcUsername is the username of the expected switch's BMC
	DefaultBmcUsername *string `json:"defaultBmcUsername"`
	// DefaultBmcPassword is the password of the expected switch's BMC
	DefaultBmcPassword *string `json:"defaultBmcPassword"`
	// SwitchSerialNumber is the serial number of the expected switch
	SwitchSerialNumber string `json:"switchSerialNumber"`
	// NvOsUsername is the NVOS username of the expected switch
	NvOsUsername *string `json:"nvOsUsername"`
	// NvOsPassword is the NVOS password of the expected switch
	NvOsPassword *string `json:"nvOsPassword"`
	// Labels is the labels of the expected switch
	Labels map[string]string `json:"labels"`
}

// Validate ensure the values passed in request are acceptable
func (escr *APIExpectedSwitchCreateRequest) Validate() error {
	err := validation.ValidateStruct(escr,
		validation.Field(&escr.SiteID,
			validation.Required.Error(validationErrorValueRequired),
			validationis.UUID.Error(validationErrorInvalidUUID)),
		validation.Field(&escr.BmcMacAddress,
			validation.Required.Error(validationErrorValueRequired),
			validationis.MAC),
		validation.Field(&escr.DefaultBmcUsername,
			validation.Length(0, 16).Error("BMC username must be 16 characters or less")),
		validation.Field(&escr.DefaultBmcPassword,
			validation.Length(0, 20).Error("BMC password must be 20 characters or less")),
		validation.Field(&escr.SwitchSerialNumber,
			validation.Required.Error(validationErrorValueRequired),
			validation.Match(util.NotAllWhitespaceRegexp).Error("Switch serial number consists only of whitespace"),
			validation.Length(1, 32).Error("Switch serial number must be 32 characters or less")),
	)

	if err != nil {
		return err
	}

	// Labels validation
	if escr.Labels != nil {
		if len(escr.Labels) > util.LabelCountMax {
			return validation.Errors{
				"labels": util.ErrValidationLabelCount,
			}
		}

		for key, value := range escr.Labels {
			if key == "" {
				return validation.Errors{
					"labels": util.ErrValidationLabelKeyEmpty,
				}
			}

			// Key validation
			if len(key) > util.LabelKeyMaxLength {
				return validation.Errors{
					"labels": util.ErrValidationLabelKeyLength,
				}
			}

			// Value validation
			if len(value) > util.LabelValueMaxLength {
				return validation.Errors{
					"labels": util.ErrValidationLabelValueLength,
				}
			}
		}
	}

	return nil
}

// APIExpectedSwitchUpdateRequest is the data structure to capture user request to update an ExpectedSwitch
type APIExpectedSwitchUpdateRequest struct {
	// ID is required for batch updates (must be empty or match path value for single update)
	ID *string `json:"id"`
	// BmcMacAddress is the MAC address of the expected switch's BMC
	BmcMacAddress *string `json:"bmcMacAddress"`
	// DefaultBmcUsername is the username of the expected switch's BMC
	DefaultBmcUsername *string `json:"defaultBmcUsername"`
	// DefaultBmcPassword is the password of the expected switch's BMC
	DefaultBmcPassword *string `json:"defaultBmcPassword"`
	// SwitchSerialNumber is the serial number of the expected switch
	SwitchSerialNumber *string `json:"switchSerialNumber"`
	// NvOsUsername is the NVOS username of the expected switch
	NvOsUsername *string `json:"nvOsUsername"`
	// NvOsPassword is the NVOS password of the expected switch
	NvOsPassword *string `json:"nvOsPassword"`
	// Labels is the labels of the expected switch
	Labels map[string]string `json:"labels"`
}

// Validate ensure the values passed in request are acceptable
func (esur *APIExpectedSwitchUpdateRequest) Validate() error {
	if esur.ID != nil {
		if *esur.ID == "" {
			return validation.Errors{
				"id": errors.New("ID cannot be empty"),
			}
		}
		if _, err := uuid.Parse(*esur.ID); err != nil {
			return validation.Errors{
				"id": errors.New("ID must be a valid UUID"),
			}
		}
	}

	err := validation.ValidateStruct(esur,
		validation.Field(&esur.DefaultBmcUsername,
			validation.NilOrNotEmpty.Error("BMC Username cannot be empty"),
			validation.When(esur.DefaultBmcUsername != nil && *esur.DefaultBmcUsername != "",
				validation.Match(util.NotAllWhitespaceRegexp).Error("BMC Username consists only of whitespace")),
			validation.Length(1, 16).Error("BMC Username must be 1-16 characters")),
		validation.Field(&esur.DefaultBmcPassword,
			validation.NilOrNotEmpty.Error("BMC Password cannot be empty"),
			validation.When(esur.DefaultBmcPassword != nil && *esur.DefaultBmcPassword != "",
				validation.Match(util.NotAllWhitespaceRegexp).Error("BMC Password consists only of whitespace")),
			validation.Length(1, 20).Error("BMC Password must be 1-20 characters")),
		validation.Field(&esur.SwitchSerialNumber,
			validation.NilOrNotEmpty.Error("Switch Serial Number cannot be empty"),
			validation.When(esur.SwitchSerialNumber != nil && *esur.SwitchSerialNumber != "",
				validation.Match(util.NotAllWhitespaceRegexp).Error("Switch Serial Number consists only of whitespace")),
			validation.Length(1, 32).Error("Switch Serial Number must be 1-32 characters")),
	)

	if err != nil {
		return err
	}

	// Labels validation
	if esur.Labels != nil {
		if len(esur.Labels) > util.LabelCountMax {
			return validation.Errors{
				"labels": util.ErrValidationLabelCount,
			}
		}

		for key, value := range esur.Labels {
			if key == "" {
				return validation.Errors{
					"labels": util.ErrValidationLabelKeyEmpty,
				}
			}

			// Key validation
			if len(key) > util.LabelKeyMaxLength {
				return validation.Errors{
					"labels": util.ErrValidationLabelKeyLength,
				}
			}

			// Value validation
			if len(value) > util.LabelValueMaxLength {
				return validation.Errors{
					"labels": util.ErrValidationLabelValueLength,
				}
			}
		}
	}

	return nil
}

// APIExpectedSwitch is the data structure to capture API representation of an ExpectedSwitch
type APIExpectedSwitch struct {
	// ID is the ID of this Expected Switch
	ID uuid.UUID `json:"id"`
	// BmcMacAddress is the MAC address of the expected switch's BMC
	BmcMacAddress string `json:"bmcMacAddress"`
	// SiteID is the ID of the site this switch belongs to
	SiteID uuid.UUID `json:"siteId"`
	// Site is the site information
	Site *APISite `json:"site,omitempty"`
	// SwitchSerialNumber is the serial number of the expected switch
	SwitchSerialNumber string `json:"switchSerialNumber"`
	// Labels is the labels of the expected switch
	Labels map[string]string `json:"labels"`
	// Created indicates the ISO datetime string for when the ExpectedSwitch was created
	Created time.Time `json:"created"`
	// Updated indicates the ISO datetime string for when the ExpectedSwitch was last updated
	Updated time.Time `json:"updated"`
}

// NewAPIExpectedSwitch accepts a DB layer ExpectedSwitch object and returns an API object
func NewAPIExpectedSwitch(dbModel *cdbm.ExpectedSwitch) *APIExpectedSwitch {
	apies := &APIExpectedSwitch{
		ID:                 dbModel.ID,
		BmcMacAddress:      dbModel.BmcMacAddress,
		SiteID:             dbModel.SiteID,
		SwitchSerialNumber: dbModel.SwitchSerialNumber,
		Labels:             dbModel.Labels,
		Created:            dbModel.Created,
		Updated:            dbModel.Updated,
	}

	// Expand Site details if available
	if dbModel.Site != nil {
		site := NewAPISite(*dbModel.Site, []cdbm.StatusDetail{}, nil)
		apies.Site = &site
	}

	return apies
}
