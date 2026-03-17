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
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tmocks "go.temporal.io/sdk/mocks"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"

	ibpActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/infinibandpartition"
)

type CreateInfiniBandPartitionTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateInfiniBandPartitionTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *CreateInfiniBandPartitionTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateInfiniBandPartitionTestSuite) Test_CreateInfiniBandPartitionWorkflow_Success() {
	var ibpManager ibpActivity.ManageInfiniBandPartition

	siteID := uuid.New()
	ibpID := uuid.New()

	// Mock CreateInfiniBandPartitionViaSiteAgent activity
	s.env.RegisterActivity(ibpManager.CreateInfiniBandPartitionViaSiteAgent)
	s.env.OnActivity(ibpManager.CreateInfiniBandPartitionViaSiteAgent, mock.Anything, siteID, ibpID).Return(nil)

	// execute createInfiniBandPartition workflow
	s.env.ExecuteWorkflow(CreateInfiniBandPartition, siteID, ibpID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateInfiniBandPartitionTestSuite) Test_CreateInfiniBandPartitionWorkflow_ActivityFails() {

	var ibpManager ibpActivity.ManageInfiniBandPartition

	siteID := uuid.New()
	ibpID := uuid.New()

	// Mock CreateInfiniBandPartitionViaSiteAgent activity failure
	s.env.RegisterActivity(ibpManager.CreateInfiniBandPartitionViaSiteAgent)
	s.env.OnActivity(ibpManager.CreateInfiniBandPartitionViaSiteAgent, mock.Anything, siteID, ibpID).Return(errors.New("CreateInfiniBandPartitionViaSiteAgent Failure"))

	// execute createInfiniBandPartition workflow
	s.env.ExecuteWorkflow(CreateInfiniBandPartition, siteID, ibpID)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("CreateInfiniBandPartitionViaSiteAgent Failure", applicationErr.Error())
}

func (s *CreateInfiniBandPartitionTestSuite) Test_ExecuteCreateInfiniBandPartitionWorkflow_Success() {
	ctx := context.Background()
	siteID := uuid.New()
	ibpID := uuid.New()

	wid := "test-workflow-id"

	wrun := &tmocks.WorkflowRun{}
	wrun.On("GetID").Return(wid)

	tc := &tmocks.Client{}

	tc.Mock.On("ExecuteWorkflow", context.Background(), mock.AnythingOfType("internal.StartWorkflowOptions"),
		mock.Anything, siteID, ibpID).Return(wrun, nil)

	rwid, err := ExecuteCreateInfiniBandPartitionWorkflow(ctx, tc, siteID, ibpID)
	s.NoError(err)
	s.Equal(wid, *rwid)
}

func TestCreateInfiniBandPartitionSuite(t *testing.T) {
	suite.Run(t, new(CreateInfiniBandPartitionTestSuite))
}
