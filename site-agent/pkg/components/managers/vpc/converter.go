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
	"errors"
	"reflect"
	"strings"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type VPCReqTransformer struct {
	FromVersion string
	ToVersion   string
	Op          string
	Request     interface{}
}

type VPCRespTransformer struct {
	FromVersion string
	ToVersion   string
	Op          string
	Response    interface{}
}

type SiteVpcV1 struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id           *wflows.UUID           `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Name         string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Organization string                 `protobuf:"bytes,3,opt,name=organization,proto3" json:"organization,omitempty"`
	Version      string                 `protobuf:"bytes,99,opt,name=version,proto3" json:"version,omitempty"`
	Created      *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=created,proto3" json:"created,omitempty"`
	Updated      *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=updated,proto3" json:"updated,omitempty"`
	Deleted      *timestamppb.Timestamp `protobuf:"bytes,6,opt,name=deleted,proto3" json:"deleted,omitempty"`
}

type SiteVpcInfoV1 struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status       wflows.WorkflowStatus `protobuf:"varint,1,opt,name=status,proto3,enum=workflows.v1.common.WorkflowStatus" json:"status,omitempty"`
	ObjectStatus wflows.ObjectStatus   `protobuf:"varint,2,opt,name=object_status,json=objectStatus,proto3,enum=workflows.v1.common.ObjectStatus" json:"object_status,omitempty"`
	StatusMsg    string                `protobuf:"bytes,3,opt,name=status_msg,json=statusMsg,proto3" json:"status_msg,omitempty"`
	Vpc          *SiteVpcV1            `protobuf:"bytes,4,opt,name=vpc,proto3" json:"vpc,omitempty"`
}

func (tf *VPCReqTransformer) VPCRequestConverter() (vpc *wflows.Vpc, err error) {

	if reflect.ValueOf(tf.Op).IsZero() {
		// We cant convert
		ManagerAccess.Data.EB.Log.Error().Str("Operation", tf.Op).Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Invalid operation")
		return vpc, errors.New("VPCRequestConverter: Invalid operation")
	}
	if reflect.ValueOf(tf.FromVersion).IsZero() || reflect.ValueOf(tf.ToVersion).IsZero() {
		// We cant convert
		ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Invalid fromVersion or toVersion")
		return vpc, errors.New("VPCRequestConverter: Invalid fromVersion or toVersion")

	}

	// Check the operator
	switch strings.ToLower(tf.Op) {
	case "create":
		switch tf.FromVersion {
		case "v1":
			switch tf.ToVersion {
			case "v2":
				// Lets convert the vpc request from v1 -> v2
				ResourceReq := tf.Request.(*wflows.CreateVPCRequest)
				vpcRequest := &wflows.Vpc{
					Name:                 ResourceReq.Name,
					TenantOrganizationId: ResourceReq.TenantOrganizationId,
				}
				ManagerAccess.Data.EB.Log.Info().Str("Operation", tf.Op).Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Converted successfully")

				return vpcRequest, nil

			default:
				ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Unsupported toVersion")
			}
		case "v2":
			switch tf.ToVersion {
			case "v2":
				// No conversion, this is the required format for this version
				ResourceReq := tf.Request.(*wflows.CreateVPCRequest)
				vpcRequest := &wflows.Vpc{
					Name:                 ResourceReq.Name,
					TenantOrganizationId: ResourceReq.TenantOrganizationId,
				}
				ManagerAccess.Data.EB.Log.Info().Str("Operation", tf.Op).Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Converted successfully")

				return vpcRequest, nil

			default:
				ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Unsupported toVersion")
			}

		default:
			ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Unsupported fromVersion")

		}
	case "update":
		switch tf.FromVersion {
		case "v2":
			// Here the desired format is same as v2, fall through
			fallthrough
		case "v1":
			switch tf.ToVersion {
			case "v2":
				// Lets convert the vpc request from v1 -> v2
				ResourceReq := tf.Request.(*wflows.UpdateVPCRequest)
				vpcRequest := &wflows.Vpc{
					Id:                   &wflows.VpcId{Value: ResourceReq.Id.Value},
					Name:                 ResourceReq.Name,
					TenantOrganizationId: ResourceReq.TenantOrganizationId,
				}
				ManagerAccess.Data.EB.Log.Info().Str("Operation", tf.Op).Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Converted successfully")
				return vpcRequest, nil
			default:
				ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Unsupported toVersion")
			}

		default:
			ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Unsupported fromVersion")

		}
	case "delete":
		switch tf.FromVersion {
		case "v1":
			switch tf.ToVersion {
			case "v2":
				// Lets convert the vpc request from v1 -> v2
				ManagerAccess.Data.EB.Log.Info().Str("Operation", tf.Op).Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: No conversion for deletion")

			default:
				ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Unsupported toVersion")
			}
		default:
			ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Unsupported fromVersion")

		}
	default:
		// We cant convert
		ManagerAccess.Data.EB.Log.Error().Str("Operation", tf.Op).Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCRequestConverter: Invalid operation")
		return vpc, errors.New("VPCRequestConverter: Invalid operation")

	}

	return vpc, err

}

func (tf *VPCRespTransformer) VPCResponseConverter() (vpcresponse interface{}, err error) {

	if reflect.ValueOf(tf.Op).IsZero() {
		// We cant convert
		ManagerAccess.Data.EB.Log.Error().Str("Operation", tf.Op).Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCResponseConverter: Invalid operation")
		return vpcresponse, errors.New("VPCResponseConverter: Invalid operation")
	}
	if reflect.ValueOf(tf.FromVersion).IsZero() || reflect.ValueOf(tf.ToVersion).IsZero() {
		// We cant convert
		ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCResponseConverter: Invalid fromVersion or toVersion")
		return vpcresponse, errors.New("VPCResponseConverter: Invalid fromVersion or toVersion")

	}

	// Check the operator
	switch strings.ToLower(tf.Op) {
	case "publish":
		switch tf.FromVersion {
		case "v2":
			switch tf.ToVersion {
			case "v1":
				// Lets convert the vpc response from v1 -> v2
				ResourceReq := tf.Response.(*wflows.VPCInfo)
				response := &SiteVpcInfoV1{Vpc: &SiteVpcV1{}}
				response.ObjectStatus = ResourceReq.ObjectStatus
				response.Status = ResourceReq.Status
				response.StatusMsg = ResourceReq.StatusMsg
				if ResourceReq.Vpc != nil {
					response.Vpc.Created = ResourceReq.Vpc.Created
					response.Vpc.Deleted = ResourceReq.Vpc.Deleted
					response.Vpc.Updated = ResourceReq.Vpc.Updated
					response.Vpc.Id = &wflows.UUID{Value: ResourceReq.Vpc.Id.Value}
					response.Vpc.Name = ResourceReq.Vpc.Name
					response.Vpc.Organization = ResourceReq.Vpc.TenantOrganizationId
				}

				ManagerAccess.Data.EB.Log.Info().Str("Operation", tf.Op).Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCResponseConverter: Converted successfully")

				return response, nil

			default:
				ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCResponseConverter: Unsupported toVersion")
			}
		default:
			ManagerAccess.Data.EB.Log.Error().Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCResponseConverter: Unsupported fromVersion")

		}
	default:
		// We cant convert
		ManagerAccess.Data.EB.Log.Error().Str("Operation", tf.Op).Str("fromVersion", tf.FromVersion).Str("toVersion", tf.ToVersion).Msg("VPCResponseConverter: Invalid operation")
		return vpcresponse, errors.New("VPCResponseConverter: Invalid operation")

	}

	return vpcresponse, err

}
