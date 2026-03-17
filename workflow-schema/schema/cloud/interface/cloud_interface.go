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

package wflowinterface

import (
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.temporal.io/sdk/workflow"
)

// VPCInterface - cloud workflow interface for vpc updates
type VPCInterface interface {
	UpdateVpcInfo(ctx workflow.Context, SiteID string, TransactionID *wflows.TransactionID, VPCInfo *wflows.VPCInfo) (err error)
	UpdateVpcInventory(ctx workflow.Context, SiteID string, VPCInventory *wflows.VPCInventory) (err error)
}

// MachineInterface - cloud workflow interface for machine updates
type MachineInterface interface {
	UpdateMachineInventory(ctx workflow.Context, SiteID string, MachineInventory *wflows.MachineInventory) (err error)
}

// SubnetInterface - cloud workflow interface for Subnet updates
type SubnetInterface interface {
	UpdateSubnetInfo(ctx workflow.Context, SiteID string, TransactionID *wflows.TransactionID, SubnetInfo *wflows.SubnetInfo) (err error)
	UpdateSubnetInventory(ctx workflow.Context, SiteID string, SubnetInventory *wflows.SubnetInventory) (err error)
}

// InstanceInterface - cloud workflow interface for Instance updates
type InstanceInterface interface {
	UpdateInstanceInfo(ctx workflow.Context, SiteID string, TransactionID *wflows.TransactionID, Instance *wflows.InstanceInfo) (err error)
	UpdateInstanceRebootInfo(ctx workflow.Context, SiteID string, TransactionID *wflows.TransactionID, Instance *wflows.InstanceRebootInfo) (err error)
	UpdateInstanceInventory(ctx workflow.Context, SiteID string, InstanceInventory *wflows.InstanceInventory) (err error)
}

// SSHKeyGroupInterface - cloud workflow interface for SSHKeyGroup updates
type SSHKeyGroupInterface interface {
	UpdateSSHKeyGroupInfo(ctx workflow.Context, SiteID string, TransactionID *wflows.TransactionID, SSHKeyGroupInfo *wflows.SSHKeyGroupInfo) (err error)
	UpdateSSHKeyGroupInventory(ctx workflow.Context, SiteID string, SSHKeyGroupInventory *wflows.SSHKeyGroupInventory) (err error)
}

// InfiniBandPartitionInterface - cloud workflow interface for InfiniBandPartition updates
type InfiniBandPartitionInterface interface {
	UpdateInfiniBandPartitionInfo(ctx workflow.Context, SiteID string, TransactionID *wflows.TransactionID, InfiniBandPartitionInfo *wflows.InfiniBandPartitionInfo) (err error)
	UpdateInfiniBandPartitionInventory(ctx workflow.Context, SiteID string, InfiniBandPartitionInventory *wflows.InfiniBandPartitionInventory) (err error)
}
