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

package vpc

import (
	"context"
	"fmt"
	"time"

	computils "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/common"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/utils"

	workflowtypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/workflow"
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/conftypes"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type activityType int

const (
	activityCreate activityType = iota
	activityUpdate
	activityDelete
	activityGetByName
	activityGetList
	activityPublish
)

// TODO(deprecated): Remove any related VpcDelete references after carbide-rest-api and agent are updated everywhere.  We've moved to sync workflow.
// TODO: Remove VpcCreate an any related references (deprecated).  We've moved to sync workflow.
var activityStr = []string{"VpcCreate", "VpcUpdate", "VpcDelete", "VpcGetByName", "VpcGetList", "VPCCollectInventory", "VpcPublish"}

type vpcWorkflowMetadata struct {
	activity activityType
	response *wflows.VPCInfo
	respList *wflows.GetVPCResponse
}

// ResourceType - Resource Type
func (v *vpcWorkflowMetadata) ResourceType() string {
	return computils.ResourceTypeVpc
}

// ActivityType - Activity Type
func (v *vpcWorkflowMetadata) ActivityType() string {
	return activityStr[v.activity]
}

// DoDbOP - Do Db OP
func (v *vpcWorkflowMetadata) DoDbOP() (act computils.OpType) {
	switch v.activity {
	case activityCreate:
		act = computils.OpCreate
	case activityUpdate:
		act = computils.OpUpdate
	case activityDelete:
		act = computils.OpDelete
	case activityGetByName, activityGetList:
		act = computils.OpNone
	}
	return
}

// DoSiteControllerOP - Do Site Controller OP
func (v *vpcWorkflowMetadata) DoSiteControllerOP(ctx context.Context,
	TransactionID *wflows.TransactionID, req interface{}) (interface{}, error) {
	switch v.activity {
	case activityCreate:
		return ManagerAccess.Data.EB.Managers.Carbide.GetClient().Networks().CreateVPC(ctx, req.(*wflows.Vpc))
	case activityDelete:
		return ManagerAccess.Data.EB.Managers.Carbide.GetClient().Networks().DeleteVPC(ctx, req.(string))
	case activityGetByName:
		return ManagerAccess.Data.EB.Managers.Carbide.GetClient().Networks().GetAllVPCs(ctx, req.(*wflows.VpcSearchFilter), utils.CarbideApiPageSize)
	case activityGetList:
		return ManagerAccess.Data.EB.Managers.Carbide.GetClient().Networks().GetAllVPCs(ctx, req.(*wflows.VpcSearchFilter), utils.CarbideApiPageSize)
	}
	panic(fmt.Sprintf("invalid activity type: %v", v.activity))
}

// ActivityInvoke - Activity Invoke
func (v *vpcWorkflowMetadata) ActivityInvoke() (act interface{}) {
	vpcwflowinstance := &Workflows{}
	switch v.activity {
	case activityCreate:
		act = vpcwflowinstance.CreateVPCActivity
	case activityUpdate:
		act = vpcwflowinstance.UpdateVPCActivity
	case activityDelete:
		act = vpcwflowinstance.DeleteVPCActivity
	case activityGetByName:
		act = vpcwflowinstance.GetVPCByNameActivity
	case activityGetList:
		act = vpcwflowinstance.CollectVPCListActivity
	}

	return
}

// ActivityPublish - Get the VPC publish activity
func (v *vpcWorkflowMetadata) ActivityPublish() interface{} {
	vpcwflowinstance := &Workflows{}
	switch v.activity {
	case activityCreate, activityUpdate, activityDelete:
		return vpcwflowinstance.PublishVPCActivity
	case activityGetByName:
		return vpcwflowinstance.PublishVPCListActivity
	case activityGetList:
		return vpcwflowinstance.PublishVPCListActivity
	default:
		panic(fmt.Sprintf("invalid activity type: %v", v.activity))
	}
}

// ResponseState - Response State
func (v *vpcWorkflowMetadata) ResponseState(status wflows.WorkflowStatus, objectStatus wflows.ObjectStatus, statusMsg string) {
	switch v.activity {
	case activityCreate, activityUpdate, activityDelete:
		v.response.StatusMsg = statusMsg
		v.response.ObjectStatus = objectStatus
		v.response.Status = status
	case activityGetByName, activityGetList:
		v.respList.StatusMsg = statusMsg
		v.respList.Status = status
	default:
		panic(fmt.Sprintf("invalid activity type: %v", v.activity))
	}
}

// Response object
func (v *vpcWorkflowMetadata) Response() interface{} {
	switch v.activity {
	case activityCreate, activityUpdate, activityDelete:
		return v.response
	case activityGetByName, activityGetList:
		return v.respList
	default:
		panic(fmt.Sprintf("invalid activity type: %v", v.activity))
	}
}

// Statistics - VPC Stats
func (v *vpcWorkflowMetadata) Statistics() *workflowtypes.MgrState {
	return ManagerAccess.Data.EB.Managers.Workflow.VpcState
}

// Workflows - Temporal registration
type Workflows struct {
	tcPublish   client.Client
	tcSubscribe client.Client
	cfg         *conftypes.Config
}

// NewVPCWorkflows creates an instance for VPCWorkflows
func NewVPCWorkflows(TMPublish client.Client, TMSubscribe client.Client, CurrentCFG *conftypes.Config) Workflows {
	return Workflows{
		tcPublish:   TMPublish,
		tcSubscribe: TMSubscribe,
		cfg:         CurrentCFG,
	}
}

// CreateVPC creates a new VPCWorkflow and publishes result to cloud
func (api *API) CreateVPC(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.CreateVPCRequest) (err error) {
	wflowMetadata := &vpcWorkflowMetadata{activity: activityCreate}
	return doWflow(ctx, TransactionID, ResourceRequest, wflowMetadata, ResourceRequest.Options)
}

// UpdateVPC updates a VPCWorkflow
func (api *API) UpdateVPC(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.UpdateVPCRequest) (err error) {
	wflowMetadata := &vpcWorkflowMetadata{activity: activityUpdate}
	return doWflow(ctx, TransactionID, ResourceRequest, wflowMetadata, ResourceRequest.Options)
}

// DeleteVPC deletes a VPC
func (api *API) DeleteVPC(ctx workflow.Context, TransactionID *wflows.TransactionID, ResourceRequest *wflows.DeleteVPCRequest) (err error) {
	wflowMetadata := &vpcWorkflowMetadata{activity: activityDelete}
	return doWflow(ctx, TransactionID, ResourceRequest, wflowMetadata, ResourceRequest.Options)
}

// GetVPCByName gets a VPC
func (api *API) GetVPCByName(ctx workflow.Context, ResourceID string, VPCName string) (*wflows.GetVPCResponse, error) {
	transaction := &wflows.TransactionID{
		ResourceId: ResourceID,
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}
	resourceReq := &wflows.GetVPCByNameRequest{Name: VPCName}
	wflowMetadata := &vpcWorkflowMetadata{activity: activityGetByName}
	err := doWflow(ctx, transaction, resourceReq, wflowMetadata, &wflows.WorkflowOptions{})
	return wflowMetadata.respList, err
}

func doWflow(ctx workflow.Context, transactionID *wflows.TransactionID, resourceRequest interface{}, wflowMetadata *vpcWorkflowMetadata, retryOptions *wflows.WorkflowOptions) error {
	wflowMetadata.response = &wflows.VPCInfo{
		Status: wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
		Vpc:    &wflows.Vpc{},
	}
	wflowMetadata.respList = &wflows.GetVPCResponse{List: &wflows.VpcList{}}

	actErr, pubErr := ManagerAccess.API.Orchestrator.DoWorkflow(ctx, transactionID, resourceRequest, wflowMetadata, retryOptions)
	if actErr != nil {
		return actErr
	}
	return pubErr
}

/*
// NOTE: Brief overview of the workflow ordering mechanism

Cloud Api -> R1, R2, R3

Postgres Database
< transaction advisory lock>  update objects in the database R1, R2, R3

Internal Enqueue
Queue b/w Api & Worker ( Temporal queue)
U1, U2

U2
U1

Object timestamp in incremental order
  U1
  U2 -> higher timestamp

  CreateOrUpdate<compressedUpdate>
  Delete


Cloud Worker
Should read from the database
R1,
R2,
R3

WID: Readable ID-uniqueID

Object ID:
Request {
	CloudID
	RequestID - Timestamp( with microseconds resolution)
}

External Enqueue (Temporal queue)
Site A Queue: R1, R2, R3
              ID1, ID2, ID3
Site A:
Site Orchestrator:
   Dequeue:
   1. Out of Order ( R2, R1, R3 )
   We are relying on the IDs to ensure the order of the Requests


ID1 ( W2), ID2 (W3), ID3 (W1)


1. if my T1 < Td in the database, then drop this Request



goroutine 1: T1
goroutine 2: T2

1. if my T1 < Td in the database, then drop this Request
2. if my T1 > Td in the database, then enqueue this Request
3. if my T1 = Td in the database, then enqueue this Request

goroutine 1: T1
goroutine 2: T2

1. if my T1 < Td in the database, then drop this Request
2. if my T1 > Td in the database, then enqueue this Request
3. if my T1 = Td in the database, then enqueue this Request

goroutine 1: T1
goroutine 2: T2

1. if my T1 < Td in the database, then drop this Request
2. if my T1 > Td in the database, then enqueue this Request
3. if my T1 = Td in the database, then enqueue this Request

goroutine 1: T1
goroutine 2: T2

1. if my T1 < Td in the database, then drop this Request
2. if my T1 > Td in the database, then enqueue this Request
3. if my T1 = Td in the database, then enqueue this Request

goroutine 1: T1
goroutine 2: T2

1. if my T1 < Td in the database, then drop this Request
2. if my T1 > Td in the database, then enqueue this Request
3. if my T1 = Td in the database, then enqueue this Request

goroutine 1: T1
goroutine 2: T2

1. if my T1 < Td in the database, then drop this Request
2. if my T1 > Td in the database, then enqueue this Request
3. if my T1 = Td in the database, then enqueue this Request

goroutine 1: T1
goroutine 2: T2

How will we gurantee the execution of the Requests in the order of the IDs?

Establish the rules of Locking:
When can a lock Request be approved?
1. My current ID should be always greater than the ID in the site postgres database
2. Compare the Object Generation to decide whether to process or whom to process next
3. Wait for sometime for in-flight Requests to come


Site Controller:
 process(R1)
  process(R2)
   process(R3)


   Summary:
   1. Handling Temporal failure after carbide-rest-api persists the user Request
   2. Site Agent Queue Request timestamp:
	   A. Use the status timestamp
   3. Resoultion of the timestamp: microseconds
   4. All update Requests will be sepearate row in the status table
	  This status timestamp is the one that will be sent from carbide-rest-api - worker -> site agent
*/
