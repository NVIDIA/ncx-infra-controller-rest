/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package managerapi

import (
	"context"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/grpc/client"
)

// RLAExpansion - RLA Expansion
type RLAExpansion interface{}

// RLAInterface - interface to RLA
type RLAInterface interface {
	// List all the apis of RLA here
	Init()
	Start()
	CreateGRPCClient() error
	GetGRPCClient() *client.RlaClient
	UpdateGRPCClientState(err error)
	CreateGRPCClientActivity(ctx context.Context, ResourceID string) (client *client.RlaClient, err error)
	RegisterGRPC()
	RegisterSubscriber() error
	GetState() []string
	GetGRPCClientVersion() int64
	RLAExpansion
}
