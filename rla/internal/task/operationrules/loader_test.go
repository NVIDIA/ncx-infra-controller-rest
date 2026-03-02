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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
)

func TestYAMLRuleLoader_InvalidOperations(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantErrMsg  string
	}{
		{
			name: "invalid power control operation",
			yamlContent: `version: v1
rules:
  - name: "Test Invalid Power Op"
    operation_type: power_control
    operation: invalid_operation_xyz
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
`,
			wantErrMsg: "invalid operation code 'invalid_operation_xyz' for operation type 'power_control'",
		},
		{
			name: "invalid firmware control operation",
			yamlContent: `version: v1
rules:
  - name: "Test Invalid Firmware Op"
    operation_type: firmware_control
    operation: bad_firmware_op
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
`,
			wantErrMsg: "invalid operation code 'bad_firmware_op' for operation type 'firmware_control'",
		},
		{
			name: "power operation for firmware type",
			yamlContent: `version: v1
rules:
  - name: "Test Wrong Type"
    operation_type: firmware_control
    operation: power_on
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
`,
			wantErrMsg: "invalid operation code 'power_on' for operation type 'firmware_control'",
		},
		{
			name: "firmware operation for power type",
			yamlContent: `version: v1
rules:
  - name: "Test Wrong Type"
    operation_type: power_control
    operation: upgrade
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
`,
			wantErrMsg: "invalid operation code 'upgrade' for operation type 'power_control'",
		},
		{
			name: "empty operation name",
			yamlContent: `version: v1
rules:
  - name: "Test Empty Operation"
    operation_type: power_control
    operation: ""
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
`,
			wantErrMsg: "invalid operation code '' for operation type 'power_control'",
		},
		{
			name: "multiple rules with one invalid",
			yamlContent: `version: v1
rules:
  - name: "Valid Rule"
    operation_type: power_control
    operation: power_on
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
  - name: "Invalid Rule"
    operation_type: power_control
    operation: invalid_op
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
`,
			wantErrMsg: "invalid operation code 'invalid_op' for operation type 'power_control'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary YAML file
			tmpDir := t.TempDir()
			yamlPath := filepath.Join(tmpDir, "test-rules.yaml")
			if err := os.WriteFile(yamlPath, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatalf("Failed to create test YAML file: %v", err)
			}

			// Create loader
			loader, err := NewYAMLRuleLoader(yamlPath)
			if err != nil {
				t.Fatalf("Failed to create YAML loader: %v", err)
			}

			// Load rules - should fail
			_, err = loader.Load()
			if err == nil {
				t.Fatal("Expected error but got nil")
			}

			// Check error message
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("Expected error containing %q, got %q", tt.wantErrMsg, err.Error())
			}
		})
	}
}

func TestYAMLRuleLoader_ValidOperations(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantRules   map[common.TaskType][]string // operation type -> list of operation names
	}{
		{
			name: "valid power control operations",
			yamlContent: `version: v1
rules:
  - name: "Power On"
    operation_type: power_control
    operation: power_on
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
  - name: "Power Off"
    operation_type: power_control
    operation: power_off
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
  - name: "Force Restart"
    operation_type: power_control
    operation: force_restart
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
`,
			wantRules: map[common.TaskType][]string{
				common.TaskTypePowerControl: {"power_on", "power_off", "force_restart"},
			},
		},
		{
			name: "valid firmware control operations",
			yamlContent: `version: v1
rules:
  - name: "Upgrade"
    operation_type: firmware_control
    operation: upgrade
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
  - name: "Downgrade"
    operation_type: firmware_control
    operation: downgrade
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
  - name: "Rollback"
    operation_type: firmware_control
    operation: rollback
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
`,
			wantRules: map[common.TaskType][]string{
				common.TaskTypeFirmwareControl: {"upgrade", "downgrade", "rollback"},
			},
		},
		{
			name: "mixed valid operations",
			yamlContent: `version: v1
rules:
  - name: "Power On"
    operation_type: power_control
    operation: power_on
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
  - name: "Power Off"
    operation_type: power_control
    operation: power_off
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
  - name: "Upgrade"
    operation_type: firmware_control
    operation: upgrade
    steps:
      - component_type: compute
        stage: 1
        max_parallel: 1
        timeout: 10m
`,
			wantRules: map[common.TaskType][]string{
				common.TaskTypePowerControl:    {"power_on", "power_off"},
				common.TaskTypeFirmwareControl: {"upgrade"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary YAML file
			tmpDir := t.TempDir()
			yamlPath := filepath.Join(tmpDir, "test-rules.yaml")
			if err := os.WriteFile(yamlPath, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatalf("Failed to create test YAML file: %v", err)
			}

			// Create loader
			loader, err := NewYAMLRuleLoader(yamlPath)
			if err != nil {
				t.Fatalf("Failed to create YAML loader: %v", err)
			}

			// Load rules - should succeed
			rules, err := loader.Load()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify rules were loaded correctly
			for opType, expectedOps := range tt.wantRules {
				typeRules, ok := rules[opType]
				if !ok {
					t.Errorf("Expected rules for operation type %v, but none found", opType)
					continue
				}

				if len(typeRules) != len(expectedOps) {
					t.Errorf("Expected %d rules for %v, got %d", len(expectedOps), opType, len(typeRules))
				}

				for _, opName := range expectedOps {
					if _, exists := typeRules[opName]; !exists {
						t.Errorf("Expected rule for operation %q under type %v, but not found", opName, opType)
					}
				}
			}
		})
	}
}
