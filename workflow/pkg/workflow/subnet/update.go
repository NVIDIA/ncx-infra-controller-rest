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

package subnet

import (
	"fmt"
	"time"

	cwm "github.com/NVIDIA/ncx-infra-controller-rest/workflow/internal/metrics"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	subnetActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/subnet"

	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

// UpdateSubnetInfo is a Temporal workflow that Site Agent calls to update Subnet information
func UpdateSubnetInfo(ctx workflow.Context, siteID string, transactionID *cwssaws.TransactionID, subnetInfo *cwssaws.SubnetInfo) error {
	logger := log.With().Str("Workflow", "UpdateSubnetInfo").Str("Site ID", siteID).Logger()

	logger.Info().Msg("starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    15,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 2 * time.Minute,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var subnetManager subnetActivity.ManageSubnet

	err := workflow.ExecuteActivity(ctx, subnetManager.UpdateSubnetInDB, transactionID, subnetInfo).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to execute activity: UpdateSubnetInDB")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// UpdateSubnetInventory is a workflow called by Site Agent to update Subnet inventory for a Site
func UpdateSubnetInventory(ctx workflow.Context, siteID string, subnetInventory *cwssaws.SubnetInventory) (err error) {
	logger := log.With().Str("Workflow", "UpdateSubnetInventory").Str("Site ID", siteID).Logger()

	startTime := time.Now()

	logger.Info().Msg("starting workflow")

	parsedSiteID, err := uuid.Parse(siteID)
	if err != nil {
		logger.Warn().Err(err).Msg(fmt.Sprintf("workflow triggered with invalid site ID: %s", siteID))
		return err
	}

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    5 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    30 * time.Second,
		MaximumAttempts:    2,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 30 * time.Second,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var subnetManager subnetActivity.ManageSubnet

	// Execute UpdateSubnetsInDB activity and get lifecycle events
	var subnetLifecycleEvents []cwm.InventoryObjectLifecycleEvent
	err = workflow.ExecuteActivity(ctx, subnetManager.UpdateSubnetsInDB, parsedSiteID, subnetInventory).Get(ctx, &subnetLifecycleEvents)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to execute activity: UpdateSubnetsInDB")
	}

	// Record subnet lifecycle metrics
	var lifecycleMetricsManager subnetActivity.ManageSubnetLifecycleMetrics
	serr := workflow.ExecuteActivity(ctx, lifecycleMetricsManager.RecordSubnetStatusTransitionMetrics, parsedSiteID, subnetLifecycleEvents).Get(ctx, nil)
	if serr != nil {
		logger.Warn().Err(serr).Msg("failed to execute activity: RecordSubnetStatusTransitionMetrics")
	}

	// Record latency for this inventory call
	var inventoryMetricsManager cwm.ManageInventoryMetrics

	serr = workflow.ExecuteActivity(ctx, inventoryMetricsManager.RecordLatency, parsedSiteID, "UpdateSubnetInventory", err != nil, time.Since(startTime)).Get(ctx, nil)
	if serr != nil {
		logger.Warn().Err(serr).Msg("failed to execute activity: RecordLatency")
	}

	logger.Info().Msg("completing workflow")

	// Return original error from inventory activity, if any
	return err
}
