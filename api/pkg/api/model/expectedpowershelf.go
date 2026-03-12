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

// APIExpectedPowerShelfCreateRequest is the data structure to capture request to create a new ExpectedPowerShelf
type APIExpectedPowerShelfCreateRequest struct {
	// SiteID is the ID of the Site
	SiteID string `json:"siteId"`
	// BmcMacAddress is the MAC address of the expected power shelf's BMC
	BmcMacAddress string `json:"bmcMacAddress"`
	// DefaultBmcUsername is the username of the expected power shelf's BMC
	DefaultBmcUsername *string `json:"defaultBmcUsername"`
	// DefaultBmcPassword is the password of the expected power shelf's BMC
	DefaultBmcPassword *string `json:"defaultBmcPassword"`
	// ShelfSerialNumber is the serial number of the expected power shelf
	ShelfSerialNumber string `json:"shelfSerialNumber"`
	// IpAddress is the IP address of the expected power shelf
	IpAddress *string `json:"ipAddress"`
	// Labels is the labels of the expected power shelf
	Labels map[string]string `json:"labels"`
}

// Validate ensure the values passed in request are acceptable
func (epcr *APIExpectedPowerShelfCreateRequest) Validate() error {
	err := validation.ValidateStruct(epcr,
		validation.Field(&epcr.SiteID,
			validation.Required.Error(validationErrorValueRequired),
			validationis.UUID.Error(validationErrorInvalidUUID)),
		validation.Field(&epcr.BmcMacAddress,
			validation.Required.Error(validationErrorValueRequired),
			validationis.MAC),
		validation.Field(&epcr.DefaultBmcUsername,
			validation.Length(0, 16).Error("BMC username must be 16 characters or less")),
		validation.Field(&epcr.DefaultBmcPassword,
			validation.Length(0, 20).Error("BMC password must be 20 characters or less")),
		validation.Field(&epcr.ShelfSerialNumber,
			validation.Required.Error(validationErrorValueRequired),
			validation.Match(util.NotAllWhitespaceRegexp).Error("Shelf serial number consists only of whitespace"),
			validation.Length(1, 32).Error("Shelf serial number must be 32 characters or less")),
	)

	if err != nil {
		return err
	}

	// Labels validation
	if epcr.Labels != nil {
		if len(epcr.Labels) > util.LabelCountMax {
			return validation.Errors{
				"labels": util.ErrValidationLabelCount,
			}
		}

		for key, value := range epcr.Labels {
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

// APIExpectedPowerShelfUpdateRequest is the data structure to capture user request to update an ExpectedPowerShelf
type APIExpectedPowerShelfUpdateRequest struct {
	// ID is required for batch updates (must be empty or match path value for single update)
	ID *string `json:"id"`
	// BmcMacAddress is the MAC address of the expected power shelf's BMC
	BmcMacAddress *string `json:"bmcMacAddress"`
	// DefaultBmcUsername is the username of the expected power shelf's BMC
	DefaultBmcUsername *string `json:"defaultBmcUsername"`
	// DefaultBmcPassword is the password of the expected power shelf's BMC
	DefaultBmcPassword *string `json:"defaultBmcPassword"`
	// ShelfSerialNumber is the serial number of the expected power shelf
	ShelfSerialNumber *string `json:"shelfSerialNumber"`
	// IpAddress is the IP address of the expected power shelf
	IpAddress *string `json:"ipAddress"`
	// Labels is the labels of the expected power shelf
	Labels map[string]string `json:"labels"`
}

// Validate ensure the values passed in request are acceptable
func (epur *APIExpectedPowerShelfUpdateRequest) Validate() error {
	if epur.ID != nil {
		if *epur.ID == "" {
			return validation.Errors{
				"id": errors.New("ID cannot be empty"),
			}
		}
		if _, err := uuid.Parse(*epur.ID); err != nil {
			return validation.Errors{
				"id": errors.New("ID must be a valid UUID"),
			}
		}
	}

	err := validation.ValidateStruct(epur,
		validation.Field(&epur.DefaultBmcUsername,
			validation.NilOrNotEmpty.Error("BMC Username cannot be empty"),
			validation.When(epur.DefaultBmcUsername != nil && *epur.DefaultBmcUsername != "",
				validation.Match(util.NotAllWhitespaceRegexp).Error("BMC Username consists only of whitespace")),
			validation.Length(1, 16).Error("BMC Username must be 1-16 characters")),
		validation.Field(&epur.DefaultBmcPassword,
			validation.NilOrNotEmpty.Error("BMC Password cannot be empty"),
			validation.When(epur.DefaultBmcPassword != nil && *epur.DefaultBmcPassword != "",
				validation.Match(util.NotAllWhitespaceRegexp).Error("BMC Password consists only of whitespace")),
			validation.Length(1, 20).Error("BMC Password must be 1-20 characters")),
		validation.Field(&epur.ShelfSerialNumber,
			validation.NilOrNotEmpty.Error("Shelf Serial Number cannot be empty"),
			validation.When(epur.ShelfSerialNumber != nil && *epur.ShelfSerialNumber != "",
				validation.Match(util.NotAllWhitespaceRegexp).Error("Shelf Serial Number consists only of whitespace")),
			validation.Length(1, 32).Error("Shelf Serial Number must be 1-32 characters")),
	)

	if err != nil {
		return err
	}

	// Labels validation
	if epur.Labels != nil {
		if len(epur.Labels) > util.LabelCountMax {
			return validation.Errors{
				"labels": util.ErrValidationLabelCount,
			}
		}

		for key, value := range epur.Labels {
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

// APIExpectedPowerShelf is the data structure to capture API representation of an ExpectedPowerShelf
type APIExpectedPowerShelf struct {
	// ID is the ID of this Expected Power Shelf
	ID uuid.UUID `json:"id"`
	// BmcMacAddress is the MAC address of the expected power shelf's BMC
	BmcMacAddress string `json:"bmcMacAddress"`
	// SiteID is the ID of the site this power shelf belongs to
	SiteID uuid.UUID `json:"siteId"`
	// Site is the site information
	Site *APISite `json:"site,omitempty"`
	// ShelfSerialNumber is the serial number of the expected power shelf
	ShelfSerialNumber string `json:"shelfSerialNumber"`
	// IpAddress is the IP address of the expected power shelf
	IpAddress *string `json:"ipAddress"`
	// Labels is the labels of the expected power shelf
	Labels map[string]string `json:"labels"`
	// Created indicates the ISO datetime string for when the ExpectedPowerShelf was created
	Created time.Time `json:"created"`
	// Updated indicates the ISO datetime string for when the ExpectedPowerShelf was last updated
	Updated time.Time `json:"updated"`
}

// NewAPIExpectedPowerShelf accepts a DB layer ExpectedPowerShelf object and returns an API object
func NewAPIExpectedPowerShelf(dbModel *cdbm.ExpectedPowerShelf) *APIExpectedPowerShelf {
	apieps := &APIExpectedPowerShelf{
		ID:                dbModel.ID,
		BmcMacAddress:     dbModel.BmcMacAddress,
		SiteID:            dbModel.SiteID,
		ShelfSerialNumber: dbModel.ShelfSerialNumber,
		IpAddress:         dbModel.IpAddress,
		Labels:            dbModel.Labels,
		Created:           dbModel.Created,
		Updated:           dbModel.Updated,
	}

	// Expand Site details if available
	if dbModel.Site != nil {
		site := NewAPISite(*dbModel.Site, []cdbm.StatusDetail{}, nil)
		apieps.Site = &site
	}

	return apieps
}
