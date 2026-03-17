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

package client

import wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"

// ComputeGetter is the interface for compute workflows
type ComputeGetter interface {
	Compute() ComputeInterface
}

// ComputeInterface for machine gRPC apis
type ComputeInterface interface {
	MachineInterface
	// Instance Interface
	InstanceInterface
	// SSHKeyGroup Interface
	SSHKeyGroupInterface
	// OperatingSystem Interface
	OperatingSystemInterface
	// Tenant Interface
	TenantInterface
}

type compute struct {
	// carbide client
	carbide wflows.ForgeClient
}

func newCompute(carbide wflows.ForgeClient) *compute {
	return &compute{carbide: carbide}
}
