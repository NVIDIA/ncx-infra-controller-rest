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

package location

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocation(t *testing.T) {
	loc := Location{
		Region:     "NA",
		DataCenter: "DC1",
		Room:       "Room1",
		Position:   "Pos1",
	}

	jsonData, err := json.Marshal(loc)
	assert.NoError(t, err)

	newLoc := New(jsonData)
	assert.Equal(t, loc, newLoc)

	invalidJSON := []byte("invalid")
	unknownLoc := New(invalidJSON)
	assert.Equal(t, "", unknownLoc.Region)
	assert.Equal(t, "", unknownLoc.DataCenter)
	assert.Equal(t, "", unknownLoc.Room)
	assert.Equal(t, "", unknownLoc.Position)
}

func TestLocation_IsValid(t *testing.T) {
	testCases := map[string]struct {
		location    *Location
		expected    bool
		description string
	}{
		"valid location with all fields populated": {
			location: &Location{
				Region:     "NA",
				DataCenter: "DC1",
				Room:       "Room1",
				Position:   "Rack-A1",
			},
			expected:    true,
			description: "should return true when all fields are non-empty",
		},
		"nil location": {
			location:    nil,
			expected:    false,
			description: "should return false for nil location",
		},
		"empty region": {
			location: &Location{
				Region:     "",
				DataCenter: "DC1",
				Room:       "Room1",
				Position:   "Rack-A1",
			},
			expected:    false,
			description: "should return false when region is empty",
		},
		"empty data center": {
			location: &Location{
				Region:     "NA",
				DataCenter: "",
				Room:       "Room1",
				Position:   "Rack-A1",
			},
			expected:    false,
			description: "should return false when data center is empty",
		},
		"empty room": {
			location: &Location{
				Region:     "NA",
				DataCenter: "DC1",
				Room:       "",
				Position:   "Rack-A1",
			},
			expected:    false,
			description: "should return false when room is empty",
		},
		"empty position": {
			location: &Location{
				Region:     "NA",
				DataCenter: "DC1",
				Room:       "Room1",
				Position:   "",
			},
			expected:    false,
			description: "should return false when position is empty",
		},
		"all fields empty": {
			location: &Location{
				Region:     "",
				DataCenter: "",
				Room:       "",
				Position:   "",
			},
			expected:    false,
			description: "should return false when all fields are empty",
		},
		"multiple empty fields": {
			location: &Location{
				Region:     "NA",
				DataCenter: "",
				Room:       "",
				Position:   "Rack-A1",
			},
			expected:    false,
			description: "should return false when multiple fields are empty",
		},
		"whitespace-only fields": {
			location: &Location{
				Region:     " ",
				DataCenter: "DC1",
				Room:       "Room1",
				Position:   "Rack-A1",
			},
			expected:    true,
			description: "should return true for whitespace-only fields (whitespace is considered non-empty)",
		},
		"single character fields": {
			location: &Location{
				Region:     "A",
				DataCenter: "1",
				Room:       "B",
				Position:   "2",
			},
			expected:    true,
			description: "should return true for single character fields",
		},
		"special characters in fields": {
			location: &Location{
				Region:     "US-WEST",
				DataCenter: "DC_1",
				Room:       "Room-2A",
				Position:   "Rack#001",
			},
			expected:    true,
			description: "should return true for fields with special characters",
		},
		"unicode characters in fields": {
			location: &Location{
				Region:     "亚洲",
				DataCenter: "数据中心1",
				Room:       "机房1",
				Position:   "位置A",
			},
			expected:    true,
			description: "should return true for fields with unicode characters",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			result := tc.location.IsValid()
			assert.Equal(t, tc.expected, result, tc.description)
		})
	}
}
