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

package site

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"go.temporal.io/sdk/client"

	siteActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/site"
	"github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/queue"
)

// MonitorHealthForAllSites is a Temporal cron workflow to periodically checks if Site inventory has been received from Site Agent
// TODO: Once health check is available across all Sites, retire this workflow
func MonitorHealthForAllSites(ctx workflow.Context) error {
	logger := log.With().Str("Workflow", "Site").Str("Action", "MonitorHealthAll").Logger()

	logger.Info().Msg("starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    3 * time.Minute,
		MaximumAttempts:    15,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 3 * time.Minute,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var siteManager siteActivity.ManageSite

	err := workflow.ExecuteActivity(ctx, siteManager.MonitorInventoryReceiptForAllSites).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to execute activity: MonitorInventoryReceiptForAllSites")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// ExecuteMonitorHealthForAllSitesWorkflow is a helper function to trigger execution of MonitorHealthForAllSites workflow
func ExecuteMonitorHealthForAllSitesWorkflow(ctx context.Context, tc client.Client) (*string, error) {
	workflowOptions := client.StartWorkflowOptions{
		ID:           "site-monitor-health-all",
		CronSchedule: "@every 3m",
		TaskQueue:    queue.CloudTaskQueue,
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, MonitorHealthForAllSites)

	if err != nil {
		log.Error().Err(err).Msg("failed to execute workflow: MonitorHealthForAllSites")
		return nil, err
	}

	wid := we.GetID()

	return &wid, nil
}

// CheckHealthForAllSites is a Temporal cron workflow to periodically check health status for all Sites
func CheckHealthForAllSites(ctx workflow.Context) error {
	logger := log.With().Str("Workflow", "Site").Str("Action", "CheckHealthForAllSites").Logger()

	logger.Info().Msg("starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 1.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    1,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 1 * time.Minute,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var siteManager siteActivity.ManageSite
	var siteIDs []uuid.UUID

	// Get all sites
	err := workflow.ExecuteActivity(ctx, siteManager.GetAllSiteIDs).Get(ctx, &siteIDs)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to execute activity: GetAllSiteIDs")
		return err
	}

	// Check health for site via site agent
	for _, siteID := range siteIDs {
		err := workflow.ExecuteActivity(ctx, siteManager.CheckHealthForSiteViaSiteAgent, siteID).Get(ctx, nil)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to execute activity: CheckHealthForSiteViaSiteAgent")
			/*
				// uncomment below when every site agent has 'GetHealth' enabled
				// Check if the error is StartToClose timeout,
				// it suggests 'GetHealth' workflow may not enable at Site Agent
				// keep site status the current
				var timeoutErr *temporal.TimeoutError
				if errors.As(err, &timeoutErr) {
					switch timeoutErr.TimeoutType() {
					case enums.TIMEOUT_TYPE_START_TO_CLOSE:
						logger.Info().Msg(fmt.Sprintf("StartToClose timeout for siteID: %s", siteID))
						continue
					}
				}*/
			err = workflow.ExecuteActivity(ctx, siteManager.OnCheckHealthForSiteViaSiteAgentError, siteID, nil).Get(ctx, nil)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to execute activity: OnCheckHealthForSiteViaSiteAgentError")
			}
			return err
		}
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// ExecuteCheckHealthForAllSitesWorkflow is a helper function to trigger execution of ExecuteGetHealthForAllSiteAgents workflow
func ExecuteCheckHealthForAllSitesWorkflow(ctx context.Context, tc client.Client) (*string, error) {
	workflowOptions := client.StartWorkflowOptions{
		ID:           "get-site-agent-health-all",
		CronSchedule: "@every 1m",
		TaskQueue:    queue.CloudTaskQueue,
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, CheckHealthForAllSites)

	if err != nil {
		log.Error().Err(err).Msg("failed to execute workflow: CheckHealthForAllSites")
		return nil, err
	}

	wid := we.GetID()

	return &wid, nil
}

// MonitorSiteTemporalNamespaces is a Temporal cron workflow to periodically check and delete orphaned Temporal namespaces
func MonitorSiteTemporalNamespaces(ctx workflow.Context) error {
	logger := log.With().Str("Workflow", "Site").Str("Action", "MonitorSiteTemporalNamespaces").Logger()

	logger.Info().Msg("starting workflow")

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 1.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    2,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var siteManager siteActivity.ManageSite

	// Execute activity to get temporal namespaces
	err := workflow.ExecuteActivity(ctx, siteManager.DeleteOrphanedSiteTemporalNamespaces).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to execute activity: DeleteOrphanedSiteTemporalNamespaces")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// ExecuteMonitorSiteTemporalNamespaces is a helper function to trigger execution of MonitorSiteTemporalNamespaces workflow
func ExecuteMonitorSiteTemporalNamespaces(ctx context.Context, tc client.Client) (*string, error) {
	workflowOptions := client.StartWorkflowOptions{
		ID:           "monitor-site-temporal-namespaces",
		CronSchedule: "@every 1h", // Run hourly
		TaskQueue:    queue.CloudTaskQueue,
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, MonitorSiteTemporalNamespaces)
	if err != nil {
		log.Error().Err(err).Msg("failed to execute workflow: MonitorSiteTemporalNamespaces")
		return nil, err
	}

	wid := we.GetID()

	return &wid, nil
}

// MonitorTemporalCertExpirationForAllSites is a Temporal cron workflow to periodically rotate certs and OTPs
func MonitorTemporalCertExpirationForAllSites(ctx workflow.Context) error {
	logger := log.With().Str("Workflow", "MonitorTemporalCertExpirationForAllSites").Logger()
	logger.Info().Msg("Starting workflow")

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    3 * time.Minute,
		MaximumAttempts:    15,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	// Execute the activity
	err := workflow.ExecuteActivity(ctx, siteActivity.ManageSite.CheckOTPExpirationAndRenewForAllSites).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to execute activity: CheckOTPExpirationAndRenewForAllSites")
		return err
	}

	logger.Info().Msg("completing workflow")

	return nil
}

// ExecuteMonitorTemporalCertExpirationForAllSites is a helper function to trigger execution of MonitorTemporalCertExpirationForAllSites
func ExecuteMonitorTemporalCertExpirationForAllSites(ctx context.Context, tc client.Client) (*string, error) {
	workflowOptions := client.StartWorkflowOptions{
		ID:           "rotate-certs-and-otps",
		CronSchedule: "@every 24h", // Run daily
		TaskQueue:    queue.CloudTaskQueue,
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, MonitorTemporalCertExpirationForAllSites)
	if err != nil {
		log.Error().Err(err).Msg("failed to execute workflow: MonitorTemporalCertExpirationForAllSites")
		return nil, err
	}

	wid := we.GetID()

	return &wid, nil
}

// UpdateAgentCertExpiry updates the AgentCertExpiry field for a site
func UpdateAgentCertExpiry(ctx workflow.Context, siteIDStr string, certExpiry time.Time) error {
	logger := log.With().Str("Workflow", "UpdateAgentCertExpiry").Str("SiteID", siteIDStr).Logger()
	logger.Info().Msg("Starting workflow to update AgentCertExpiry")

	// Parse siteID
	siteID, err := uuid.Parse(siteIDStr)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid siteID")
		return err
	}

	// Set up activity options
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    1 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	// Create an instance of ManageSite activities
	var manageSite siteActivity.ManageSite

	// Execute the activity
	err = workflow.ExecuteActivity(ctx, manageSite.UpdateAgentCertExpiry, siteID, certExpiry).Get(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to execute UpdateAgentCertExpiry activity")
		return err
	}

	logger.Info().Msg("Workflow completed successfully")
	return nil
}
