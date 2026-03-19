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

package infinibandpartition

import (
	"errors"
	"testing"

	ibpActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/infinibandpartition"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/types/known/timestamppb"

	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

type UpdateInfiniBandPartitionTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateInfiniBandPartitionTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *UpdateInfiniBandPartitionTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateInfiniBandPartitionTestSuite) Test_UpdateInfiniBandPartitionInfo_Success() {
	var InfiniBandPartitionManager ibpActivity.ManageInfiniBandPartition

	siteID := uuid.New()

	transactionID := &cwssaws.TransactionID{
		ResourceId: uuid.New().String(),
		Timestamp:  timestamppb.Now(),
	}

	ibpInfo := &cwssaws.InfiniBandPartitionInfo{
		Status:    cwssaws.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS,
		StatusMsg: "InfiniBandPartition creation in progress",
		IbPartition: &cwssaws.IBPartition{
			Id: &cwssaws.IBPartitionId{Value: uuid.New().String()},
			Config: &cwssaws.IBPartitionConfig{
				Name:                 uuid.New().String(),
				TenantOrganizationId: uuid.NewString(),
			},
		},
	}

	// Mock UpdateInfiniBandPartitionViaSiteAgent activity
	s.env.RegisterActivity(InfiniBandPartitionManager.UpdateInfiniBandPartitionInDB)
	s.env.OnActivity(InfiniBandPartitionManager.UpdateInfiniBandPartitionInDB, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// execute UpdateInfiniBandPartitionInfo workflow
	s.env.ExecuteWorkflow(UpdateInfiniBandPartitionInfo, siteID.String(), transactionID, ibpInfo)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateInfiniBandPartitionTestSuite) Test_UpdateInfiniBandPartitionInfo_ActivityFails() {

	var InfiniBandPartitionManager ibpActivity.ManageInfiniBandPartition

	siteID := uuid.New()

	transactionID := &cwssaws.TransactionID{
		ResourceId: uuid.New().String(),
		Timestamp:  timestamppb.Now(),
	}

	ibpInfo := &cwssaws.InfiniBandPartitionInfo{
		Status:    cwssaws.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS,
		StatusMsg: "InfiniBandPartition creation in progress",
		IbPartition: &cwssaws.IBPartition{
			Id: &cwssaws.IBPartitionId{Value: uuid.New().String()},
			Config: &cwssaws.IBPartitionConfig{
				Name:                 uuid.New().String(),
				TenantOrganizationId: uuid.NewString(),
			},
		},
	}

	// Mock UpdateInfiniBandPartitionViaSiteAgent activity failure
	s.env.RegisterActivity(InfiniBandPartitionManager.UpdateInfiniBandPartitionInDB)
	s.env.OnActivity(InfiniBandPartitionManager.UpdateInfiniBandPartitionInDB, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("UpdateInfiniBandPartitionInfo Failure"))

	// execute UpdateInfiniBandPartitionStatus workflow
	s.env.ExecuteWorkflow(UpdateInfiniBandPartitionInfo, siteID.String(), transactionID, ibpInfo)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("UpdateInfiniBandPartitionInfo Failure", applicationErr.Error())
}

func (s *UpdateInfiniBandPartitionTestSuite) Test_UpdateInfiniBandPartitionInventory_Success() {

	var ibpManager ibpActivity.ManageInfiniBandPartition

	siteID := uuid.New()
	ibpInventory := &cwssaws.InfiniBandPartitionInventory{
		IbPartitions: []*cwssaws.IBPartition{
			{
				Id: &cwssaws.IBPartitionId{Value: uuid.NewString()},
			},
			{
				Id: &cwssaws.IBPartitionId{Value: uuid.NewString()},
			},
		},
	}

	// Mock UpdateInfiniBandPartitionViaSiteAgent activity
	s.env.RegisterActivity(ibpManager.UpdateInfiniBandPartitionsInDB)
	s.env.OnActivity(ibpManager.UpdateInfiniBandPartitionsInDB, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// execute UpdateInfiniBandPartitionInventory workflow
	s.env.ExecuteWorkflow(UpdateInfiniBandPartitionInventory, siteID.String(), ibpInventory)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateInfiniBandPartitionTestSuite) Test_UpdateInfiniBandPartitionInventory_ActivityFails() {

	var ibpManager ibpActivity.ManageInfiniBandPartition

	siteID := uuid.New()
	InfiniBandPartitionInventory := &cwssaws.InfiniBandPartitionInventory{
		IbPartitions: []*cwssaws.IBPartition{
			{
				Id: &cwssaws.IBPartitionId{Value: uuid.NewString()},
			},
			{
				Id: &cwssaws.IBPartitionId{Value: uuid.NewString()},
			},
		},
	}

	// Mock UpdateInfiniBandPartitionsViaSiteAgent activity failure
	s.env.RegisterActivity(ibpManager.UpdateInfiniBandPartitionsInDB)
	s.env.OnActivity(ibpManager.UpdateInfiniBandPartitionsInDB, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("UpdateInfiniBandPartitionInventory Failure"))

	// execute UpdateInfiniBandPartitionStatus workflow
	s.env.ExecuteWorkflow(UpdateInfiniBandPartitionInventory, siteID.String(), InfiniBandPartitionInventory)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("UpdateInfiniBandPartitionInventory Failure", applicationErr.Error())
}

func TestUpdateInfiniBandPartitionSuite(t *testing.T) {
	suite.Run(t, new(UpdateInfiniBandPartitionTestSuite))
}
