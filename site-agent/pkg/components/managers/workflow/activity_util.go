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
	"context"
	"fmt"

	common "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/common"
	workflowtypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/workflow"
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
)

// DoActivity - Execute the Activity
func (w *API) DoActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq interface{}, wflowMd workflowtypes.WorkflowMetadata) (interface{}, error) {
	activityType := wflowMd.ActivityType()
	resourceType := wflowMd.ResourceType()

	status := wflows.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS
	objectStatus := wflows.ObjectStatus_OBJECT_STATUS_IN_PROGRESS
	statusMsg := resourceType + " started " + wflowMd.ActivityType() + " activity"
	wflowMd.ResponseState(status, objectStatus, statusMsg)

	// Use temporal logger for temporal logs
	logger := activity.GetLogger(ctx)
	withLogger := log.With(logger, "Activity", activityType, "ResourceReq", ResourceReq)

	response, err := doActivity(ctx, ResourceVer, ResourceID, ResourceReq, wflowMd, withLogger)
	if err != nil {
		logMsg := fmt.Sprintf("%v: Error %v - %v", resourceType, activityType, err)
		withLogger.Info(logMsg)
		statusMsg = err.Error()
		objectStatus = wflows.ObjectStatus_OBJECT_STATUS_UNSPECIFIED
		if err == common.ErrResourceStale {
			status = wflows.WorkflowStatus_WORKFLOW_STATUS_SUCCESS
			err = nil
		} else {
			status = wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE
		}
	} else {
		status = wflows.WorkflowStatus_WORKFLOW_STATUS_SUCCESS
		switch wflowMd.DoDbOP() {
		case common.OpCreate:
			objectStatus = wflows.ObjectStatus_OBJECT_STATUS_CREATED
		case common.OpUpdate:
			objectStatus = wflows.ObjectStatus_OBJECT_STATUS_UPDATED
		case common.OpDelete:
			objectStatus = wflows.ObjectStatus_OBJECT_STATUS_DELETED
		}
		statusMsg = wflowMd.ResourceType() + " completed " + wflowMd.ActivityType() + " activity"
	}
	wflowMd.ResponseState(status, objectStatus, statusMsg)
	if err != nil {
		err = temporal.NewApplicationError(err.Error(), "", wflowMd.Response())
	}
	logMsg := fmt.Sprintf("%v: Completed %v", resourceType, activityType)
	withLogger.Info(logMsg)
	return response, err
}

func doActivity(ctx context.Context, ResourceVer uint64, ResourceID string,
	ResourceReq interface{}, wflowMd workflowtypes.WorkflowMetadata,
	withLogger log.Logger) (interface{}, error) {
	activityType := wflowMd.ActivityType()
	resourceType := wflowMd.ResourceType()

	ctx, span := otel.Tracer("elektra-site-agent").Start(ctx, "Actv-"+activityType+"-"+resourceType)
	span.SetAttributes(attribute.String("activityType", activityType))
	span.SetAttributes(attribute.String("resourceType", resourceType))
	span.SetAttributes(attribute.String("resourceID", ResourceID))
	span.SetAttributes(attribute.String("resourceVer", fmt.Sprint(ResourceVer)))
	span.SetAttributes(attribute.String("resourceReq", fmt.Sprint(ResourceReq)))
	defer span.End()

	logMsg := fmt.Sprintf("%v: Starting the Activity %v", resourceType, activityType)
	ManagerAccess.Data.EB.Log.Info().Interface("Request", ResourceReq).Msg(logMsg)
	withLogger.Info(logMsg)

	// Make sure GRPC client is available
	if ManagerAccess.Data.EB.Managers.Carbide.GetClient() == nil {
		ManagerAccess.Data.EB.Log.Info().Str("Workflow", activityType).Msgf("%v: GRPC client is not available creating one", resourceType)
		err := ManagerAccess.API.Carbide.CreateGRPCClient()
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
	}

	if ManagerAccess.Data.EB.Managers.Carbide.GetClient() == nil {
		err := fmt.Errorf("%v: Failed to create grpc client connection handle", resourceType)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	TransactionID := &wflows.TransactionID{ResourceId: ResourceID,
		Timestamp: nil}
	response, err := wflowMd.DoSiteControllerOP(ctx, TransactionID, ResourceReq)
	ManagerAccess.API.Carbide.UpdateGRPCClientState(err)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "activity successful")
	}

	return response, err
}
