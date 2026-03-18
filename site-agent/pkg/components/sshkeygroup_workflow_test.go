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

package elektra

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/managers/carbide"
	ibp "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/managers/sshkeygroup"
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/types/known/timestamppb"

	log "github.com/rs/zerolog/log"
)

type CreateSSHKeyGroupTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateSSHKeyGroupTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *CreateSSHKeyGroupTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

type CreateSSHKeyGroupFailureTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateSSHKeyGroupFailureTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *CreateSSHKeyGroupFailureTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

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

type DeleteSSHKeyGroupTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteSSHKeyGroupTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *DeleteSSHKeyGroupTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateSSHKeyGroupTestSuite) TestCreateSSHKeyGroupWorkflowSuccess() {
	log.Info().Msg("TestCreateSSHKeyGroupWorkflowSuccess Start")
	wflowinstance := ibp.Workflows{}

	keySetID := uuid.NewString()
	req := &wflows.CreateSSHKeyGroupRequest{
		KeysetId: keySetID,
		PublicKeys: []*wflows.TenantPublicKey{
			{
				PublicKey: "test-key",
			},
		},
		TenantOrganizationId: "test-tenant-org",
		Version:              "121312410",
	}

	transaction := &wflows.TransactionID{
		ResourceId: keySetID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock CreateSSHKeyGroupActivity activity
	s.env.RegisterActivity(wflowinstance.CreateSSHKeyGroupActivity)
	s.env.RegisterActivity(wflowinstance.PublishSSHKeyGroupActivity)
	s.env.OnActivity(wflowinstance.PublishSSHKeyGroupActivity, mock.Anything, transaction, mock.Anything).Return("", nil)

	// execute createSSHKeyGroup workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.SSHKeyGroup.CreateSSHKeyGroup, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	resourceResp := &wflows.SSHKeyGroupInfo{}
	err := s.env.GetWorkflowResult(resourceResp)
	if err != nil {
		log.Info().Err(err).Msg(err.Error())
	}
	log.Info().Msg("TestCreateSSHKeyGroupWorkflowSuccess End")
}

func (s *CreateSSHKeyGroupFailureTestSuite) TestCreateSSHKeyGroupWorkflowFailure() {
	log.Info().Msg("TestCreateSSHKeyGroupWorkflowFailure Start")
	wflowinstance := ibp.Workflows{}

	keySetID := uuid.NewString()
	req := &wflows.CreateSSHKeyGroupRequest{
		KeysetId: keySetID,
		PublicKeys: []*wflows.TenantPublicKey{
			{
				PublicKey: "test-key",
			},
		},
		TenantOrganizationId: "test-tenant-org",
		Version:              "121312410",
	}

	transaction := &wflows.TransactionID{
		ResourceId: keySetID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock CreateSSHKeyGroupActivity activity
	s.env.RegisterActivity(wflowinstance.CreateSSHKeyGroupActivity)
	s.env.RegisterActivity(wflowinstance.PublishSSHKeyGroupActivity)
	s.env.OnActivity(wflowinstance.PublishSSHKeyGroupActivity, mock.Anything, transaction, mock.Anything).Return("", errors.New("CreateSSHKeyGroupViaSiteAgent Failure"))

	// Execute CreateSSHKeyGroup workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.SSHKeyGroup.CreateSSHKeyGroup, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	log.Info().Msg("TestCreateSSHKeyGroupWorkflowFailure End")
}

func (s *UpdateSSHKeyGroupTestSuite) TestUpdateSSHKeyGroupWorkflowSuccess() {
	log.Info().Msg("TestUpdateSSHKeyGroupWorkflowSuccess Start")
	wflowinstance := ibp.Workflows{}

	keySetID := DefaultTestTenantKeysetID
	req := &wflows.UpdateSSHKeyGroupRequest{
		KeysetId: keySetID,
		PublicKeys: []*wflows.TenantPublicKey{
			{
				PublicKey: "test-key",
			},
			{
				PublicKey: "test-key-2",
			},
		},
		TenantOrganizationId: "test-tenant-org",
		Version:              "121312411",
	}

	transaction := &wflows.TransactionID{
		ResourceId: keySetID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock CreateSSHKeyGroupActivity activity
	s.env.RegisterActivity(wflowinstance.UpdateSSHKeyGroupActivity)
	s.env.RegisterActivity(wflowinstance.PublishSSHKeyGroupActivity)
	s.env.OnActivity(wflowinstance.PublishSSHKeyGroupActivity, mock.Anything, transaction, mock.Anything).Return("", nil)

	// execute createSSHKeyGroup workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.SSHKeyGroup.UpdateSSHKeyGroup, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	resourceResp := &wflows.SSHKeyGroupInfo{}
	err := s.env.GetWorkflowResult(resourceResp)
	if err != nil {
		log.Info().Err(err).Msg(err.Error())
	}
	log.Info().Msg("TestUpdateSSHKeyGroupWorkflowSuccess End")
}

func (s *DeleteSSHKeyGroupTestSuite) TestDeleteSSHKeyGroupWorkflowSuccess() {
	log.Info().Msg("TestDeleteSSHKeyGroupWorkflowSuccess Start")
	wflowinstance := ibp.Workflows{}

	keySetID := DefaultTestTenantKeysetID
	req := &wflows.DeleteSSHKeyGroupRequest{
		KeysetId:             keySetID,
		TenantOrganizationId: "test-tenant-org",
	}

	transaction := &wflows.TransactionID{
		ResourceId: keySetID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock DeleteSSHKeyGroupActivity activity
	s.env.RegisterActivity(wflowinstance.DeleteSSHKeyGroupActivity)
	s.env.RegisterActivity(wflowinstance.PublishSSHKeyGroupActivity)
	s.env.OnActivity(wflowinstance.PublishSSHKeyGroupActivity, mock.Anything, transaction, mock.Anything).Return("", nil)

	// execute DeleteSSHKeyGroup workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.SSHKeyGroup.DeleteSSHKeyGroup, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	resourceResp := &wflows.SSHKeyGroupInfo{}
	err := s.env.GetWorkflowResult(resourceResp)
	if err != nil {
		log.Info().Msg(err.Error())
	}
	log.Info().Msg("TestDeleteSSHKeyGroupWorkflowSuccess End")
}

// TestSSHKeyGroupWorkflows tests various SSHKeyGroup workflows
func TestSSHKeyGroupWorkflows(t *testing.T) {
	TestInitElektra(t)

	stats := ibp.ManagerAccess.Data.EB.Managers.Workflow.SSHKeyGroupState
	stats.WflowActFail.Store(0)
	stats.WflowActSucc.Store(0)
	stats.WflowPubFail.Store(0)
	stats.WflowPubSucc.Store(0)
	wflowActFail = 0
	wflowActSucc = 1
	wflowPubFail = 0
	wflowPubSucc = 1

	carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcFail.Store(0)
	carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcSucc.Store(0)
	wflowGrpcFail = 0
	wflowGrpcSucc = 1

	_, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "CreateSSHKeyGroupTestSuite")
	suite.Run(t, new(CreateSSHKeyGroupTestSuite))
	span.End()

	// failures has multiple tries
	_, span = otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "CreateSSHKeyGroupFailureTestSuite")
	wflowGrpcFail += 7
	wflowActFail++
	wflowPubFail++
	suite.Run(t, new(CreateSSHKeyGroupFailureTestSuite))
	span.End()

	_, span = otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "UpdateSSHKeyGroupTestSuite")
	wflowGrpcSucc++
	wflowActSucc++
	wflowPubSucc++
	suite.Run(t, new(UpdateSSHKeyGroupTestSuite))
	span.End()

	_, span = otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "DeleteSSHKeyGroupTestSuite")
	wflowGrpcSucc++
	wflowActSucc++
	wflowPubSucc++
	suite.Run(t, new(DeleteSSHKeyGroupTestSuite))
	span.End()
}
