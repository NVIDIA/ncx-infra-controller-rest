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

package infinibandpartition

import (
	"context"
	"fmt"
	common "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/common"
	workflowtypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/workflow"
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/conftypes"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

type activityType int

const (
	activityCreate activityType = iota
	activityUpdate
	activityDelete
	activityGetList
	activityPublish
)

var activityStr = []string{"InfiniBandPartitionCreate", "InfiniBandPartitionDelete", "InfiniBandPartitionGetByName", "InfiniBandPartitionGetList", "InfiniBandPartitionCollectInventory", "InfiniBandPartitionPublish"}

type ibpWorkflowMetadata struct {
	activity activityType
	response *wflows.InfiniBandPartitionInfo
	respInv  *wflows.InfiniBandPartitionInventory
}

// Todo: Reconcile with DB interfaces
// ResourceType - Resource Type
func (ibpm *ibpWorkflowMetadata) ResourceType() string {
	return common.ResourceTypeInfiniBandPartition
}

// ActivityType - Activity Type
func (ibpm *ibpWorkflowMetadata) ActivityType() string {
	return activityStr[ibpm.activity]
}

// DoDbOP - Do Db OP
func (ibpm *ibpWorkflowMetadata) DoDbOP() (act common.OpType) {
	switch ibpm.activity {
	case activityCreate:
		act = common.OpCreate
	case activityDelete:
		act = common.OpDelete
	case activityGetList:
		act = common.OpNone
	}
	return
}

// Todo: Reconcile with gRPC & wflow interfaces
// DoSiteControllerOP - Do Site Controller OP
func (ibpm *ibpWorkflowMetadata) DoSiteControllerOP(ctx context.Context,
	TransactionID *wflows.TransactionID, req interface{}) (interface{}, error) {
	switch ibpm.activity {
	case activityCreate:
		return ManagerAccess.Data.EB.Managers.Carbide.GetClient().Networks().CreateInfiniBandPartition(ctx, req.(*wflows.CreateInfiniBandPartitionRequest))
	case activityDelete:
		return ManagerAccess.Data.EB.Managers.Carbide.GetClient().Networks().DeleteInfiniBandPartition(ctx, req.(*wflows.DeleteInfiniBandPartitionRequest))
	default:
		panic(fmt.Sprintf("invalid activity type: %v", ibpm.activity))
	}
}

// ActivityInvoke - Activity Invoke
func (ibpm *ibpWorkflowMetadata) ActivityInvoke() (act interface{}) {
	ibpwflowinstance := &Workflows{}
	switch ibpm.activity {
	case activityCreate:
		act = ibpwflowinstance.CreateInfiniBandPartitionActivity
	case activityDelete:
		act = ibpwflowinstance.DeleteInfiniBandPartitionActivity
	}
	return
}

// ActivityPublish - Get the InfiniBandPartition publish activity
func (ibpm *ibpWorkflowMetadata) ActivityPublish() interface{} {
	ibpwflowinstance := &Workflows{}
	switch ibpm.activity {
	case activityCreate, activityDelete:
		return ibpwflowinstance.PublishInfiniBandPartitionActivity
	default:
		panic(fmt.Sprintf("invalid activity type: %v", ibpm.activity))
	}
}

// ResponseState - Response State
func (ibpm *ibpWorkflowMetadata) ResponseState(status wflows.WorkflowStatus, objectStatus wflows.ObjectStatus, statusMsg string) {
	switch ibpm.activity {
	case activityCreate, activityDelete:
		ibpm.response.StatusMsg = statusMsg
		ibpm.response.ObjectStatus = objectStatus
		ibpm.response.Status = status
	default:
		panic(fmt.Sprintf("invalid activity type: %v", ibpm.activity))
	}
}

// Response object
func (ibpm *ibpWorkflowMetadata) Response() interface{} {
	switch ibpm.activity {
	case activityCreate, activityDelete:
		return ibpm.response
	default:
		panic(fmt.Sprintf("invalid activity type: %v", ibpm.activity))
	}
}

// Statistics - InfiniBandPartition Stats
func (ibpm *ibpWorkflowMetadata) Statistics() *workflowtypes.MgrState {
	// Todo: Add stats here
	//return ManagerAccess.Data.EB.Managers.Workflow.InfiniBandPartitionState
	return &workflowtypes.MgrState{}
}

// Workflows - Temporal registration
type Workflows struct {
	tcPublish   client.Client
	tcSubscribe client.Client
	cfg         *conftypes.Config
}

// NewInfiniBandPartitionWorkflows creates an instance for InfiniBandPartitionWorkflows
func NewInfiniBandPartitionWorkflows(TMPublish client.Client, TMSubscribe client.Client, CurrentCFG *conftypes.Config) Workflows {
	return Workflows{
		tcPublish:   TMPublish,
		tcSubscribe: TMSubscribe,
		cfg:         CurrentCFG,
	}
}

// CreateInfiniBandPartition creates a new InfiniBandPartitionWorkflow and publishes result to cloud
func (api *API) CreateInfiniBandPartition(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.CreateInfiniBandPartitionRequest) (err error) {
	wflowMetadata := &ibpWorkflowMetadata{activity: activityCreate}
	return doWflow(ctx, TransactionID, ResourceRequest, wflowMetadata, ResourceRequest.Options)
}

// DeleteInfiniBandPartition deletes a InfiniBandPartition
func (api *API) DeleteInfiniBandPartition(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.DeleteInfiniBandPartitionRequest) (err error) {
	wflowMetadata := &ibpWorkflowMetadata{activity: activityDelete}
	return doWflow(ctx, TransactionID, ResourceRequest, wflowMetadata, ResourceRequest.Options)
}

func doWflow(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest interface{}, wflowMetadata *ibpWorkflowMetadata, retryOptions *wflows.WorkflowOptions) error {
	wflowMetadata.response = &wflows.InfiniBandPartitionInfo{
		Status:      wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
		IbPartition: &wflows.IBPartition{},
	}
	wflowMetadata.respInv = &wflows.InfiniBandPartitionInventory{
		InventoryStatus: wflows.InventoryStatus_INVENTORY_STATUS_FAILED,
	}

	actErr, pubErr := ManagerAccess.API.Orchestrator.DoWorkflow(ctx, transactionID, resourceRequest, wflowMetadata, retryOptions)
	if actErr != nil {
		return actErr
	}
	return pubErr
}
