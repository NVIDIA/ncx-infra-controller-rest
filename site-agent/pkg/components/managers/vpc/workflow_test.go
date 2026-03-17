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
	"testing"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_vpcWorkflowMetadata_ResponseState(t *testing.T) {
	createdVpc := &wflows.Vpc{
		Id: &wflows.VpcId{Value: uuid.NewString()},
	}

	type fields struct {
		activity activityType
		response *wflows.VPCInfo
		respList *wflows.GetVPCResponse
	}
	type args struct {
		status       wflows.WorkflowStatus
		objectStatus wflows.ObjectStatus
		statusMsg    string
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *wflows.VPCInfo
	}{
		// TODO: Remove test for create (deprecated).  We've moved to sync workflow.
		{
			name: "test success response state for create",
			fields: fields{
				activity: activityCreate,
				response: &wflows.VPCInfo{
					Vpc: createdVpc,
				},
			},
			args: args{
				status:       wflows.WorkflowStatus_WORKFLOW_STATUS_SUCCESS,
				objectStatus: wflows.ObjectStatus_OBJECT_STATUS_CREATED,
				statusMsg:    "vpc was successfully created",
			},
			wantResp: &wflows.VPCInfo{
				Vpc:          createdVpc,
				Status:       wflows.WorkflowStatus_WORKFLOW_STATUS_SUCCESS,
				ObjectStatus: wflows.ObjectStatus_OBJECT_STATUS_CREATED,
				StatusMsg:    "vpc was successfully created",
			},
		},
		{
			name: "test failure response state for delete",
			fields: fields{
				activity: activityDelete,
				response: &wflows.VPCInfo{
					Vpc: nil,
				},
			},
			args: args{
				status:    wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
				statusMsg: "vpc deletion failed",
			},
			wantResp: &wflows.VPCInfo{
				Status:    wflows.WorkflowStatus_WORKFLOW_STATUS_FAILURE,
				StatusMsg: "vpc deletion failed",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &vpcWorkflowMetadata{
				activity: tt.fields.activity,
				response: tt.fields.response,
				respList: tt.fields.respList,
			}
			v.ResponseState(tt.args.status, tt.args.objectStatus, tt.args.statusMsg)

			if tt.wantResp != nil {
				assert.Equal(t, tt.wantResp.Status, v.response.Status)
				assert.Equal(t, tt.wantResp.ObjectStatus, v.response.ObjectStatus)
				assert.Equal(t, tt.wantResp.StatusMsg, v.response.StatusMsg)
			}
		})
	}
}
