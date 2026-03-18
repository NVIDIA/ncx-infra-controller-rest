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
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"

	tmocks "go.temporal.io/sdk/mocks"

	subnetActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/subnet"
)

type DeleteSubnetTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteSubnetTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *DeleteSubnetTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteSubnetTestSuite) Test_DeleteSubnetWorkflow_Success() {
	var subnetManager subnetActivity.ManageSubnet

	vpcID := uuid.New()
	subnetID := uuid.New()

	// Mock DeleteSubnetViaSiteAgent activity
	s.env.RegisterActivity(subnetManager.DeleteSubnetViaSiteAgent)
	s.env.OnActivity(subnetManager.DeleteSubnetViaSiteAgent, mock.Anything, subnetID, vpcID).Return(nil)

	// execute deleteVPC workflow
	s.env.ExecuteWorkflow(DeleteSubnet, subnetID, vpcID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteSubnetTestSuite) Test_DeleteSubnetWorkflow_ActivityFails() {
	var subnetManager subnetActivity.ManageSubnet

	vpcID := uuid.New()
	subnetID := uuid.New()

	// Mock DeleteSubnetViaSiteAgent activity failure
	s.env.RegisterActivity(subnetManager.DeleteSubnetViaSiteAgent)
	s.env.OnActivity(subnetManager.DeleteSubnetViaSiteAgent, mock.Anything, subnetID, vpcID).Return(errors.New("DeleteSubnetViaSiteAgent Failure"))

	// execute DeleteSubnet workflow
	s.env.ExecuteWorkflow(DeleteSubnet, subnetID, vpcID)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("DeleteSubnetViaSiteAgent Failure", applicationErr.Error())
}

func (s *DeleteSubnetTestSuite) Test_ExecuteDeleteSubnetWorkflow_Success() {
	ctx := context.Background()

	vpcID := uuid.New()
	subnetID := uuid.New()

	wid := "test-workflow-id"

	wrun := &tmocks.WorkflowRun{}
	wrun.On("GetID").Return(wid)

	tc := &tmocks.Client{}

	tc.Mock.On("ExecuteWorkflow", context.Background(), mock.AnythingOfType("internal.StartWorkflowOptions"),
		mock.Anything, subnetID, vpcID).Return(wrun, nil)

	rwid, err := ExecuteDeleteSubnetWorkflow(ctx, tc, subnetID, vpcID)
	s.NoError(err)
	s.Equal(wid, *rwid)
}

func TestDeleteSubnetSuite(t *testing.T) {
	suite.Run(t, new(DeleteSubnetTestSuite))
}
