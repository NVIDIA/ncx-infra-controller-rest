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

package operationrules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/common/devicetypes"
)

// TestBringDownRule_StageOrdering pins the bring-down sequence:
//
//	Stage 1: Compute    PowerControl(power_off) + verify off
//	Stage 2: Compute    PausePowerOnGate
//	Stage 3: NVLSwitch  PowerControl(power_off) + verify off
//	Stage 4: PowerShelf PowerControl(power_off) + verify off
//
// The compute stages must come before NVLSwitch / PowerShelf so that any
// in-flight workload can be drained before the fabric and power infrastructure
// disappear from under it. PausePowerOnGate is a separate stage immediately
// after compute power-off so the per-machine power-on gate is closed while the
// machine is already down — this prevents Carbide's reconcile loop from
// re-powering the compute the instant we proceed with switch / shelf.
func TestBringDownRule_StageOrdering(t *testing.T) {
	rule := buildBringDownRule()

	require.NotNil(t, rule)
	assert.Equal(t, common.TaskTypeBringUp, rule.OperationType)
	assert.Equal(t, SequenceBringDown, rule.OperationCode)

	steps := rule.RuleDefinition.Steps
	require.Len(t, steps, 4, "bring-down rule must have exactly 4 stages")

	expected := []struct {
		stage         int
		componentType devicetypes.ComponentType
		mainAction    string
		powerOp       string // empty when the action is not PowerControl
		hasVerifyOff  bool
	}{
		{1, devicetypes.ComponentTypeCompute, ActionPowerControl, "power_off", true},
		{2, devicetypes.ComponentTypeCompute, ActionPausePowerOnGate, "", false},
		{3, devicetypes.ComponentTypeNVLSwitch, ActionPowerControl, "power_off", true},
		{4, devicetypes.ComponentTypePowerShelf, ActionPowerControl, "power_off", true},
	}

	for i, exp := range expected {
		step := steps[i]
		assert.Equal(t, exp.stage, step.Stage, "step %d stage", i)
		assert.Equal(t, exp.componentType, step.ComponentType, "step %d component type", i)
		assert.Equal(t, exp.mainAction, step.MainOperation.Name, "step %d main action", i)

		if exp.powerOp != "" {
			assert.Equal(
				t,
				exp.powerOp,
				step.MainOperation.Parameters[ParamOperation],
				"step %d power op param", i,
			)
		}

		if exp.hasVerifyOff {
			require.Len(t, step.PostOperation, 1, "step %d expected one post-op", i)
			post := step.PostOperation[0]
			assert.Equal(t, ActionVerifyPowerStatus, post.Name)
			assert.Equal(t, "off", post.Parameters[ParamExpectedStatus])
		} else {
			assert.Empty(t, step.PostOperation, "step %d expected no post-op", i)
		}
	}
}

// TestBringDownRule_RegisteredInHardcodedMap guards against accidentally
// removing the bring-down rule from the hardcoded rule map. Callers resolve
// rules by (TaskType, OpCode); without this entry the resolver returns nil
// and the workflow has no steps to execute.
func TestBringDownRule_RegisteredInHardcodedMap(t *testing.T) {
	key := ruleKey(common.TaskTypeBringUp, SequenceBringDown)
	rule, ok := hardcodedRuleMap[key]
	require.True(t, ok, "bring-down rule must be registered under key %q", key)
	require.NotNil(t, rule)
	assert.Equal(t, SequenceBringDown, rule.OperationCode)
}
