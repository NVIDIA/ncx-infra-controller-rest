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
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tmocks "go.temporal.io/sdk/mocks"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"

	instanceActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/instance"
)

type CreateInstanceTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateInstanceTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *CreateInstanceTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateInstanceTestSuite) Test_CreateInstanceWorkflow_Success() {
	var instanceManager instanceActivity.ManageInstance

	instanceID := uuid.New()

	// Mock CreateInstanceViaSiteAgent activity
	s.env.RegisterActivity(instanceManager.CreateInstanceViaSiteAgent)
	s.env.OnActivity(instanceManager.CreateInstanceViaSiteAgent, mock.Anything, instanceID).Return(nil)

	// execute createInstance workflow
	s.env.ExecuteWorkflow(CreateInstance, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateInstanceTestSuite) Test_CreateInstanceWorkflow_CreateInstanceViaSiteAgentActivityFailsErrorActivitySuccess() {
	var instanceManager instanceActivity.ManageInstance

	instanceID := uuid.New()

	// Mock CreateInstanceViaSiteAgent activity failure
	s.env.RegisterActivity(instanceManager.CreateInstanceViaSiteAgent)
	s.env.OnActivity(instanceManager.CreateInstanceViaSiteAgent, mock.Anything, instanceID).Return(errors.New("CreateInstanceViaSiteAgent Failure"))

	// Mock CreateInstanceError activity success
	s.env.RegisterActivity(instanceManager.OnCreateInstanceError)
	s.env.OnActivity(instanceManager.OnCreateInstanceError, mock.Anything, instanceID, mock.Anything).Return(nil)

	// execute createInstance workflow
	s.env.ExecuteWorkflow(CreateInstance, instanceID)
	s.True(s.env.IsWorkflowCompleted())

	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateInstanceTestSuite) Test_CreateInstanceWorkflow_CreateInstanceViaSiteAgentActivityFailsAndErrorActivityFails() {
	var instanceManager instanceActivity.ManageInstance

	instanceID := uuid.New()

	// Mock CreateInstanceViaSiteAgent activity failure
	s.env.RegisterActivity(instanceManager.CreateInstanceViaSiteAgent)
	s.env.OnActivity(instanceManager.CreateInstanceViaSiteAgent, mock.Anything, instanceID).Return(errors.New("CreateInstanceViaSiteAgent Failure"))

	// Mock CreateInstanceError activity fails
	s.env.RegisterActivity(instanceManager.OnCreateInstanceError)
	s.env.OnActivity(instanceManager.OnCreateInstanceError, mock.Anything, instanceID, mock.Anything).Return(errors.New("CreateInstanceError Failure"))

	// execute createInstance workflow
	s.env.ExecuteWorkflow(CreateInstance, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("CreateInstanceError Failure", applicationErr.Error())
}

func (s *CreateInstanceTestSuite) Test_ExecuteCreateInstanceWorkflow_Success() {
	ctx := context.Background()

	instanceID := uuid.New()

	wid := "test-workflow-id"

	wrun := &tmocks.WorkflowRun{}
	wrun.On("GetID").Return(wid)

	tc := &tmocks.Client{}

	tc.Mock.On("ExecuteWorkflow", context.Background(), mock.AnythingOfType("internal.StartWorkflowOptions"), mock.Anything,
		instanceID).Return(wrun, nil)

	rwid, err := ExecuteCreateInstanceWorkflow(ctx, tc, instanceID)
	s.NoError(err)
	s.Equal(wid, *rwid)
}

func (s *CreateInstanceTestSuite) Test_ExecuteCreateInstanceWorkflow_Failure() {
	ctx := context.Background()

	instanceID := uuid.New()

	wid := "test-workflow-id"

	wrun := &tmocks.WorkflowRun{}
	wrun.On("GetID").Return(wid)

	tc := &tmocks.Client{}

	tc.Mock.On("ExecuteWorkflow", context.Background(), mock.AnythingOfType("internal.StartWorkflowOptions"), mock.Anything,
		instanceID).Return(wrun, fmt.Errorf("failed to execute workflow"))

	_, err := ExecuteCreateInstanceWorkflow(ctx, tc, instanceID)
	s.Error(err)
}

func TestCreateInstanceSuite(t *testing.T) {
	suite.Run(t, new(CreateInstanceTestSuite))
}
