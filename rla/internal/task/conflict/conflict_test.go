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

package conflict

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/operation"
	taskcommon "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	taskdef "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
)

// makeTask is a test helper that constructs a minimal Task for conflict tests.
func makeTask(rackID uuid.UUID, opType taskcommon.TaskType, opCode string, componentUUIDs ...uuid.UUID) *taskdef.Task {
	return &taskdef.Task{
		ID:             uuid.New(),
		RackID:         rackID,
		ComponentUUIDs: componentUUIDs,
		Operation: operation.Wrapper{
			Type: opType,
			Code: opCode,
		},
		Status: taskcommon.TaskStatusRunning,
	}
}

func TestOperationSpec_Matches(t *testing.T) {
	tests := []struct {
		name     string
		spec     OperationSpec
		target   OperationSpec
		expected bool
	}{
		{
			name:     "exact match",
			spec:     OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			target:   OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			expected: true,
		},
		{
			name:     "wildcard type matches any type",
			spec:     OperationSpec{OperationType: "*", OperationCode: "power_on"},
			target:   OperationSpec{OperationType: "anything", OperationCode: "power_on"},
			expected: true,
		},
		{
			name:     "wildcard code matches any code",
			spec:     OperationSpec{OperationType: "power_control", OperationCode: "*"},
			target:   OperationSpec{OperationType: "power_control", OperationCode: "power_off"},
			expected: true,
		},
		{
			name:     "both wildcards match anything",
			spec:     OperationSpec{OperationType: "*", OperationCode: "*"},
			target:   OperationSpec{OperationType: "anything", OperationCode: "anything"},
			expected: true,
		},
		{
			name:     "type mismatch",
			spec:     OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			target:   OperationSpec{OperationType: "firmware_control", OperationCode: "power_on"},
			expected: false,
		},
		{
			name:     "code mismatch",
			spec:     OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			target:   OperationSpec{OperationType: "power_control", OperationCode: "power_off"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.spec.Matches(tc.target))
		})
	}
}

func TestConflictDimension_Overlaps_Rack(t *testing.T) {
	sharedRack := uuid.New()

	tests := []struct {
		name     string
		a        *taskdef.Task
		b        *taskdef.Task
		expected bool
	}{
		{
			name:     "same rack overlaps",
			a:        makeTask(sharedRack, taskcommon.TaskTypePowerControl, "power_on"),
			b:        makeTask(sharedRack, taskcommon.TaskTypePowerControl, "power_off"),
			expected: true,
		},
		{
			name:     "different racks do not overlap",
			a:        makeTask(uuid.New(), taskcommon.TaskTypePowerControl, "power_on"),
			b:        makeTask(uuid.New(), taskcommon.TaskTypePowerControl, "power_on"),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, DimensionRack.Overlaps(tc.a, tc.b))
		})
	}
}

func TestConflictDimension_Overlaps_ComponentUUID(t *testing.T) {
	rackID := uuid.New()
	shared := uuid.New()

	tests := []struct {
		name     string
		a        *taskdef.Task
		b        *taskdef.Task
		expected bool
	}{
		{
			name:     "shared component UUID overlaps",
			a:        makeTask(rackID, taskcommon.TaskTypePowerControl, "power_on", shared, uuid.New()),
			b:        makeTask(rackID, taskcommon.TaskTypePowerControl, "power_on", uuid.New(), shared),
			expected: true,
		},
		{
			name:     "no shared component UUIDs do not overlap",
			a:        makeTask(rackID, taskcommon.TaskTypePowerControl, "power_on", uuid.New()),
			b:        makeTask(rackID, taskcommon.TaskTypePowerControl, "power_on", uuid.New()),
			expected: false,
		},
		{
			name:     "empty component UUIDs do not overlap",
			a:        makeTask(rackID, taskcommon.TaskTypePowerControl, "power_on"),
			b:        makeTask(rackID, taskcommon.TaskTypePowerControl, "power_on"),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, DimensionComponentUUID.Overlaps(tc.a, tc.b))
		})
	}
}

func TestConflictEntry_Matches(t *testing.T) {
	entry := ConflictEntry{
		A: OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
		B: OperationSpec{OperationType: "firmware_control", OperationCode: "upgrade"},
	}

	tests := []struct {
		name     string
		p        OperationSpec
		q        OperationSpec
		expected bool
	}{
		{
			name:     "forward match",
			p:        OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			q:        OperationSpec{OperationType: "firmware_control", OperationCode: "upgrade"},
			expected: true,
		},
		{
			name:     "reversed match (symmetric)",
			p:        OperationSpec{OperationType: "firmware_control", OperationCode: "upgrade"},
			q:        OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			expected: true,
		},
		{
			name:     "no match",
			p:        OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			q:        OperationSpec{OperationType: "power_control", OperationCode: "power_off"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, entry.Matches(tc.p, tc.q))
		})
	}
}

func TestConflictRule_Conflicts_ExclusiveMode(t *testing.T) {
	rule := &ConflictRule{} // empty ConflictingPairs → exclusive mode
	rackID := uuid.New()
	incoming := OperationSpec{OperationType: "power_control", OperationCode: "power_on"}

	tests := []struct {
		name        string
		activeTasks []*taskdef.Task
		expected    bool
	}{
		{
			name:        "nil active tasks",
			activeTasks: nil,
			expected:    false,
		},
		{
			name:        "empty active tasks",
			activeTasks: []*taskdef.Task{},
			expected:    false,
		},
		{
			name: "one active task",
			activeTasks: []*taskdef.Task{
				makeTask(rackID, taskcommon.TaskTypeFirmwareControl, "upgrade"),
			},
			expected: true,
		},
		{
			name: "multiple active tasks",
			activeTasks: []*taskdef.Task{
				makeTask(rackID, taskcommon.TaskTypePowerControl, "power_off"),
				makeTask(rackID, taskcommon.TaskTypeFirmwareControl, "upgrade"),
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, rule.Conflicts(incoming, tc.activeTasks))
		})
	}
}

func TestConflictRule_Conflicts_PairMode(t *testing.T) {
	rackID := uuid.New()
	rule := &ConflictRule{
		ConflictingPairs: []ConflictEntry{
			{
				A: OperationSpec{OperationType: "power_control", OperationCode: "*"},
				B: OperationSpec{OperationType: "firmware_control", OperationCode: "*"},
			},
			{
				A: OperationSpec{OperationType: "bring_up", OperationCode: "*"},
				B: OperationSpec{OperationType: "bring_up", OperationCode: "*"},
			},
		},
	}

	tests := []struct {
		name        string
		incoming    OperationSpec
		activeTasks []*taskdef.Task
		expected    bool
	}{
		{
			name:        "no active tasks",
			incoming:    OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			activeTasks: []*taskdef.Task{},
			expected:    false,
		},
		{
			name:     "active task matches a pair",
			incoming: OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			activeTasks: []*taskdef.Task{
				makeTask(rackID, taskcommon.TaskTypeFirmwareControl, "upgrade"),
			},
			expected: true,
		},
		{
			name:     "active task does not match any pair",
			incoming: OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
			activeTasks: []*taskdef.Task{
				makeTask(rackID, taskcommon.TaskTypePowerControl, "power_off"),
			},
			expected: false,
		},
		{
			name:     "active task matches second pair",
			incoming: OperationSpec{OperationType: "bring_up", OperationCode: "full"},
			activeTasks: []*taskdef.Task{
				makeTask(rackID, taskcommon.TaskTypeBringUp, "full"),
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, rule.Conflicts(tc.incoming, tc.activeTasks))
		})
	}
}

func TestConflictResolver_HasConflict(t *testing.T) {
	rackA := uuid.New()
	rackB := uuid.New()

	tests := []struct {
		name          string
		setupStore    func(*mockStore)
		rackID        uuid.UUID
		expectedValue bool
		expectedErr   bool
	}{
		{
			name:          "no active tasks on rack",
			setupStore:    func(_ *mockStore) {},
			rackID:        rackA,
			expectedValue: false,
		},
		{
			name: "one active task on rack",
			setupStore: func(s *mockStore) {
				s.activeTasks[rackA] = []*taskdef.Task{
					makeTask(rackA, taskcommon.TaskTypePowerControl, "power_off"),
				}
			},
			rackID:        rackA,
			expectedValue: true,
		},
		{
			name: "multiple active tasks on rack",
			setupStore: func(s *mockStore) {
				s.activeTasks[rackA] = []*taskdef.Task{
					makeTask(rackA, taskcommon.TaskTypePowerControl, "power_off"),
					makeTask(rackA, taskcommon.TaskTypeFirmwareControl, "upgrade"),
				}
			},
			rackID:        rackA,
			expectedValue: true,
		},
		{
			name: "active task on different rack does not affect checked rack",
			setupStore: func(s *mockStore) {
				s.activeTasks[rackA] = []*taskdef.Task{
					makeTask(rackA, taskcommon.TaskTypePowerControl, "power_on"),
				}
			},
			rackID:        rackB,
			expectedValue: false,
		},
		{
			name: "store error is propagated",
			setupStore: func(s *mockStore) {
				s.listActiveErr = errors.New("db connection lost")
			},
			rackID:      rackA,
			expectedErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockStore()
			tc.setupStore(store)
			resolver := NewConflictResolver(store)

			hasConflict, err := resolver.HasConflict(
				context.Background(),
				OperationSpec{OperationType: "power_control", OperationCode: "power_on"},
				tc.rackID,
			)

			if tc.expectedErr {
				require.Error(t, err)
				assert.False(t, hasConflict)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedValue, hasConflict)
			}
		})
	}
}
