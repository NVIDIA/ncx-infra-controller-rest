/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package carbide

import (
	"context"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/grpc/client"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/log"
)

// CreateGRPCClientActivity - Create GRPC client Activity
func (Carbide *API) CreateGRPCClientActivity(ctx context.Context, ResourceID string) (client *client.CarbideClient, err error) {
	// Create the VPC
	ManagerAccess.Data.EB.Log.Info().Interface("Request", ResourceID).Msg("Carbide: Starting  the gRPC connection Activity")

	// Use temporal logger for temporal logs
	logger := activity.GetLogger(ctx)
	withLogger := log.With(logger, "Activity", "CreateGRPCClientActivity", "ResourceReq", ResourceID)
	withLogger.Info("Carbide: Starting  the gRPC connection Activity")

	// Create the client
	ManagerAccess.Data.EB.Log.Info().Interface("Request", ResourceID).Msg("Carbide: Creating  grpc client")

	err = Carbide.CreateGRPCClient()
	if err != nil {
		return nil, err
	}
	return Carbide.GetGRPCClient(), nil
}

// RegisterGRPC - Register GRPC
func (Carbide *API) RegisterGRPC() {
	// Register activity
	activityRegisterOptions := activity.RegisterOptions{
		Name: "CreateGRPCClientActivity",
	}

	ManagerAccess.Data.EB.Managers.Workflow.Temporal.Worker.RegisterActivityWithOptions(
		ManagerAccess.API.Carbide.CreateGRPCClientActivity, activityRegisterOptions,
	)
	ManagerAccess.Data.EB.Log.Info().Msg("Carbide: successfully registered GRPC client activity")
}
