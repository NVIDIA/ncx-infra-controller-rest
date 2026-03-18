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

package vpc

import (
	"context"
	"reflect"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
)

// PublishVPCListActivity - Publish VPC Activity
func (ac *Workflows) PublishVPCListActivity(ctx context.Context, TransactionID *wflows.TransactionID, vpcResp *wflows.GetVPCResponse) (workflowID string, err error) {
	ManagerAccess.Data.EB.Log.Info().Interface("Request", TransactionID).Msgf("VPC: Starting Publish Activity %v", vpcResp)

	// Use temporal logger for temporal logs
	logger := activity.GetLogger(ctx)
	withLogger := log.With(logger, "Activity", "PublishVPCListActivity", "ResourceReq", TransactionID)
	withLogger.Info("VPC: Starting the Publish VPC List Activity")

	if vpcResp == nil || vpcResp.List == nil {
		withLogger.Info("VPC: Empty Nil Response")
		return
	}

	for _, v := range vpcResp.List.Vpcs {
		vpcInfo := &wflows.VPCInfo{Status: vpcResp.Status, StatusMsg: vpcResp.StatusMsg, Vpc: v}
		ManagerAccess.Data.EB.Log.Info().Msgf("VPC: Publish List Activity %v", vpcInfo)
	}
	return
}

// PublishVPCActivity - Publish VPC Activity
func (ac *Workflows) PublishVPCActivity(ctx context.Context, TransactionID *wflows.TransactionID, VpcInfo *wflows.VPCInfo) (workflowID string, err error) {
	ManagerAccess.Data.EB.Log.Info().Interface("Request", TransactionID).Msgf("VPC: Starting Publish Activity %v", VpcInfo)

	// Use temporal logger for temporal logs
	logger := activity.GetLogger(ctx)
	withLogger := log.With(logger, "Activity", "PublishVPCActivity", "ResourceReq", TransactionID)
	withLogger.Info("VPC: Starting the Publish VPC Activity")

	workflowOptions := client.StartWorkflowOptions{
		ID:        TransactionID.ResourceId,
		TaskQueue: ManagerAccess.Conf.EB.Temporal.TemporalPublishQueue,
	}
	var vpcresponse interface{}
	// Lets check if we need to convert the response
	if !reflect.ValueOf(ManagerAccess.Conf.EB.CloudVersion).IsZero() && !reflect.ValueOf(ManagerAccess.Conf.EB.SiteVersion).IsZero() && ManagerAccess.Conf.EB.CloudVersion != ManagerAccess.Conf.EB.SiteVersion {
		// We may need to convert
		// Transform the message according to the version
		transformRequest := &VPCRespTransformer{
			// This is the request coming from Site Controller
			FromVersion: ManagerAccess.Conf.EB.SiteVersion,
			// This is the request going to Cloud
			ToVersion: ManagerAccess.Conf.EB.CloudVersion,
			Op:        "publish",
			Response:  VpcInfo,
		}
		vpcresponse, err = transformRequest.VPCResponseConverter()
		if err != nil {
			// Return error
			ManagerAccess.Data.EB.Log.Info().Interface("Request", TransactionID).Msgf("VPC: Cannot convert VPC Publish response %v", VpcInfo)
			return "", err
		}
	} else {
		// Use the response as is
		ManagerAccess.Data.EB.Log.Info().Interface("Request", TransactionID).Msg("VPC: Using the response as is")
		vpcresponse = VpcInfo

	}

	we, err := ac.tcPublish.ExecuteWorkflow(context.Background(), workflowOptions, "UpdateVpcInfo",
		ManagerAccess.Conf.EB.Temporal.TemporalSubscribeNamespace, TransactionID, vpcresponse)
	if err != nil {
		return "", err
	}

	wid := we.GetID()
	return wid, nil
}
