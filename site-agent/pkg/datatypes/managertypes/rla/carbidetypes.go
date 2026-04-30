/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package rlatypes

import (
	"github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/grpc/client"
	"go.uber.org/atomic"
)

// State - RLA state
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

// RLA represents the gRPC client for RLA and state
type RLA struct {
	Client *client.RlaAtomicClient
	State  *State
}

// NewRLAInstance creates a new instance of RLA
func NewRLAInstance() *RLA {
	rla := &RLA{
		State:  &State{},
		Client: client.NewRlaAtomicClient(&client.RlaClientConfig{}),
	}

	return rla
}

// GetClient returns the RLA client
func (c *RLA) GetClient() *client.RlaClient {
	return c.Client.GetClient()
}
