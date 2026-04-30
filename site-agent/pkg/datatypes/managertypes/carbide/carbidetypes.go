/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package carbidetypes

import (
	"github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/grpc/client"
	"go.uber.org/atomic"
)

// State - Carbide state
type State struct {
	// GrpcFail the number of times the rpc has failed
	GrpcFail atomic.Uint64
	// GrpcSucc the number of times the rpc has succeeded
	GrpcSucc atomic.Uint64
	// HealthStatus current health state
	HealthStatus atomic.Uint64
	// Err is error message
	Err string
	// WflowMetrics workflow metrics
	WflowMetrics WorkflowMetrics
}

// Carbide represents the gRPC client for Carbide and state
type Carbide struct {
	Client *client.CarbideAtomicClient
	State  *State
}

// NewCarbideInstance creates a new instance of Carbide
func NewCarbideInstance() *Carbide {
	carbide := &Carbide{
		State:  &State{},
		Client: client.NewCarbideAtomicClient(&client.CarbideClientConfig{}),
	}

	return carbide
}

// GetClient returns the Carbide client
func (c *Carbide) GetClient() *client.CarbideClient {
	return c.Client.GetClient()
}
