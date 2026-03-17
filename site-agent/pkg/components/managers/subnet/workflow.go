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

package subnet

import (
	"context"
	"fmt"

	computils "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/common"
	"github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/conftypes"
	workflowtypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/workflow"
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"go.temporal.io/sdk/client"
	workflow "go.temporal.io/sdk/workflow"
)

type activityType int

const (
	activityCreate activityType = iota
	activityUpdate
	activityDelete
	activityPublish
)

// TODO: Remove SubnetCreate an any related references.  We've moved to sync workflow (deprecated)
// TODO: Remove SubnetDelete (deprecated) and any related references after carbide-rest-api and site-agent are updated everywhere.  We're moving to sync workflow.
var activityStr = []string{"SubnetCreate", "SubnetUpdate", "SubnetDelete", "SubnetGet", "SubnetPublish"}

type subnetWorkflowMetadata struct {
	activity     activityType
	response     *wflows.SubnetInfo
	responseList *wflows.SubnetInventory
}

// ResourceType - Resource Type
func (w *subnetWorkflowMetadata) ResourceType() string {
	return computils.ResourceTypeSubnet
}

// ActivityType - Activity Type
func (w *subnetWorkflowMetadata) ActivityType() (s string) {
	return activityStr[w.activity]
}

// DoDbOP - Do Db OP
func (w *subnetWorkflowMetadata) DoDbOP() computils.OpType {
	switch w.activity {
	case activityCreate:
		return computils.OpCreate
	case activityUpdate:
		return computils.OpUpdate
	case activityDelete:
		return computils.OpDelete
	default:
		panic(fmt.Sprintf("invalid activity type: %v", w.activity))
	}
}

// DoSiteControllerOP - Do Site Controller OP
func (w *subnetWorkflowMetadata) DoSiteControllerOP(ctx context.Context, TransactionID *wflows.TransactionID, req interface{}) (interface{}, error) {
	networks := ManagerAccess.Data.EB.Managers.Carbide.GetClient().Networks()
	switch w.activity {
	case activityCreate:
		return networks.CreateNetworkSegment(ctx, req.(*wflows.CreateSubnetRequest))
	// Implement this when functionality is available in Site Controller
	// case activityUpdate:
	// 	return networks.UpdateNetworkSegment(ctx, TransactionID, req.(*wflows.UpdateSubnetRequest))
	case activityDelete:
		return networks.DeleteNetworkSegment(ctx, req.(*wflows.DeleteSubnetRequest))
	default:
		panic(fmt.Sprintf("invalid activity type: %v", w.activity))
	}
}

// ActivityInvoke - Activity Invoke
func (w *subnetWorkflowMetadata) ActivityInvoke() interface{} {
	instance := &Workflows{}
	switch w.activity {
	case activityCreate:
		return instance.CreateSubnetActivity
	// Implement this when functionality is available in Site Controller
	// case activityUpdate:
	// 	return instance.UpdateSubnetActivity
	case activityDelete:
		return instance.DeleteSubnetActivity
	default:
		panic(fmt.Sprintf("invalid activity type: %v", w.activity))
	}
}

// ActivityPublish - Get the Subnet publish activity
func (w *subnetWorkflowMetadata) ActivityPublish() interface{} {
	Subnetwflowinstance := Workflows{}
	switch w.activity {
	case activityCreate, activityUpdate, activityDelete:
		return Subnetwflowinstance.PublishSubnetActivity
	default:
		panic(fmt.Sprintf("invalid activity type: %v", w.activity))
	}
}

// ResponseState - Response State
func (w *subnetWorkflowMetadata) ResponseState(status wflows.WorkflowStatus, objectStatus wflows.ObjectStatus, statusMsg string) {
	switch w.activity {
	case activityCreate, activityUpdate, activityDelete:
		w.response.StatusMsg = statusMsg
		w.response.ObjectStatus = objectStatus
		w.response.Status = status
		return
	default:
		panic(fmt.Sprintf("invalid activity type: %v", w.activity))
	}
}

// Response GRPC
func (w *subnetWorkflowMetadata) Response() interface{} {
	switch w.activity {
	case activityCreate, activityUpdate, activityDelete:
		return w.response
	default:
		panic(fmt.Sprintf("invalid activity type: %v", w.activity))
	}
}

// Statistics of subnet
func (w *subnetWorkflowMetadata) Statistics() *workflowtypes.MgrState {
	return ManagerAccess.Data.EB.Managers.Workflow.SubnetState
}

// Workflows - Temporal registration
type Workflows struct {
	tcPublish   client.Client
	tcSubscribe client.Client
	cfg         *conftypes.Config
}

// NewSubnetWorkflows creates an instance for SubnetWorkflows
func NewSubnetWorkflows(TMPublish client.Client, TMSubscribe client.Client, CurrentCFG *conftypes.Config) Workflows {
	return Workflows{
		tcPublish:   TMPublish,
		tcSubscribe: TMSubscribe,
		cfg:         CurrentCFG,
	}
}

// CreateSubnet - temporal create subnet workflow
func (sub *API) CreateSubnet(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.CreateSubnetRequest) (err error) {
	wflowMd := &subnetWorkflowMetadata{activity: activityCreate,
		response: &wflows.SubnetInfo{Status: wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
			ObjectStatus:   wflows.ObjectStatus_OBJECT_STATUS_UNSPECIFIED,
			NetworkSegment: &wflows.NetworkSegment{},
		}}
	return doWflow(ctx, TransactionID, ResourceRequest, wflowMd, ResourceRequest.Options)
}

// Implement this when functionality is available in Site Controller
// UpdateSubnet - temporal update subnet workflow
// func (sub *API) UpdateSubnet(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.UpdateSubnetRequest) (err error) {
// 	wflowMd := &subnetWorkflowMetadata{activity: activityUpdate,
// 		response: &wflows.SubnetInfo{Status: wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
// 			ObjectStatus:   wflows.ObjectStatus_OBJECT_STATUS_UNSPECIFIED,
// 			NetworkSegment: &wflows.NetworkSegment{},
// 		}}
// 	return doWflow(ctx, TransactionID, ResourceRequest, wflowMd, ResourceRequest.Options)
// }

// DeleteSubnet - temporal delete subnet workflow
func (sub *API) DeleteSubnet(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.DeleteSubnetRequest) (err error) {
	wflowMd := &subnetWorkflowMetadata{activity: activityDelete,
		response: &wflows.SubnetInfo{Status: wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
			ObjectStatus:   wflows.ObjectStatus_OBJECT_STATUS_UNSPECIFIED,
			NetworkSegment: &wflows.NetworkSegment{},
		}}
	return doWflow(ctx, TransactionID, ResourceRequest.NetworkSegmentId.Value, wflowMd,
		ResourceRequest.Options)
}

func doWflow(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest interface{}, wflowMetadata *subnetWorkflowMetadata, retryOptions *wflows.WorkflowOptions) error {
	wflowMetadata.response = &wflows.SubnetInfo{
		Status:         wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
		NetworkSegment: &wflows.NetworkSegment{},
	}
	wflowMetadata.responseList = &wflows.SubnetInventory{
		InventoryStatus: wflows.InventoryStatus_INVENTORY_STATUS_FAILED,
	}

	actErr, pubErr := ManagerAccess.API.Orchestrator.DoWorkflow(ctx, transactionID, resourceRequest, wflowMetadata, retryOptions)
	if actErr != nil {
		return actErr
	}
	return pubErr
}
