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

package activity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
)

func TestSetFirmwareUpdateTimeWindowActivity(t *testing.T) {
	ctx := context.Background()
	baseTime := time.Now()

	testCases := map[string]struct {
		request     operations.SetFirmwareUpdateTimeWindowRequest
		expectError bool
	}{
		"empty component list returns early without error": {
			request: operations.SetFirmwareUpdateTimeWindowRequest{
				ComponentIDs: []string{},
				StartTime:    baseTime,
				EndTime:      baseTime.Add(time.Hour),
			},
			expectError: false,
		},
		"nil component list returns early without error": {
			request: operations.SetFirmwareUpdateTimeWindowRequest{
				ComponentIDs: nil,
				StartTime:    baseTime,
				EndTime:      baseTime.Add(time.Hour),
			},
			expectError: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := SetFirmwareUpdateTimeWindow(ctx, tc.request)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
