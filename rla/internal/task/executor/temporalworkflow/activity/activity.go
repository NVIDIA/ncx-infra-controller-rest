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

	"github.com/google/uuid"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/componentmanager"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/operations"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/task"
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
		UpdateTaskStatus,
		FirmwareControl,
		GetFirmwareUpdateStatus,
		AllowBringUp,
		GetBringUpState,
	}
}

// AllowBringUp opens the power-on gate for the target components.
func AllowBringUp(
	ctx context.Context,
	target common.Target,
) error {
	cm, err := validAndGetComponentManager(target)
	if err != nil {
		return err
	}

	return cm.AllowBringUp(ctx, target)
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

// FirmwareControl initiates firmware update without waiting for completion.
// This activity returns immediately after the update request is accepted.
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
		return nil, fmt.Errorf("target is invalid: %w", err)
	}

	return GetComponentManager(target.Type), nil
}
