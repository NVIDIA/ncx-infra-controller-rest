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
	"github.com/NVIDIA/ncx-infra-controller-rest/api/pkg/api/model/util"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// APICarbideObjectMetadata mirrors the carbide-api gRPC `Metadata` message.
type APICarbideObjectMetadata struct {
	// Name is the user-assigned name of the object.
	Name *string `json:"name"`
	// Description is the user-supplied description of the object.
	Description *string `json:"description"`
	// Labels are key/value pairs attached to the object.
	Labels map[string]string `json:"labels"`
}

// ValidateOnCreate runs the validation rules that apply when an object is
// being created. Name is required; Description and Labels are optional.
func (m APICarbideObjectMetadata) ValidateOnCreate() error {
	if err := validation.ValidateStruct(&m,
		validation.Field(&m.Name,
			validation.Required.Error(validationErrorStringLength),
			validation.By(util.ValidateNameCharacters),
			validation.When(m.Name != nil,
				validation.Length(2, 256).Error(validationErrorStringLength)),
		),
		validation.Field(&m.Description,
			validation.When(m.Description != nil,
				validation.Length(0, 1024).Error(validationErrorDescriptionStringLength)),
		),
	); err != nil {
		return err
	}
	return util.ValidateLabels(m.Labels)
}

// ValidateOnUpdate runs the validation rules that apply when an object is
// being updated. All fields are optional; what's present must still validate.
func (m APICarbideObjectMetadata) ValidateOnUpdate() error {
	if err := validation.ValidateStruct(&m,
		validation.Field(&m.Name,
			validation.When(m.Name != nil,
				validation.By(util.ValidateNameCharacters),
				validation.Length(2, 256).Error(validationErrorStringLength)),
		),
		validation.Field(&m.Description,
			validation.When(m.Description != nil,
				validation.Length(0, 1024).Error(validationErrorDescriptionStringLength)),
		),
	); err != nil {
		return err
	}
	return util.ValidateLabels(m.Labels)
}

// NewAPICarbideObjectMetadataFromVpc builds metadata from a DB Vpc row.
// Each adopting type gets its own constructor that pulls from its DB shape.
func NewAPICarbideObjectMetadataFromVpc(v cdbm.Vpc) APICarbideObjectMetadata {
	return APICarbideObjectMetadata{
		Name:        &v.Name,
		Description: v.Description,
		Labels:      v.Labels,
	}
}
