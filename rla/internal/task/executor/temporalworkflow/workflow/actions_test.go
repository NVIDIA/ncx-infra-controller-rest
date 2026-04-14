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

package workflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	activitypkg "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/executor/temporalworkflow/activity"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/operationrules"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/operations"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/common/devicetypes"
)

func TestToFirmwareControlTaskInfo(t *testing.T) {
	tests := map[string]struct {
		input       any
		expected    operations.FirmwareControlTaskInfo
		expectError bool
	}{
		"concrete value": {
			input: operations.FirmwareControlTaskInfo{
				Operation:     operations.FirmwareOperationUpgrade,
				TargetVersion: "1.3.1",
			},
			expected: operations.FirmwareControlTaskInfo{
				Operation:     operations.FirmwareOperationUpgrade,
				TargetVersion: "1.3.1",
			},
		},
		"pointer": {
			input: &operations.FirmwareControlTaskInfo{
				Operation:     operations.FirmwareOperationUpgrade,
				TargetVersion: "2.0.0",
				StartTime:     1000,
				EndTime:       2000,
			},
			expected: operations.FirmwareControlTaskInfo{
				Operation:     operations.FirmwareOperationUpgrade,
				TargetVersion: "2.0.0",
				StartTime:     1000,
				EndTime:       2000,
			},
		},
		"nil pointer": {
			input:       (*operations.FirmwareControlTaskInfo)(nil),
			expectError: true,
		},
		"map from JSON deserialization": {
			input: map[string]interface{}{
				"operation":      float64(1),
				"target_version": "1.3.1",
			},
			expected: operations.FirmwareControlTaskInfo{
				Operation:     operations.FirmwareOperationUpgrade,
				TargetVersion: "1.3.1",
			},
		},
		"map without target_version": {
			input: map[string]interface{}{
				"operation": float64(1),
			},
			expected: operations.FirmwareControlTaskInfo{
				Operation: operations.FirmwareOperationUpgrade,
			},
		},
		"map with all fields": {
			input: map[string]interface{}{
				"operation":      float64(1),
				"target_version": "r1.3.8",
				"start_time":     float64(1000),
				"end_time":       float64(2000),
			},
			expected: operations.FirmwareControlTaskInfo{
				Operation:     operations.FirmwareOperationUpgrade,
				TargetVersion: "r1.3.8",
				StartTime:     1000,
				EndTime:       2000,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := toFirmwareControlTaskInfo(tc.input)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestFirmwareControlAction_TargetVersionSurvivesMapRoundtrip simulates the
// real Temporal child-workflow deserialization: operationInfo arrives as a
// map[string]interface{} (what json.Unmarshal produces for an `any` target).
// Without the toFirmwareControlTaskInfo fix, the activity would receive an
// empty TargetVersion because the map re-serialization loses the field.
func TestFirmwareControlAction_TargetVersionSurvivesMapRoundtrip(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var capturedInfo operations.FirmwareControlTaskInfo
	mockStart := func(ctx context.Context, target common.Target, info operations.FirmwareControlTaskInfo) error {
		capturedInfo = info
		return nil
	}
	mockStatus := func(ctx context.Context, target common.Target) (*activitypkg.GetFirmwareStatusResult, error) {
		return &activitypkg.GetFirmwareStatusResult{
			Statuses: map[string]operations.FirmwareUpdateStatus{
				"switch-1": {
					ComponentID: "switch-1",
					State:       operations.FirmwareUpdateStateCompleted,
				},
			},
		}, nil
	}

	env.RegisterActivityWithOptions(mockStart,
		activity.RegisterOptions{Name: "FirmwareControl"})
	env.RegisterActivityWithOptions(mockStatus,
		activity.RegisterOptions{Name: "GetFirmwareStatus"})

	env.OnActivity(mockStart, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(mockStatus, mock.Anything, mock.Anything).Return(
		&activitypkg.GetFirmwareStatusResult{
			Statuses: map[string]operations.FirmwareUpdateStatus{
				"switch-1": {
					ComponentID: "switch-1",
					State:       operations.FirmwareUpdateStateCompleted,
				},
			},
		}, nil)

	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypeNVLSwitch,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionFirmwareControl,
			Parameters: map[string]any{
				operationrules.ParamPollInterval: "1s",
				operationrules.ParamPollTimeout:  "10s",
			},
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypeNVLSwitch,
		ComponentIDs: []string{"switch-1"},
	}
	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypeNVLSwitch: target,
	}

	// Simulate what Temporal does when deserializing an any-typed child
	// workflow parameter: marshal the struct to JSON, then unmarshal into
	// interface{} — producing a map[string]interface{}.
	original := &operations.FirmwareControlTaskInfo{
		Operation:     operations.FirmwareOperationUpgrade,
		TargetVersion: "1.3.1",
	}
	raw, err := json.Marshal(original)
	require.NoError(t, err)
	var asAny any
	require.NoError(t, json.Unmarshal(raw, &asAny))

	// asAny is now map[string]interface{}{"operation":1,"target_version":"1.3.1"}
	// — exactly what the child workflow receives from Temporal.
	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		asAny, allTargets)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
	assert.Equal(t, operations.FirmwareOperationUpgrade, capturedInfo.Operation)
	assert.Equal(t, "1.3.1", capturedInfo.TargetVersion,
		"TargetVersion must survive map[string]interface{} roundtrip")
}
