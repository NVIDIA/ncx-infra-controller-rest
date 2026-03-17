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
	"context"
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
)

// PublishSubnetActivity - Publish Subnet Activity
func (ac *Workflows) PublishSubnetActivity(ctx context.Context, TransactionID *wflows.TransactionID, SubnetInfo *wflows.SubnetInfo) (workflowID string, err error) {
	ManagerAccess.Data.EB.Log.Info().Interface("Request", TransactionID).Msg("Subnet: Starting  the Publish Subnet Activity")

	// Use temporal logger for temporal logs
	logger := activity.GetLogger(ctx)
	withLogger := log.With(logger, "Activity", "PublishSubnetActivity", "ResourceReq", TransactionID)
	withLogger.Info("Subnet: Starting the Publish Subnet Activity")

	workflowOptions := client.StartWorkflowOptions{
		ID:        TransactionID.ResourceId,
		TaskQueue: ManagerAccess.Conf.EB.Temporal.TemporalPublishQueue,
	}

	we, err := ac.tcPublish.ExecuteWorkflow(ctx, workflowOptions, "UpdateSubnetInfo", ManagerAccess.Conf.EB.Temporal.TemporalSubscribeNamespace, TransactionID, SubnetInfo)
	if err != nil {
		return "", err
	}

	wid := we.GetID()
	return wid, nil
}
