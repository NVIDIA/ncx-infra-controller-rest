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
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/activity" //nolint
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"   //nolint
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

var bringUpActivityOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 20 * time.Minute,
	RetryPolicy: &temporal.RetryPolicy{
		MaximumAttempts:    3,
		InitialInterval:    5 * time.Second,
		MaximumInterval:    1 * time.Minute,
		BackoffCoefficient: 2,
	},
}

const (
	bringUpPMCPollInterval     = 30 * time.Second
	bringUpPMCPollTimeout      = 10 * time.Minute
	bringUpPSUPollInterval     = 10 * time.Second
	bringUpPSUPollTimeout      = 5 * time.Minute
	bringUpStatePollInterval   = 30 * time.Second
	bringUpStatePollTimeout    = 15 * time.Minute
	bringUpComputePollInterval = 15 * time.Second
	bringUpComputePollTimeout  = 10 * time.Minute
)

// BringUp orchestrates the rack bring-up sequence:
//
//	Phase A: PowerShelf — wait PMC ready, turn on PSUs
//	Phase B: NVSwitch   — placeholder
//	Phase C: Compute    — open gate, poll bring-up, reboot
func BringUp(
	ctx workflow.Context,
	reqInfo task.ExecutionInfo,
	info *operations.BringUpTaskInfo,
) error {
	if reqInfo.Rack == nil ||
		len(reqInfo.Rack.Components) == 0 {
		return fmt.Errorf("no components provided")
	}

	ctx = workflow.WithActivityOptions(
		ctx, bringUpActivityOptions,
	)

	if err := updateRunningTaskStatus(
		ctx, reqInfo.TaskID,
	); err != nil {
		return err
	}

	typeToTargets := buildTargets(&reqInfo)

	// --- Phase A: PowerShelf ---
	if err := bringUpPhasePowerShelf(
		ctx, typeToTargets,
	); err != nil {
		return updateFinishedTaskStatus(
			ctx, reqInfo.TaskID, err,
		)
	}

	// --- Phase B: NVSwitch (placeholder) ---
	log.Info().Msg(
		"BringUp Phase B: NVSwitch — placeholder, skipping",
	)

	// --- Phase C: Compute ---
	if err := bringUpPhaseCompute(
		ctx, typeToTargets,
	); err != nil {
		return updateFinishedTaskStatus(
			ctx, reqInfo.TaskID, err,
		)
	}

	return updateFinishedTaskStatus(
		ctx, reqInfo.TaskID, nil,
	)
}

// bringUpPhasePowerShelf handles:
//  1. Wait for all PMC ready via PSM GetPowershelves
//  2. Turn on all PSUs
//  3. Validate PSU states
func bringUpPhasePowerShelf(
	ctx workflow.Context,
	typeToTargets map[devicetypes.ComponentType]common.Target,
) error {
	target, ok := typeToTargets[devicetypes.ComponentTypePowerShelf]
	if !ok || len(target.ComponentIDs) == 0 {
		log.Info().Msg(
			"BringUp Phase A: no PowerShelf components, skipping",
		)
		return nil
	}

	log.Info().
		Int("count", len(target.ComponentIDs)).
		Msg("BringUp Phase A: PowerShelf started")

	// Step 1: Wait for all PMC ready (poll GetPowerStatus)
	log.Info().Msg("Waiting for all PowerShelf PMCs to be ready")
	if err := waitForPowerShelfPMCReady(
		ctx, target,
	); err != nil {
		return fmt.Errorf(
			"PowerShelf PMC readiness check failed: %w", err,
		)
	}

	// Step 2: Turn on all PSUs
	log.Info().Msg("Turning on all PowerShelf PSUs")
	powerOnInfo := operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationPowerOn,
	}
	err := workflow.ExecuteActivity(
		ctx, "PowerControl", target, powerOnInfo,
	).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf(
			"PowerShelf PSU power on failed: %w", err,
		)
	}

	// Step 3: Validate PSU states
	log.Info().Msg("Validating PowerShelf PSU states")
	if err := pollPowerStatus(
		ctx,
		target,
		operations.PowerStatusOn,
		bringUpPSUPollInterval,
		bringUpPSUPollTimeout,
	); err != nil {
		return fmt.Errorf(
			"PowerShelf PSU validation failed: %w", err,
		)
	}

	log.Info().Msg("BringUp Phase A: PowerShelf completed")
	return nil
}

// waitForPowerShelfPMCReady polls GetPowerStatus until the PSM
// responds successfully for all powershelf components, meaning
// all PMCs are reachable.
func waitForPowerShelfPMCReady(
	ctx workflow.Context,
	target common.Target,
) error {
	deadline := workflow.Now(ctx).Add(bringUpPMCPollTimeout)

	for {
		var statusMap map[string]operations.PowerStatus
		err := workflow.ExecuteActivity(
			ctx, "GetPowerStatus", target,
		).Get(ctx, &statusMap)
		if err == nil && len(statusMap) >= len(target.ComponentIDs) {
			log.Info().
				Int("count", len(statusMap)).
				Msg("All PowerShelf PMCs are reachable")
			return nil
		}

		if workflow.Now(ctx).After(deadline) {
			if err != nil {
				return fmt.Errorf(
					"timed out waiting for PMC ready: %w", err,
				)
			}
			return fmt.Errorf(
				"timed out: expected %d PMCs, got %d",
				len(target.ComponentIDs), len(statusMap),
			)
		}

		log.Debug().
			Err(err).
			Msg("PMCs not all ready, retrying")
		_ = workflow.Sleep(ctx, bringUpPMCPollInterval)
	}
}

// bringUpPhaseCompute handles:
//  1. AllowBringUpAndPowerOn for each compute tray
//  2. Poll GetBringUpState until all brought up
//  3. Query and check compute status (log only)
//  4. Reboot all compute
//  5. Query and check compute status again (log only)
func bringUpPhaseCompute(
	ctx workflow.Context,
	typeToTargets map[devicetypes.ComponentType]common.Target,
) error {
	target, ok := typeToTargets[devicetypes.ComponentTypeCompute]
	if !ok || len(target.ComponentIDs) == 0 {
		log.Info().Msg(
			"BringUp Phase C: no Compute components, skipping",
		)
		return nil
	}

	log.Info().
		Int("count", len(target.ComponentIDs)).
		Msg("BringUp Phase C: Compute started")

	// Step 1: AllowBringUpAndPowerOn
	log.Info().Msg("Sending AllowBringUpAndPowerOn for compute")
	err := workflow.ExecuteActivity(
		ctx, "AllowBringUpAndPowerOn", target,
	).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf(
			"AllowBringUpAndPowerOn failed: %w", err,
		)
	}

	// Step 2: Poll bring-up state until all brought up
	log.Info().Msg("Waiting for compute bring-up to complete")
	if err := pollBringUpState(ctx, target); err != nil {
		return fmt.Errorf(
			"compute bring-up polling failed: %w", err,
		)
	}

	// Step 3: Query compute status (power + firmware)
	logComputeStatus(ctx, target, "pre-reboot")

	// Step 4: Wait for all compute ready, then reboot
	log.Info().Msg("Waiting for all compute to be powered on")
	if err := pollPowerStatus(
		ctx,
		target,
		operations.PowerStatusOn,
		bringUpComputePollInterval,
		bringUpComputePollTimeout,
	); err != nil {
		return fmt.Errorf(
			"compute power on wait failed: %w", err,
		)
	}

	log.Info().Msg("Triggering reboot for all compute")
	rebootInfo := operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationForceRestart,
	}
	err = workflow.ExecuteActivity(
		ctx, "PowerControl", target, rebootInfo,
	).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("compute reboot failed: %w", err)
	}

	// Brief pause after reboot
	_ = workflow.Sleep(ctx, 30*time.Second)

	// Step 5: Query compute status again
	logComputeStatus(ctx, target, "post-reboot")

	log.Info().Msg("BringUp Phase C: Compute completed")
	return nil
}

// pollBringUpState polls GetBringUpState until all
// components reach MachineBringUpStateMachineCreated.
func pollBringUpState(
	ctx workflow.Context,
	target common.Target,
) error {
	deadline := workflow.Now(ctx).Add(bringUpStatePollTimeout)

	for {
		if workflow.Now(ctx).After(deadline) {
			return fmt.Errorf(
				"timed out waiting for compute bring-up "+
					"(timeout %v)", bringUpStatePollTimeout,
			)
		}

		_ = workflow.Sleep(ctx, bringUpStatePollInterval)

		var result activity.GetBringUpStateResult
		err := workflow.ExecuteActivity(
			ctx, "GetBringUpState", target,
		).Get(ctx, &result)
		if err != nil {
			log.Warn().Err(err).Msg(
				"Failed to get bring-up state, will retry",
			)
			continue
		}

		allReady := true
		for componentID, state := range result.States {
			if !state.IsBroughtUp() {
				allReady = false
				log.Debug().
					Str("component_id", componentID).
					Str("state", state.String()).
					Msg("Compute not yet brought up")
			}
		}

		if allReady {
			log.Info().
				Int("count", len(result.States)).
				Msg("All compute components brought up")
			return nil
		}
	}
}

// pollPowerStatus polls GetPowerStatus until all components
// reach the expected status.
func pollPowerStatus(
	ctx workflow.Context,
	target common.Target,
	expected operations.PowerStatus,
	pollInterval time.Duration,
	pollTimeout time.Duration,
) error {
	deadline := workflow.Now(ctx).Add(pollTimeout)

	for {
		if workflow.Now(ctx).After(deadline) {
			return fmt.Errorf(
				"timed out waiting for power status %s "+
					"(timeout %v)", expected, pollTimeout,
			)
		}

		_ = workflow.Sleep(ctx, pollInterval)

		var statusMap map[string]operations.PowerStatus
		err := workflow.ExecuteActivity(
			ctx, "GetPowerStatus", target,
		).Get(ctx, &statusMap)
		if err != nil {
			log.Warn().Err(err).Msg(
				"Failed to get power status, will retry",
			)
			continue
		}

		allMatch := true
		for componentID, actual := range statusMap {
			if actual != expected {
				allMatch = false
				log.Debug().
					Str("component_id", componentID).
					Str("expected", string(expected)).
					Str("actual", string(actual)).
					Msg("Power status mismatch")
			}
		}

		if allMatch && len(statusMap) >= len(target.ComponentIDs) {
			log.Info().
				Str("expected", string(expected)).
				Int("count", len(statusMap)).
				Msg("All components reached expected power status")
			return nil
		}
	}
}

// logComputeStatus queries and logs compute power status.
// This is informational only and does not block the workflow.
func logComputeStatus(
	ctx workflow.Context,
	target common.Target,
	phase string,
) {
	var statusMap map[string]operations.PowerStatus
	err := workflow.ExecuteActivity(
		ctx, "GetPowerStatus", target,
	).Get(ctx, &statusMap)
	if err != nil {
		log.Warn().Err(err).
			Str("phase", phase).
			Msg("Failed to query compute power status")
		return
	}

	onCount := 0
	offCount := 0
	for _, status := range statusMap {
		switch status {
		case operations.PowerStatusOn:
			onCount++
		case operations.PowerStatusOff:
			offCount++
		}
	}

	log.Info().
		Str("phase", phase).
		Int("total", len(statusMap)).
		Int("on", onCount).
		Int("off", offCount).
		Msg("Compute status check")
}
