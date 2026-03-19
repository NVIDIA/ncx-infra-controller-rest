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

package sshkeygroup

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/types/known/timestamppb"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"

	sshKeyGroupActivity "github.com/NVIDIA/ncx-infra-controller-rest/workflow/pkg/activity/sshkeygroup"
)

type UpdateSSHKeyGroupTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateSSHKeyGroupTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *UpdateSSHKeyGroupTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateSSHKeyGroupTestSuite) Test_UpdateSSHKeyGroupInfo_Success() {
	var sshKeyGroupManager sshKeyGroupActivity.ManageSSHKeyGroup

	siteID := uuid.New()
	sshKeyGroupID := cdb.GetStrPtr(uuid.New().String())

	transactionID := &cwssaws.TransactionID{
		ResourceId: uuid.New().String(),
		Timestamp:  timestamppb.Now(),
	}

	sshKeyGroupInfo := &cwssaws.SSHKeyGroupInfo{
		Status:    cwssaws.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS,
		StatusMsg: "SSHKeyGroup syncing in progress",
		TenantKeyset: &cwssaws.TenantKeyset{
			Version: "1234",
		},
	}

	// Mock updateSSHKeyGroupViaSiteAgent activity
	s.env.RegisterActivity(sshKeyGroupManager.UpdateSSHKeyGroupInDB)
	s.env.RegisterActivity(sshKeyGroupManager.UpdateSSHKeyGroupStatusInDB)
	s.env.OnActivity(sshKeyGroupManager.UpdateSSHKeyGroupInDB, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(sshKeyGroupID, nil)
	s.env.OnActivity(sshKeyGroupManager.UpdateSSHKeyGroupStatusInDB, mock.Anything, *sshKeyGroupID).Return(nil)

	// execute updateSSHKeyGroupInfo workflow
	s.env.ExecuteWorkflow(UpdateSSHKeyGroupInfo, siteID.String(), transactionID, sshKeyGroupInfo)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateSSHKeyGroupTestSuite) Test_UpdateSSHKeyGroupInfo_ActivityFails() {
	var sshKeyGroupManager sshKeyGroupActivity.ManageSSHKeyGroup

	siteID := uuid.New()

	transactionID := &cwssaws.TransactionID{
		ResourceId: uuid.New().String(),
		Timestamp:  timestamppb.Now(),
	}

	sshKeyGroupInfo := &cwssaws.SSHKeyGroupInfo{
		Status:    cwssaws.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS,
		StatusMsg: "SSHKeyGroup syncing in progress",
		TenantKeyset: &cwssaws.TenantKeyset{
			Version: "1234",
		},
	}

	// Mock updateSSHKeyGroupViaSiteAgent activity failure
	s.env.RegisterActivity(sshKeyGroupManager.UpdateSSHKeyGroupInDB)
	s.env.RegisterActivity(sshKeyGroupManager.UpdateSSHKeyGroupStatusInDB)
	s.env.OnActivity(sshKeyGroupManager.UpdateSSHKeyGroupInDB, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("UpdateSSHKeyGroupInDB Failure"))

	// execute updateSSHKeyGroupStatus workflow
	s.env.ExecuteWorkflow(UpdateSSHKeyGroupInfo, siteID.String(), transactionID, sshKeyGroupInfo)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("UpdateSSHKeyGroupInDB Failure", applicationErr.Error())
}

func (s *UpdateSSHKeyGroupTestSuite) Test_UpdateSSHKeyGroupInfo_UpdateSSHKeyGroupStatusInDBActivityFails() {
	var sshKeyGroupManager sshKeyGroupActivity.ManageSSHKeyGroup

	siteID := uuid.New()
	sshKeyGroupIDStr := uuid.NewString()

	transactionID := &cwssaws.TransactionID{
		ResourceId: sshKeyGroupIDStr,
		Timestamp:  timestamppb.Now(),
	}

	sshKeyGroupInfo := &cwssaws.SSHKeyGroupInfo{
		Status:    cwssaws.WorkflowStatus_WORKFLOW_STATUS_IN_PROGRESS,
		StatusMsg: "SSHKeyGroup syncing in progress",
		TenantKeyset: &cwssaws.TenantKeyset{
			Version: "1234",
		},
	}

	// Mock updateSSHKeyGroupViaSiteAgent activity failure
	s.env.RegisterActivity(sshKeyGroupManager.UpdateSSHKeyGroupInDB)
	s.env.RegisterActivity(sshKeyGroupManager.UpdateSSHKeyGroupStatusInDB)
	s.env.OnActivity(sshKeyGroupManager.UpdateSSHKeyGroupInDB, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&sshKeyGroupIDStr, nil)
	s.env.OnActivity(sshKeyGroupManager.UpdateSSHKeyGroupStatusInDB, mock.Anything, sshKeyGroupIDStr).Return(errors.New("UpdateSSHKeyGroupStatusInDB Failure"))

	// execute updateSSHKeyGroupStatus workflow
	s.env.ExecuteWorkflow(UpdateSSHKeyGroupInfo, siteID.String(), transactionID, sshKeyGroupInfo)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("UpdateSSHKeyGroupStatusInDB Failure", applicationErr.Error())
}

func (s *UpdateSSHKeyGroupTestSuite) Test_UpdateSSHKeyGroupInventory_Success() {
	var sshKeyGroupManager sshKeyGroupActivity.ManageSSHKeyGroup

	siteID := uuid.New()
	sshKeyGroupIDs := []string{uuid.New().String(), uuid.New().String()}

	sshKeyGroupInventory := &cwssaws.SSHKeyGroupInventory{
		TenantKeysets: []*cwssaws.TenantKeyset{
			{
				KeysetIdentifier: &cwssaws.TenantKeysetIdentifier{
					KeysetId: "1234",
				},
				Version: "1234",
			},
			{
				KeysetIdentifier: &cwssaws.TenantKeysetIdentifier{
					KeysetId: "1235",
				},
				Version: "1235",
			},
		},
	}

	// Mock UpdateSSHKeyGroupsInDB activity
	s.env.RegisterActivity(sshKeyGroupManager.UpdateSSHKeyGroupsInDB)
	s.env.OnActivity(sshKeyGroupManager.UpdateSSHKeyGroupsInDB, mock.Anything, mock.Anything, mock.Anything).Return(sshKeyGroupIDs, nil)
	s.env.OnActivity(sshKeyGroupManager.UpdateSSHKeyGroupStatusInDB, mock.Anything, mock.Anything).Return(nil)

	// execute UpdateSSHKeyGroupInventory workflow
	s.env.ExecuteWorkflow(UpdateSSHKeyGroupInventory, siteID.String(), sshKeyGroupInventory)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateSSHKeyGroupTestSuite) Test_UpdateSSHKeyGroupInventory_ActivityFails() {
	var sshKeyGroupManager sshKeyGroupActivity.ManageSSHKeyGroup

	siteID := uuid.New()
	sshKeyGroupInventory := &cwssaws.SSHKeyGroupInventory{
		TenantKeysets: []*cwssaws.TenantKeyset{
			{
				KeysetIdentifier: &cwssaws.TenantKeysetIdentifier{
					KeysetId: "1234",
				},
				Version: "1234",
			},
			{
				KeysetIdentifier: &cwssaws.TenantKeysetIdentifier{
					KeysetId: "1235",
				},
				Version: "1235",
			},
		},
	}

	// Mock UpdateVpcsViaSiteAgent activity failure
	s.env.RegisterActivity(sshKeyGroupManager.UpdateSSHKeyGroupsInDB)
	s.env.OnActivity(sshKeyGroupManager.UpdateSSHKeyGroupsInDB, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("UpdateSSHKeyGroupInventory Failure"))

	// execute UpdateVPCStatus workflow
	s.env.ExecuteWorkflow(UpdateSSHKeyGroupInventory, siteID.String(), sshKeyGroupInventory)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	s.Equal("UpdateSSHKeyGroupInventory Failure", applicationErr.Error())
}

func TestUpdateSSHKeyGroupSuite(t *testing.T) {
	suite.Run(t, new(UpdateSSHKeyGroupTestSuite))
}
