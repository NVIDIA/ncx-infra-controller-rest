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
	"time"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

// hardcodedRuleMap contains pre-built default rules, initialized once at startup
var hardcodedRuleMap map[string]*OperationRule

func init() {
	// Build all hardcoded rules once at startup
	powerOnRule := buildPowerOnRule()
	forcePowerOnRule := buildForcePowerOnRule()
	powerOffRule := buildPowerOffRule()
	forcePowerOffRule := buildForcePowerOffRule()
	restartRule := buildRestartRule()
	forceRestartRule := buildForceRestartRule()
	firmwareUpgradeRule := buildFirmwareUpgradeRule()
	bringUpRule := buildBringUpRule()

	// Populate lookup map
	hardcodedRuleMap = map[string]*OperationRule{
		ruleKey(common.TaskTypePowerControl, SequencePowerOn):       powerOnRule,
		ruleKey(common.TaskTypePowerControl, SequenceForcePowerOn):  forcePowerOnRule,
		ruleKey(common.TaskTypePowerControl, SequencePowerOff):      powerOffRule,
		ruleKey(common.TaskTypePowerControl, SequenceForcePowerOff): forcePowerOffRule,
		ruleKey(common.TaskTypePowerControl, SequenceRestart):       restartRule,
		ruleKey(common.TaskTypePowerControl, SequenceForceRestart):  forceRestartRule,
		ruleKey(common.TaskTypeFirmwareControl, SequenceUpgrade):    firmwareUpgradeRule,
		ruleKey(common.TaskTypeFirmwareControl, SequenceDowngrade):  firmwareUpgradeRule, // Same rule
		ruleKey(common.TaskTypeFirmwareControl, SequenceRollback):   firmwareUpgradeRule, // Same rule
		ruleKey(common.TaskTypeBringUp, SequenceBringUp):            bringUpRule,
	}
}

// ruleKey generates a lookup key for the hardcoded rule map
func ruleKey(operationType common.TaskType, operation string) string {
	return string(operationType) + ":" + operation
}

// buildPowerOnRule creates the hardcoded default rule for power on operations
func buildPowerOnRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Power On",
		Description:   "Fallback rule when no other rule is available",
		OperationType: common.TaskTypePowerControl,
		OperationCode: SequencePowerOn,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         1,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
						{
							// Wait for downstream components to become reachable
							Name:         ActionVerifyReachability,
							Timeout:      3 * time.Minute,
							PollInterval: 10 * time.Second,
							Parameters: map[string]any{
								ParamComponentTypes: []string{
									"compute",
									"nvlswitch",
								},
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         3,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
				},
			},
		},
	}
}

// buildPowerOffRule creates the hardcoded default rule for power off operations
func buildPowerOffRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Power Off",
		Description:   "Fallback rule when no other rule is available",
		OperationType: common.TaskTypePowerControl,
		OperationCode: SequencePowerOff,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         3,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					PreOperation: []ActionConfig{
						{
							// Settle time: wait for downstream components to
							// fully shut down
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 30 * time.Second,
							},
						},
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
					},
				},
			},
		},
	}
}

// buildRestartRule creates the hardcoded default rule for restart operations
func buildRestartRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Restart",
		Description:   "Composite rule: power off all components then power on",
		OperationType: common.TaskTypePowerControl,
		OperationCode: SequenceRestart,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				// === Power Off Sequence (Stages 1-3) ===
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         3,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
					},
				},
				// === Power On Sequence (Stages 4-6) ===
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         4,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
						{
							// Wait for downstream components to become
							// reachable
							Name:         ActionVerifyReachability,
							Timeout:      3 * time.Minute,
							PollInterval: 10 * time.Second,
							Parameters: map[string]any{
								ParamComponentTypes: []string{
									"compute",
									"nvlswitch",
								},
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         5,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         6,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Verify power status after operation
							Name:         ActionVerifyPowerStatus,
							Timeout:      15 * time.Second,
							PollInterval: 5 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
				},
			},
		},
	}
}

// buildFirmwareUpgradeRule creates the hardcoded default rule for firmware operations
func buildFirmwareUpgradeRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Firmware Upgrade",
		Description:   "Fallback rule when no other rule is available",
		OperationType: common.TaskTypeFirmwareControl,
		OperationCode: SequenceUpgrade,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       30 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    30 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name: ActionFirmwareControl,
						Parameters: map[string]any{
							ParamPollInterval: "2m",
							ParamPollTimeout:  "30m",
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         1, // Parallel with compute
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       30 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    30 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name: ActionFirmwareControl,
						Parameters: map[string]any{
							ParamPollInterval: "2m",
							ParamPollTimeout:  "30m",
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         1, // Parallel with others
					MaxParallel:   0, // All components together (legacy behavior)
					Timeout:       30 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    30 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name: ActionFirmwareControl,
						Parameters: map[string]any{
							ParamPollInterval: "2m",
							ParamPollTimeout:  "30m",
						},
					},
				},
			},
		},
	}
}

// buildForcePowerOnRule creates the hardcoded default rule for
// forced power on operations (no verification)
func buildForcePowerOnRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Force Power On",
		Description:   "Fallback rule for forced power on (no verification)",
		OperationType: common.TaskTypePowerControl,
		OperationCode: SequenceForcePowerOn,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         1,
					MaxParallel:   0,
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   0,
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         3,
					MaxParallel:   0,
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Brief settle time before final verification
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 10 * time.Second,
							},
						},
					},
				},
				// === Final Verification Stage (Stage 4) ===
				// Verify all components in parallel
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         4,
					MaxParallel:   0,
					Timeout:       2 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      1 * time.Minute,
						PollInterval: 5 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "on",
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         4, // Parallel with PowerShelf
					MaxParallel:   0,
					Timeout:       2 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      1 * time.Minute,
						PollInterval: 5 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "on",
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         4, // Parallel with others
					MaxParallel:   0,
					Timeout:       2 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      1 * time.Minute,
						PollInterval: 5 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "on",
						},
					},
				},
			},
		},
	}
}

// buildForcePowerOffRule creates the hardcoded default rule for
// forced power off operations (no verification, no settle time)
func buildForcePowerOffRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Force Power Off",
		Description:   "Fallback rule for forced power off (no verification)",
		OperationType: common.TaskTypePowerControl,
		OperationCode: SequenceForcePowerOff,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   0,
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 10 * time.Second,
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   0,
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 5 * time.Second,
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         3,
					MaxParallel:   0,
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Brief settle time before final verification
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 10 * time.Second,
							},
						},
					},
				},
				// === Final Verification Stage (Stage 4) ===
				// Verify all components in parallel
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         4,
					MaxParallel:   0,
					Timeout:       2 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      1 * time.Minute,
						PollInterval: 5 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "off",
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         4, // Parallel with PowerShelf
					MaxParallel:   0,
					Timeout:       2 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      1 * time.Minute,
						PollInterval: 5 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "off",
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         4, // Parallel with others
					MaxParallel:   0,
					Timeout:       2 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      1 * time.Minute,
						PollInterval: 5 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "off",
						},
					},
				},
			},
		},
	}
}

// buildBringUpRule creates the hardcoded default rule for
// rack bring-up.
//
// Stage 1: PowerShelf — wait PMC ready, turn on PSUs, verify
// Stage 2: Compute    — open gate, wait bring-up, verify,
//
//	reboot, verify again
func buildBringUpRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Bring-Up",
		Description:   "Fallback rule for rack bring-up",
		OperationType: common.TaskTypeBringUp,
		OperationCode: SequenceBringUp,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				// === Stage 1: PowerShelf ===
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         1,
					MaxParallel:   0,
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    10 * time.Second,
						BackoffCoefficient: 2.0,
					},
					PreOperation: []ActionConfig{
						{
							Name:         ActionWaitPMCReady,
							Timeout:      10 * time.Minute,
							PollInterval: 30 * time.Second,
						},
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      5 * time.Minute,
							PollInterval: 10 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
				},
				// === Stage 2: Compute ===
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         2,
					MaxParallel:   0,
					Timeout:       30 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    10 * time.Second,
						BackoffCoefficient: 2.0,
					},
					PreOperation: []ActionConfig{
						{
							Name: ActionAllowBringUp,
						},
						{
							Name:         ActionWaitBringUp,
							Timeout:      15 * time.Minute,
							PollInterval: 30 * time.Second,
						},
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      10 * time.Minute,
							PollInterval: 15 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "force_restart",
						},
					},
					PostOperation: []ActionConfig{
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 30 * time.Second,
							},
						},
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      10 * time.Minute,
							PollInterval: 15 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
				},
			},
		},
	}
}

// buildForceRestartRule creates the hardcoded default rule for
// forced restart operations (no verification)
func buildForceRestartRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Force Restart",
		Description:   "Forced restart: power off then on (no verification)",
		OperationType: common.TaskTypePowerControl,
		OperationCode: SequenceForceRestart,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				// === Power Off Sequence (Stages 1-3) ===
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   0,
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 10 * time.Second,
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   0,
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 5 * time.Second,
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         3,
					MaxParallel:   0,
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Brief pause between off and on
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 5 * time.Second,
							},
						},
					},
				},
				// === Power On Sequence (Stages 4-6) ===
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         4,
					MaxParallel:   0,
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 30 * time.Second,
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         5,
					MaxParallel:   0,
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 15 * time.Second,
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         6,
					MaxParallel:   0,
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
					},
					PostOperation: []ActionConfig{
						{
							// Brief settle time before final verification
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 10 * time.Second,
							},
						},
					},
				},
				// === Final Verification Stage (Stage 7) ===
				// Verify all components in parallel
				{
					ComponentType: devicetypes.ComponentTypePowerShelf,
					Stage:         7,
					MaxParallel:   0,
					Timeout:       2 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      1 * time.Minute,
						PollInterval: 5 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "on",
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         7, // Parallel with PowerShelf
					MaxParallel:   0,
					Timeout:       2 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      1 * time.Minute,
						PollInterval: 5 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "on",
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         7, // Parallel with others
					MaxParallel:   0,
					Timeout:       2 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      1 * time.Minute,
						PollInterval: 5 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "on",
						},
					},
				},
			},
		},
	}
}
