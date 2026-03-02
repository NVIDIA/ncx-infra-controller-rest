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
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/workflow"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operationrules"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

// actionExecutionContext holds the context needed for action execution
type actionExecutionContext struct {
	workflowContext workflow.Context
	config          operationrules.ActionConfig
	target          common.Target
	allTargets      map[devicetypes.ComponentType]common.Target
	operationInfo   any
}

// actionExecutor defines the signature for action execution functions
type actionExecutor func(actx actionExecutionContext) error

// actionExecutorRegistry maps action names to their executor functions
var actionExecutorRegistry = map[string]actionExecutor{
	operationrules.ActionSleep:              executeSleepAction,
	operationrules.ActionPowerControl:       executePowerControlAction,
	operationrules.ActionVerifyPowerStatus:  executeVerifyPowerStatusAction,
	operationrules.ActionVerifyReachability: executeVerifyReachabilityAction,
	operationrules.ActionGetPowerStatus:     executeGetPowerStatusAction,
	operationrules.ActionFirmwareControl:    executeFirmwareControlAction,
}

// executeActionList executes a list of actions sequentially
func executeActionList(
	ctx workflow.Context,
	actions []operationrules.ActionConfig,
	target common.Target,
	allTargets map[devicetypes.ComponentType]common.Target,
	operationInfo any,
) error {
	for i, action := range actions {
		if err := executeAction(ctx, action, target, allTargets, operationInfo); err != nil {
			return fmt.Errorf("action %d (%s) failed: %w", i, action.Name, err)
		}
	}
	return nil
}

// executeAction executes a single action using the registry
func executeAction(
	ctx workflow.Context,
	config operationrules.ActionConfig,
	target common.Target,
	allTargets map[devicetypes.ComponentType]common.Target,
	operationInfo any,
) error {
	executor, ok := actionExecutorRegistry[config.Name]
	if !ok {
		return fmt.Errorf("unknown action: %s", config.Name)
	}

	actx := actionExecutionContext{
		workflowContext: ctx,
		config:          config,
		target:          target,
		allTargets:      allTargets,
		operationInfo:   operationInfo,
	}

	return executor(actx)
}

// executeSleepAction handles Sleep action
func executeSleepAction(actx actionExecutionContext) error {
	duration := parseDurationParam(
		actx.config.Parameters[operationrules.ParamDuration],
	)
	log.Debug().
		Dur("duration", duration).
		Msg("Sleeping")
	return workflow.Sleep(actx.workflowContext, duration)
}

// executePowerControlAction handles PowerControl action
func executePowerControlAction(actx actionExecutionContext) error {
	return executeGenericActivity(
		actx.workflowContext,
		"PowerControl",
		actx.target,
		actx.operationInfo,
	)
}

// executeVerifyPowerStatusAction handles VerifyPowerStatus action
func executeVerifyPowerStatusAction(actx actionExecutionContext) error {
	expectedStatus := actx.config.Parameters[operationrules.ParamExpectedStatus].(string)
	return verifyPowerStatus(
		actx.workflowContext,
		actx.target,
		expectedStatus,
		actx.config.Timeout,
		actx.config.PollInterval,
	)
}

// executeVerifyReachabilityAction handles VerifyReachability action
func executeVerifyReachabilityAction(actx actionExecutionContext) error {
	// Extract component types parameter
	var componentTypes []string
	switch v := actx.config.Parameters[operationrules.ParamComponentTypes].(type) {
	case []string:
		componentTypes = v
	case []any:
		componentTypes = make([]string, len(v))
		for i, item := range v {
			componentTypes[i] = item.(string)
		}
	}

	return verifyReachability(
		actx.workflowContext,
		actx.allTargets,
		componentTypes,
		actx.config.Timeout,
		actx.config.PollInterval,
	)
}

// executeGetPowerStatusAction handles GetPowerStatus action
func executeGetPowerStatusAction(actx actionExecutionContext) error {
	return executeGenericActivity(
		actx.workflowContext,
		"GetPowerStatus",
		actx.target,
		nil,
	)
}

// executeFirmwareControlAction handles FirmwareControl action
func executeFirmwareControlAction(actx actionExecutionContext) error {
	return executeGenericActivity(
		actx.workflowContext,
		"FirmwareControl",
		actx.target,
		actx.operationInfo,
	)
}

// executeGenericActivity executes a Temporal activity with the given name
func executeGenericActivity(
	ctx workflow.Context,
	activityName string,
	target common.Target,
	activityInfo any,
) error {
	// Build activity arguments
	var args []any
	args = append(args, target)
	if activityInfo != nil {
		args = append(args, activityInfo)
	}

	// Execute activity
	return workflow.ExecuteActivity(ctx, activityName, args...).Get(ctx, nil)
}

// verifyPowerStatus polls GetPowerStatus until expected status is reached
func verifyPowerStatus(
	ctx workflow.Context,
	target common.Target,
	expectedStatus string,
	timeout time.Duration,
	pollInterval time.Duration,
) error {
	// Convert string to PowerStatus
	var expected operations.PowerStatus
	switch expectedStatus {
	case "on":
		expected = operations.PowerStatusOn
	case "off":
		expected = operations.PowerStatusOff
	default:
		return fmt.Errorf(
			"invalid expected_status '%s', must be 'on' or 'off'",
			expectedStatus,
		)
	}

	log.Debug().
		Str("component_type", devicetypes.ComponentTypeToString(target.Type)).
		Strs("component_ids", target.ComponentIDs).
		Str("expected_status", expectedStatus).
		Dur("timeout", timeout).
		Dur("poll_interval", pollInterval).
		Msg("Starting power status verification")

	deadline := workflow.Now(ctx).Add(timeout)
	attempt := 0

	for {
		attempt++

		// Call GetPowerStatus activity
		var statusMap map[string]operations.PowerStatus
		actErr := workflow.ExecuteActivity(
			ctx,
			"GetPowerStatus",
			target,
		).Get(ctx, &statusMap)

		if actErr == nil {
			// Check if all components have expected status
			allMatch := true
			for componentID, status := range statusMap {
				if status != expected {
					log.Debug().
						Str("component_id", componentID).
						Str("current_status", string(status)).
						Str("expected_status", string(expected)).
						Msg("Component status mismatch")
					allMatch = false
					break
				}
			}

			if allMatch {
				log.Debug().
					Int("attempts", attempt).
					Int("component_count", len(statusMap)).
					Str("expected_status", string(expected)).
					Msg("All components reached expected power status")
				return nil
			}
		} else {
			log.Debug().
				Err(actErr).
				Int("attempt", attempt).
				Msg("GetPowerStatus failed, will retry")
		}

		// Check timeout
		if workflow.Now(ctx).After(deadline) {
			return fmt.Errorf(
				"timeout after %v waiting for power status %s (attempts: %d)",
				timeout,
				expected,
				attempt,
			)
		}

		// Sleep before next poll (durable sleep in workflow)
		workflow.Sleep(ctx, pollInterval)
	}
}

// verifyReachability polls GetPowerStatus for multiple component types
// until all are reachable.
func verifyReachability(
	ctx workflow.Context,
	allTargets map[devicetypes.ComponentType]common.Target,
	componentTypes []string,
	timeout time.Duration,
	pollInterval time.Duration,
) error {
	// Convert string component types to enum
	typesToCheck := make([]devicetypes.ComponentType, 0, len(componentTypes))
	for _, ctStr := range componentTypes {
		ct := devicetypes.ComponentTypeFromString(ctStr)
		if ct == devicetypes.ComponentTypeUnknown {
			return fmt.Errorf("invalid component type: %s", ctStr)
		}
		typesToCheck = append(typesToCheck, ct)
	}

	log.Debug().
		Strs("component_types", componentTypes).
		Dur("timeout", timeout).
		Dur("poll_interval", pollInterval).
		Msg("Starting reachability verification")

	deadline := workflow.Now(ctx).Add(timeout)
	reachable := make(map[devicetypes.ComponentType]bool)

	for {
		// Try to reach each component type
		for _, ct := range typesToCheck {
			// Skip if already verified reachable
			if reachable[ct] {
				continue
			}

			target, ok := allTargets[ct]
			if !ok {
				log.Debug().
					Str("component_type", devicetypes.ComponentTypeToString(ct)).
					Msg("Component type not in target map, skipping")
				reachable[ct] = true
				continue
			}

			// Try to get power status (BMC is reachable if this succeeds)
			var statusMap map[string]operations.PowerStatus
			err := workflow.ExecuteActivity(
				ctx,
				"GetPowerStatus",
				target,
			).Get(ctx, &statusMap)

			if err != nil {
				log.Debug().
					Str("component_type", devicetypes.ComponentTypeToString(ct)).
					Err(err).
					Msg("Component type not yet reachable")
			} else {
				log.Debug().
					Str("component_type", devicetypes.ComponentTypeToString(ct)).
					Msg("Component type is reachable")
				reachable[ct] = true
			}
		}

		// Check if all types are reachable
		allReachable := true
		for _, ct := range typesToCheck {
			if !reachable[ct] {
				allReachable = false
				break
			}
		}

		if allReachable {
			log.Debug().
				Strs("component_types", componentTypes).
				Msg("All component types are reachable")
			return nil
		}

		// Check timeout
		if workflow.Now(ctx).After(deadline) {
			unreachable := []string{}
			for _, ct := range typesToCheck {
				if !reachable[ct] {
					unreachable = append(
						unreachable,
						devicetypes.ComponentTypeToString(ct),
					)
				}
			}
			return fmt.Errorf(
				"timeout after %v waiting for components to become reachable: %v",
				timeout,
				unreachable,
			)
		}

		// Sleep before next poll (durable sleep in workflow)
		workflow.Sleep(ctx, pollInterval)
	}
}
