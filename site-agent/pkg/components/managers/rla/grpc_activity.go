/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package rla

import (
	"context"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/grpc/client"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/log"
)

// CreateGRPCClientActivity - Create GRPC client Activity
func (RLA *API) CreateGRPCClientActivity(ctx context.Context, ResourceID string) (client *client.RlaClient, err error) {
	ManagerAccess.Data.EB.Log.Info().Interface("Request", ResourceID).Msg("RLA: Starting the gRPC connection Activity")

	// Use temporal logger for temporal logs
	logger := activity.GetLogger(ctx)
	withLogger := log.With(logger, "Activity", "CreateGRPCClientActivity", "ResourceReq", ResourceID)
	withLogger.Info("RLA: Starting the gRPC connection Activity")

	// Create the client
	ManagerAccess.Data.EB.Log.Info().Interface("Request", ResourceID).Msg("RLA: Creating gRPC client")

	err = RLA.CreateGRPCClient()
	if err != nil {
		return nil, err
	}
	return RLA.GetGRPCClient(), nil
}

// RegisterGRPC - Register GRPC
func (RLA *API) RegisterGRPC() {
	// Register activity
	activityRegisterOptions := activity.RegisterOptions{
		Name: "CreateRlaGrpcClientActivity",
	}

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		ManagerAccess.API.RLA.CreateGRPCClientActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("RLA: successfully registered GRPC client activity")
}
