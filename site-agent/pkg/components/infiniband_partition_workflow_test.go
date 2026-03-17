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
	ibp "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/managers/infinibandpartition"
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

type CreateInfiniBandPartitionFailureTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateInfiniBandPartitionFailureTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *CreateInfiniBandPartitionFailureTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

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

type DeleteInfiniBandPartitionTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteInfiniBandPartitionTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *DeleteInfiniBandPartitionTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateInfiniBandPartitionFailureTestSuite) TestCreateInfiniBandPartitionWorkflowFailure() {
	log.Info().Msg("TestCreateInfiniBandPartitionWorkflowFailure Start")
	wflowinstance := ibp.Workflows{}

	partitionID := uuid.NewString()
	req := &wflows.CreateInfiniBandPartitionRequest{
		IbPartitionId:        &wflows.UUID{Value: partitionID},
		Name:                 "test-InfiniBandPartition-1",
		TenantOrganizationId: "test-tenant-org",
	}

	transaction := &wflows.TransactionID{
		ResourceId: partitionID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock CreateInfiniBandPartitionActivity activity
	s.env.RegisterActivity(wflowinstance.CreateInfiniBandPartitionActivity)
	s.env.RegisterActivity(wflowinstance.PublishInfiniBandPartitionActivity)
	s.env.OnActivity(wflowinstance.PublishInfiniBandPartitionActivity, mock.Anything, transaction, mock.Anything).Return("", errors.New("PublishInfiniBandPartitionActivity Failure"))

	// Execute CreateInfiniBandPartition workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.InfiniBandPartition.CreateInfiniBandPartition, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)

	var applicationErr *temporal.ApplicationError
	s.True(errors.As(err, &applicationErr))
	log.Info().Msg("TestCreateInfiniBandPartitionWorkflowFailure End")
}

func (s *CreateInfiniBandPartitionTestSuite) TestCreateInfiniBandPartitionWorkflowSuccess() {
	log.Info().Msg("TestCreateInfiniBandPartitionWorkflowSuccess Start")
	wflowinstance := ibp.Workflows{}

	partitionID := DefaultTestIBParitionID

	req := &wflows.CreateInfiniBandPartitionRequest{
		IbPartitionId:        &wflows.UUID{Value: partitionID},
		Name:                 "test-InfiniBandPartition-2",
		TenantOrganizationId: "test-tenant-org",
	}

	transaction := &wflows.TransactionID{
		ResourceId: partitionID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock CreateInfiniBandPartitionActivity activity
	s.env.RegisterActivity(wflowinstance.CreateInfiniBandPartitionActivity)
	s.env.RegisterActivity(wflowinstance.PublishInfiniBandPartitionActivity)
	s.env.OnActivity(wflowinstance.PublishInfiniBandPartitionActivity, mock.Anything, transaction, mock.Anything).Return("", nil)

	// execute createInfiniBandPartition workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.InfiniBandPartition.CreateInfiniBandPartition, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	resourceResp := &wflows.InfiniBandPartitionInfo{}
	err := s.env.GetWorkflowResult(resourceResp)
	if err != nil {
		log.Info().Msg(err.Error())
	}
	log.Info().Msg("TestCreateInfiniBandPartitionWorkflowSuccess End")
}

func (s *DeleteInfiniBandPartitionTestSuite) TestDeleteInfiniBandPartitionWorkflowSuccess() {
	log.Info().Msg("TestDeleteInfiniBandPartitionWorkflowSuccess Start")
	wflowinstance := ibp.Workflows{}

	partitionID := DefaultTestIBParitionID
	req := &wflows.DeleteInfiniBandPartitionRequest{
		Id: &wflows.UUID{Value: partitionID},
	}

	transaction := &wflows.TransactionID{
		ResourceId: partitionID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	// Mock DeleteInfiniBandPartitionActivity activity
	s.env.RegisterActivity(wflowinstance.DeleteInfiniBandPartitionActivity)
	s.env.RegisterActivity(wflowinstance.PublishInfiniBandPartitionActivity)
	s.env.OnActivity(wflowinstance.PublishInfiniBandPartitionActivity, mock.Anything, transaction, mock.Anything).Return("", nil)

	// execute DeleteInfiniBandPartition workflow
	s.env.ExecuteWorkflow(testElektra.manager.API.InfiniBandPartition.DeleteInfiniBandPartition, transaction, req)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	resourceResp := &wflows.InfiniBandPartitionInfo{}
	err := s.env.GetWorkflowResult(resourceResp)
	if err != nil {
		log.Info().Msg(err.Error())
	}
	log.Info().Msg("TestDeleteInfiniBandPartitionWorkflowSuccess End")
}

// TestInfiniBandPartitionWorkflows tests various InfiniBandPartition workflows
func TestInfiniBandPartitionWorkflows(t *testing.T) {
	TestInitElektra(t)

	stats := ibp.ManagerAccess.Data.EB.Managers.Workflow.InfiniBandPartitionState
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

	_, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "CreateInfiniBandPartitionTestSuite")
	suite.Run(t, new(CreateInfiniBandPartitionTestSuite))
	span.End()

	// failures has multiple tries
	_, span = otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "CreateInfiniBandPartitionFailureTestSuite")
	wflowGrpcFail += 7
	wflowActFail++
	wflowPubFail++
	suite.Run(t, new(CreateInfiniBandPartitionFailureTestSuite))
	span.End()

	_, span = otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "UpdateInfiniBandPartitionTestSuite")
	wflowGrpcSucc++
	wflowActSucc++
	wflowPubSucc++
	suite.Run(t, new(UpdateInfiniBandPartitionTestSuite))
	span.End()

	_, span = otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(context.Background(), "DeleteInfiniBandPartitionTestSuite")
	wflowGrpcSucc++
	wflowActSucc++
	wflowPubSucc++
	suite.Run(t, new(DeleteInfiniBandPartitionTestSuite))
	span.End()

	computils.GetSAStatus(computils.InfiniBandPartitionStatus)
}
