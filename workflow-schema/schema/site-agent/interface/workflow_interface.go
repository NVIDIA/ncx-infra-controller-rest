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

// VPCInterface - interface to VPC
type VPCInterface interface {
	// Cloud Interfaces
	CreateVPC(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.CreateVPCRequest) (err error)
	DeleteVPC(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.DeleteVPCRequest) (err error)
	// Internal interfaces
	UpdateVPC(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.UpdateVPCRequest) (err error)
	GetVPCByID(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.GetVPCByIdRequest) (resourceResponse *wflows.GetVPCResponse, err error)
	GetVPCByName(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.GetVPCByNameRequest) (resourceResponse *wflows.GetVPCResponse, err error)
	GetVPCAll(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.GetVPCAllRequest) (err error)
}

// SubnetInterface - interface to Subnet
type SubnetInterface interface {
	// Cloud Interfaces
	CreateSubnet(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.CreateSubnetRequest) (err error)
	DeleteSubnet(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.DeleteSubnetRequest) (err error)
	// Internal Interfaces
}

// InstanceInterface - interface to Instances
type InstanceInterface interface {
	// Instance Cloud Interfaces
	CreateInstance(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.CreateInstanceRequest) (err error)
	// UpdateInstance moved to synchronous workflow
	GetInstance(ctx workflow.Context, transactionID *wflows.TransactionID, InstanceID *wflows.UUID) (InstanceInfo *wflows.InstanceInfo, err error)
	DeleteInstance(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.DeleteInstanceRequest) (err error)
	RebootInstance(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.RebootInstanceRequest) (err error)

	// Internal Interfaces
	GetInstanceAll()
}

// Healthcheck Interface - functions to check the health of site agent
type HealthInterface interface {
	// Healthcheck interface - synchronized heart beat function to return health of site agent
	// We can think about omitting Transaction ID or updating it with a simple timestamp for the check
	GetHealth(ctx workflow.Context, transactionID *wflows.TransactionID) (HealthStatus *wflows.HealthStatus, err error)
}

// SSHKeyGroup - interface containing workflows managing SSHKeyGroup configs
type SSHKeyGroupInterface interface {
	CreateSSHKeyGroup(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.CreateSSHKeyGroupRequest) (err error)
	UpdateSSHKeyGroup(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.UpdateSSHKeyGroupRequest) (err error)
	GetSSHKeyGroup(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.GetSSHKeyGroup) (ResourceResp *wflows.GetSSHKeyGroupResponse, err error)
	ListSSHKeyGroup(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.GetSSHKeyGroup) (ResourceResp *wflows.GetSSHKeyGroupResponse, err error)
	DeleteSSHKeyGroup(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.DeleteSSHKeyGroupRequest) (err error)
}

// InfiniBandPartitionInterface - interface containing workflows for managing InfiniBandPartition configs
type InfiniBandPartitionInterface interface {
	CreateInfiniBandPartition(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.CreateInfiniBandPartitionRequest) (err error)
	GetInfiniBandPartition(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.GetInfiniBandPartitionRequest) (err error)
	DeleteInfiniBandPartition(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest *wflows.DeleteInfiniBandPartitionRequest) (err error)
}

// MachineInterface - interface containing workflows for managing Machines
type MachineInterface interface {
	SetMachineMaintenance(ctx workflow.Context, request *wflows.MaintenanceRequest) (err error)
}
