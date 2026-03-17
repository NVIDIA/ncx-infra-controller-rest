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
	"github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/managers/vpc"
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/types/known/timestamppb"

	computils "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/utils"
	log "github.com/rs/zerolog/log"
)

type CreateVpcTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateVpcTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *CreateVpcTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

type CreateVpcFailureTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateVpcFailureTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *CreateVpcFailureTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

type UpdateVpcTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateVpcTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *UpdateVpcTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

type GetVpcTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *GetVpcTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *GetVpcTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

type DeleteVpcTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteVpcTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *DeleteVpcTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateVpcFailureTestSuite) TestCreateVPCWorkflowFailure() {
	log.Info().Msg("TestCreateVPCWorkflowFailure Start")
	vpcID := uuid.NewString()
	vpcwflowinstance := vpc.Workflows{}
	req := &wflows.CreateVPCRequest{
		VpcId:                &wflows.UUID{Value: vpcID},
		Name:                 "test-vpc",
		TenantOrganizationId: "test-tenant-org",
	}

	transaction := &wflows.TransactionID{
		ResourceId: vpcID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock CreateVPCActivity activity
	s.env.RegisterActivity(vpcwflowinstance.CreateVPCActivity)
	s.env.RegisterActivity(vpcwflowinstance.PublishVPCActivity)
	s.env.OnActivity(vpcwflowinstance.PublishVPCActivity, mock.Anything, transaction, mock.Anything).Return("", errors.New("PublishVPCActivity Failure"))

	// execute createVPC workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.VPC.CreateVPC, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	log.Info().Msg("TestCreateVPCWorkflowFailure End")
}

func (s *CreateVpcTestSuite) TestCreateVPCWorkflowSuccess() {
	log.Info().Msg("TestCreateVPCWorkflowSuccess Start")
	vpcwflowinstance := vpc.Workflows{}
	vpcID := uuid.NewString()
	vpcReq := &wflows.CreateVPCRequest{
		VpcId:                &wflows.UUID{Value: vpcID},
		Name:                 "test-vpc",
		TenantOrganizationId: "test-tenant-org",
	}

	transaction := &wflows.TransactionID{
		ResourceId: vpcID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock CreateVPCActivity activity
	s.env.RegisterActivity(vpcwflowinstance.CreateVPCActivity)
	s.env.RegisterActivity(vpcwflowinstance.PublishVPCActivity)
	s.env.OnActivity(vpcwflowinstance.PublishVPCActivity, mock.Anything, transaction, mock.Anything).Return("", nil)

	// execute createVPC workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.VPC.CreateVPC, transaction, vpcReq)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	resourceResp := &wflows.VPCInfo{}
	err := s.env.GetWorkflowResult(resourceResp)
	if err != nil {
		log.Info().Msg(err.Error())
	}
	log.Info().Msg("TestCreateVPCWorkflowSuccess End")
}

func (s *GetVpcTestSuite) TestGetVPCWorkflow() {
	log.Info().Msg("TestGetVPCWorkflow Start")
	vpcwflowinstance := vpc.Workflows{}

	// Mock CreateVPCActivity activity
	s.env.RegisterActivity(vpcwflowinstance.GetVPCByNameActivity)
	s.env.RegisterActivity(vpcwflowinstance.PublishVPCListActivity)

	// Execute CreateVPC workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.VPC.GetVPCByName, uuid.NewString(), "test-vpc")
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	resourceResp := &wflows.GetVPCResponse{}
	err := s.env.GetWorkflowResult(resourceResp)
	if err != nil {
		log.Info().Msg(err.Error())
	}
	log.Info().Msg("TestGetVPCWorkflow End")
}

func (s *UpdateVpcTestSuite) TestUpdateVPCWorkflowSuccess() {
	log.Info().Msg("TestUpdateVPCWorkflowSuccess Start")
	vpcwflowinstance := vpc.Workflows{}
	req := &wflows.UpdateVPCRequest{
		Id:                   &wflows.UUID{Value: DefaultTestVpcID},
		Name:                 "test-vpc",
		TenantOrganizationId: "test-tenant-org",
	}

	transaction := &wflows.TransactionID{
		ResourceId: req.Id.Value,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock UpdateVPCActivity activity
	s.env.RegisterActivity(vpcwflowinstance.UpdateVPCActivity)
	s.env.RegisterActivity(vpcwflowinstance.PublishVPCActivity)
	s.env.OnActivity(vpcwflowinstance.PublishVPCActivity, mock.Anything, transaction, mock.Anything).Return("", nil)

	// execute UpdateVPC workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.VPC.UpdateVPC, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	resourceResp := &wflows.VPCInfo{}
	err := s.env.GetWorkflowResult(resourceResp)
	if err != nil {
		log.Info().Msg(err.Error())
	}
	log.Info().Msg("TestUpdateVPCWorkflowSuccess End")
}

func (s *DeleteVpcTestSuite) TestDeleteVPCWorkflowSuccess() {
	log.Info().Msg("TestDeleteVPCWorkflowSuccess Start")
	vpcwflowinstance := vpc.Workflows{}
	req := &wflows.DeleteVPCRequest{
		Id: &wflows.UUID{Value: DefaultTestVpcID},
	}

	transaction := &wflows.TransactionID{
		ResourceId: req.Id.Value,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock DeleteVPCActivity activity
	s.env.RegisterActivity(vpcwflowinstance.DeleteVPCActivity)
	s.env.RegisterActivity(vpcwflowinstance.PublishVPCActivity)
	s.env.OnActivity(vpcwflowinstance.PublishVPCActivity, mock.Anything, transaction, mock.Anything).Return("", nil)

	// execute DeleteVPC workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.VPC.DeleteVPC, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	resourceResp := &wflows.VPCInfo{}
	err := s.env.GetWorkflowResult(resourceResp)
	if err != nil {
		log.Info().Msg(err.Error())
	}
	log.Info().Msg("TestDeleteVPCWorkflowSuccess End")
}

// TestVpcWorkflows tests various VPC workflows
func TestVpcWorkflows(t *testing.T) {
	TestInitElektra(t)

	stats := vpc.ManagerAccess.Data.EB.Managers.Workflow.VpcState
	stats.WflowActFail.Store(0)
	stats.WflowActSucc.Store(0)
	stats.WflowPubFail.Store(0)
	stats.WflowPubSucc.Store(0)
	wflowActFail = 0
	wflowActSucc = 0
	wflowPubFail = 0
	wflowPubSucc = 0

	_, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "CreateVpcTestSuite")
	carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcFail.Store(0)
	carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcSucc.Store(0)
	wflowGrpcFail = 0
	wflowGrpcSucc = 1
	wflowActSucc++
	wflowPubSucc++
	suite.Run(t, new(CreateVpcTestSuite))
	span.End()

	_, span = otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "CreateVpcFailureTestSuite")
	// failures has multiple tries
	wflowGrpcSucc++
	wflowActSucc++
	wflowPubFail++
	suite.Run(t, new(CreateVpcFailureTestSuite))
	span.End()

	_, span = otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "GetVpcTestSuite")
	wflowGrpcSucc++
	wflowActSucc++
	wflowPubSucc++
	suite.Run(t, new(GetVpcTestSuite))
	span.End()

	_, span = otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "DeleteVpcTestSuite")
	wflowGrpcSucc++
	wflowActSucc++
	wflowPubSucc++
	suite.Run(t, new(DeleteVpcTestSuite))
	span.End()

	computils.GetSAStatus(computils.VPCStatus)
	time.Sleep(16 * time.Second)
}
