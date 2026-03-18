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

// InstanceExpansion - Instance Expansion
type InstanceExpansion interface{}

// InstanceInterface - interface to Instance
type InstanceInterface interface {
	// List all the apis of Instance here
	Init()
	RegisterSubscriber() error
	RegisterPublisher() error
	RegisterCron() error

	// Temporal Workflows - Subscriber
	//Create Instance (deprecated)
	CreateInstance(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.CreateInstanceRequest) (err error)
	//Delete Instance (deprecated)
	DeleteInstance(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.DeleteInstanceRequest) (err error)
	//RebootInstance (deprecated)
	RebootInstance(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.RebootInstanceRequest) (err error)
	GetState() []string

	InstanceExpansion
}
