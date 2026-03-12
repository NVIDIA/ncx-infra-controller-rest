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
	"time"

	"github.com/nvidia/bare-metal-manager-rest/site-workflow/pkg/activity"
	cwssaws "github.com/nvidia/bare-metal-manager-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DiscoverExpectedSwitchInventory is a workflow to fetch Expected Switch inventory on Site and publish to Cloud
func DiscoverExpectedSwitchInventory(ctx workflow.Context) error {
	logger := log.With().Str("Workflow", "DiscoverExpectedSwitchInventory").Logger()

	logger.Info().Msg("Starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    10 * time.Second,
		// This is executed every 3 minutes, so we don't want too many retry attempts
		MaximumAttempts: 2,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 2 * time.Minute,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	// Invoke activity
	var inventoryManager activity.ManageExpectedSwitchInventory

	err := workflow.ExecuteActivity(ctx, inventoryManager.DiscoverExpectedSwitchInventory).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Str("Activity", "DiscoverExpectedSwitchInventory").Msg("Failed to execute activity from workflow")
		return err
	}

	logger.Info().Msg("Completing workflow")

	return nil
}

// CreateExpectedSwitch is a workflow to create a new Expected Switch using the CreateExpectedSwitchOnSite activity
func CreateExpectedSwitch(ctx workflow.Context, request *cwssaws.ExpectedSwitch) error {
	logger := log.With().Str("Workflow", "ExpectedSwitch").Str("Action", "Create").Str("ID", request.GetId().GetValue()).Str("Expected MAC address", request.BmcMacAddress).Str("Serial", request.SwitchSerialNumber).Logger()

	logger.Info().Msg("starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    10 * time.Second,
		MaximumAttempts:    2,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 2 * time.Minute,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var expectedSwitchManager activity.ManageExpectedSwitch

	err := workflow.ExecuteActivity(ctx, expectedSwitchManager.CreateExpectedSwitchOnSite, request).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Str("Activity", "CreateExpectedSwitchOnSite").Msg("Failed to execute activity from workflow")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// UpdateExpectedSwitch is a workflow to update an Expected Switch using the UpdateExpectedSwitchOnSite activity
func UpdateExpectedSwitch(ctx workflow.Context, request *cwssaws.ExpectedSwitch) error {
	logger := log.With().Str("Workflow", "ExpectedSwitch").Str("Action", "Update").Str("ID", request.GetId().GetValue()).Str("Expected MAC address", request.BmcMacAddress).Str("Serial", request.SwitchSerialNumber).Logger()

	logger.Info().Msg("starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    10 * time.Second,
		MaximumAttempts:    2,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 2 * time.Minute,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var expectedSwitchManager activity.ManageExpectedSwitch

	err := workflow.ExecuteActivity(ctx, expectedSwitchManager.UpdateExpectedSwitchOnSite, request).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Str("Activity", "UpdateExpectedSwitchOnSite").Msg("Failed to execute activity from workflow")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// DeleteExpectedSwitch is a workflow to Delete an Expected Switch using the DeleteExpectedSwitchOnSite activity
func DeleteExpectedSwitch(ctx workflow.Context, request *cwssaws.ExpectedSwitchRequest) error {
	logger := log.With().Str("Workflow", "ExpectedSwitch").Str("Action", "Delete").Str("ID", request.GetId().GetValue()).Str("optional MAC address", request.BmcMacAddress).Logger()

	logger.Info().Msg("starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    10 * time.Second,
		MaximumAttempts:    2,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 2 * time.Minute,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var expectedSwitchManager activity.ManageExpectedSwitch

	err := workflow.ExecuteActivity(ctx, expectedSwitchManager.DeleteExpectedSwitchOnSite, request).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Str("Activity", "DeleteExpectedSwitchOnSite").Msg("Failed to execute activity from workflow")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}
