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
	"encoding/json"
	"testing"
	"time"

	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

func TestNewStageIterator(t *testing.T) {
	t.Run("sequential stages", func(t *testing.T) {
		ruleDef := &RuleDefinition{
			Steps: []SequenceStep{
				{ComponentType: devicetypes.ComponentTypeCompute, Stage: 1, MaxParallel: 1},
				{ComponentType: devicetypes.ComponentTypeNVLSwitch, Stage: 2, MaxParallel: 2},
				{ComponentType: devicetypes.ComponentTypePowerShelf, Stage: 3, MaxParallel: 1},
			},
		}

		iter := NewStageIterator(ruleDef)

		// First stage
		stage1 := iter.Next()
		if stage1 == nil {
			t.Fatal("expected first stage, got nil")
		}
		if stage1.Number != 1 {
			t.Errorf("expected stage number 1, got %d", stage1.Number)
		}
		if len(stage1.Steps) != 1 || stage1.Steps[0].ComponentType != devicetypes.ComponentTypeCompute {
			t.Errorf("stage 1: unexpected steps")
		}

		// Second stage
		stage2 := iter.Next()
		if stage2 == nil {
			t.Fatal("expected second stage, got nil")
		}
		if stage2.Number != 2 {
			t.Errorf("expected stage number 2, got %d", stage2.Number)
		}
		if len(stage2.Steps) != 1 || stage2.Steps[0].ComponentType != devicetypes.ComponentTypeNVLSwitch {
			t.Errorf("stage 2: unexpected steps")
		}

		// Third stage
		stage3 := iter.Next()
		if stage3 == nil {
			t.Fatal("expected third stage, got nil")
		}
		if stage3.Number != 3 {
			t.Errorf("expected stage number 3, got %d", stage3.Number)
		}
		if len(stage3.Steps) != 1 || stage3.Steps[0].ComponentType != devicetypes.ComponentTypePowerShelf {
			t.Errorf("stage 3: unexpected steps")
		}

		// No more stages
		stage4 := iter.Next()
		if stage4 != nil {
			t.Errorf("expected nil after all stages, got stage")
		}
	})

	t.Run("stages with gaps", func(t *testing.T) {
		ruleDef := &RuleDefinition{
			Steps: []SequenceStep{
				{ComponentType: devicetypes.ComponentTypeCompute, Stage: 1, MaxParallel: 1},
				{ComponentType: devicetypes.ComponentTypeNVLSwitch, Stage: 5, MaxParallel: 2},
			},
		}

		iter := NewStageIterator(ruleDef)

		// Should get stage 1, then stage 5 (gap skipped automatically)
		stage1 := iter.Next()
		if stage1 == nil || len(stage1.Steps) != 1 {
			t.Error("expected stage 1")
		}
		if stage1.Number != 1 {
			t.Errorf("expected stage number 1, got %d", stage1.Number)
		}

		stage2 := iter.Next()
		if stage2 == nil || len(stage2.Steps) != 1 {
			t.Error("expected stage 5 (as second iteration)")
		}
		if stage2.Number != 5 {
			t.Errorf("expected stage number 5, got %d", stage2.Number)
		}

		if iter.Next() != nil {
			t.Error("expected nil after all stages")
		}
	})

	t.Run("multiple steps in same stage", func(t *testing.T) {
		ruleDef := &RuleDefinition{
			Steps: []SequenceStep{
				{ComponentType: devicetypes.ComponentTypeCompute, Stage: 1, MaxParallel: 1},
				{ComponentType: devicetypes.ComponentTypeNVLSwitch, Stage: 1, MaxParallel: 2},
			},
		}

		iter := NewStageIterator(ruleDef)
		stage := iter.Next()

		if stage == nil {
			t.Fatal("expected stage, got nil")
		}
		if stage.Number != 1 {
			t.Errorf("expected stage number 1, got %d", stage.Number)
		}
		if len(stage.Steps) != 2 {
			t.Errorf("expected 2 steps in stage 1, got %d", len(stage.Steps))
		}

		if iter.Next() != nil {
			t.Error("expected nil after single stage")
		}
	})

	t.Run("empty rule definition", func(t *testing.T) {
		ruleDef := &RuleDefinition{
			Steps: []SequenceStep{},
		}

		iter := NewStageIterator(ruleDef)
		if iter.Next() != nil {
			t.Error("expected nil for empty rule definition")
		}
	})

	t.Run("nil rule definition", func(t *testing.T) {
		iter := NewStageIterator(nil)
		if iter.Next() != nil {
			t.Error("expected nil for nil rule definition")
		}
	})
}

func TestStageIterator_HasNext(t *testing.T) {
	ruleDef := &RuleDefinition{
		Steps: []SequenceStep{
			{ComponentType: devicetypes.ComponentTypeCompute, Stage: 1, MaxParallel: 1},
			{ComponentType: devicetypes.ComponentTypeNVLSwitch, Stage: 2, MaxParallel: 2},
		},
	}

	iter := NewStageIterator(ruleDef)

	if !iter.HasNext() {
		t.Error("expected HasNext=true at start")
	}

	iter.Next()
	if !iter.HasNext() {
		t.Error("expected HasNext=true after first Next()")
	}

	iter.Next()
	if iter.HasNext() {
		t.Error("expected HasNext=false after all stages consumed")
	}
}

func TestStageIterator_Reset(t *testing.T) {
	ruleDef := &RuleDefinition{
		Steps: []SequenceStep{
			{ComponentType: devicetypes.ComponentTypeCompute, Stage: 1, MaxParallel: 1},
			{ComponentType: devicetypes.ComponentTypeNVLSwitch, Stage: 2, MaxParallel: 2},
		},
	}

	iter := NewStageIterator(ruleDef)

	// Consume all stages
	iter.Next()
	iter.Next()
	if iter.Next() != nil {
		t.Error("expected nil after consuming all stages")
	}

	// Reset and iterate again
	iter.Reset()
	stage1 := iter.Next()
	if stage1 == nil {
		t.Error("expected first stage after reset")
	}

	stage2 := iter.Next()
	if stage2 == nil {
		t.Error("expected second stage after reset")
	}

	if iter.Next() != nil {
		t.Error("expected nil after second full iteration")
	}
}

func TestStageIterator_StandardLoop(t *testing.T) {
	ruleDef := &RuleDefinition{
		Steps: []SequenceStep{
			{ComponentType: devicetypes.ComponentTypeCompute, Stage: 1, MaxParallel: 1},
			{ComponentType: devicetypes.ComponentTypeNVLSwitch, Stage: 2, MaxParallel: 2},
			{ComponentType: devicetypes.ComponentTypePowerShelf, Stage: 3, MaxParallel: 1},
		},
	}

	// Test standard iteration pattern
	iter := NewStageIterator(ruleDef)
	count := 0
	for stage := iter.Next(); stage != nil; stage = iter.Next() {
		count++
		if len(stage.Steps) == 0 {
			t.Error("stage should have steps")
		}
	}

	if count != 3 {
		t.Errorf("expected 3 iterations, got %d", count)
	}
}

func TestSequenceStep_MarshalJSON(t *testing.T) {
	t.Run("with all duration fields", func(t *testing.T) {
		step := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   2,
			DelayAfter:    30 * time.Second,
			Timeout:       10 * time.Minute,
			RetryPolicy: &RetryPolicy{
				MaxAttempts:        3,
				InitialInterval:    5 * time.Second,
				BackoffCoefficient: 2.0,
				MaxInterval:        1 * time.Minute,
			},
		}

		data, err := json.Marshal(step)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		// Unmarshal to verify the format
		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		// Verify durations are strings
		if delayAfter, ok := result["delay_after"].(string); !ok || delayAfter != "30s" {
			t.Errorf("expected delay_after='30s', got %v", result["delay_after"])
		}
		if timeout, ok := result["timeout"].(string); !ok || timeout != "10m0s" {
			t.Errorf("expected timeout='10m0s', got %v", result["timeout"])
		}
	})

	t.Run("with zero durations", func(t *testing.T) {
		step := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   2,
			DelayAfter:    0,
			Timeout:       0,
		}

		data, err := json.Marshal(step)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		// Zero durations are represented as "0s" (not omitted)
		if delayAfter, ok := result["delay_after"].(string); !ok || delayAfter != "0s" {
			t.Errorf("expected delay_after='0s', got %v", result["delay_after"])
		}
		if timeout, ok := result["timeout"].(string); !ok || timeout != "0s" {
			t.Errorf("expected timeout='0s', got %v", result["timeout"])
		}
	})
}

func TestSequenceStep_UnmarshalJSON(t *testing.T) {
	t.Run("valid duration strings", func(t *testing.T) {
		// Use a SequenceStep to marshal first, then unmarshal to test round-trip
		original := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   2,
			DelayAfter:    30 * time.Second,
			Timeout:       10 * time.Minute,
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		// Now unmarshal it back
		var step SequenceStep
		if err := json.Unmarshal(jsonData, &step); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if step.DelayAfter != 30*time.Second {
			t.Errorf("expected delay_after=30s, got %v", step.DelayAfter)
		}
		if step.Timeout != 10*time.Minute {
			t.Errorf("expected timeout=10m, got %v", step.Timeout)
		}
		if step.ComponentType != devicetypes.ComponentTypeCompute {
			t.Errorf("expected component_type=Compute, got %v", step.ComponentType)
		}
		if step.Stage != 1 {
			t.Errorf("expected stage=1, got %d", step.Stage)
		}
		if step.MaxParallel != 2 {
			t.Errorf("expected max_parallel=2, got %d", step.MaxParallel)
		}
	})

	t.Run("missing duration fields", func(t *testing.T) {
		// Create a step without durations
		original := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   2,
			// No DelayAfter or Timeout set (will be 0)
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		// Unmarshal back
		var step SequenceStep
		if err := json.Unmarshal(jsonData, &step); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if step.DelayAfter != 0 {
			t.Errorf("expected delay_after=0, got %v", step.DelayAfter)
		}
		if step.Timeout != 0 {
			t.Errorf("expected timeout=0, got %v", step.Timeout)
		}
	})

	t.Run("invalid delay_after format", func(t *testing.T) {
		// Manually construct JSON with invalid delay_after
		// Use integer value for component_type (ComponentTypeCompute = 1)
		jsonData := []byte(`{
			"component_type": 1,
			"stage": 1,
			"max_parallel": 2,
			"delay_after": "invalid"
		}`)

		var step SequenceStep
		err := json.Unmarshal(jsonData, &step)
		if err == nil {
			t.Error("expected error for invalid delay_after format")
		}
	})

	t.Run("invalid timeout format", func(t *testing.T) {
		// Manually construct JSON with invalid timeout
		// Use integer value for component_type (ComponentTypeCompute = 1)
		jsonData := []byte(`{
			"component_type": 1,
			"stage": 1,
			"max_parallel": 2,
			"timeout": "not-a-duration"
		}`)

		var step SequenceStep
		err := json.Unmarshal(jsonData, &step)
		if err == nil {
			t.Error("expected error for invalid timeout format")
		}
	})
}

func TestSequenceStep_MarshalUnmarshal_RoundTrip(t *testing.T) {
	original := SequenceStep{
		ComponentType: devicetypes.ComponentTypeNVLSwitch,
		Stage:         2,
		MaxParallel:   5,
		DelayAfter:    15 * time.Second,
		Timeout:       20 * time.Minute,
		RetryPolicy: &RetryPolicy{
			MaxAttempts:        3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaxInterval:        1 * time.Minute,
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var decoded SequenceStep
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify all fields match
	if decoded.ComponentType != original.ComponentType {
		t.Errorf("component_type mismatch: got %v, want %v", decoded.ComponentType, original.ComponentType)
	}
	if decoded.Stage != original.Stage {
		t.Errorf("stage mismatch: got %d, want %d", decoded.Stage, original.Stage)
	}
	if decoded.MaxParallel != original.MaxParallel {
		t.Errorf("max_parallel mismatch: got %d, want %d", decoded.MaxParallel, original.MaxParallel)
	}
	if decoded.DelayAfter != original.DelayAfter {
		t.Errorf("delay_after mismatch: got %v, want %v", decoded.DelayAfter, original.DelayAfter)
	}
	if decoded.Timeout != original.Timeout {
		t.Errorf("timeout mismatch: got %v, want %v", decoded.Timeout, original.Timeout)
	}
}

func TestRetryPolicy_MarshalJSON(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		policy := RetryPolicy{
			MaxAttempts:        3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaxInterval:        1 * time.Minute,
		}

		data, err := json.Marshal(policy)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		// Verify durations are strings
		if initialInterval, ok := result["initial_interval"].(string); !ok || initialInterval != "5s" {
			t.Errorf("expected initial_interval='5s', got %v", result["initial_interval"])
		}
		if maxInterval, ok := result["max_interval"].(string); !ok || maxInterval != "1m0s" {
			t.Errorf("expected max_interval='1m0s', got %v", result["max_interval"])
		}
	})

	t.Run("with zero max_interval", func(t *testing.T) {
		policy := RetryPolicy{
			MaxAttempts:        3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaxInterval:        0,
		}

		data, err := json.Marshal(policy)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		// Zero max_interval is represented as "0s" (not omitted)
		if maxInterval, ok := result["max_interval"].(string); !ok || maxInterval != "0s" {
			t.Errorf("expected max_interval='0s', got %v", result["max_interval"])
		}
	})
}

func TestRetryPolicy_UnmarshalJSON(t *testing.T) {
	t.Run("valid retry policy", func(t *testing.T) {
		jsonData := `{
			"max_attempts": 3,
			"initial_interval": "5s",
			"backoff_coefficient": 2.0,
			"max_interval": "1m"
		}`

		var policy RetryPolicy
		if err := json.Unmarshal([]byte(jsonData), &policy); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if policy.MaxAttempts != 3 {
			t.Errorf("expected max_attempts=3, got %d", policy.MaxAttempts)
		}
		if policy.InitialInterval != 5*time.Second {
			t.Errorf("expected initial_interval=5s, got %v", policy.InitialInterval)
		}
		if policy.BackoffCoefficient != 2.0 {
			t.Errorf("expected backoff_coefficient=2.0, got %f", policy.BackoffCoefficient)
		}
		if policy.MaxInterval != 1*time.Minute {
			t.Errorf("expected max_interval=1m, got %v", policy.MaxInterval)
		}
	})

	t.Run("missing max_interval", func(t *testing.T) {
		jsonData := `{
			"max_attempts": 3,
			"initial_interval": "5s",
			"backoff_coefficient": 2.0
		}`

		var policy RetryPolicy
		if err := json.Unmarshal([]byte(jsonData), &policy); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if policy.MaxInterval != 0 {
			t.Errorf("expected max_interval=0, got %v", policy.MaxInterval)
		}
	})

	t.Run("invalid initial_interval", func(t *testing.T) {
		jsonData := `{
			"max_attempts": 3,
			"initial_interval": "invalid",
			"backoff_coefficient": 2.0
		}`

		var policy RetryPolicy
		err := json.Unmarshal([]byte(jsonData), &policy)
		if err == nil {
			t.Error("expected error for invalid initial_interval")
		}
	})

	t.Run("invalid max_interval", func(t *testing.T) {
		jsonData := `{
			"max_attempts": 3,
			"initial_interval": "5s",
			"backoff_coefficient": 2.0,
			"max_interval": "not-a-duration"
		}`

		var policy RetryPolicy
		err := json.Unmarshal([]byte(jsonData), &policy)
		if err == nil {
			t.Error("expected error for invalid max_interval")
		}
	})
}

func TestRetryPolicy_MarshalUnmarshal_RoundTrip(t *testing.T) {
	original := RetryPolicy{
		MaxAttempts:        5,
		InitialInterval:    10 * time.Second,
		BackoffCoefficient: 1.5,
		MaxInterval:        5 * time.Minute,
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var decoded RetryPolicy
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify all fields match
	if decoded.MaxAttempts != original.MaxAttempts {
		t.Errorf("max_attempts mismatch: got %d, want %d", decoded.MaxAttempts, original.MaxAttempts)
	}
	if decoded.InitialInterval != original.InitialInterval {
		t.Errorf("initial_interval mismatch: got %v, want %v", decoded.InitialInterval, original.InitialInterval)
	}
	if decoded.BackoffCoefficient != original.BackoffCoefficient {
		t.Errorf("backoff_coefficient mismatch: got %f, want %f", decoded.BackoffCoefficient, original.BackoffCoefficient)
	}
	if decoded.MaxInterval != original.MaxInterval {
		t.Errorf("max_interval mismatch: got %v, want %v", decoded.MaxInterval, original.MaxInterval)
	}
}

func TestSequenceStep_Validate(t *testing.T) {
	t.Run("valid step", func(t *testing.T) {
		step := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   2,
			DelayAfter:    30 * time.Second,
			Timeout:       10 * time.Minute,
		}

		if err := step.Validate(); err != nil {
			t.Errorf("expected valid step, got error: %v", err)
		}
	})

	t.Run("invalid component type", func(t *testing.T) {
		step := SequenceStep{
			ComponentType: devicetypes.ComponentTypeUnknown,
			Stage:         1,
			MaxParallel:   2,
		}

		err := step.Validate()
		if err == nil {
			t.Error("expected error for unknown component type")
		}
	})

	t.Run("invalid stage number", func(t *testing.T) {
		step := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         0,
			MaxParallel:   2,
		}

		err := step.Validate()
		if err == nil {
			t.Error("expected error for stage < 1")
		}
	})

	t.Run("negative max_parallel", func(t *testing.T) {
		step := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   -1,
		}

		err := step.Validate()
		if err == nil {
			t.Error("expected error for negative max_parallel")
		}
	})

	t.Run("negative delay_after", func(t *testing.T) {
		step := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   2,
			DelayAfter:    -5 * time.Second,
		}

		err := step.Validate()
		if err == nil {
			t.Error("expected error for negative delay_after")
		}
	})

	t.Run("negative timeout", func(t *testing.T) {
		step := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   2,
			Timeout:       -10 * time.Minute,
		}

		err := step.Validate()
		if err == nil {
			t.Error("expected error for negative timeout")
		}
	})

	t.Run("invalid retry policy", func(t *testing.T) {
		step := SequenceStep{
			ComponentType: devicetypes.ComponentTypeCompute,
			Stage:         1,
			MaxParallel:   2,
			RetryPolicy: &RetryPolicy{
				MaxAttempts:        0, // Invalid
				InitialInterval:    5 * time.Second,
				BackoffCoefficient: 2.0,
			},
		}

		err := step.Validate()
		if err == nil {
			t.Error("expected error for invalid retry policy")
		}
	})
}

func TestRetryPolicy_Validate(t *testing.T) {
	t.Run("valid policy", func(t *testing.T) {
		policy := RetryPolicy{
			MaxAttempts:        3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaxInterval:        1 * time.Minute,
		}

		if err := policy.Validate(); err != nil {
			t.Errorf("expected valid policy, got error: %v", err)
		}
	})

	t.Run("invalid max_attempts", func(t *testing.T) {
		policy := RetryPolicy{
			MaxAttempts:        0,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		}

		err := policy.Validate()
		if err == nil {
			t.Error("expected error for max_attempts < 1")
		}
	})

	t.Run("zero initial_interval", func(t *testing.T) {
		policy := RetryPolicy{
			MaxAttempts:        3,
			InitialInterval:    0,
			BackoffCoefficient: 2.0,
		}

		err := policy.Validate()
		if err == nil {
			t.Error("expected error for zero initial_interval")
		}
	})

	t.Run("negative initial_interval", func(t *testing.T) {
		policy := RetryPolicy{
			MaxAttempts:        3,
			InitialInterval:    -5 * time.Second,
			BackoffCoefficient: 2.0,
		}

		err := policy.Validate()
		if err == nil {
			t.Error("expected error for negative initial_interval")
		}
	})

	t.Run("invalid backoff_coefficient", func(t *testing.T) {
		policy := RetryPolicy{
			MaxAttempts:        3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 0.5,
		}

		err := policy.Validate()
		if err == nil {
			t.Error("expected error for backoff_coefficient < 1.0")
		}
	})

	t.Run("negative max_interval", func(t *testing.T) {
		policy := RetryPolicy{
			MaxAttempts:        3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaxInterval:        -1 * time.Minute,
		}

		err := policy.Validate()
		if err == nil {
			t.Error("expected error for negative max_interval")
		}
	})
}

func TestRuleDefinition_Validate(t *testing.T) {
	t.Run("valid rule definition", func(t *testing.T) {
		ruleDef := RuleDefinition{
			Version: "v1",
			Steps: []SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   2,
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   1,
				},
			},
		}

		if err := ruleDef.Validate(); err != nil {
			t.Errorf("expected valid rule definition, got error: %v", err)
		}
	})

	t.Run("empty steps", func(t *testing.T) {
		ruleDef := RuleDefinition{
			Version: "v1",
			Steps:   []SequenceStep{},
		}

		err := ruleDef.Validate()
		if err != nil {
			t.Errorf("empty steps should be valid (used by bring-up/firmware), got error: %v", err) //nolint
		}
	})

	t.Run("duplicate component type in same stage", func(t *testing.T) {
		ruleDef := RuleDefinition{
			Version: "v1",
			Steps: []SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   2,
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   1,
				},
			},
		}

		err := ruleDef.Validate()
		if err == nil {
			t.Error("expected error for duplicate component type in same stage")
		}
	})

	t.Run("same component type in different stages is allowed", func(t *testing.T) {
		ruleDef := RuleDefinition{
			Version: "v1",
			Steps: []SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   2,
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         2,
					MaxParallel:   1,
				},
			},
		}

		if err := ruleDef.Validate(); err != nil {
			t.Errorf("expected valid rule definition (same component in different stages), got error: %v", err)
		}
	})
}
