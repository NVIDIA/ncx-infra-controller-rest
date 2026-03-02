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

package activity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/carbideapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
)

var taskStatusUpdater task.TaskStatusUpdater

// SetTaskStatusUpdater registers the updater used by activities.
func SetTaskStatusUpdater(updater task.TaskStatusUpdater) {
	taskStatusUpdater = updater
}

func InjectExpectation(
	ctx context.Context,
	target common.Target,
	info operations.InjectExpectationTaskInfo,
) error {
	cm, err := validAndGetComponentManager(target)
	if err != nil {
		return err
	}

	return cm.InjectExpectation(ctx, target, info)
}

func PowerControl(
	ctx context.Context,
	target common.Target,
	info operations.PowerControlTaskInfo,
) error {
	cm, err := validAndGetComponentManager(target)
	if err != nil {
		return err
	}

	return cm.PowerControl(ctx, target, info)
}

func GetPowerStatus(
	ctx context.Context,
	target common.Target,
) (map[string]operations.PowerStatus, error) {
	cm, err := validAndGetComponentManager(target)
	if err != nil {
		return nil, err
	}

	return cm.GetPowerStatus(ctx, target)
}

func FirmwareControl(
	ctx context.Context,
	target common.Target,
	info operations.FirmwareControlTaskInfo,
) error {
	cm, err := validAndGetComponentManager(target)
	if err != nil {
		return err
	}

	return cm.FirmwareControl(ctx, target, info)
}

// UpdateTaskStatus is a Temporal activity that updates task status by ID.
func UpdateTaskStatus(
	ctx context.Context,
	arg *task.TaskStatusUpdate,
) error {
	if taskStatusUpdater == nil {
		return fmt.Errorf("task status updater is not configured")
	}

	if arg == nil || arg.ID == uuid.Nil {
		return fmt.Errorf("invalid task identifier")
	}

	return taskStatusUpdater.UpdateTaskStatus(ctx, arg)
}

func GetAllActivities() []any {
	return []any{
		InjectExpectation,
		PowerControl,
		GetPowerStatus,
		FirmwareControl,
		UpdateTaskStatus,
		SetFirmwareUpdateTimeWindow,
		StartFirmwareUpdate,
		GetFirmwareUpdateStatus,
		AllowBringUpAndPowerOn,
		GetBringUpState,
	}
}

// AllowBringUpAndPowerOn opens the Carbide power-on gate
// for the target components.
func AllowBringUpAndPowerOn(
	ctx context.Context,
	target common.Target,
) error {
	cm, err := validAndGetComponentManager(target)
	if err != nil {
		return err
	}

	return cm.AllowBringUpAndPowerOn(ctx, target)
}

// GetBringUpStateResult is the result of GetBringUpState.
type GetBringUpStateResult struct {
	States map[string]operations.MachineBringUpState
}

// GetBringUpState returns the bring-up state for target
// components.
func GetBringUpState(
	ctx context.Context,
	target common.Target,
) (*GetBringUpStateResult, error) {
	cm, err := validAndGetComponentManager(target)
	if err != nil {
		return nil, err
	}

	states, err := cm.GetBringUpState(ctx, target)
	if err != nil {
		return nil, err
	}

	return &GetBringUpStateResult{States: states}, nil
}

// StartFirmwareUpdate initiates firmware update without waiting for completion.
// This activity returns immediately after the update request is accepted.
func StartFirmwareUpdate(
	ctx context.Context,
	target common.Target,
	info operations.FirmwareControlTaskInfo,
) error {
	cm, err := validAndGetComponentManager(target)
	if err != nil {
		return err
	}

	return cm.StartFirmwareUpdate(ctx, target, info)
}

// GetFirmwareUpdateStatusResult is the result of GetFirmwareUpdateStatus activity.
type GetFirmwareUpdateStatusResult struct {
	Statuses map[string]operations.FirmwareUpdateStatus
}

// GetFirmwareUpdateStatus returns the current status of firmware updates.
// This activity is designed to be called repeatedly in a polling loop.
func GetFirmwareUpdateStatus(
	ctx context.Context,
	target common.Target,
) (*GetFirmwareUpdateStatusResult, error) {
	cm, err := validAndGetComponentManager(target)
	if err != nil {
		return nil, err
	}

	statuses, err := cm.GetFirmwareUpdateStatus(ctx, target)
	if err != nil {
		return nil, err
	}

	return &GetFirmwareUpdateStatusResult{Statuses: statuses}, nil
}

func validAndGetComponentManager(
	target common.Target,
) (componentmanager.ComponentManager, error) {
	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("target is invalid: %v", err)
	}

	return GetComponentManager(target.Type), nil
}

// SetFirmwareUpdateTimeWindow sets the firmware update time window for the given components
// via Carbide API. Returns error if the API call fails.
func SetFirmwareUpdateTimeWindow(
	ctx context.Context,
	req operations.SetFirmwareUpdateTimeWindowRequest,
) error {
	if len(req.ComponentIDs) == 0 {
		log.Warn().Msg("No component IDs provided for SetFirmwareUpdateTimeWindow")
		return nil
	}

	log.Info().
		Strs("component_ids", req.ComponentIDs).
		Time("start_time", req.StartTime).
		Time("end_time", req.EndTime).
		Msg("Setting firmware update time window")

	client, err := carbideapi.NewClient(time.Minute * 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create carbide API client")
		return err
	}

	// Carbide API uses machine_id terminology
	err = client.SetFirmwareUpdateTimeWindow(ctx, req.ComponentIDs, req.StartTime, req.EndTime)
	if err != nil {
		log.Error().Err(err).Strs("component_ids", req.ComponentIDs).Msg("Failed to set firmware update time window")
		return err
	}

	log.Info().Strs("component_ids", req.ComponentIDs).Msg("Successfully set firmware update time window")
	return nil
}
