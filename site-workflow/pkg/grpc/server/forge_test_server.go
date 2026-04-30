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

package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gogo/status"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"

	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/rs/zerolog/log"

	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

var (
	// DefaultPort is the default port that the server listens at
	DefaultPort = ":11079"
	// DefaultVpcId is the default VPC ID for testing
	DefaultVpcId = "00000000-0000-4000-8000-000000000001"
	// DefaultNetworkSegmentId is the default NetworkSegment ID for testing
	DefaultNetworkSegmentId = "00000000-0000-4000-8000-000000000002"
	// DefaultTenantKeysetId is the default TenantKeyset ID for testing
	DefaultTenantKeysetId = "00000000-0000-4000-8000-000000000003"
	// DefaultIBParitionId is the default IBPartition ID for testing
	DefaultIBParitionId = "00000000-0000-4000-8000-000000000004"
	// DefaultInstanceId is the default Instance ID for testing
	DefaultInstanceId = "00000000-0000-4000-8000-000000000005"
	// DefaultInstanceTypeId is the default InstanceType ID for testing
	DefaultInstanceTypeId = "00000000-0000-4000-8000-000000000006"
	// DefaultDpuExtensionServiceId is the default DPU Extension Service ID for testing
	DefaultDpuExtensionServiceId = "00000000-0000-4000-8000-000000000007"
	// DefaultOsImageId is the default OS Image ID for testing
	DefaultOsImageId = "00000000-0000-4000-8000-000000000008"
	// DefaultVpcPrefixId is the default VPCPrefix ID for testing
	DefaultVpcPrefixId = "00000000-0000-4000-8000-000000000009"
	// DefaultNetworkSecurityGroupId is the default NetworkSecurityGroup ID for testing
	DefaultNetworkSecurityGroupId = "00000000-0000-4000-8000-00000000000a"
	// DefaultVpcPeeringId is the default VPCPeering ID for testing
	DefaultVpcPeeringId = "00000000-0000-4000-8000-00000000000b"
	// DefaultNVLinkLogicalPartitionId is the default NVLinkLogicalPartition ID for testing
	DefaultNVLinkLogicalPartitionId = "00000000-0000-4000-8000-00000000000c"
)

// ForgeServerImpl implements interface ForgeServer
type ForgeServerImpl struct {
	cwssaws.UnimplementedForgeServer
	v   map[string]*cwssaws.Vpc
	ns  map[string]*cwssaws.NetworkSegment
	ins map[string]*cwssaws.Instance
	m   map[string]*cwssaws.Machine
	t   map[string]*cwssaws.Tenant
	tk  map[string]*cwssaws.TenantKeyset
	ibp map[string]*cwssaws.IBPartition
	em  map[string]*cwssaws.ExpectedMachine
	eps map[string]*cwssaws.ExpectedPowerShelf
	es  map[string]*cwssaws.ExpectedSwitch
	er  map[string]*cwssaws.ExpectedRack
	it  map[string]*cwssaws.InstanceType
	des map[string]*cwssaws.DpuExtensionService
	osi map[string]*cwssaws.OsImage
	vpr map[string]*cwssaws.VpcPrefix
	nsg map[string]*cwssaws.NetworkSecurityGroup
	vpp map[string]*cwssaws.VpcPeering
	nvl map[string]*cwssaws.NVLinkLogicalPartition
	sku map[string]*cwssaws.Sku
}

var logger = log.With().Str("Component", "Mock Carbide gRPC Server").Logger()

// Version implements interface ForgeServer
func (f *ForgeServerImpl) Version(ctx context.Context, req *cwssaws.VersionRequest) (*cwssaws.BuildInfo, error) {
	return &cwssaws.BuildInfo{
		BuildVersion: "1.0.0",
	}, nil
}

// CreateVpc implements interface ForgeServer
func (f *ForgeServerImpl) CreateVpc(c context.Context, req *cwssaws.VpcCreationRequest) (*cwssaws.Vpc, error) {
	if req == nil || req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	nid := DefaultVpcId
	_, ok := f.v[DefaultVpcId]
	if ok {
		// Default VPC already exists, create a new one with a different ID
		nid = uuid.NewString()
	}

	nv := &cwssaws.Vpc{
		Id:                   &cwssaws.VpcId{Value: nid},
		Name:                 req.Name,
		TenantOrganizationId: req.TenantOrganizationId,
	}
	f.v[nid] = nv

	return nv, nil
}

// UpdateVpc implements interface ForgeServer
func (f *ForgeServerImpl) UpdateVpc(c context.Context, req *cwssaws.VpcUpdateRequest) (*cwssaws.VpcUpdateResult, error) {
	if req == nil || req.Id == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	nv, ok := f.v[req.Id.Value]
	if ok {
		if req.Name != "" {
			nv.Name = req.Name
		}
		return &cwssaws.VpcUpdateResult{}, nil
	}

	return nil, status.Errorf(codes.NotFound, "VPC with ID %q not found", req.Id.Value)
}

// DeleteVpc implements interface ForgeServer
func (f *ForgeServerImpl) DeleteVpc(c context.Context, req *cwssaws.VpcDeletionRequest) (*cwssaws.VpcDeletionResult, error) {
	if req == nil || req.Id == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	_, ok := f.v[req.Id.Value]
	if ok {
		delete(f.v, req.Id.Value)
		return &cwssaws.VpcDeletionResult{}, nil
	}

	return nil, status.Errorf(codes.NotFound, "VPC with ID %q not found", req.Id.Value)
}

// FindVpcIds implements interface ForgeServer
func (f *ForgeServerImpl) FindVpcIds(ctx context.Context, req *cwssaws.VpcSearchFilter) (*cwssaws.VpcIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.VpcIdList{}
	for id := range f.v {
		response.VpcIds = append(response.VpcIds, &cwssaws.VpcId{Value: id})
	}
	return &response, nil
}

// FindVpcsByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindVpcsByIds(ctx context.Context, req *cwssaws.VpcsByIdsRequest) (*cwssaws.VpcList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.VpcList{}
	for _, id := range req.VpcIds {
		if obj, ok := f.v[id.GetValue()]; ok {
			response.Vpcs = append(response.Vpcs, obj)
		}
	}
	return &response, nil
}

// FindVpcs implements interface ForgeServer
func (f *ForgeServerImpl) FindVpcs(c context.Context, req *cwssaws.VpcSearchQuery) (*cwssaws.VpcList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	res := []*cwssaws.Vpc{}

	for _, v := range f.v {
		res = append(res, v)
	}

	if req.Id != nil && req.Id.Value != "" {
		v, ok := f.v[req.Id.Value]
		if ok {
			res = []*cwssaws.Vpc{v}
		} else {
			res = []*cwssaws.Vpc{}
		}
	}

	if req.Name != nil {
		filtered := []*cwssaws.Vpc{}
		for _, nv := range f.v {
			if nv.Name == *req.Name {
				filtered = append(filtered, nv)
			}
		}
		res = filtered
	}

	return &cwssaws.VpcList{Vpcs: res}, nil
}

// CreateNetworkSegment implements interface ForgeServer
func (f *ForgeServerImpl) CreateNetworkSegment(c context.Context, req *cwssaws.NetworkSegmentCreationRequest) (*cwssaws.NetworkSegment, error) {
	if req == nil || req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	nid := DefaultNetworkSegmentId
	_, ok := f.ns[DefaultNetworkSegmentId]
	if ok {
		// Default Network Segment already exists, create a new one with a different ID
		nid = uuid.NewString()
	}

	nns := &cwssaws.NetworkSegment{
		Id:          &cwssaws.NetworkSegmentId{Value: nid},
		Name:        req.Name,
		VpcId:       req.VpcId,
		SubdomainId: req.SubdomainId,
		Mtu:         req.Mtu,
		Prefixes:    req.Prefixes,
	}
	f.ns[nid] = nns

	return nns, nil
}

// DeleteNetworkSegment implements interface ForgeServer
func (f *ForgeServerImpl) DeleteNetworkSegment(c context.Context, req *cwssaws.NetworkSegmentDeletionRequest) (*cwssaws.NetworkSegmentDeletionResult, error) {
	if req == nil || req.Id == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	_, ok := f.ns[req.Id.Value]

	if ok {
		delete(f.ns, req.Id.Value)
		return &cwssaws.NetworkSegmentDeletionResult{}, nil
	}

	return nil, status.Errorf(codes.NotFound, "NetworkSegment with ID %q not found", req.Id.Value)
}

// FindNetworkSegmentIds implements interface ForgeServer
func (f *ForgeServerImpl) FindNetworkSegmentIds(ctx context.Context, req *cwssaws.NetworkSegmentSearchFilter) (*cwssaws.NetworkSegmentIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.NetworkSegmentIdList{}
	for id := range f.ns {
		response.NetworkSegmentsIds = append(response.NetworkSegmentsIds, &cwssaws.NetworkSegmentId{Value: id})
	}
	return &response, nil
}

// FindNetworkSegmentsByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindNetworkSegmentsByIds(ctx context.Context, req *cwssaws.NetworkSegmentsByIdsRequest) (*cwssaws.NetworkSegmentList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.NetworkSegmentList{}
	for _, id := range req.NetworkSegmentsIds {
		if obj, ok := f.ns[id.GetValue()]; ok {
			response.NetworkSegments = append(response.NetworkSegments, obj)
		}
	}
	return &response, nil
}

// CreateInstance implements interface ForgeServer
func (f *ForgeServerImpl) AllocateInstance(ctx context.Context, req *cwssaws.InstanceAllocationRequest) (*cwssaws.Instance, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	nid := uuid.NewString()
	if req.InstanceId != nil {
		nid = req.InstanceId.Value
	}

	_, ok := f.ins[nid]
	if !ok {
		ifcsts := []*cwssaws.InstanceInterfaceStatus{}
		for _, ifcreq := range req.Config.Network.Interfaces {
			ifcst := &cwssaws.InstanceInterfaceStatus{
				MacAddress: getStrPtr(generateMacAddress()),
				Addresses: []string{
					generateIPAddress(),
				},
			}
			if ifcreq.FunctionType == cwssaws.InterfaceFunctionType_VIRTUAL_FUNCTION {
				vfid := uint32(generateInteger(16))
				ifcst.VirtualFunctionId = &vfid
			}
			ifcsts = append(ifcsts, ifcst)
		}

		nins := cwssaws.Instance{
			Id:        &cwssaws.InstanceId{Value: nid},
			MachineId: req.MachineId,
			Config:    req.Config,
			Status: &cwssaws.InstanceStatus{
				Tenant: &cwssaws.InstanceTenantStatus{
					State: cwssaws.TenantState_PROVISIONING,
				},
				Network: &cwssaws.InstanceNetworkStatus{
					Interfaces: ifcsts,
				},
			},
		}

		f.ins[nid] = &nins

		m := cwssaws.Machine{
			Id:    req.MachineId,
			State: "Ready",
		}

		_, ok := f.m[req.MachineId.Id]
		if !ok {
			f.m[req.MachineId.Id] = &m
		}

		return &nins, nil
	}

	return nil, status.Errorf(codes.Internal, "Failed to create Instance")
}

// DeleteInstance implements interface ForgeServer
func (f *ForgeServerImpl) ReleaseInstance(c context.Context, req *cwssaws.InstanceReleaseRequest) (*cwssaws.InstanceReleaseResult, error) {
	if req == nil || req.Id == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	_, ok := f.ins[req.Id.Value]
	if ok {
		delete(f.ins, req.Id.Value)
		return &cwssaws.InstanceReleaseResult{}, nil
	}

	return nil, status.Errorf(codes.NotFound, "Instance with ID %q not found", req.Id.Value)
}

// FindInstances implements interface ForgeServer
func (f *ForgeServerImpl) FindInstanceIds(ctx context.Context, req *cwssaws.InstanceSearchFilter) (*cwssaws.InstanceIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.InstanceIdList{}
	for id := range f.ins {
		response.InstanceIds = append(response.InstanceIds, &cwssaws.InstanceId{Value: id})
	}
	return &response, nil
}

// FindInstances implements interface ForgeServer
func (f *ForgeServerImpl) FindInstancesByIds(ctx context.Context, req *cwssaws.InstancesByIdsRequest) (*cwssaws.InstanceList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.InstanceList{}
	for _, id := range req.InstanceIds {
		if obj, ok := f.ins[id.GetValue()]; ok {
			response.Instances = append(response.Instances, obj)
		}
	}
	return &response, nil
}

// InvokeInstancePower implements interface ForgeServer
func (f *ForgeServerImpl) InvokeInstancePower(c context.Context, req *cwssaws.InstancePowerRequest) (*cwssaws.InstancePowerResult, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	_, ok := f.m[req.MachineId.Id]
	if ok {
		if req.Operation == cwssaws.InstancePowerRequest_POWER_RESET {
			return &cwssaws.InstancePowerResult{}, nil
		}

		return &cwssaws.InstancePowerResult{}, status.Errorf(codes.InvalidArgument, "Invalid operation in request")
	}

	return nil, status.Errorf(codes.NotFound, "Machine with ID %q not found", req.MachineId.Id)
}

func (f *ForgeServerImpl) FindMachineIds(ctx context.Context, req *cwssaws.MachineSearchConfig) (*cwssaws.MachineIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.MachineIdList{}
	for id := range f.m {
		response.MachineIds = append(response.MachineIds, &cwssaws.MachineId{Id: id})
	}

	return &response, nil
}

// FindMachinesByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindMachinesByIds(ctx context.Context, req *cwssaws.MachinesByIdsRequest) (*cwssaws.MachineList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.MachineList{}
	for _, id := range req.MachineIds {
		if obj, ok := f.m[id.Id]; ok {
			response.Machines = append(response.Machines, obj)
		}
	}
	return &response, nil
}

func (f *ForgeServerImpl) SetMaintenance(context.Context, *cwssaws.MaintenanceRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// CreateTenantKeyset implements interface ForgeServer
func (f *ForgeServerImpl) CreateTenantKeyset(c context.Context, req *cwssaws.CreateTenantKeysetRequest) (*cwssaws.CreateTenantKeysetResponse, error) {
	if req == nil || req.KeysetIdentifier == nil || req.KeysetIdentifier.KeysetId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	nid := DefaultTenantKeysetId
	_, ok := f.tk[DefaultTenantKeysetId]
	if ok {
		// Default TenantKeyset already exists, create a new one with a different ID
		nid = uuid.NewString()
	}

	ntk := &cwssaws.TenantKeyset{
		KeysetIdentifier: &cwssaws.TenantKeysetIdentifier{
			KeysetId: nid,
		},
		KeysetContent: req.KeysetContent,
		Version:       req.Version,
	}

	f.tk[nid] = ntk

	result := &cwssaws.CreateTenantKeysetResponse{
		Keyset: ntk,
	}

	return result, nil
}

// UpdateTenantKeyset implements interface ForgeServer
func (f *ForgeServerImpl) UpdateTenantKeyset(c context.Context, req *cwssaws.UpdateTenantKeysetRequest) (*cwssaws.UpdateTenantKeysetResponse, error) {
	if req == nil || req.KeysetIdentifier == nil || req.KeysetIdentifier.KeysetId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	eid := req.KeysetIdentifier.KeysetId

	_, ok := f.tk[eid]
	if ok {
		f.tk[eid].KeysetContent = req.KeysetContent
		f.tk[eid].Version = req.Version

		return &cwssaws.UpdateTenantKeysetResponse{}, nil
	}

	return nil, status.Errorf(codes.Internal, "TenantKeyset with ID not found")
}

// DeleteTenantKeyset implements interface ForgeServer
func (f *ForgeServerImpl) DeleteTenantKeyset(c context.Context, req *cwssaws.DeleteTenantKeysetRequest) (*cwssaws.DeleteTenantKeysetResponse, error) {
	if req == nil || req.KeysetIdentifier == nil || req.KeysetIdentifier.KeysetId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	eid := req.KeysetIdentifier.KeysetId

	_, ok := f.tk[eid]
	if ok {
		delete(f.tk, eid)
		return &cwssaws.DeleteTenantKeysetResponse{}, nil
	}

	return nil, status.Errorf(codes.NotFound, "TenantKeyset with ID %q not found", eid)
}

// FindTenantKeysetIds implements interface ForgeServer
func (f *ForgeServerImpl) FindTenantKeysetIds(ctx context.Context, req *cwssaws.TenantKeysetSearchFilter) (*cwssaws.TenantKeysetIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.TenantKeysetIdList{}
	for id := range f.tk {
		response.KeysetIds = append(response.KeysetIds, &cwssaws.TenantKeysetIdentifier{KeysetId: id})
	}
	return &response, nil
}

// FindTenantKeysetsByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindTenantKeysetsByIds(ctx context.Context, req *cwssaws.TenantKeysetsByIdsRequest) (*cwssaws.TenantKeySetList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.TenantKeySetList{}
	for _, id := range req.KeysetIds {
		if obj, ok := f.tk[id.KeysetId]; ok {
			response.Keyset = append(response.Keyset, obj)
		}
	}
	return &response, nil
}

// CreateIBPartition implements interface ForgeServer
func (f *ForgeServerImpl) CreateIBPartition(c context.Context, req *cwssaws.IBPartitionCreationRequest) (*cwssaws.IBPartition, error) {
	if req == nil || req.Config == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	nid := DefaultIBParitionId
	_, ok := f.ibp[DefaultNetworkSegmentId]
	if ok {
		// Default IBPartition already exists, create a new one with a different ID
		nid = uuid.NewString()
	}

	nibp := &cwssaws.IBPartition{
		Id: &cwssaws.IBPartitionId{Value: nid},
		Config: &cwssaws.IBPartitionConfig{
			Name:                 req.Config.Name,
			TenantOrganizationId: req.Config.TenantOrganizationId,
		},
	}

	f.ibp[nid] = nibp
	return nibp, nil
}

// UpdateIBPartition implements interface ForgeServer
func (f *ForgeServerImpl) UpdateIBPartition(c context.Context, req *cwssaws.IBPartitionUpdateRequest) (*cwssaws.IBPartition, error) {
	if req == nil || req.Id == nil || req.Id.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	ibp, ok := f.ibp[req.Id.Value]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "IB Partition with ID %q not found", req.Id.Value)
	}
	if req.Config != nil {
		if ibp.Config == nil {
			ibp.Config = &cwssaws.IBPartitionConfig{}
		}
		if req.Config.Name != "" {
			ibp.Config.Name = req.Config.Name
		}
		if req.Config.TenantOrganizationId != "" {
			ibp.Config.TenantOrganizationId = req.Config.TenantOrganizationId
		}
		if req.Config.Pkey != nil {
			ibp.Config.Pkey = req.Config.Pkey
		}
	}
	if req.Metadata != nil {
		ibp.Metadata = req.Metadata
	}
	return ibp, nil
}

// DeleteIBPartition implements interface ForgeServer
func (f *ForgeServerImpl) DeleteIBPartition(c context.Context, req *cwssaws.IBPartitionDeletionRequest) (*cwssaws.IBPartitionDeletionResult, error) {
	if req == nil || req.Id == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	_, ok := f.ibp[req.Id.Value]

	if ok {
		delete(f.ibp, req.Id.Value)
		return &cwssaws.IBPartitionDeletionResult{}, nil
	}

	return nil, status.Errorf(codes.NotFound, "IB Partition with ID %q not found", req.Id.Value)
}

// FindIBPartitionIds implements interface ForgeServer
func (f *ForgeServerImpl) FindIBPartitionIds(ctx context.Context, req *cwssaws.IBPartitionSearchFilter) (*cwssaws.IBPartitionIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.IBPartitionIdList{}
	for id := range f.ibp {
		response.IbPartitionIds = append(response.IbPartitionIds, &cwssaws.IBPartitionId{Value: id})
	}
	return &response, nil
}

// FindIBPartitionsByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindIBPartitionsByIds(ctx context.Context, req *cwssaws.IBPartitionsByIdsRequest) (*cwssaws.IBPartitionList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.IBPartitionList{}
	for _, id := range req.IbPartitionIds {
		if obj, ok := f.ibp[id.GetValue()]; ok {
			response.IbPartitions = append(response.IbPartitions, obj)
		}
	}
	return &response, nil
}

// FindDpuExtensionServiceIds implements interface ForgeServer
func (f *ForgeServerImpl) FindDpuExtensionServiceIds(ctx context.Context, req *cwssaws.DpuExtensionServiceSearchFilter) (*cwssaws.DpuExtensionServiceIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.DpuExtensionServiceIdList{}
	for id := range f.des {
		response.ServiceIds = append(response.ServiceIds, id)
	}
	return &response, nil
}

// FindDpuExtensionServicesByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindDpuExtensionServicesByIds(ctx context.Context, req *cwssaws.DpuExtensionServicesByIdsRequest) (*cwssaws.DpuExtensionServiceList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.DpuExtensionServiceList{}
	for _, id := range req.ServiceIds {
		if obj, ok := f.des[id]; ok {
			response.Services = append(response.Services, obj)
		}
	}
	return &response, nil
}

// nsg map[string]*cwssaws.NetworkSecurityGroup
// vpp map[string]*cwssaws.VpcPeering
// nvl map[string]*cwssaws.NVLinkLogicalPartition

// ListOsImage implements interface ForgeServer
func (f *ForgeServerImpl) ListOsImage(ctx context.Context, req *cwssaws.ListOsImageRequest) (*cwssaws.ListOsImageResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.ListOsImageResponse{}
	for id := range f.osi {
		if obj, ok := f.osi[id]; ok {
			response.Images = append(response.Images, obj)
		}
	}
	return &response, nil
}

// SearchVpcPrefixes implements interface ForgeServer
func (f *ForgeServerImpl) SearchVpcPrefixes(ctx context.Context, req *cwssaws.VpcPrefixSearchQuery) (*cwssaws.VpcPrefixIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.VpcPrefixIdList{}
	for id := range f.vpr {
		response.VpcPrefixIds = append(response.VpcPrefixIds, &cwssaws.VpcPrefixId{Value: id})
	}
	return &response, nil
}

// GetVpcPrefixes implements interface ForgeServer
func (f *ForgeServerImpl) GetVpcPrefixes(ctx context.Context, req *cwssaws.VpcPrefixGetRequest) (*cwssaws.VpcPrefixList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.VpcPrefixList{}
	for _, id := range req.VpcPrefixIds {
		if obj, ok := f.vpr[id.GetValue()]; ok {
			response.VpcPrefixes = append(response.VpcPrefixes, obj)
		}
	}
	return &response, nil
}

// FindNetworkSecurityGroupIds implements interface ForgeServer
func (f *ForgeServerImpl) FindNetworkSecurityGroupIds(ctx context.Context, req *cwssaws.FindNetworkSecurityGroupIdsRequest) (*cwssaws.FindNetworkSecurityGroupIdsResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.FindNetworkSecurityGroupIdsResponse{}
	for id := range f.nsg {
		response.NetworkSecurityGroupIds = append(response.NetworkSecurityGroupIds, id)
	}
	return &response, nil
}

// FindNetworkSecurityGroupsByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindNetworkSecurityGroupsByIds(ctx context.Context, req *cwssaws.FindNetworkSecurityGroupsByIdsRequest) (*cwssaws.FindNetworkSecurityGroupsByIdsResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.FindNetworkSecurityGroupsByIdsResponse{}
	for _, id := range req.NetworkSecurityGroupIds {
		if obj, ok := f.nsg[id]; ok {
			response.NetworkSecurityGroups = append(response.NetworkSecurityGroups, obj)
		}
	}
	return &response, nil
}

// FindVpcPeeringIds implements interface ForgeServer
func (f *ForgeServerImpl) FindVpcPeeringIds(ctx context.Context, req *cwssaws.VpcPeeringSearchFilter) (*cwssaws.VpcPeeringIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.VpcPeeringIdList{}
	for id := range f.vpp {
		response.VpcPeeringIds = append(response.VpcPeeringIds, &cwssaws.VpcPeeringId{Value: id})
	}
	return &response, nil
}

// FindVpcPeeringsByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindVpcPeeringsByIds(ctx context.Context, req *cwssaws.VpcPeeringsByIdsRequest) (*cwssaws.VpcPeeringList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.VpcPeeringList{}
	for _, id := range req.VpcPeeringIds {
		if obj, ok := f.vpp[id.Value]; ok {
			response.VpcPeerings = append(response.VpcPeerings, obj)
		}
	}
	return &response, nil
}

// GetAllSkuIds implements interface ForgeServer
func (f *ForgeServerImpl) GetAllSkuIds(ctx context.Context, req *emptypb.Empty) (*cwssaws.SkuIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.SkuIdList{}
	for id := range f.sku {
		response.Ids = append(response.Ids, id)
	}
	return &response, nil
}

// FindSkusByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindSkusByIds(ctx context.Context, req *cwssaws.SkusByIdsRequest) (*cwssaws.SkuList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.SkuList{}
	for _, id := range req.Ids {
		if obj, ok := f.sku[id]; ok {
			response.Skus = append(response.Skus, obj)
		}
	}
	return &response, nil
}

// FindTenantOrganizationIds implements interface ForgeServer
func (f *ForgeServerImpl) FindTenantOrganizationIds(ctx context.Context, req *cwssaws.TenantSearchFilter) (*cwssaws.TenantOrganizationIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.TenantOrganizationIdList{}
	for id := range f.t {
		response.TenantOrganizationIds = append(response.TenantOrganizationIds, id)
	}
	return &response, nil
}

// FindTenantsByOrganizationIds implements interface ForgeServer
func (f *ForgeServerImpl) FindTenantsByOrganizationIds(ctx context.Context, req *cwssaws.TenantByOrganizationIdsRequest) (*cwssaws.TenantList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.TenantList{}
	for _, id := range req.OrganizationIds {
		if obj, ok := f.t[id]; ok {
			response.Tenants = append(response.Tenants, obj)
		}
	}
	return &response, nil
}

// FindNVLinkLogicalPartitionIds
func (f *ForgeServerImpl) FindNVLinkLogicalPartitionIds(ctx context.Context, req *cwssaws.NVLinkLogicalPartitionSearchFilter) (*cwssaws.NVLinkLogicalPartitionIdList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.NVLinkLogicalPartitionIdList{}
	for id := range f.nvl {
		response.PartitionIds = append(response.PartitionIds, &cwssaws.NVLinkLogicalPartitionId{Value: id})
	}
	return &response, nil
}

// FindNVLinkLogicalPartitionsByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindNVLinkLogicalPartitionsByIds(ctx context.Context, req *cwssaws.NVLinkLogicalPartitionsByIdsRequest) (*cwssaws.NVLinkLogicalPartitionList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.NVLinkLogicalPartitionList{}
	for _, id := range req.PartitionIds {
		if obj, ok := f.nvl[id.Value]; ok {
			response.Partitions = append(response.Partitions, obj)
		}
	}
	return &response, nil
}

// FindInstanceTypeIds implements interface ForgeServer
func (f *ForgeServerImpl) FindInstanceTypeIds(ctx context.Context, req *cwssaws.FindInstanceTypeIdsRequest) (*cwssaws.FindInstanceTypeIdsResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.FindInstanceTypeIdsResponse{}
	for id := range f.it {
		response.InstanceTypeIds = append(response.InstanceTypeIds, id)
	}
	return &response, nil
}

// FindInstanceTypesByIds implements interface ForgeServer
func (f *ForgeServerImpl) FindInstanceTypesByIds(ctx context.Context, req *cwssaws.FindInstanceTypesByIdsRequest) (*cwssaws.FindInstanceTypesByIdsResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.FindInstanceTypesByIdsResponse{}
	for _, id := range req.InstanceTypeIds {
		if obj, ok := f.it[id]; ok {
			response.InstanceTypes = append(response.InstanceTypes, obj)
		}
	}
	return &response, nil
}

// AddExpectedMachine implements interface ForgeServer
func (f *ForgeServerImpl) AddExpectedMachine(ctx context.Context, req *cwssaws.ExpectedMachine) (*emptypb.Empty, error) {
	if req == nil || req.Id == nil || req.Id.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for AddExpectedMachine")
	}
	if req.BmcMacAddress == "" {
		return nil, status.Errorf(codes.InvalidArgument, "MAC address not provided for AddExpectedMachine")
	}
	if req.ChassisSerialNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Chassis Serial Number not provided for AddExpectedMachine")
	}
	f.em[req.Id.Value] = req
	return &emptypb.Empty{}, nil
}

// UpdateExpectedMachine implements interface ForgeServer
func (f *ForgeServerImpl) UpdateExpectedMachine(ctx context.Context, req *cwssaws.ExpectedMachine) (*emptypb.Empty, error) {
	if req == nil || req.Id == nil || req.Id.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for UpdateExpectedMachine")
	}
	if req.BmcMacAddress == "" {
		return nil, status.Errorf(codes.InvalidArgument, "MAC address not provided for UpdateExpectedMachine")
	}
	if req.ChassisSerialNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Chassis Serial Number not provided for UpdateExpectedMachine")
	}
	if _, ok := f.em[req.Id.Value]; !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedMachine with ID %q not found", req.Id.Value)
	}
	f.em[req.Id.Value] = req
	return &emptypb.Empty{}, nil
}

// DeleteExpectedMachine implements interface ForgeServer
func (f *ForgeServerImpl) DeleteExpectedMachine(ctx context.Context, req *cwssaws.ExpectedMachineRequest) (*emptypb.Empty, error) {
	if req == nil || req.Id == nil || req.Id.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for DeleteExpectedMachine")
	}
	if _, ok := f.em[req.Id.Value]; !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedMachine with ID %q not found", req.Id.Value)
	}
	delete(f.em, req.Id.Value)
	return &emptypb.Empty{}, nil
}

// CreateExpectedMachines implements interface ForgeServer
func (f *ForgeServerImpl) CreateExpectedMachines(ctx context.Context, req *cwssaws.BatchExpectedMachineOperationRequest) (*cwssaws.BatchExpectedMachineOperationResponse, error) {
	if req == nil || req.GetExpectedMachines() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request for CreateExpectedMachines")
	}
	emList := req.GetExpectedMachines().GetExpectedMachines()
	out := &cwssaws.BatchExpectedMachineOperationResponse{
		Results: make([]*cwssaws.ExpectedMachineOperationResult, 0, len(emList)),
	}
	for _, em := range emList {
		if em == nil {
			msg := "nil expected machine entry"
			out.Results = append(out.Results, &cwssaws.ExpectedMachineOperationResult{
				Success:         false,
				ErrorMessage:    &msg,
				ExpectedMachine: nil,
			})
			continue
		}
		result := &cwssaws.ExpectedMachineOperationResult{
			Id:              em.Id,
			Success:         true,
			ExpectedMachine: em,
		}
		if em.GetId() == nil || em.GetId().GetValue() == "" {
			result.Success = false
			msg := "ID not provided"
			result.ErrorMessage = &msg
			result.ExpectedMachine = nil
		} else if em.GetBmcMacAddress() == "" {
			result.Success = false
			msg := "MAC address not provided"
			result.ErrorMessage = &msg
			result.ExpectedMachine = nil
		} else if em.GetChassisSerialNumber() == "" {
			result.Success = false
			msg := "Chassis Serial Number not provided"
			result.ErrorMessage = &msg
			result.ExpectedMachine = nil
		} else {
			f.em[em.Id.Value] = em
		}
		out.Results = append(out.Results, result)
	}
	return out, nil
}

// UpdateExpectedMachines implements interface ForgeServer
func (f *ForgeServerImpl) UpdateExpectedMachines(ctx context.Context, req *cwssaws.BatchExpectedMachineOperationRequest) (*cwssaws.BatchExpectedMachineOperationResponse, error) {
	if req == nil || req.GetExpectedMachines() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request for UpdateExpectedMachines")
	}
	emList := req.GetExpectedMachines().GetExpectedMachines()
	out := &cwssaws.BatchExpectedMachineOperationResponse{
		Results: make([]*cwssaws.ExpectedMachineOperationResult, 0, len(emList)),
	}
	for _, em := range emList {
		if em == nil {
			msg := "nil expected machine entry"
			out.Results = append(out.Results, &cwssaws.ExpectedMachineOperationResult{
				Success:         false,
				ErrorMessage:    &msg,
				ExpectedMachine: nil,
			})
			continue
		}
		result := &cwssaws.ExpectedMachineOperationResult{
			Id:              em.Id,
			Success:         true,
			ExpectedMachine: em,
		}
		if em.GetId() == nil || em.GetId().GetValue() == "" {
			result.Success = false
			msg := "ID not provided"
			result.ErrorMessage = &msg
			result.ExpectedMachine = nil
		} else if em.GetBmcMacAddress() == "" {
			result.Success = false
			msg := "MAC address not provided"
			result.ErrorMessage = &msg
			result.ExpectedMachine = nil
		} else if em.GetChassisSerialNumber() == "" {
			result.Success = false
			msg := "Chassis Serial Number not provided"
			result.ErrorMessage = &msg
			result.ExpectedMachine = nil
		} else if _, ok := f.em[em.Id.Value]; !ok {
			result.Success = false
			msg := fmt.Sprintf("ExpectedMachine with ID %q not found", em.Id.Value)
			result.ErrorMessage = &msg
			result.ExpectedMachine = nil
		} else {
			f.em[em.Id.Value] = em
		}
		out.Results = append(out.Results, result)
	}
	return out, nil
}

// GetAllExpectedMachines implements interface ForgeServer
func (f *ForgeServerImpl) GetAllExpectedMachines(ctx context.Context, req *emptypb.Empty) (*cwssaws.ExpectedMachineList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.ExpectedMachineList{}
	for _, em := range f.em {
		response.ExpectedMachines = append(response.ExpectedMachines, em)
	}

	return &response, nil
}

// GetAllExpectedMachinesLinked implements interface ForgeServer
func (f *ForgeServerImpl) GetAllExpectedMachinesLinked(ctx context.Context, req *emptypb.Empty) (*cwssaws.LinkedExpectedMachineList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.LinkedExpectedMachineList{}

	return &response, nil
}

// AddExpectedPowerShelf implements interface ForgeServer
func (f *ForgeServerImpl) AddExpectedPowerShelf(ctx context.Context, req *cwssaws.ExpectedPowerShelf) (*emptypb.Empty, error) {
	if req == nil || req.ExpectedPowerShelfId == nil || req.ExpectedPowerShelfId.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for AddExpectedPowerShelf")
	}
	if req.BmcMacAddress == "" {
		return nil, status.Errorf(codes.InvalidArgument, "MAC address not provided for AddExpectedPowerShelf")
	}
	if req.ShelfSerialNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Shelf Serial Number not provided for AddExpectedPowerShelf")
	}
	f.eps[req.ExpectedPowerShelfId.Value] = req
	return &emptypb.Empty{}, nil
}

// UpdateExpectedPowerShelf implements interface ForgeServer
func (f *ForgeServerImpl) UpdateExpectedPowerShelf(ctx context.Context, req *cwssaws.ExpectedPowerShelf) (*emptypb.Empty, error) {
	if req == nil || req.ExpectedPowerShelfId == nil || req.ExpectedPowerShelfId.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for UpdateExpectedPowerShelf")
	}
	if req.BmcMacAddress == "" {
		return nil, status.Errorf(codes.InvalidArgument, "MAC address not provided for UpdateExpectedPowerShelf")
	}
	if req.ShelfSerialNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Shelf Serial Number not provided for UpdateExpectedPowerShelf")
	}
	if _, ok := f.eps[req.ExpectedPowerShelfId.Value]; !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedPowerShelf with ID %q not found", req.ExpectedPowerShelfId.Value)
	}
	f.eps[req.ExpectedPowerShelfId.Value] = req
	return &emptypb.Empty{}, nil
}

// DeleteExpectedPowerShelf implements interface ForgeServer
func (f *ForgeServerImpl) DeleteExpectedPowerShelf(ctx context.Context, req *cwssaws.ExpectedPowerShelfRequest) (*emptypb.Empty, error) {
	if req == nil || req.ExpectedPowerShelfId == nil || req.ExpectedPowerShelfId.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for DeleteExpectedPowerShelf")
	}
	if _, ok := f.eps[req.ExpectedPowerShelfId.Value]; !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedPowerShelf with ID %q not found", req.ExpectedPowerShelfId.Value)
	}
	delete(f.eps, req.ExpectedPowerShelfId.Value)
	return &emptypb.Empty{}, nil
}

// GetExpectedPowerShelf implements interface ForgeServer
func (f *ForgeServerImpl) GetExpectedPowerShelf(ctx context.Context, req *cwssaws.ExpectedPowerShelfRequest) (*cwssaws.ExpectedPowerShelf, error) {
	if req == nil || req.ExpectedPowerShelfId == nil || req.ExpectedPowerShelfId.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for GetExpectedPowerShelf")
	}
	eps, ok := f.eps[req.ExpectedPowerShelfId.Value]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedPowerShelf with ID %q not found", req.ExpectedPowerShelfId.Value)
	}
	return eps, nil
}

// GetAllExpectedPowerShelves implements interface ForgeServer
func (f *ForgeServerImpl) GetAllExpectedPowerShelves(ctx context.Context, req *emptypb.Empty) (*cwssaws.ExpectedPowerShelfList, error) {
	res := make([]*cwssaws.ExpectedPowerShelf, 0, len(f.eps))
	for _, eps := range f.eps {
		res = append(res, eps)
	}
	return &cwssaws.ExpectedPowerShelfList{ExpectedPowerShelves: res}, nil
}

// GetAllExpectedPowerShelvesLinked implements interface ForgeServer
func (f *ForgeServerImpl) GetAllExpectedPowerShelvesLinked(ctx context.Context, req *emptypb.Empty) (*cwssaws.LinkedExpectedPowerShelfList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.LinkedExpectedPowerShelfList{}
	return &response, nil
}

// AddExpectedSwitch implements interface ForgeServer
func (f *ForgeServerImpl) AddExpectedSwitch(ctx context.Context, req *cwssaws.ExpectedSwitch) (*emptypb.Empty, error) {
	if req == nil || req.ExpectedSwitchId == nil || req.ExpectedSwitchId.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for AddExpectedSwitch")
	}
	if req.BmcMacAddress == "" {
		return nil, status.Errorf(codes.InvalidArgument, "MAC address not provided for AddExpectedSwitch")
	}
	if req.SwitchSerialNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Switch Serial Number not provided for AddExpectedSwitch")
	}
	f.es[req.ExpectedSwitchId.Value] = req
	return &emptypb.Empty{}, nil
}

// UpdateExpectedSwitch implements interface ForgeServer
func (f *ForgeServerImpl) UpdateExpectedSwitch(ctx context.Context, req *cwssaws.ExpectedSwitch) (*emptypb.Empty, error) {
	if req == nil || req.ExpectedSwitchId == nil || req.ExpectedSwitchId.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for UpdateExpectedSwitch")
	}
	if req.BmcMacAddress == "" {
		return nil, status.Errorf(codes.InvalidArgument, "MAC address not provided for UpdateExpectedSwitch")
	}
	if req.SwitchSerialNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Switch Serial Number not provided for UpdateExpectedSwitch")
	}
	if _, ok := f.es[req.ExpectedSwitchId.Value]; !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedSwitch with ID %q not found", req.ExpectedSwitchId.Value)
	}
	f.es[req.ExpectedSwitchId.Value] = req
	return &emptypb.Empty{}, nil
}

// DeleteExpectedSwitch implements interface ForgeServer
func (f *ForgeServerImpl) DeleteExpectedSwitch(ctx context.Context, req *cwssaws.ExpectedSwitchRequest) (*emptypb.Empty, error) {
	if req == nil || req.ExpectedSwitchId == nil || req.ExpectedSwitchId.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for DeleteExpectedSwitch")
	}
	if _, ok := f.es[req.ExpectedSwitchId.Value]; !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedSwitch with ID %q not found", req.ExpectedSwitchId.Value)
	}
	delete(f.es, req.ExpectedSwitchId.Value)
	return &emptypb.Empty{}, nil
}

// GetExpectedSwitch implements interface ForgeServer
func (f *ForgeServerImpl) GetExpectedSwitch(ctx context.Context, req *cwssaws.ExpectedSwitchRequest) (*cwssaws.ExpectedSwitch, error) {
	if req == nil || req.ExpectedSwitchId == nil || req.ExpectedSwitchId.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for GetExpectedSwitch")
	}
	es, ok := f.es[req.ExpectedSwitchId.Value]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedSwitch with ID %q not found", req.ExpectedSwitchId.Value)
	}
	return es, nil
}

// GetAllExpectedSwitches implements interface ForgeServer
func (f *ForgeServerImpl) GetAllExpectedSwitches(ctx context.Context, req *emptypb.Empty) (*cwssaws.ExpectedSwitchList, error) {
	res := make([]*cwssaws.ExpectedSwitch, 0, len(f.es))
	for _, es := range f.es {
		res = append(res, es)
	}
	return &cwssaws.ExpectedSwitchList{ExpectedSwitches: res}, nil
}

// GetAllExpectedSwitchesLinked implements interface ForgeServer
func (f *ForgeServerImpl) GetAllExpectedSwitchesLinked(ctx context.Context, req *emptypb.Empty) (*cwssaws.LinkedExpectedSwitchList, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	response := cwssaws.LinkedExpectedSwitchList{}
	return &response, nil
}

// AddExpectedRack implements interface ForgeServer
func (f *ForgeServerImpl) AddExpectedRack(ctx context.Context, req *cwssaws.ExpectedRack) (*emptypb.Empty, error) {
	if req == nil || req.RackId == nil || req.RackId.Id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for AddExpectedRack")
	}
	if req.RackProfileId == nil || req.RackProfileId.Id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Rack Profile ID not provided for AddExpectedRack")
	}
	f.er[req.RackId.Id] = req
	return &emptypb.Empty{}, nil
}

// UpdateExpectedRack implements interface ForgeServer
func (f *ForgeServerImpl) UpdateExpectedRack(ctx context.Context, req *cwssaws.ExpectedRack) (*emptypb.Empty, error) {
	if req == nil || req.RackId == nil || req.RackId.Id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for UpdateExpectedRack")
	}
	if req.RackProfileId == nil || req.RackProfileId.Id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Rack Profile ID not provided for UpdateExpectedRack")
	}
	if _, ok := f.er[req.RackId.Id]; !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedRack with ID %q not found", req.RackId.Id)
	}
	f.er[req.RackId.Id] = req
	return &emptypb.Empty{}, nil
}

// DeleteExpectedRack implements interface ForgeServer
func (f *ForgeServerImpl) DeleteExpectedRack(ctx context.Context, req *cwssaws.ExpectedRackRequest) (*emptypb.Empty, error) {
	if req == nil || req.RackId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for DeleteExpectedRack")
	}
	if _, ok := f.er[req.RackId]; !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedRack with ID %q not found", req.RackId)
	}
	delete(f.er, req.RackId)
	return &emptypb.Empty{}, nil
}

// GetExpectedRack implements interface ForgeServer
func (f *ForgeServerImpl) GetExpectedRack(ctx context.Context, req *cwssaws.ExpectedRackRequest) (*cwssaws.ExpectedRack, error) {
	if req == nil || req.RackId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ID not provided for GetExpectedRack")
	}
	er, ok := f.er[req.RackId]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "ExpectedRack with ID %q not found", req.RackId)
	}
	return er, nil
}

// GetAllExpectedRacks implements interface ForgeServer
func (f *ForgeServerImpl) GetAllExpectedRacks(ctx context.Context, req *emptypb.Empty) (*cwssaws.ExpectedRackList, error) {
	res := make([]*cwssaws.ExpectedRack, 0, len(f.er))
	for _, er := range f.er {
		res = append(res, er)
	}
	return &cwssaws.ExpectedRackList{ExpectedRacks: res}, nil
}

// ReplaceAllExpectedRacks implements interface ForgeServer
func (f *ForgeServerImpl) ReplaceAllExpectedRacks(ctx context.Context, req *cwssaws.ExpectedRackList) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	for _, er := range req.ExpectedRacks {
		if er == nil || er.RackId == nil || er.RackId.Id == "" {
			return nil, status.Errorf(codes.InvalidArgument, "ID not provided for ReplaceAllExpectedRacks")
		}
		if er.RackProfileId == nil || er.RackProfileId.Id == "" {
			return nil, status.Errorf(codes.InvalidArgument, "Rack Profile ID not provided for ReplaceAllExpectedRacks")
		}
	}
	f.er = make(map[string]*cwssaws.ExpectedRack)
	for _, er := range req.ExpectedRacks {
		f.er[er.RackId.Id] = er
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllExpectedRacks implements interface ForgeServer
func (f *ForgeServerImpl) DeleteAllExpectedRacks(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	f.er = make(map[string]*cwssaws.ExpectedRack)
	return &emptypb.Empty{}, nil
}

// LoadTestMachines loads test machines into the server
func (f *ForgeServerImpl) LoadTestMachines() {
	nid := uuid.NewString()

	var memSize uint32 = 16384

	f.m[nid] = &cwssaws.Machine{
		Id:    &cwssaws.MachineId{Id: nid},
		State: "Ready",
		Interfaces: []*cwssaws.MachineInterface{
			{
				Id:                   &cwssaws.MachineInterfaceId{Value: uuid.NewString()},
				AttachedDpuMachineId: &cwssaws.MachineId{Id: uuid.NewString()},
				MachineId:            &cwssaws.MachineId{Id: nid},
				SegmentId:            &cwssaws.NetworkSegmentId{Value: uuid.NewString()},
				Hostname:             "forge.nvidia.com",
				PrimaryInterface:     true,
				MacAddress:           generateMacAddress(),
				Address:              []string{generateIPAddress()},
			},
		},
		DiscoveryInfo: &cwssaws.DiscoveryInfo{
			Cpus: []*cwssaws.Cpu{
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "1571.080",
					Number:    0,
					Core:      0,
					Socket:    0,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "1571.080",
					Number:    1,
					Core:      0,
					Socket:    0,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3371.751",
					Number:    2,
					Core:      0,
					Socket:    1,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3017.142",
					Number:    3,
					Core:      0,
					Socket:    1,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3507.275",
					Number:    4,
					Core:      1,
					Socket:    0,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3255.853",
					Number:    5,
					Core:      1,
					Socket:    0,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3530.777",
					Number:    6,
					Core:      1,
					Socket:    1,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3250.777",
					Number:    7,
					Core:      1,
					Socket:    1,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3250.777",
					Number:    8,
					Core:      2,
					Socket:    0,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3250.777",
					Number:    9,
					Core:      2,
					Socket:    0,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3250.777",
					Number:    10,
					Core:      2,
					Socket:    1,
				},
				{
					Vendor:    "GenuineIntel",
					Model:     "Intel(R) Xeon(R) Gold 6354 CPU @ 3.00GHz",
					Frequency: "3250.777",
					Number:    11,
					Core:      2,
					Socket:    1,
				},
			},
			NetworkInterfaces: []*cwssaws.NetworkInterface{
				{
					PciProperties: &cwssaws.PciDeviceProperties{
						Vendor:      "0x14e4",
						Device:      "0x165f",
						Path:        "/devices/pci0000:00/0000:00:1c.5/0000:04:00.0/net/eno8303",
						Description: getStrPtr("NetXtreme BCM5720 2-port Gigabit Ethernet PCIe (PowerEdge Rx5xx LOM Board)"),
					},
				},
				{
					PciProperties: &cwssaws.PciDeviceProperties{
						Vendor:      "0x14e4",
						Device:      "0x165f",
						Path:        "/devices/pci0000:00/0000:00:1c.5/0000:04:00.1/net/eno8403",
						Description: getStrPtr("NetXtreme BCM5720 2-port Gigabit Ethernet PCIe (PowerEdge Rx5xx LOM Board)"),
					},
				},
				{
					PciProperties: &cwssaws.PciDeviceProperties{
						Vendor:      "0x14e4",
						Device:      "0x16d7",
						Path:        "/devices/pci0000:30/0000:30:04.0/0000:31:00.0/net/eno12399np0",
						Description: getStrPtr("BCM57414 NetXtreme-E 10Gb/25Gb RDMA Ethernet Controller"),
					},
				},
				{
					PciProperties: &cwssaws.PciDeviceProperties{
						Vendor:      "0x14e4",
						Device:      "0x16d7",
						Path:        "/devices/pci0000:30/0000:30:04.0/0000:31:00.1/net/eno12409np1",
						Description: getStrPtr("BCM57414 NetXtreme-E 10Gb/25Gb RDMA Ethernet Controller"),
					},
				},
				{
					PciProperties: &cwssaws.PciDeviceProperties{
						Vendor:      "0x15b3",
						Device:      "0xa2d6",
						Path:        "/devices/pci0000:b0/0000:b0:02.0/0000:b1:00.0/net/enp177s0f0np0",
						NumaNode:    1,
						Description: getStrPtr("MT42822 BlueField-2 integrated ConnectX-6 Dx network controller"),
					},
				},
				{
					PciProperties: &cwssaws.PciDeviceProperties{
						Vendor:      "0x15b3",
						Device:      "0xa2d6",
						Path:        "/devices/pci0000:b0/0000:b0:02.0/0000:b1:00.1/net/enp177s0f1np1",
						NumaNode:    1,
						Description: getStrPtr("MT42822 BlueField-2 integrated ConnectX-6 Dx network controller"),
					},
				},
			},
			BlockDevices: []*cwssaws.BlockDevice{
				{
					Model:    "NO_MODEL",
					Revision: "NO_REVISION",
				},
				{
					Model:    "LOGICAL_VOLUME",
					Revision: "3.53",
					Serial:   "600508b1001cb4d1a278bf3ee7a72228",
				},
				{
					Model:    "Dell Ent NVMe CM6 RI 1.92TB",
					Revision: "2.1.3",
				},
				{
					Model:    "SSDPF2KE016T9L",
					Revision: "2CV1L028",
				},
				{
					Model:    "DELLBOSS_VD",
					Revision: "MV.R00-0",
				},
			},
			DmiData: &cwssaws.DmiData{
				BoardName:     "7Z23CTOLWW",
				BoardVersion:  "06",
				BiosVersion:   "U8E122J-1.51",
				ProductSerial: "J1050ACR",
				BoardSerial:   ".C1KS2CS001G.",
				ChassisSerial: "J1050ACR",
				BiosDate:      "03/30/2023",
				ProductName:   "ThinkSystem SR670 V2",
				SysVendor:     "Lenovo",
			},
			NvmeDevices: []*cwssaws.NvmeDevice{
				{
					Model:       "Dell Ent NVMe CM6 RI 1.92TB",
					FirmwareRev: "2.1.3",
				},
				{
					Model:       "Dell Ent NVMe CM6 RI 1.92TB",
					FirmwareRev: "2.1.3",
				},
				{
					Model:       "Dell Ent NVMe CM6 RI 1.92TB",
					FirmwareRev: "2.1.3",
				},
			},
			Gpus: []*cwssaws.Gpu{
				{
					Name:           "NVIDIA H100 PCIe",
					Serial:         "1654422005434",
					DriverVersion:  "530.30.02",
					VbiosVersion:   "96.00.30.00.01",
					InforomVersion: "1010.0200.00.02",
					TotalMemory:    "81559 MiB",
					Frequency:      "1755 MHz",
					PciBusId:       "00000000:17:00.0",
				},
			},
			MemoryDevices: []*cwssaws.MemoryDevice{
				{
					SizeMb:  &memSize,
					MemType: getStrPtr("DDR4"),
				},
				{
					SizeMb:  &memSize,
					MemType: getStrPtr("DDR4"),
				},
				{
					SizeMb:  &memSize,
					MemType: getStrPtr("DDR4"),
				},
				{
					SizeMb:  &memSize,
					MemType: getStrPtr("DDR4"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
				{
					SizeMb:  &memSize,
					MemType: getStrPtr("DDR4"),
				},
				{
					SizeMb:  &memSize,
					MemType: getStrPtr("DDR4"),
				},
				{
					SizeMb:  &memSize,
					MemType: getStrPtr("DDR4"),
				},
				{
					SizeMb:  &memSize,
					MemType: getStrPtr("DDR4"),
				},
				{
					SizeMb:  nil,
					MemType: getStrPtr("UNKNOWN"),
				},
			},
			InfinibandInterfaces: []*cwssaws.InfinibandInterface{
				{
					PciProperties: &cwssaws.PciDeviceProperties{
						Vendor:      "Mellanox Technologies",
						Device:      "MT28908 Family [ConnectX-6]",
						Path:        "/devices/pci0000:c9/0000:c9:02.0/0000:ca:00.0/infiniband/rocep202s0f0",
						NumaNode:    1,
						Description: getStrPtr("MT28908 Family [ConnectX-6]"),
						Slot:        getStrPtr("0000:ca:00.0"),
					},
					Guid: "1070fd0300bd43ac",
				},
				{
					PciProperties: &cwssaws.PciDeviceProperties{
						Vendor:      "Mellanox Technologies",
						Device:      "MT28908 Family [ConnectX-6]",
						Path:        "/devices/pci0000:c9/0000:c9:02.0/0000:ca:00.1/infiniband/rocep202s0f1",
						NumaNode:    1,
						Description: getStrPtr("MT28908 Family [ConnectX-6]"),
						Slot:        getStrPtr("0000:ca:00.1"),
					},
					Guid: "1070fd0300bd43ad",
				},
			},
		},
	}
}

// ForgeTest tests the grpc server
func ForgeTest(secs int) {
	listener, err := net.Listen("tcp", DefaultPort)
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	reflection.Register(s)

	forgeServer := &ForgeServerImpl{
		v:   make(map[string]*cwssaws.Vpc),
		ns:  make(map[string]*cwssaws.NetworkSegment),
		ins: make(map[string]*cwssaws.Instance),
		m:   make(map[string]*cwssaws.Machine),
		tk:  make(map[string]*cwssaws.TenantKeyset),
		ibp: make(map[string]*cwssaws.IBPartition),
		em:  make(map[string]*cwssaws.ExpectedMachine),
		eps: make(map[string]*cwssaws.ExpectedPowerShelf),
		es:  make(map[string]*cwssaws.ExpectedSwitch),
		er:  make(map[string]*cwssaws.ExpectedRack),
	}
	forgeServer.LoadTestMachines()

	cwssaws.RegisterForgeServer(s, forgeServer)

	if secs != 0 {
		timer := time.AfterFunc(time.Second*time.Duration(secs), func() {
			s.GracefulStop()
			logger.Info().Msgf("Timer started for: %v seconds", secs)
		})
		defer timer.Stop()
	}

	logger.Info().Msg("Started API server")

	err = s.Serve(listener)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to start API server")
	}

	logger.Info().Msg("Stopped API server")
}
