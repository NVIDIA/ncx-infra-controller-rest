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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operationrules"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

// Mock activities for testing
func mockVerifyPowerStatus(
	ctx context.Context,
	target common.Target,
	expectedStatus operations.PowerStatus,
	timeout time.Duration,
	pollInterval time.Duration,
) error {
	return nil
}

func mockVerifyReachability(
	ctx context.Context,
	allTargets map[devicetypes.ComponentType]common.Target,
	componentTypes []string,
	timeout time.Duration,
	pollInterval time.Duration,
) error {
	return nil
}

// TestGenericComponentStepWorkflow_ActionBased tests the new action-based
// execution with pre/main/post operations
func TestGenericComponentStepWorkflow_ActionBased(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register activities with correct names
	env.RegisterActivityWithOptions(mockPowerControl,
		activity.RegisterOptions{Name: "PowerControl"})
	env.RegisterActivityWithOptions(mockGetPowerStatus,
		activity.RegisterOptions{Name: "GetPowerStatus"})
	env.RegisterActivityWithOptions(mockVerifyPowerStatus,
		activity.RegisterOptions{Name: "VerifyPowerStatus"})
	env.RegisterActivityWithOptions(mockVerifyReachability,
		activity.RegisterOptions{Name: "VerifyReachability"})

	// Mock activity responses
	env.OnActivity(mockPowerControl, mock.Anything, mock.Anything,
		mock.Anything).Return(nil)
	// GetPowerStatus returns map of component IDs to power status
	env.OnActivity(mockGetPowerStatus, mock.Anything,
		mock.Anything).Return(map[string]operations.PowerStatus{
		"test-powershelf-1": operations.PowerStatusOn,
	}, nil)
	env.OnActivity(mockVerifyPowerStatus, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(mockVerifyReachability, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Create test step with action-based configuration
	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypePowerShelf,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		RetryPolicy: &operationrules.RetryPolicy{
			MaxAttempts:        3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionPowerControl,
		},
		PostOperation: []operationrules.ActionConfig{
			{
				Name:         operationrules.ActionVerifyPowerStatus,
				Timeout:      15 * time.Second,
				PollInterval: 5 * time.Second,
				Parameters: map[string]any{
					operationrules.ParamExpectedStatus: "on",
				},
			},
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypePowerShelf,
		ComponentIDs: []string{"test-powershelf-1"},
	}

	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypePowerShelf: target,
	}

	operationInfo := &operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationPowerOn,
	}

	// Execute workflow
	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		operationInfo, allTargets)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

// TestGenericComponentStepWorkflow_WithSleepAction tests Sleep action in
// post-operations
func TestGenericComponentStepWorkflow_WithSleepAction(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register activities with correct names
	env.RegisterActivityWithOptions(mockPowerControl,
		activity.RegisterOptions{Name: "PowerControl"})

	// Mock activity responses
	env.OnActivity(mockPowerControl, mock.Anything, mock.Anything,
		mock.Anything).Return(nil)

	// Create test step with Sleep action
	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypePowerShelf,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionPowerControl,
		},
		PostOperation: []operationrules.ActionConfig{
			{
				Name: operationrules.ActionSleep,
				Parameters: map[string]any{
					operationrules.ParamDuration: 5 * time.Second,
				},
			},
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypePowerShelf,
		ComponentIDs: []string{"test-powershelf-1"},
	}

	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypePowerShelf: target,
	}

	operationInfo := &operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationPowerOn,
	}

	// Execute workflow
	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		operationInfo, allTargets)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

// TestGenericComponentStepWorkflow_VerificationFailure tests workflow
// behavior when verification fails
func TestGenericComponentStepWorkflow_VerificationFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register activities with correct names
	env.RegisterActivityWithOptions(mockPowerControl,
		activity.RegisterOptions{Name: "PowerControl"})
	env.RegisterActivity(mockVerifyPowerStatus)

	// Mock activity responses - verification fails
	env.OnActivity(mockPowerControl, mock.Anything, mock.Anything,
		mock.Anything).Return(nil)
	env.OnActivity(mockVerifyPowerStatus, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
		mock.Anything).Return(errors.New("verification timeout"))

	// Create test step with verification
	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypePowerShelf,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionPowerControl,
		},
		PostOperation: []operationrules.ActionConfig{
			{
				Name:         operationrules.ActionVerifyPowerStatus,
				Timeout:      15 * time.Second,
				PollInterval: 5 * time.Second,
				Parameters: map[string]any{
					operationrules.ParamExpectedStatus: "on",
				},
			},
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypePowerShelf,
		ComponentIDs: []string{"test-powershelf-1"},
	}

	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypePowerShelf: target,
	}

	operationInfo := &operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationPowerOn,
	}

	// Execute workflow
	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		operationInfo, allTargets)

	// Verify workflow completed with error
	assert.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(),
		"post-operation failed")
}

// TestGenericComponentStepWorkflow_PreOperation tests pre-operation
// execution
func TestGenericComponentStepWorkflow_PreOperation(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register activities with correct names
	env.RegisterActivityWithOptions(mockPowerControl,
		activity.RegisterOptions{Name: "PowerControl"})

	// Mock activity responses
	env.OnActivity(mockPowerControl, mock.Anything, mock.Anything,
		mock.Anything).Return(nil)

	// Create test step with pre-operation Sleep
	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypePowerShelf,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		PreOperation: []operationrules.ActionConfig{
			{
				Name: operationrules.ActionSleep,
				Parameters: map[string]any{
					operationrules.ParamDuration: 5 * time.Second,
				},
			},
		},
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionPowerControl,
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypePowerShelf,
		ComponentIDs: []string{"test-powershelf-1"},
	}

	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypePowerShelf: target,
	}

	operationInfo := &operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationPowerOff,
	}

	// Execute workflow
	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		operationInfo, allTargets)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

// TestGenericComponentStepWorkflow_EmptyMainOperation tests backward
// compatibility with legacy activityName parameter
func TestGenericComponentStepWorkflow_EmptyMainOperation(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register activities with correct names
	env.RegisterActivityWithOptions(mockPowerControl,
		activity.RegisterOptions{Name: "PowerControl"})

	// Mock activity responses
	env.OnActivity(mockPowerControl, mock.Anything, mock.Anything,
		mock.Anything).Return(nil)

	// Create test step WITHOUT MainOperation (use legacy activityName)
	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypePowerShelf,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypePowerShelf,
		ComponentIDs: []string{"test-powershelf-1"},
	}

	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypePowerShelf: target,
	}

	operationInfo := &operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationPowerOn,
	}

	// Execute workflow with legacy activityName parameter
	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target,
		"PowerControl", operationInfo, allTargets)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}
