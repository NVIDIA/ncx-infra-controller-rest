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
	"time"

	"github.com/google/uuid"
)

type mockClient struct{}

// NewMockClient returns a gRPC client that returns mock values so it can be used in unit tests.
func NewMockClient() Client {
	return &mockClient{}
}

func (c *mockClient) Close() error {
	return nil
}

func (c *mockClient) PowerControl(_ context.Context, uuids []string, _ PowerAction) ([]PowerControlResult, error) {
	var results []PowerControlResult
	for _, id := range uuids {
		results = append(results, PowerControlResult{
			UUID:   id,
			Status: StatusSuccess,
		})
	}
	return results, nil
}

func (c *mockClient) QueueUpdate(_ context.Context, switchUUID string, bundleVersion string, components []NVSwitchComponent) ([]FirmwareUpdateInfo, error) {
	if len(components) == 0 {
		components = []NVSwitchComponent{
			NVSwitchComponentBMC,
			NVSwitchComponentCPLD,
			NVSwitchComponentBIOS,
			NVSwitchComponentNVOS,
		}
	}

	var results []FirmwareUpdateInfo
	for i, comp := range components {
		results = append(results, FirmwareUpdateInfo{
			ID:            uuid.New().String(),
			SwitchUUID:    switchUUID,
			Component:     comp,
			BundleVersion: bundleVersion,
			State:         UpdateStateQueued,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			SequenceOrder: int32(i + 1),
		})
	}
	return results, nil
}

func (c *mockClient) GetUpdate(_ context.Context, updateID string) (*FirmwareUpdateInfo, error) {
	return &FirmwareUpdateInfo{
		ID:        updateID,
		State:     UpdateStateCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (c *mockClient) ListBundles(_ context.Context) ([]FirmwareBundle, error) {
	return []FirmwareBundle{
		{
			Version:     "1.0.0",
			Description: "Mock firmware bundle",
			Components: []ComponentInfo{
				{Name: "BMC", Version: "1.0.0", Strategy: "redfish"},
				{Name: "CPLD", Version: "1.0.0", Strategy: "ssh"},
				{Name: "BIOS", Version: "1.0.0", Strategy: "redfish"},
				{Name: "NVOS", Version: "1.0.0", Strategy: "ssh"},
			},
		},
	}, nil
}
