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

	activitypkg "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/activity"
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

// TestGenericComponentStepWorkflow_VerifyReachabilityRequireAll tests that
// VerifyReachability with require_all=true succeeds when GetPowerStatus
// returns all requested individual components.
func TestGenericComponentStepWorkflow_VerifyReachabilityRequireAll(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivityWithOptions(mockGetPowerStatus,
		activity.RegisterOptions{Name: "GetPowerStatus"})
	env.RegisterActivityWithOptions(mockPowerControl,
		activity.RegisterOptions{Name: "PowerControl"})

	env.OnActivity(mockGetPowerStatus, mock.Anything, mock.Anything).Return(
		map[string]operations.PowerStatus{
			"ps-1": operations.PowerStatusOff,
		}, nil)
	env.OnActivity(mockPowerControl, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypePowerShelf,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		PreOperation: []operationrules.ActionConfig{
			{
				Name:         operationrules.ActionVerifyReachability,
				Timeout:      5 * time.Second,
				PollInterval: 1 * time.Second,
				Parameters: map[string]any{
					operationrules.ParamComponentTypes: []string{"powershelf"},
					operationrules.ParamRequireAll:     true,
				},
			},
		},
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionPowerControl,
			Parameters: map[string]any{
				operationrules.ParamOperation: "power_on",
			},
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypePowerShelf,
		ComponentIDs: []string{"ps-1"},
	}
	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypePowerShelf: target,
	}

	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		&operations.BringUpTaskInfo{}, allTargets)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

// TestGenericComponentStepWorkflow_AllowBringUpAndWait tests AllowBringUp as
// MainOperation and WaitBringUp as PostOperation.
func TestGenericComponentStepWorkflow_AllowBringUpAndWait(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	mockAllowBringUp := func(ctx context.Context, target common.Target) error {
		return nil
	}
	mockGetBringUp := func(ctx context.Context, target common.Target) (*activitypkg.GetBringUpStateResult, error) {
		return nil, nil
	}

	env.RegisterActivityWithOptions(mockAllowBringUp,
		activity.RegisterOptions{Name: "AllowBringUpAndPowerOn"})
	env.RegisterActivityWithOptions(mockGetBringUp,
		activity.RegisterOptions{Name: "GetBringUpState"})

	env.OnActivity(mockAllowBringUp, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(mockGetBringUp, mock.Anything, mock.Anything).Return(
		&activitypkg.GetBringUpStateResult{
			States: map[string]operations.MachineBringUpState{
				"compute-1": operations.MachineBringUpStateMachineCreated,
			},
		}, nil)

	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypeCompute,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionAllowBringUp,
		},
		PostOperation: []operationrules.ActionConfig{
			{
				Name:         operationrules.ActionWaitBringUp,
				Timeout:      5 * time.Second,
				PollInterval: 1 * time.Second,
			},
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypeCompute,
		ComponentIDs: []string{"compute-1"},
	}
	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypeCompute: target,
	}

	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		&operations.BringUpTaskInfo{}, allTargets)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

// TestGenericComponentStepWorkflow_FirmwareControlAction tests the
// FirmwareControl action executor with start + poll pattern.
func TestGenericComponentStepWorkflow_FirmwareControlAction(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	mockStart := func(ctx context.Context, target common.Target, info operations.FirmwareControlTaskInfo) error {
		return nil
	}
	mockStatus := func(ctx context.Context, target common.Target) (*activitypkg.GetFirmwareUpdateStatusResult, error) {
		return nil, nil
	}

	env.RegisterActivityWithOptions(mockStart,
		activity.RegisterOptions{Name: "StartFirmwareUpdate"})
	env.RegisterActivityWithOptions(mockStatus,
		activity.RegisterOptions{Name: "GetFirmwareUpdateStatus"})

	env.OnActivity(mockStart, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(mockStatus, mock.Anything, mock.Anything).Return(
		&activitypkg.GetFirmwareUpdateStatusResult{
			Statuses: map[string]operations.FirmwareUpdateStatus{
				"compute-1": {
					ComponentID: "compute-1",
					State:       operations.FirmwareUpdateStateCompleted,
				},
			},
		}, nil)

	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypeCompute,
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
		Type:         devicetypes.ComponentTypeCompute,
		ComponentIDs: []string{"compute-1"},
	}
	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypeCompute: target,
	}

	info := &operations.FirmwareControlTaskInfo{
		Operation: operations.FirmwareOperationUpgrade,
		StartTime: time.Now().Unix(),
		EndTime:   time.Now().Add(2 * time.Hour).Unix(),
	}

	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		info, allTargets)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

// TestGenericComponentStepWorkflow_PowerControlWithParamOperation tests that
// PowerControl action constructs PowerControlTaskInfo from ParamOperation
// when the workflow's operationInfo is a different type (cross-workflow use).
func TestGenericComponentStepWorkflow_PowerControlWithParamOperation(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivityWithOptions(mockPowerControl,
		activity.RegisterOptions{Name: "PowerControl"})

	env.OnActivity(mockPowerControl, mock.Anything, mock.Anything,
		mock.Anything).Return(nil)

	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypeCompute,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionPowerControl,
			Parameters: map[string]any{
				operationrules.ParamOperation: "force_power_off",
			},
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypeCompute,
		ComponentIDs: []string{"compute-1"},
	}
	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypeCompute: target,
	}

	// Pass FirmwareControlTaskInfo as operationInfo -- NOT PowerControlTaskInfo.
	// The action executor should use ParamOperation instead.
	firmwareInfo := &operations.FirmwareControlTaskInfo{
		Operation: operations.FirmwareOperationUpgrade,
		StartTime: time.Now().Unix(),
		EndTime:   time.Now().Add(2 * time.Hour).Unix(),
	}

	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		firmwareInfo, allTargets)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

// TestGenericComponentStepWorkflow_InjectExpectationAction tests the
// InjectExpectation action executor used in ingestion and full bring-up rules.
func TestGenericComponentStepWorkflow_InjectExpectationAction(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	mockInjectExpectation := func(
		ctx context.Context,
		target common.Target,
		info operations.InjectExpectationTaskInfo,
	) error {
		return nil
	}

	env.RegisterActivityWithOptions(mockInjectExpectation,
		activity.RegisterOptions{Name: "InjectExpectation"})

	env.OnActivity(mockInjectExpectation, mock.Anything, mock.Anything,
		mock.Anything).Return(nil)

	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypePowerShelf,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionInjectExpectation,
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypePowerShelf,
		ComponentIDs: []string{"ps-1", "ps-2"},
	}
	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypePowerShelf: target,
	}

	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		&operations.BringUpTaskInfo{}, allTargets)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

// TestGenericComponentStepWorkflow_InjectExpectationFailure verifies that
// InjectExpectation action propagates activity errors.
func TestGenericComponentStepWorkflow_InjectExpectationFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	mockInjectExpectation := func(
		ctx context.Context,
		target common.Target,
		info operations.InjectExpectationTaskInfo,
	) error {
		return nil
	}

	env.RegisterActivityWithOptions(mockInjectExpectation,
		activity.RegisterOptions{Name: "InjectExpectation"})

	env.OnActivity(mockInjectExpectation, mock.Anything, mock.Anything,
		mock.Anything).Return(errors.New("component manager service unavailable"))

	step := operationrules.SequenceStep{
		ComponentType: devicetypes.ComponentTypeCompute,
		Stage:         1,
		MaxParallel:   0,
		Timeout:       10 * time.Minute,
		MainOperation: operationrules.ActionConfig{
			Name: operationrules.ActionInjectExpectation,
		},
	}

	target := common.Target{
		Type:         devicetypes.ComponentTypeCompute,
		ComponentIDs: []string{"compute-1"},
	}
	allTargets := map[devicetypes.ComponentType]common.Target{
		devicetypes.ComponentTypeCompute: target,
	}

	env.ExecuteWorkflow(GenericComponentStepWorkflow, step, target, "",
		&operations.BringUpTaskInfo{}, allTargets)

	assert.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
