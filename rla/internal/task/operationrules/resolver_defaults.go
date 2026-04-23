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

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/common/devicetypes"
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
	ingestRule := buildIngestRule()

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
		ruleKey(common.TaskTypeBringUp, SequenceIngest):             ingestRule,
	}
}

// ruleKey generates a lookup key for the hardcoded rule map
func ruleKey(operationType common.TaskType, operation string) string {
	return string(operationType) + ":" + operation
}

// buildPowerOnRule creates the hardcoded default rule for power on operations.
// PowerShelf is excluded — managed out-of-band.
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
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         1,
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
							Name:         ActionVerifyPowerStatus,
							Timeout:      3 * time.Minute,
							PollInterval: 10 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         2,
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
							Name:         ActionVerifyPowerStatus,
							Timeout:      3 * time.Minute,
							PollInterval: 10 * time.Second,
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

// buildPowerOffRule creates the hardcoded default rule for power off operations.
// PowerShelf is excluded — managed out-of-band.
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
							Name:         ActionVerifyPowerStatus,
							Timeout:      3 * time.Minute,
							PollInterval: 10 * time.Second,
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
							Name:         ActionVerifyPowerStatus,
							Timeout:      3 * time.Minute,
							PollInterval: 10 * time.Second,
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

// buildRestartRule creates the hardcoded default rule for graceful restart.
// PowerShelf is excluded — managed out-of-band.
// Each stage explicitly specifies the power operation to avoid inheriting the
// composite "restart" operation from the task context (which would send
// BMC GRACEFUL_RESTART — an atomic off→on — instead of separate off/on).
func buildRestartRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Restart",
		Description:   "Composite rule: graceful power off all components then power on",
		OperationType: common.TaskTypePowerControl,
		OperationCode: SequenceRestart,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				// === Power Off Sequence (Stages 1-2) ===
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
						Parameters: map[string]any{
							ParamOperation: "power_off",
						},
					},
					PostOperation: []ActionConfig{
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      5 * time.Minute,
							PollInterval: 15 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
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
						Parameters: map[string]any{
							ParamOperation: "power_off",
						},
					},
					PostOperation: []ActionConfig{
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      5 * time.Minute,
							PollInterval: 15 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
					},
				},
				// === Power On Sequence (Stages 3-4) ===
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         3,
					MaxParallel:   0,
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "power_on",
						},
					},
					PostOperation: []ActionConfig{
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      3 * time.Minute,
							PollInterval: 10 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         4,
					MaxParallel:   0,
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "power_on",
						},
					},
					PostOperation: []ActionConfig{
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      3 * time.Minute,
							PollInterval: 10 * time.Second,
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

// buildFirmwareUpgradeRule creates the hardcoded default rule for firmware
// operations. PowerShelf is excluded — managed out-of-band.
//
//	Stage 1: Compute firmware update
//	Stage 2: NVLSwitch firmware update
//	Stage 3: NVLSwitch power recycle (off → sleep → on → verify)
//	Stage 4: Compute power recycle (off → sleep → on → verify)
func buildFirmwareUpgradeRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Firmware Upgrade",
		Description:   "Fallback rule when no other rule is available",
		OperationType: common.TaskTypeFirmwareControl,
		OperationCode: SequenceUpgrade,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				// === Stage 1: Compute firmware update ===
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   0,
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
				// === Stage 2: NVLSwitch firmware update ===
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         2,
					MaxParallel:   0,
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
				// === Stage 3: NVLSwitch power recycle ===
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         3,
					MaxParallel:   0,
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					PreOperation: []ActionConfig{
						{
							Name: ActionPowerControl,
							Parameters: map[string]any{
								ParamOperation: "force_power_off",
							},
						},
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      5 * time.Minute,
							PollInterval: 15 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 10 * time.Second,
							},
						},
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "power_on",
						},
					},
					PostOperation: []ActionConfig{
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      5 * time.Minute,
							PollInterval: 15 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "on",
							},
						},
					},
				},
				// === Stage 4: Compute power recycle ===
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         4,
					MaxParallel:   0,
					Timeout:       10 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					PreOperation: []ActionConfig{
						{
							Name: ActionPowerControl,
							Parameters: map[string]any{
								ParamOperation: "force_power_off",
							},
						},
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      5 * time.Minute,
							PollInterval: 15 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: 10 * time.Second,
							},
						},
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "power_on",
						},
					},
					PostOperation: []ActionConfig{
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      5 * time.Minute,
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

// buildForcePowerOnRule creates the hardcoded default rule for
// forced power on operations (no verification).
// PowerShelf is excluded — managed out-of-band.
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
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         1,
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
					Stage:         2,
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
				// === Final Verification Stage (Stage 3) ===
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         3,
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
					Stage:         3, // Parallel with NVLSwitch
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
// forced power off operations (no verification, no settle time).
// PowerShelf is excluded — managed out-of-band.
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
				// === Final Verification Stage (Stage 3) ===
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         3,
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
					Stage:         3, // Parallel with NVLSwitch
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

// buildBringUpRule creates the hardcoded default rule for rack bring-up.
// PowerShelf is excluded — managed out-of-band.
//
// Stage 1: Compute    — power on (bring-up gate), verify power on
// Stage 2: Compute    — firmware check vs desired, trigger update + poll if needed
// Stage 3: NVLSwitch  — power on (bring-up), verify power on
// Stage 4: NVLSwitch  — verify firmware consistency, trigger update + poll if needed
// Stage 5: NVLSwitch  — restart
// Stage 6: Compute    — restart (GPU restart after switch firmware update)
func buildBringUpRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Bring-Up",
		Description:   "Full bring-up: power on, firmware update, restart",
		OperationType: common.TaskTypeBringUp,
		OperationCode: SequenceBringUp,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				// === Stage 1: Compute — power on, verify ===
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   0,
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    10 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "power_on",
						},
					},
					PostOperation: []ActionConfig{
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
				// === Stage 2: Compute — firmware update (auto-resolve desired) ===
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         2,
					MaxParallel:   0,
					Timeout:       60 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    30 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionFirmwareControl,
						Parameters: map[string]any{
							ParamPollInterval: "2m",
							ParamPollTimeout:  "45m",
						},
					},
				},
				// === Stage 3: NVLSwitch — power on, verify ===
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         3,
					MaxParallel:   0,
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    10 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "power_on",
						},
					},
					PostOperation: []ActionConfig{
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
				// === Stage 4: NVLSwitch — verify consistency + firmware update ===
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         4,
					MaxParallel:   0,
					Timeout:       60 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    30 * time.Second,
						BackoffCoefficient: 2.0,
					},
					PreOperation: []ActionConfig{
						{
							Name: ActionVerifyFirmwareConsistency,
						},
					},
					MainOperation: ActionConfig{
						Name: ActionFirmwareControl,
						Parameters: map[string]any{
							ParamPollInterval: "2m",
							ParamPollTimeout:  "45m",
						},
					},
				},
				// === Stage 5: NVLSwitch — restart ===
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         5,
					MaxParallel:   0,
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    10 * time.Second,
						BackoffCoefficient: 2.0,
					},
					PreOperation: []ActionConfig{
						{
							Name: ActionPowerControl,
							Parameters: map[string]any{
								ParamOperation: "power_off",
							},
						},
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      5 * time.Minute,
							PollInterval: 15 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: "30s",
							},
						},
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "power_on",
						},
					},
					PostOperation: []ActionConfig{
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
				// === Stage 6: Compute — restart (GPU restart) ===
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         6,
					MaxParallel:   0,
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    10 * time.Second,
						BackoffCoefficient: 2.0,
					},
					PreOperation: []ActionConfig{
						{
							Name: ActionPowerControl,
							Parameters: map[string]any{
								ParamOperation: "power_off",
							},
						},
						{
							Name:         ActionVerifyPowerStatus,
							Timeout:      5 * time.Minute,
							PollInterval: 15 * time.Second,
							Parameters: map[string]any{
								ParamExpectedStatus: "off",
							},
						},
						{
							Name: ActionSleep,
							Parameters: map[string]any{
								ParamDuration: "30s",
							},
						},
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "power_on",
						},
					},
					PostOperation: []ActionConfig{
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

// buildIngestRule creates the default rule for ingestion-only operations.
// PowerShelf is excluded — managed out-of-band.
// This rule registers expected components with their respective component
// manager services without performing power or firmware operations. All component types
// are ingested in parallel within a single stage.
func buildIngestRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Ingestion",
		Description:   "Ingestion-only: register components with component manager services",
		OperationType: common.TaskTypeBringUp,
		OperationCode: SequenceIngest,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         1,
					MaxParallel:   0,
					Timeout:       10 * time.Minute,
					MainOperation: ActionConfig{
						Name: ActionInjectExpectation,
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         1, // Parallel with Compute
					MaxParallel:   0,
					Timeout:       10 * time.Minute,
					MainOperation: ActionConfig{
						Name: ActionInjectExpectation,
					},
				},
			},
		},
	}
}

// buildForceRestartRule creates the hardcoded default rule for forced restart
// operations. PowerShelf is excluded — managed out-of-band.
// Skips per-stage verification for speed but verifies the "off"
// state before proceeding to power on, ensuring a real power cycle occurs.
func buildForceRestartRule() *OperationRule {
	return &OperationRule{
		Name:          "Hardcoded Default Force Restart",
		Description:   "Forced restart: power off, verify off, then power on",
		OperationType: common.TaskTypePowerControl,
		OperationCode: SequenceForceRestart,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps: []SequenceStep{
				// === Power Off Sequence (Stages 1-2) ===
				// Explicit force_power_off to avoid sending BMC FORCE_RESTART
				// (which is an atomic off→on cycle, not just off).
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
						Parameters: map[string]any{
							ParamOperation: "force_power_off",
						},
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
						Parameters: map[string]any{
							ParamOperation: "force_power_off",
						},
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
				// === Verify Off Stage (Stage 3) ===
				// Confirm all components are actually off before powering
				// back on. Without this, a silent power-off failure would
				// result in a "successful restart" that never power-cycled.
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         3,
					MaxParallel:   0,
					Timeout:       6 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      5 * time.Minute,
						PollInterval: 15 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "off",
						},
					},
				},
				{
					ComponentType: devicetypes.ComponentTypeCompute,
					Stage:         3, // Parallel with NVLSwitch
					MaxParallel:   0,
					Timeout:       6 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        2,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 1.5,
					},
					MainOperation: ActionConfig{
						Name:         ActionVerifyPowerStatus,
						Timeout:      5 * time.Minute,
						PollInterval: 15 * time.Second,
						Parameters: map[string]any{
							ParamExpectedStatus: "off",
						},
					},
				},
				// === Power On Sequence (Stages 4-5) ===
				// Explicit force_power_on to match the force semantics.
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         4,
					MaxParallel:   0,
					Timeout:       15 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    5 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "force_power_on",
						},
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
					Stage:         5,
					MaxParallel:   0,
					Timeout:       20 * time.Minute,
					RetryPolicy: &RetryPolicy{
						MaxAttempts:        3,
						InitialInterval:    1 * time.Second,
						BackoffCoefficient: 2.0,
					},
					MainOperation: ActionConfig{
						Name: ActionPowerControl,
						Parameters: map[string]any{
							ParamOperation: "force_power_on",
						},
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
				// === Final Verification Stage (Stage 6) ===
				{
					ComponentType: devicetypes.ComponentTypeNVLSwitch,
					Stage:         6,
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
					Stage:         6, // Parallel with NVLSwitch
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
