/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package nvswitchmanager

import (
	"time"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/nsmapi"
)

const (
	// ProviderName is the unique identifier for the NV-Switch Manager provider.
	ProviderName = "nvswitchmanager"

	// DefaultTimeout is the default timeout for NV-Switch Manager gRPC calls.
	DefaultTimeout = time.Minute
)

// Config holds configuration for the NV-Switch Manager provider.
type Config struct {
	// Timeout is the gRPC call timeout for NV-Switch Manager operations.
	Timeout time.Duration
}

// Provider wraps an nsmapi.Client and provides it to component manager implementations.
type Provider struct {
	client nsmapi.Client
}

// New creates a new Provider using the provided configuration.
func New(config Config) (*Provider, error) {
	client, err := nsmapi.NewClient(config.Timeout)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create NV-Switch Manager client")
		return nil, err
	}
	log.Info().Msg("Successfully created NV-Switch Manager client")
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
func NewFromClient(client nsmapi.Client) *Provider {
	return &Provider{client: client}
}

// Name returns the unique identifier for this provider type.
func (p *Provider) Name() string {
	return ProviderName
}

// Client returns the underlying nsmapi.Client.
func (p *Provider) Client() nsmapi.Client {
	return p.client
}

// Close closes the underlying NV-Switch Manager client connection.
func (p *Provider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}
