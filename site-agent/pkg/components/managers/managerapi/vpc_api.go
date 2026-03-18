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

package managerapi

import (
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.temporal.io/sdk/workflow"
)

// VPCExpansion - VPC Expansion
type VPCExpansion interface{}

// VPCInterface - interface to VPC
type VPCInterface interface {
	// List all the apis of VPC here
	Init()
	RegisterSubscriber() error
	RegisterPublisher() error
	RegisterCron() error

	// Cloud Workflow APIs
	CreateVPC(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.CreateVPCRequest) (err error)
	DeleteVPC(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.DeleteVPCRequest) (err error)
	// 	UpdateVpcInfo(ctx workflow.Context, SiteID string, TransactionID *wflows.TransactionID, VPCInfo *wflows.VPCInfo) (err error)

	// CRUD VPC APIs
	UpdateVPC(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.UpdateVPCRequest) (err error)
	// GetVPCByID(ctx workflow.Context, ResourceID string, VPCID string) (ResourceResponse *wflows.GetVPCResponse, err error)
	GetVPCByName(ctx workflow.Context, ResourceID string, VPCName string) (ResourceResponse *wflows.GetVPCResponse, err error)
	// GetVPCAll(ctx workflow.Context, ResourceID string) (ResourceResponse *wflows.GetVPCResponse, err error)
	// DeleteVPCByIDWorkflow(ctx workflow.Context, ResourceID string, VPCID string) (err error)

	// CreateVPC
	// RegisterWorkflows() error
	// RegisterActivities() error
	GetState() []string
	VPCExpansion
}
