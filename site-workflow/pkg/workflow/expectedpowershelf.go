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

// DiscoverExpectedPowerShelfInventory is a workflow to fetch Expected Power Shelf inventory on Site and publish to Cloud
func DiscoverExpectedPowerShelfInventory(ctx workflow.Context) error {
	logger := log.With().Str("Workflow", "DiscoverExpectedPowerShelfInventory").Logger()

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
	var inventoryManager activity.ManageExpectedPowerShelfInventory

	err := workflow.ExecuteActivity(ctx, inventoryManager.DiscoverExpectedPowerShelfInventory).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Str("Activity", "DiscoverExpectedPowerShelfInventory").Msg("Failed to execute activity from workflow")
		return err
	}

	logger.Info().Msg("Completing workflow")

	return nil
}

// CreateExpectedPowerShelf is a workflow to create a new Expected Power Shelf using the CreateExpectedPowerShelfOnSite activity
func CreateExpectedPowerShelf(ctx workflow.Context, request *cwssaws.ExpectedPowerShelf) error {
	logger := log.With().Str("Workflow", "ExpectedPowerShelf").Str("Action", "Create").Str("ID", request.GetId().GetValue()).Str("Expected MAC address", request.BmcMacAddress).Str("Serial", request.ShelfSerialNumber).Logger()

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

	var expectedPowerShelfManager activity.ManageExpectedPowerShelf

	err := workflow.ExecuteActivity(ctx, expectedPowerShelfManager.CreateExpectedPowerShelfOnSite, request).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Str("Activity", "CreateExpectedPowerShelfOnSite").Msg("Failed to execute activity from workflow")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// UpdateExpectedPowerShelf is a workflow to update an Expected Power Shelf using the UpdateExpectedPowerShelfOnSite activity
func UpdateExpectedPowerShelf(ctx workflow.Context, request *cwssaws.ExpectedPowerShelf) error {
	logger := log.With().Str("Workflow", "ExpectedPowerShelf").Str("Action", "Update").Str("ID", request.GetId().GetValue()).Str("Expected MAC address", request.BmcMacAddress).Str("Serial", request.ShelfSerialNumber).Logger()

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

	var expectedPowerShelfManager activity.ManageExpectedPowerShelf

	err := workflow.ExecuteActivity(ctx, expectedPowerShelfManager.UpdateExpectedPowerShelfOnSite, request).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Str("Activity", "UpdateExpectedPowerShelfOnSite").Msg("Failed to execute activity from workflow")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// DeleteExpectedPowerShelf is a workflow to Delete an Expected Power Shelf using the DeleteExpectedPowerShelfOnSite activity
func DeleteExpectedPowerShelf(ctx workflow.Context, request *cwssaws.ExpectedPowerShelfRequest) error {
	logger := log.With().Str("Workflow", "ExpectedPowerShelf").Str("Action", "Delete").Str("ID", request.GetId().GetValue()).Str("optional MAC address", request.BmcMacAddress).Logger()

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

	var expectedPowerShelfManager activity.ManageExpectedPowerShelf

	err := workflow.ExecuteActivity(ctx, expectedPowerShelfManager.DeleteExpectedPowerShelfOnSite, request).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Str("Activity", "DeleteExpectedPowerShelfOnSite").Msg("Failed to execute activity from workflow")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}
