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
package nsmapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/certs"
	pb "github.com/nvidia/bare-metal-manager-rest/rla/internal/nsmapi/gen"
)

type grpcClient struct {
	conn        *grpc.ClientConn
	client      pb.NVSwitchManagerClient
	grpcTimeout time.Duration
}

var testingMsgOnce sync.Once

// NewClient creates a gRPC connection pool to NV-Switch Manager. Returning success does not mean that we have yet
// made an actual connection; that happens when making an actual request.
func NewClient(grpcTimeout time.Duration) (Client, error) {
	if testing.Testing() {
		testingMsgOnce.Do(func() {
			log.Info().Msg("Running unit tests, forcing mock gRPC client for NV-Switch Manager")
		})
		return NewMockClient(), nil
	}

	nsmURL := os.Getenv("NSM_API_URL")
	if nsmURL == "" {
		return nil, errors.New("NSM_API_URL not set, cannot make connections to NV-Switch Manager")
	}

	tlsConfig, _, err := certs.TLSConfig()
	if err != nil {
		if err == certs.ErrNotPresent {
			return nil, errors.New("Certificates not present, unable to authenticate with NV-Switch Manager")
		}
		return nil, err
	}

	conn, err := grpc.NewClient(nsmURL, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to NV-Switch Manager: %v", err)
	}

	return &grpcClient{
		conn:        conn,
		client:      pb.NewNVSwitchManagerClient(conn),
		grpcTimeout: grpcTimeout,
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *grpcClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// PowerControl performs a power action on the specified NV-Switch trays.
func (c *grpcClient) PowerControl(ctx context.Context, uuids []string, action PowerAction) ([]PowerControlResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.grpcTimeout)
	defer cancel()

	resp, err := c.client.PowerControl(ctx, &pb.PowerControlRequest{
		Uuids:  uuids,
		Action: powerActionToPb(action),
	})
	if err != nil {
		return nil, err
	}

	var results []PowerControlResult
	for _, r := range resp.GetResponses() {
		results = append(results, powerControlResultFromPb(r))
	}
	return results, nil
}

// QueueUpdate queues firmware updates for one or more components on a single switch.
func (c *grpcClient) QueueUpdate(ctx context.Context, switchUUID string, bundleVersion string, components []NVSwitchComponent) ([]FirmwareUpdateInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, c.grpcTimeout)
	defer cancel()

	pbComponents := make([]pb.NVSwitchComponent, 0, len(components))
	for _, comp := range components {
		pbComponents = append(pbComponents, nvSwitchComponentToPb(comp))
	}

	resp, err := c.client.QueueUpdate(ctx, &pb.QueueUpdateRequest{
		SwitchUuid:    switchUUID,
		BundleVersion: bundleVersion,
		Components:    pbComponents,
	})
	if err != nil {
		return nil, err
	}

	var results []FirmwareUpdateInfo
	for _, info := range resp.GetUpdates() {
		results = append(results, firmwareUpdateInfoFromPb(info))
	}
	return results, nil
}

// GetUpdate returns the status of a specific firmware update by ID.
func (c *grpcClient) GetUpdate(ctx context.Context, updateID string) (*FirmwareUpdateInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, c.grpcTimeout)
	defer cancel()

	resp, err := c.client.GetUpdate(ctx, &pb.GetUpdateRequest{
		UpdateId: updateID,
	})
	if err != nil {
		return nil, err
	}

	info := firmwareUpdateInfoFromPb(resp.GetUpdate())
	return &info, nil
}

// ListBundles returns all available firmware bundles.
func (c *grpcClient) ListBundles(ctx context.Context) ([]FirmwareBundle, error) {
	ctx, cancel := context.WithTimeout(ctx, c.grpcTimeout)
	defer cancel()

	resp, err := c.client.ListBundles(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	var results []FirmwareBundle
	for _, bundle := range resp.GetBundles() {
		results = append(results, firmwareBundleFromPb(bundle))
	}
	return results, nil
}
