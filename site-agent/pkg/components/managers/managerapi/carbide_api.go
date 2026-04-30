/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package managerapi

import (
	"context"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/grpc/client"
)

// CarbideExpansion - Carbide Expansion
type CarbideExpansion interface{}

// CarbideInterface - interface to Carbide
type CarbideInterface interface {
	// List all the apis of Carbide here
	Init()
	Start()
	CreateGRPCClient() error
	GetGRPCClient() *client.CarbideClient
	UpdateGRPCClientState(err error)
	CreateGRPCClientActivity(ctx context.Context, ResourceID string) (client *client.CarbideClient, err error)
	RegisterGRPC()
	GetState() []string
	GetGRPCClientVersion() int64
	CarbideExpansion
}
