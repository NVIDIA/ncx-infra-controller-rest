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

// Package nsmapi abstracts the gRPC interface used to communicate with the NV-Switch Manager (NSM) service.
// New connection pools can be created with NewClient to create a real client or NewMockClient which fakes
// everything for unit tests.
package nsmapi

import "context"

// Client allows us to have both a real implementation and a mock implementation for unit tests which can be switched transparently.
type Client interface {
	// PowerControl performs a power action on the specified NV-Switch trays.
	PowerControl(ctx context.Context, uuids []string, action PowerAction) ([]PowerControlResult, error)

	// QueueUpdate queues firmware updates for one or more components on a single switch.
	// If components is empty, all components in the bundle are updated in sequence.
	QueueUpdate(ctx context.Context, switchUUID string, bundleVersion string, components []NVSwitchComponent) ([]FirmwareUpdateInfo, error)

	// GetUpdate returns the status of a specific firmware update by ID.
	GetUpdate(ctx context.Context, updateID string) (*FirmwareUpdateInfo, error)

	// ListBundles returns all available firmware bundles.
	ListBundles(ctx context.Context) ([]FirmwareBundle, error)

	// Close closes the underlying gRPC connection.
	Close() error
}
