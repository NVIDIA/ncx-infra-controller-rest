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

package carbide

import (
	"time"

	"github.com/rs/zerolog/log"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/carbideapi"
)

const (
	// ProviderName is the unique identifier for the Carbide provider.
	ProviderName = "carbide"

	// DefaultTimeout is the default timeout for Carbide gRPC calls.
	DefaultTimeout = time.Minute

	// DefaultComputePowerDelay is the default delay between sequential
	// power control calls for compute trays. A small stagger avoids
	// overwhelming the power delivery system.
	DefaultComputePowerDelay = 2 * time.Second
)

// Config holds configuration for the Carbide provider.
type Config struct {
	// Timeout is the gRPC call timeout for Carbide operations.
	Timeout time.Duration

	// ComputePowerDelay is the delay inserted between sequential power
	// control calls when commanding multiple compute trays.
	// 0 means no delay.
	ComputePowerDelay time.Duration
}

// Provider wraps a carbideapi.Client and provides it to component manager
// implementations.
type Provider struct {
	client carbideapi.Client
}

// New creates a new Provider using the provided configuration.
func New(config Config) (*Provider, error) {
	client, err := carbideapi.NewClient(config.Timeout)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Carbide client")
		return nil, err
	}
	log.Info().Msg("Successfully created Carbide client")
	return &Provider{client: client}, nil
}

// NewWithDefault creates a new Provider with the default configuration.
func NewWithDefault() (*Provider, error) {
	return New(Config{
		Timeout: DefaultTimeout,
	})
}

// NewFromClient creates a Provider from an existing client.
// This is primarily useful for testing with mock clients.
func NewFromClient(client carbideapi.Client) *Provider {
	return &Provider{client: client}
}

// Name returns the unique identifier for this provider type.
func (p *Provider) Name() string {
	return ProviderName
}

// Client returns the underlying carbideapi.Client.
func (p *Provider) Client() carbideapi.Client {
	return p.client
}
