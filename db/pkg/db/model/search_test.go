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
	"testing"

	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeSearchQuery(t *testing.T) {
	tests := []struct {
		name        string
		input       *string
		wantQuery   string
		wantTsQuery *string
		wantOK      bool
	}{
		{
			name:   "nil",
			input:  nil,
			wantOK: false,
		},
		{
			name:   "blank",
			input:  db.GetStrPtr("   "),
			wantOK: false,
		},
		{
			name:        "valid multi word",
			input:       db.GetStrPtr(" foo bar "),
			wantQuery:   "foo bar",
			wantTsQuery: db.GetStrPtr("foo | bar"),
			wantOK:      true,
		},
		{
			name:        "valid explicit OR operator",
			input:       db.GetStrPtr("foo | bar"),
			wantQuery:   "foo | bar",
			wantTsQuery: db.GetStrPtr("foo | bar"),
			wantOK:      true,
		},
		{
			name:   "standalone operator",
			input:  db.GetStrPtr("|"),
			wantOK: false,
		},
		{
			name:   "leading operator",
			input:  db.GetStrPtr("| foo"),
			wantOK: false,
		},
		{
			name:   "trailing operator",
			input:  db.GetStrPtr("foo |"),
			wantOK: false,
		},
		{
			name:   "consecutive operators",
			input:  db.GetStrPtr("foo | |"),
			wantOK: false,
		},
		{
			name:   "unsupported NOT operator",
			input:  db.GetStrPtr("foo ! bar"),
			wantOK: false,
		},
		{
			name:   "embedded operator",
			input:  db.GetStrPtr("foo|bar"),
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotTsQuery, gotOK := normalizeSearchQuery(tt.input)
			assert.Equal(t, tt.wantQuery, gotQuery)
			assert.Equal(t, tt.wantTsQuery, gotTsQuery)
			assert.Equal(t, tt.wantOK, gotOK)
		})
	}
}
