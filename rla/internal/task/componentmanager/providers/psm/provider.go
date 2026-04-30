/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package psm

import (
	"time"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/psmapi"
)

const (
	// ProviderName is the unique identifier for the PSM provider.
	ProviderName = "psm"

	// DefaultTimeout is the default timeout for PSM gRPC calls.
	DefaultTimeout = 30 * time.Second
)

// Config holds configuration for the PSM provider.
type Config struct {
	// Timeout is the gRPC call timeout for PSM operations.
	Timeout time.Duration
}

// Provider wraps a psmapi.Client and provides it to component manager implementations.
type Provider struct {
	client psmapi.Client
}

// New creates a new Provider using the provided configuration.
func New(config Config) (*Provider, error) {
	client, err := psmapi.NewClient(config.Timeout)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create PSM client")
		return nil, err
	}
	log.Info().Msg("Successfully created PSM client")
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
func NewFromClient(client psmapi.Client) *Provider {
	return &Provider{client: client}
}

// Name returns the unique identifier for this provider type.
func (p *Provider) Name() string {
	return ProviderName
}

// Client returns the underlying psmapi.Client.
func (p *Provider) Client() psmapi.Client {
	return p.client
}

// Close closes the underlying PSM client connection.
func (p *Provider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}
