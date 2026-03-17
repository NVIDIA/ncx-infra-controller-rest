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

package instance

import (
	"errors"
	"testing"

	cwm "github.com/NVIDIA/ncx-infra-controller-rest/workflow/internal/metrics"
	instanceActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/instance"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/types/known/timestamppb"

	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

type UpdateInstanceTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateInstanceTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *UpdateInstanceTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateInstanceTestSuite) Test_UpdateInstanceInfo_Success() {
	var instanceManager instanceActivity.ManageInstance

	siteID := uuid.New()

	transactionID := &cwssaws.TransactionID{
		ResourceId: uuid.New().String(),
		Timestamp:  timestamppb.Now(),
	}

	instanceInfo := &cwssaws.InstanceInfo{
		Status:    cwssaws.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS,
		StatusMsg: "Instance creation in progress",
		Instance: &cwssaws.Instance{
			Id: &cwssaws.InstanceId{Value: uuid.New().String()},
		},
	}

	// Mock UpdateInstanceInDB activity
	s.env.RegisterActivity(instanceManager.UpdateInstanceInDB)
	s.env.OnActivity(instanceManager.UpdateInstanceInDB, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// execute UpdateInstanceInfo workflow
	s.env.ExecuteWorkflow(UpdateInstanceInfo, siteID.String(), transactionID, instanceInfo)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateInstanceTestSuite) Test_UpdateInstanceInfo_ActivityFails() {
	var instanceManager instanceActivity.ManageInstance

	siteID := uuid.New()

	transactionID := &cwssaws.TransactionID{
		ResourceId: uuid.New().String(),
		Timestamp:  timestamppb.Now(),
	}

	instanceInfo := &cwssaws.InstanceInfo{
		Status:    cwssaws.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS,
		StatusMsg: "Instance creation in progress",
		Instance: &cwssaws.Instance{
			Id: &cwssaws.InstanceId{Value: uuid.New().String()},
		},
	}

	// Mock UpdateInstanceInDB activity failure
	s.env.RegisterActivity(instanceManager.UpdateInstanceInDB)
	s.env.OnActivity(instanceManager.UpdateInstanceInDB, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("UpdateInstanceInfo Failure"))

	// execute UpdateInstanceInfo workflow
	s.env.ExecuteWorkflow(UpdateInstanceInfo, siteID.String(), transactionID, instanceInfo)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("UpdateInstanceInfo Failure", applicationErr.Error())
}

func (s *UpdateInstanceTestSuite) Test_UpdateRebootInstanceInfo_Success() {
	var instanceManager instanceActivity.ManageInstance

	siteID := uuid.New()

	transactionID := &cwssaws.TransactionID{
		ResourceId: uuid.New().String(),
		Timestamp:  timestamppb.Now(),
	}

	instanceRebootInfo := &cwssaws.InstanceRebootInfo{
		Status:    cwssaws.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS,
		StatusMsg: "Instance reboot in progress",
		MachineId: &cwssaws.MachineId{Id: uuid.New().String()},
	}

	// Mock UpdateRebootInstanceInDB activity
	s.env.RegisterActivity(instanceManager.UpdateRebootInstanceInDB)
	s.env.OnActivity(instanceManager.UpdateRebootInstanceInDB, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// execute UpdateInstanceRebootInfo workflow
	s.env.ExecuteWorkflow(UpdateInstanceRebootInfo, siteID.String(), transactionID, instanceRebootInfo)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateInstanceTestSuite) Test_UpdateRebootInstanceInfo_ActivityFails() {
	var instanceManager instanceActivity.ManageInstance

	siteID := uuid.New()

	transactionID := &cwssaws.TransactionID{
		ResourceId: uuid.New().String(),
		Timestamp:  timestamppb.Now(),
	}

	instanceRebootInfo := &cwssaws.InstanceRebootInfo{
		Status:    cwssaws.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS,
		StatusMsg: "Instance reboot in progress",
		MachineId: &cwssaws.MachineId{Id: uuid.New().String()},
	}

	// Mock UpdateRebootInstanceInDB activity failure
	s.env.RegisterActivity(instanceManager.UpdateRebootInstanceInDB)
	s.env.OnActivity(instanceManager.UpdateRebootInstanceInDB, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("UpdateRebootInstanceInfo Failure"))

	// execute UpdateInstanceRebootInfo workflow
	s.env.ExecuteWorkflow(UpdateInstanceRebootInfo, siteID.String(), transactionID, instanceRebootInfo)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("UpdateRebootInstanceInfo Failure", applicationErr.Error())
}

func (s *UpdateInstanceTestSuite) Test_UpdateInstanceInventory_Success() {
	var instanceManager instanceActivity.ManageInstance
	var lifecycleMetricsManager instanceActivity.ManageInstanceLifecycleMetrics
	var inventoryMetricsManager cwm.ManageInventoryMetrics

	siteID := uuid.New()

	instanceInventory := &cwssaws.InstanceInventory{
		Instances: []*cwssaws.Instance{},
		Timestamp: timestamppb.Now(),
	}

	// Mock UpdateInstancesInDB activity
	s.env.RegisterActivity(instanceManager.UpdateInstancesInDB)
	s.env.OnActivity(instanceManager.UpdateInstancesInDB, mock.Anything, siteID, mock.Anything).Return([]cwm.InventoryObjectLifecycleEvent{}, nil)

	// Mock RecordInstanceStatusTransitionMetrics activity
	s.env.RegisterActivity(lifecycleMetricsManager.RecordInstanceStatusTransitionMetrics)
	s.env.OnActivity(lifecycleMetricsManager.RecordInstanceStatusTransitionMetrics, mock.Anything, siteID, mock.Anything).Return(nil)

	// Mock RecordLatency activity
	s.env.RegisterActivity(inventoryMetricsManager.RecordLatency)
	s.env.OnActivity(inventoryMetricsManager.RecordLatency, mock.Anything, siteID, "UpdateInstanceInventory", false, mock.Anything).Return(nil)

	// execute UpdateInstanceInventory workflow
	s.env.ExecuteWorkflow(UpdateInstanceInventory, siteID.String(), instanceInventory)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateInstanceTestSuite) Test_UpdateInstanceInventory_ActivityFails() {
	var instanceManager instanceActivity.ManageInstance

	siteID := uuid.New()

	instanceInventory := &cwssaws.InstanceInventory{
		Instances: []*cwssaws.Instance{},
		Timestamp: timestamppb.Now(),
	}

	// Mock UpdateInstancesInDB activity failure
	s.env.RegisterActivity(instanceManager.UpdateInstancesInDB)
	s.env.OnActivity(instanceManager.UpdateInstancesInDB, mock.Anything, mock.Anything, mock.Anything).Return([]cwm.InventoryObjectLifecycleEvent{}, errors.New("UpdateInstanceInventory Failure"))

	// execute UpdateInstanceInventory workflow
	s.env.ExecuteWorkflow(UpdateInstanceInventory, siteID.String(), instanceInventory)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("UpdateInstanceInventory Failure", applicationErr.Error())
}

func TestUpdateInstanceInfoSuite(t *testing.T) {
	suite.Run(t, new(UpdateInstanceTestSuite))
}
