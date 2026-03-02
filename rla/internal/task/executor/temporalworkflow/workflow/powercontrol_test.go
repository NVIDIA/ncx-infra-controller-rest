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
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/inventoryobjects/component"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/inventoryobjects/rack"
	taskcommon "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operationrules"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	taskdef "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/deviceinfo"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/location"
)

// mockPowerControl is a mock activity function for testing
func mockPowerControl(
	ctx context.Context,
	info taskcommon.ComponentInfo,
	pcInfo *operations.PowerControlTaskInfo,
) error {
	return nil
}

func mockUpdateTaskStatus(ctx context.Context, arg *taskdef.TaskStatusUpdate) error {
	return nil
}

// mockGetPowerStatus is a mock activity function for testing power status verification.
// The actual return values are defined via env.OnActivity().Return() in each test case.
func mockGetPowerStatus(ctx context.Context, target common.Target) (map[string]operations.PowerStatus, error) {
	return nil, nil
}

// Helper function for creating test components
// - id: internal UUID (RLA database primary key)
// - name: human-readable name for DeviceInfo.Name
// - externalID: external ID for ComponentID (e.g., Carbide machine_id)
// - compType: component type
func newTestComponent(id uuid.UUID, name string, externalID string, compType devicetypes.ComponentType) *component.Component {
	return &component.Component{
		Type:        compType,
		ComponentID: externalID,
		Info: deviceinfo.DeviceInfo{
			ID:   id,
			Name: name,
		},
	}
}

// Helper function to build a rack from components for testing
func buildTestRack(components []*component.Component) *rack.Rack {
	r := rack.New(deviceinfo.DeviceInfo{ID: uuid.New(), Name: "test-rack"}, location.Location{})
	for _, c := range components {
		r.AddComponent(*c)
	}
	return r
}

// Helper function to create a default rule definition for power operations
func createDefaultPowerRuleDef(op operations.PowerOperation) *operationrules.RuleDefinition {
	switch op {
	case operations.PowerOperationPowerOn, operations.PowerOperationForcePowerOn:
		return &operationrules.RuleDefinition{
			Version: "v1",
			Steps: []operationrules.SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         1,
					MaxParallel:   1,
					DelayAfter:    30 * time.Second,
					Timeout:       10 * time.Minute,
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   1,
					DelayAfter:    15 * time.Second,
					Timeout:       15 * time.Minute,
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         3,
					MaxParallel:   1,
					Timeout:       20 * time.Minute,
				},
			},
		}
	case operations.PowerOperationPowerOff, operations.PowerOperationForcePowerOff:
		return &operationrules.RuleDefinition{
			Version: "v1",
			Steps: []operationrules.SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   1,
					DelayAfter:    10 * time.Second,
					Timeout:       20 * time.Minute,
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   1,
					DelayAfter:    5 * time.Second,
					Timeout:       15 * time.Minute,
				},
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         3,
					MaxParallel:   1,
					Timeout:       10 * time.Minute,
				},
			},
		}
	case operations.PowerOperationRestart, operations.PowerOperationForceRestart:
		// Restart uses power off sequence followed by power on sequence
		// For simplicity in tests, we'll use compute-only sequence
		return &operationrules.RuleDefinition{
			Version: "v1",
			Steps: []operationrules.SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   1,
					Timeout:       20 * time.Minute,
				},
			},
		}
	case operations.PowerOperationWarmReset, operations.PowerOperationColdReset:
		// Return empty steps for unsupported operations
		return &operationrules.RuleDefinition{
			Version: "v1",
			Steps:   []operationrules.SequenceStep{},
		}
	case operations.PowerOperationUnknown:
		// Return empty steps for unknown operations
		return &operationrules.RuleDefinition{
			Version: "v1",
			Steps:   []operationrules.SequenceStep{},
		}
	default:
		// Default minimal rule
		return &operationrules.RuleDefinition{
			Version: "v1",
			Steps:   []operationrules.SequenceStep{},
		}
	}
}

func TestPowerControlWorkflow(t *testing.T) {
	computeID1 := uuid.New()
	computeID2 := uuid.New()
	powershelfID := uuid.New()
	nvlswitchID := uuid.New()

	// Full set of components (PowerShelf, NVLSwitch, Compute)
	fullComponents := []*component.Component{
		newTestComponent(powershelfID, "powershelf-1", "ext-powershelf-1", devicetypes.ComponentTypePowerShelf),
		newTestComponent(nvlswitchID, "nvlswitch-1", "ext-nvlswitch-1", devicetypes.ComponentTypeNVLSwitch),
		newTestComponent(computeID1, "compute-1", "ext-compute-1", devicetypes.ComponentTypeCompute),
		newTestComponent(computeID2, "compute-2", "ext-compute-2", devicetypes.ComponentTypeCompute),
	}

	// Compute-only components
	computeOnlyComponents := []*component.Component{
		newTestComponent(computeID1, "compute-1", "ext-compute-1", devicetypes.ComponentTypeCompute),
		newTestComponent(computeID2, "compute-2", "ext-compute-2", devicetypes.ComponentTypeCompute),
	}

	testCases := map[string]struct {
		components    []*component.Component
		op            operations.PowerOperation
		activityError error
		expectError   bool
		errorContains string
	}{
		"power on full components success": {
			components:    fullComponents,
			op:            operations.PowerOperationPowerOn,
			activityError: nil,
			expectError:   false,
		},
		"power off full components success": {
			components:    fullComponents,
			op:            operations.PowerOperationPowerOff,
			activityError: nil,
			expectError:   false,
		},
		"force power on success": {
			components:    fullComponents,
			op:            operations.PowerOperationForcePowerOn,
			activityError: nil,
			expectError:   false,
		},
		"force power off success": {
			components:    fullComponents,
			op:            operations.PowerOperationForcePowerOff,
			activityError: nil,
			expectError:   false,
		},
		"power on compute only": {
			components:    computeOnlyComponents,
			op:            operations.PowerOperationPowerOn,
			activityError: nil,
			expectError:   false,
		},
		"empty components returns nil": {
			components:    nil,
			op:            operations.PowerOperationPowerOn,
			activityError: nil,
			expectError:   false,
		},
		"restart success": {
			components:    computeOnlyComponents,
			op:            operations.PowerOperationRestart,
			activityError: nil,
			expectError:   false,
		},
		"force restart success": {
			components:    computeOnlyComponents,
			op:            operations.PowerOperationForceRestart,
			activityError: nil,
			expectError:   false,
		},
		"warm reset not supported": {
			components:    fullComponents,
			op:            operations.PowerOperationWarmReset,
			activityError: nil,
			expectError:   true,
			errorContains: "rule definition has no steps",
		},
		"cold reset not supported": {
			components:    fullComponents,
			op:            operations.PowerOperationColdReset,
			activityError: nil,
			expectError:   true,
			errorContains: "rule definition has no steps",
		},
		"unknown operation returns error": {
			components:    fullComponents,
			op:            operations.PowerOperationUnknown,
			activityError: nil,
			expectError:   true,
			errorContains: "rule definition has no steps",
		},
		"activity failure returns error": {
			components:    computeOnlyComponents,
			op:            operations.PowerOperationPowerOn,
			activityError: errors.New("BMC connection failed"),
			expectError:   true,
			errorContains: "BMC connection failed",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestWorkflowEnvironment()

			env.RegisterActivityWithOptions(mockPowerControl, activity.RegisterOptions{
				Name: "PowerControl",
			})
			env.RegisterActivityWithOptions(mockUpdateTaskStatus, activity.RegisterOptions{
				Name: "UpdateTaskStatus",
			})
			env.RegisterActivityWithOptions(mockGetPowerStatus, activity.RegisterOptions{
				Name: "GetPowerStatus",
			})
			// Register the child workflow needed for rule-based execution
			env.RegisterWorkflow(GenericComponentStepWorkflow)

			env.OnActivity(mockPowerControl, mock.Anything, mock.Anything, mock.Anything).Return(tc.activityError)
			env.OnActivity(mockUpdateTaskStatus, mock.Anything, mock.Anything).Return(nil)

			// Track call count for restart operations which need Off then On
			callCount := 0
			// Count unique component types to know when off phase ends
			numComponentTypes := 0
			if tc.components != nil {
				typesSeen := make(map[devicetypes.ComponentType]bool)
				for _, c := range tc.components {
					typesSeen[c.Type] = true
				}
				numComponentTypes = len(typesSeen)
			}

			env.OnActivity(mockGetPowerStatus, mock.Anything, mock.Anything).Return(
				func(ctx context.Context, target common.Target) (map[string]operations.PowerStatus, error) {
					callCount++
					result := make(map[string]operations.PowerStatus)

					// Determine expected status based on operation and call sequence
					var expectedStatus operations.PowerStatus
					switch tc.op {
					case operations.PowerOperationPowerOff, operations.PowerOperationForcePowerOff:
						expectedStatus = operations.PowerStatusOff
					case operations.PowerOperationRestart, operations.PowerOperationForceRestart:
						// Restart: first phase is off (1 call per component type), then on
						// Off phase ends after we've verified each component type once
						if callCount <= numComponentTypes {
							expectedStatus = operations.PowerStatusOff
						} else {
							expectedStatus = operations.PowerStatusOn
						}
					default:
						expectedStatus = operations.PowerStatusOn
					}

					for _, componentID := range target.ComponentIDs {
						result[componentID] = expectedStatus
					}
					return result, nil
				},
			)

			info := operations.PowerControlTaskInfo{Operation: tc.op}
			reqInfo := taskdef.ExecutionInfo{
				TaskID:         uuid.New(),
				Rack:           buildTestRack(tc.components),
				RuleDefinition: createDefaultPowerRuleDef(tc.op),
			}

			env.ExecuteWorkflow(PowerControl, reqInfo, info)

			assert.True(t, env.IsWorkflowCompleted())

			wfErr := env.GetWorkflowError()
			if tc.expectError {
				assert.Error(t, wfErr)
				if tc.errorContains != "" && wfErr != nil {
					assert.Contains(t, wfErr.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, wfErr)
			}
		})
	}
}

func TestGenericComponentStepWorkflow_BackwardCompatibilityValidation(t *testing.T) {
	t.Run("missing both main_operation and activityName returns error", func(t *testing.T) {
		testSuite := &testsuite.WorkflowTestSuite{}
		env := testSuite.NewTestWorkflowEnvironment()
		env.RegisterWorkflow(GenericComponentStepWorkflow)

		step := operationrules.SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   1,
			Timeout:       10 * time.Minute,
			// No MainOperation configured, no PreOperation, no PostOperation
		}

		target := common.Target{
			Type:         devicetypes.ComponentTypeCompute,
			ComponentIDs: []string{"test-compute-1"},
		}

		allTargets := map[devicetypes.ComponentType]common.Target{
			devicetypes.ComponentTypeCompute: target,
		}

		// Call workflow with empty activityName (backward compat mode but no activity)
		env.ExecuteWorkflow(
			GenericComponentStepWorkflow,
			step,
			target,
			"", // Empty activityName - should trigger error
			nil,
			allTargets,
		)

		assert.True(t, env.IsWorkflowCompleted())
		err := env.GetWorkflowError()
		assert.Error(t, err)
		assert.Contains(
			t,
			err.Error(),
			"no main operation configured and no legacy activityName provided",
		)
	})

	t.Run("legacy mode with valid activityName succeeds", func(t *testing.T) {
		testSuite := &testsuite.WorkflowTestSuite{}
		env := testSuite.NewTestWorkflowEnvironment()
		env.RegisterWorkflow(GenericComponentStepWorkflow)

		// Register mock activity with correct name
		env.RegisterActivityWithOptions(
			mockPowerControl,
			activity.RegisterOptions{Name: "PowerControl"},
		)
		env.OnActivity("PowerControl", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		step := operationrules.SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   1,
			Timeout:       10 * time.Minute,
			// No MainOperation configured
		}

		target := common.Target{
			Type:         devicetypes.ComponentTypeCompute,
			ComponentIDs: []string{"test-compute-1"},
		}

		allTargets := map[devicetypes.ComponentType]common.Target{
			devicetypes.ComponentTypeCompute: target,
		}

		info := operations.PowerControlTaskInfo{
			Operation: operations.PowerOperationPowerOn,
		}

		// Call workflow with valid activityName (backward compat mode)
		env.ExecuteWorkflow(
			GenericComponentStepWorkflow,
			step,
			target,
			"PowerControl", // Valid activityName - should succeed
			info,
			allTargets,
		)

		assert.True(t, env.IsWorkflowCompleted())
		err := env.GetWorkflowError()
		assert.NoError(t, err)
	})
}
