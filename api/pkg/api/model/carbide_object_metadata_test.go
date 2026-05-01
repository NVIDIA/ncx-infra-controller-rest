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
	"strings"
	"testing"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAPICarbideObjectMetadata_ValidateOnCreate(t *testing.T) {
	tests := []struct {
		name     string
		metadata APICarbideObjectMetadata
		wantErr  bool
	}{
		{
			name: "minimal valid metadata",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr("test-name"),
			},
			wantErr: false,
		},
		{
			name: "full valid metadata",
			metadata: APICarbideObjectMetadata{
				Name:        cdb.GetStrPtr("test-name"),
				Description: cdb.GetStrPtr("Test description"),
				Labels:      map[string]string{"env": "prod"},
			},
			wantErr: false,
		},
		{
			name: "missing name is rejected",
			metadata: APICarbideObjectMetadata{
				Name: nil,
			},
			wantErr: true,
		},
		{
			name: "name too short (1 char)",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr("a"),
			},
			wantErr: true,
		},
		{
			name: "name too long (257 chars)",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr(strings.Repeat("a", 257)),
			},
			wantErr: true,
		},
		{
			name: "name with leading whitespace is rejected",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr(" bad-name"),
			},
			wantErr: true,
		},
		{
			name: "name with trailing whitespace is rejected",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr("bad-name "),
			},
			wantErr: true,
		},
		{
			name: "description over the 1024 char limit",
			metadata: APICarbideObjectMetadata{
				Name:        cdb.GetStrPtr("test-name"),
				Description: cdb.GetStrPtr(strings.Repeat("a", 1025)),
			},
			wantErr: true,
		},
		{
			name: "description at the 1024 char limit",
			metadata: APICarbideObjectMetadata{
				Name:        cdb.GetStrPtr("test-name"),
				Description: cdb.GetStrPtr(strings.Repeat("a", 1024)),
			},
			wantErr: false,
		},
		{
			name: "empty-string description is allowed",
			metadata: APICarbideObjectMetadata{
				Name:        cdb.GetStrPtr("test-name"),
				Description: cdb.GetStrPtr(""),
			},
			wantErr: false,
		},
		{
			name: "nil description is allowed",
			metadata: APICarbideObjectMetadata{
				Name:        cdb.GetStrPtr("test-name"),
				Description: nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.metadata.ValidateOnCreate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPICarbideObjectMetadata_ValidateOnUpdate(t *testing.T) {
	tests := []struct {
		name     string
		metadata APICarbideObjectMetadata
		wantErr  bool
	}{
		{
			name:     "empty metadata is allowed (no fields being updated)",
			metadata: APICarbideObjectMetadata{},
			wantErr:  false,
		},
		{
			name: "valid name",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr("new-name"),
			},
			wantErr: false,
		},
		{
			name: "name too short (1 char)",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr("a"),
			},
			wantErr: true,
		},
		{
			name: "name too long (257 chars)",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr(strings.Repeat("a", 257)),
			},
			wantErr: true,
		},
		{
			name: "name with leading whitespace is rejected",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr(" bad-name"),
			},
			wantErr: true,
		},
		{
			name: "name with trailing whitespace is rejected",
			metadata: APICarbideObjectMetadata{
				Name: cdb.GetStrPtr("bad-name "),
			},
			wantErr: true,
		},
		{
			name: "description over the 1024 char limit",
			metadata: APICarbideObjectMetadata{
				Description: cdb.GetStrPtr(strings.Repeat("a", 1025)),
			},
			wantErr: true,
		},
		{
			name: "labels-only update",
			metadata: APICarbideObjectMetadata{
				Labels: map[string]string{"env": "prod"},
			},
			wantErr: false,
		},
		{
			name: "all fields together",
			metadata: APICarbideObjectMetadata{
				Name:        cdb.GetStrPtr("new-name"),
				Description: cdb.GetStrPtr("new description"),
				Labels:      map[string]string{"env": "prod"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.metadata.ValidateOnUpdate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewAPICarbideObjectMetadataFromVpc(t *testing.T) {
	desc := "a description"
	labels := map[string]string{"team": "platform", "env": "prod"}

	t.Run("copies all fields", func(t *testing.T) {
		dbVpc := cdbm.Vpc{
			ID:          uuid.New(),
			Name:        "vpc-east",
			Description: &desc,
			Labels:      labels,
		}
		m := NewAPICarbideObjectMetadataFromVpc(dbVpc)
		assert.Equal(t, "vpc-east", *m.Name)
		assert.Equal(t, &desc, m.Description)
		assert.Equal(t, labels, m.Labels)
	})

	t.Run("preserves nil description", func(t *testing.T) {
		dbVpc := cdbm.Vpc{
			ID:   uuid.New(),
			Name: "vpc-east",
		}
		m := NewAPICarbideObjectMetadataFromVpc(dbVpc)
		assert.Equal(t, "vpc-east", *m.Name)
		assert.Nil(t, m.Description)
	})

	t.Run("preserves nil labels", func(t *testing.T) {
		dbVpc := cdbm.Vpc{
			ID:   uuid.New(),
			Name: "vpc-east",
		}
		m := NewAPICarbideObjectMetadataFromVpc(dbVpc)
		assert.Nil(t, m.Labels)
	})
}
