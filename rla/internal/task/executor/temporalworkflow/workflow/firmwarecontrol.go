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

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/alert"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/activity"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

const (
	// defaultFirmwareUpdatePollInterval is the default interval between status polls.
	defaultFirmwareUpdatePollInterval = 2 * time.Minute
	// defaultFirmwareUpdatePollTimeout is the default max duration to poll.
	defaultFirmwareUpdatePollTimeout = 30 * time.Minute
)

var (
	// firmwareUpdateSequence defines the order for firmware updates:
	// PowerShelf → NVLSwitch → Compute
	firmwareUpdateSequence = []devicetypes.ComponentType{
		devicetypes.ComponentTypePowerShelf,
		devicetypes.ComponentTypeNVLSwitch,
		devicetypes.ComponentTypeCompute,
	}

	// firmwareControlActivityOptions for quick activities (start update, check status)
	firmwareControlActivityOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			MaximumInterval:    1 * time.Minute,
			BackoffCoefficient: 2,
		},
	}
)

// FirmwareControl orchestrates firmware updates across component types.
// Sequence:
//  1. Check prerequisites (TODO: components online, firmware versions valid, etc.)
//  2. PowerShelf → update & poll
//  3. NVLSwitch → update & poll
//  4. Compute → update & poll
//  5. Power recycle compute (reuse PowerControl activity)
func FirmwareControl(
	ctx workflow.Context,
	reqInfo task.ExecutionInfo,
	info *operations.FirmwareControlTaskInfo,
) error {
	if reqInfo.Rack == nil || len(reqInfo.Rack.Components) == 0 {
		return fmt.Errorf("no components provided")
	}

	if err := info.Validate(); err != nil {
		return fmt.Errorf("invalid firmware control info: %w", err)
	}

	ctx = workflow.WithActivityOptions(ctx, firmwareControlActivityOptions)
	taskID := reqInfo.TaskID.String()

	if err := updateRunningTaskStatus(ctx, reqInfo.TaskID); err != nil {
		return err
	}

	// Step 1: Check prerequisites
	if err := checkFirmwareUpdatePrerequisites(ctx, &reqInfo); err != nil {
		sendAlert(alert.Alert{
			Severity:  alert.SeverityCritical,
			Message:   fmt.Sprintf("Firmware update prerequisites check failed: %v", err),
			Component: "all",
			Operation: "FirmwareUpdate",
			TaskID:    taskID,
		})
		return updateFinishedTaskStatus(ctx, reqInfo.TaskID, err)
	}

	// Build targets by component type
	typeToTargets := buildTargets(&reqInfo)

	// Step 2-4: Execute firmware update for each type in sequence
	for _, componentType := range firmwareUpdateSequence {
		target, ok := typeToTargets[componentType]
		if !ok || len(target.ComponentIDs) == 0 {
			log.Debug().
				Str("type", devicetypes.ComponentTypeToString(componentType)).
				Msg("No components for type, skipping")
			continue
		}

		componentStr := devicetypes.ComponentTypeToString(componentType)
		log.Info().
			Str("type", componentStr).
			Int("count", len(target.ComponentIDs)).
			Msg("Starting firmware update for component type")

		// --- Start firmware update ---
		if err := startFirmwareUpdate(ctx, target, info); err != nil {
			sendAlert(alert.Alert{
				Severity:  alert.SeverityCritical,
				Message:   fmt.Sprintf("%s firmware update request failed: %v", componentStr, err),
				Component: componentStr,
				Operation: "FirmwareUpdate",
				TaskID:    taskID,
			})
			return updateFinishedTaskStatus(ctx, reqInfo.TaskID, err)
		}

		// --- Poll for completion ---
		if err := pollFirmwareUpdateStatus(
			ctx, target,
			defaultFirmwareUpdatePollInterval,
			defaultFirmwareUpdatePollTimeout,
		); err != nil {
			sendAlert(alert.Alert{
				Severity:  alert.SeverityCritical,
				Message:   fmt.Sprintf("%s firmware update failed: %v", componentStr, err),
				Component: componentStr,
				Operation: "FirmwareUpdate",
				TaskID:    taskID,
			})
			return updateFinishedTaskStatus(ctx, reqInfo.TaskID, err)
		}

		log.Info().
			Str("type", componentStr).
			Msg("Firmware update completed for component type")
	}

	// Step 5: Power recycle compute nodes after all firmware updates complete
	if computeTarget, ok := typeToTargets[devicetypes.ComponentTypeCompute]; ok && len(computeTarget.ComponentIDs) > 0 {
		log.Info().Msg("Power recycling compute nodes after firmware update")
		if err := powerRecycleCompute(ctx, computeTarget); err != nil {
			sendAlert(alert.Alert{
				Severity:  alert.SeverityWarning,
				Message:   fmt.Sprintf("Compute power recycle failed: %v", err),
				Component: "Compute",
				Operation: "FirmwareUpdate",
				TaskID:    taskID,
			})
			// Don't fail the entire workflow if power recycle fails
			log.Warn().Err(err).Msg("Power recycle failed, but firmware update succeeded")
		}
	}

	return updateFinishedTaskStatus(ctx, reqInfo.TaskID, nil)
}

// startFirmwareUpdate sends the firmware update request for a single component type.
func startFirmwareUpdate(
	ctx workflow.Context,
	target common.Target,
	info *operations.FirmwareControlTaskInfo,
) error {
	log.Debug().
		Str("target", target.String()).
		Msg("Starting firmware update activity")

	err := workflow.ExecuteActivity(ctx, "StartFirmwareUpdate", target, *info).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start firmware update: %w", err)
	}

	log.Debug().
		Str("target", target.String()).
		Msg("Firmware update request accepted")

	return nil
}

// pollFirmwareUpdateStatus polls the firmware update status until all components
// complete, any fails, or the timeout is reached.
func pollFirmwareUpdateStatus(
	ctx workflow.Context,
	target common.Target,
	pollInterval time.Duration,
	pollTimeout time.Duration,
) error {
	componentStr := devicetypes.ComponentTypeToString(target.Type)

	startTime := workflow.Now(ctx)
	deadline := startTime.Add(pollTimeout)

	for {
		if workflow.Now(ctx).After(deadline) {
			return fmt.Errorf("%s firmware update timed out after %v", componentStr, pollTimeout)
		}

		// Wait before checking status
		if err := workflow.Sleep(ctx, pollInterval); err != nil {
			return fmt.Errorf("workflow sleep interrupted: %w", err)
		}

		// Check status
		var result activity.GetFirmwareUpdateStatusResult
		err := workflow.ExecuteActivity(ctx, "GetFirmwareUpdateStatus", target).Get(ctx, &result)
		if err != nil {
			log.Warn().
				Err(err).
				Str("target", target.String()).
				Msg("Failed to get firmware update status, will retry")
			continue
		}

		// Analyze status
		allCompleted := true
		var failedComponents []string
		for componentID, status := range result.Statuses {
			if status.State == operations.FirmwareUpdateStateFailed {
				failedComponents = append(failedComponents, componentID)
			}
			if status.State != operations.FirmwareUpdateStateCompleted {
				allCompleted = false
				log.Debug().
					Str("component_id", componentID).
					Str("state", status.State.String()).
					Msg("Component firmware update still in progress")
			}
		}

		// Partial failure within timeout → stop
		if len(failedComponents) > 0 {
			return fmt.Errorf("firmware update failed for components: %v", failedComponents)
		}

		if allCompleted {
			log.Info().
				Str("target", target.String()).
				Dur("duration", workflow.Now(ctx).Sub(startTime)).
				Msg("All components completed firmware update")
			return nil
		}

		log.Debug().
			Str("target", target.String()).
			Dur("elapsed", workflow.Now(ctx).Sub(startTime)).
			Msg("Firmware update still in progress, continuing to poll")
	}
}

// checkFirmwareUpdatePrerequisites validates that firmware update can proceed.
// TODO: Implement actual prerequisite checks:
// - Verify all components are online/reachable
// - Validate firmware version data in database
// - Check component power states
// - Verify sufficient disk space for firmware images
// - Ensure no conflicting operations in progress
func checkFirmwareUpdatePrerequisites(_ workflow.Context, _ *task.ExecutionInfo) error {
	// TODO: Implement prerequisite checks
	// For now, return nil (no checks performed)
	log.Info().Msg("Firmware update prerequisite checks: TODO - not yet implemented")
	return nil
}

// powerRecycleCompute performs a power cycle on compute nodes.
// Reuses the PowerControl activity: power off → power on.
func powerRecycleCompute(ctx workflow.Context, target common.Target) error {
	log.Info().Str("target", target.String()).Msg("Starting compute power cycle")

	// Power off
	powerOffInfo := operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationForcePowerOff,
	}
	err := workflow.ExecuteActivity(ctx, "PowerControl", target, powerOffInfo).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to power off compute: %w", err)
	}

	// Wait a bit before powering on
	_ = workflow.Sleep(ctx, 10*time.Second)

	// Power on
	powerOnInfo := operations.PowerControlTaskInfo{
		Operation: operations.PowerOperationPowerOn,
	}
	err = workflow.ExecuteActivity(ctx, "PowerControl", target, powerOnInfo).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to power on compute: %w", err)
	}

	log.Info().Str("target", target.String()).Msg("Compute power cycle completed")
	return nil
}
