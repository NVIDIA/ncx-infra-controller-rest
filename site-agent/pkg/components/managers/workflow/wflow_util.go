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
	"errors"
	"fmt"
	"time"

	carbidetypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/carbide"

	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	computils "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/utils"
	wflowtypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/workflow"
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

// Retry parameters for Temporal workflows
const (
	// RetryInterval- Retry Interval
	RetryInterval = 2
	// RetryCount - Retry Count
	RetryCount = 10
	// MaxTemporalActivityRetryCount - Max Temporal Activity Retry Count
	MaxTemporalActivityRetryCount = 7
)

// doValidation
func doValidation(withLogger log.Logger, TransactionID *wflows.TransactionID, modStr string) error {
	if TransactionID == nil {
		return fmt.Errorf("%v: TransactionID is nil", modStr)
	}
	if TransactionID.ResourceId == "" {
		return fmt.Errorf("%v: TransactionID.ResourceId is empty", modStr)
	}
	return nil
}

func recordWflowStart(s *wflowtypes.MgrState) {
	s.WflowStarted.Inc()
}

func recordWflowEnd(activitySucc bool, publishSucc bool, s *wflowtypes.MgrState) {
	if !activitySucc {
		s.WflowActFail.Inc()
	} else {
		s.WflowActSucc.Inc()
	}
	if !publishSucc {
		s.WflowPubFail.Inc()
	} else {
		s.WflowPubSucc.Inc()
	}
}

// DoWorkflow - Execute the Workflow
func (w *API) DoWorkflow(ctx workflow.Context, TransactionID *wflows.TransactionID,
	ResourceRequest interface{}, wflowMd wflowtypes.WorkflowMetadata,
	retryOptions *wflows.WorkflowOptions) (actErr error, pubErr error) {
	recordWflowStart(wflowMd.Statistics())
	defer func(startTime time.Time) {
		if ManagerAccess.Data.EB.Managers.Carbide.State.WflowMetrics != nil {
			status := carbidetypes.WorkflowStatusSuccess
			if actErr != nil && pubErr != nil {
				status = carbidetypes.WorkflowStatusActivityPublishFailed
			} else if actErr != nil {
				status = carbidetypes.WorkflowStatusActivityFailed
			} else if pubErr != nil {
				status = carbidetypes.WorkflowStatusPublishFailed
			}
			ManagerAccess.Data.EB.Managers.Carbide.State.WflowMetrics.RecordLatency(wflowMd.ActivityType(), status, time.Since(startTime))
		}
	}(time.Now())
	actErr, pubErr = doWorkflow(ctx, TransactionID, ResourceRequest, wflowMd, retryOptions)
	recordWflowEnd(actErr == nil, pubErr == nil, wflowMd.Statistics())
	return
}

func doWorkflow(ctx workflow.Context, TransactionID *wflows.TransactionID,
	ResourceRequest interface{}, wflowMd wflowtypes.WorkflowMetadata,
	retryOptions *wflows.WorkflowOptions) (actErr error, pubErr error) {
	resourceType := wflowMd.ResourceType()
	activityType := wflowMd.ActivityType()

	// Temporal logger for temporal logs
	logger := workflow.GetLogger(ctx)
	withLogger := log.With(logger, "-", activityType, "ResourceRequest", ResourceRequest)
	dispStr := fmt.Sprintf("%v: Starting workflow", resourceType)
	withLogger.Info(dispStr)
	ManagerAccess.Data.EB.Log.Info().Interface("Request", ResourceRequest).Msg(dispStr)

	actErr = doValidation(withLogger, TransactionID, resourceType)
	pubErr = actErr
	if actErr != nil {
		withLogger.Error(actErr.Error())
		ManagerAccess.Data.EB.Log.Error().Msg(actErr.Error())
		return
	}

	var RetryInterval time.Duration
	// Check if customized retry policy is Requested
	if retryOptions.GetRetryInt32Erval() > 0 {
		RetryInterval = time.Duration(int64(retryOptions.GetRetryInt32Erval())) * time.Second
	} else {
		// Use default retry interval
		RetryInterval = 1 * time.Second
	}
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    RetryInterval,
		BackoffCoefficient: 2.0,
		MaximumInterval:    1 * time.Minute,
		MaximumAttempts:    MaxTemporalActivityRetryCount,
	}
	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 60 * time.Second,
		// Optionally provide a customized RetryPolicy.
		RetryPolicy: retrypolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	// Invoke activity
	actErr = invokeActivity(ctx, TransactionID, ResourceRequest, wflowMd)
	if actErr != nil {
		withLogger.Error(actErr.Error())
		ManagerAccess.Data.EB.Log.Error().Interface("Error", actErr).Msg(actErr.Error())
	}

	// Publish the status to the Cloud
	var publishWorkflowID string
	pubErr = workflow.ExecuteActivity(ctx, wflowMd.ActivityPublish(), TransactionID, wflowMd.Response()).Get(ctx, &publishWorkflowID)
	if pubErr != nil {
		dispStr = fmt.Sprintf("%v: Failed to publish", resourceType)
		withLogger.Error(dispStr, "Error", pubErr, "WorkflowID", publishWorkflowID)
		ManagerAccess.Data.EB.Log.Error().Str("Id", publishWorkflowID).Interface("Error", pubErr).Msg(dispStr)
	} else {
		dispStr = fmt.Sprintf("%v: Successfully published update", resourceType)
		withLogger.Info(resourceType, dispStr, "workflowID", publishWorkflowID)
		ManagerAccess.Data.EB.Log.Info().Str("Id", publishWorkflowID).Msg(dispStr)
	}

	return
}

// invokeActivity is the op
func invokeActivity(ctx workflow.Context, TransactionID *wflows.TransactionID,
	ResourceRequest interface{},
	wflowMd wflowtypes.WorkflowMetadata) (err error) {
	resourceType := wflowMd.ResourceType()

	// 1. Make sure GRPC client is available
	if ManagerAccess.Data.EB.Managers.Carbide.GetClient() == nil {
		ManagerAccess.Data.EB.Log.Info().Str("Workflow", wflowMd.ActivityType()).Msgf("%v: GRPC client is not available creating one", resourceType)
		err = ManagerAccess.API.Carbide.CreateGRPCClient()
		if err != nil {
			wflowMd.ResponseState(wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
				wflows.ObjectStatus_OBJECT_STATUS_UNSPECIFIED, err.Error())
			return err
		}
	}
	if ManagerAccess.Data.EB.Managers.Carbide.GetClient() == nil {
		err = fmt.Errorf("%v: Failed to create grpc client connection handle", resourceType)
		wflowMd.ResponseState(wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
			wflows.ObjectStatus_OBJECT_STATUS_UNSPECIFIED, err.Error())
		return err
	}

	// 2. Execute the activity

	// Handle the ResourceRequest input here
	pc := converter.NewProtoPayloadConverter()
	payload, err := pc.ToPayload(ResourceRequest)
	if err != nil {
		ManagerAccess.Data.EB.Log.Error().Interface("Error", err).Msgf("%v: Failed to convert Request to payload", resourceType)
		wflowMd.ResponseState(wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
			wflows.ObjectStatus_OBJECT_STATUS_UNSPECIFIED, err.Error())
		return err
	}
	ManagerAccess.Data.EB.Log.Info().Interface("Payload", payload).Msgf("%v: Payload", resourceType)

	resourceVer, err := computils.ConvertTimestampToVersion(TransactionID.Timestamp)
	if err != nil {
		ManagerAccess.Data.EB.Log.Error().Interface("Error", err).Msgf("%v: Failed to convert Timestamp", resourceType)
		wflowMd.ResponseState(wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
			wflows.ObjectStatus_OBJECT_STATUS_UNSPECIFIED, err.Error())
		return err
	}

	future := workflow.ExecuteActivity(ctx, wflowMd.ActivityInvoke(), resourceVer, TransactionID.ResourceId, ResourceRequest)
	err = future.Get(ctx, wflowMd.Response())
	if err != nil {
		var applicationErr *temporal.ApplicationError
		if errors.As(err, &applicationErr) {
			applicationErr.Details(wflowMd.Response())
		}
		ManagerAccess.Data.EB.Log.Error().Interface("Error", wflowMd.Response()).Msgf("%v: Failed to %v", resourceType, wflowMd.ActivityType())
	} else {
		ManagerAccess.Data.EB.Log.Info().Msgf("%v: Successful %v", resourceType, wflowMd.ActivityType())
	}

	return err
}
