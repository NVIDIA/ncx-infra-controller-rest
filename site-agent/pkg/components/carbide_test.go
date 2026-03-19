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

package elektra

import (
	"context"
	"os"
	"testing"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/managers/carbide"
	"github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/utils"
	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
)

// TestCarbideVpc - test the carbide grpc client
func TestCarbideVpc(t *testing.T) {
	TestInitElektra(t)
	grpcClient := testElektra.manager.API.Carbide.GetGRPCClient()

	var vpc *wflows.Vpc

	tcs := []struct {
		descr     string
		expectErr bool
		index     int
	}{{
		descr:     "create",
		expectErr: false,
		index:     0,
	}, {
		descr:     "get",
		expectErr: false,
		index:     0,
	}, {
		descr:     "delete",
		expectErr: false,
		index:     0,
	},
	}
	rpcSucc := 0
	for _, tc := range tcs {
		t.Run(tc.descr, func(t *testing.T) {
			switch tc.descr {
			case "create":
				ctx := context.Background()
				ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideTest-CreateVPC")

				createRequest := &wflows.Vpc{
					Name:                 "test-vpc",
					TenantOrganizationId: "test-tenant-org",
				}

				response, err := grpcClient.Networks().CreateVPC(ctx, createRequest)
				span.End()
				carbide.ManagerAccess.API.Carbide.UpdateGRPCClientState(err)
				if err != nil {
					t.Log(err.Error())
				}
				assert.Nil(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, createRequest.Name, response.Name)
				assert.Equal(t, createRequest.TenantOrganizationId, response.TenantOrganizationId)
				rpcSucc++
				assert.Equal(t, 0,
					int(carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcFail.Load()))
				assert.Equal(t, rpcSucc,
					int(carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcSucc.Load()))
				vpc = response
				t.Log("GRPCResponse", response)
			case "get":
				ctx := context.Background()
				ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideTest-VpcSearchQuery")
				resq, err := grpcClient.Networks().GetAllVPCs(ctx, &wflows.VpcSearchFilter{}, utils.CarbideApiPageSize)
				span.End()
				carbide.ManagerAccess.API.Carbide.UpdateGRPCClientState(err)
				if err != nil {
					t.Log(err.Error())
				}
				assert.Nil(t, err)
				assert.NotNil(t, resq)
				assert.Equal(t, 1, len(resq.Vpcs))
				response := resq.Vpcs[0]
				assert.Equal(t, vpc.Id.Value, response.Id.Value)
				assert.Equal(t, vpc.Name, response.Name)
				assert.Equal(t, vpc.TenantOrganizationId, response.TenantOrganizationId)
				rpcSucc++
				assert.Equal(t, 0,
					int(carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcFail.Load()))
				assert.Equal(t, rpcSucc,
					int(carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcSucc.Load()))
				t.Log("GRPCResponse", response)
			case "delete":
				ctx := context.Background()
				ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideTest-DeleteVPC")
				response, err := grpcClient.Networks().DeleteVPC(ctx, vpc.Id.Value)
				span.End()
				carbide.ManagerAccess.API.Carbide.UpdateGRPCClientState(err)
				if err != nil {
					t.Log(err.Error())
				}
				assert.Nil(t, err)
				assert.NotNil(t, response)
				rpcSucc++
				assert.Equal(t, 0,
					int(carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcFail.Load()))
				assert.Equal(t, rpcSucc,
					int(carbide.ManagerAccess.Data.EB.Managers.Carbide.State.GrpcSucc.Load()))
				t.Log("GRPCResponse", response)
			default:
				panic("invalid operation name")
			}
		})
	}
}

// TestCarbideSubnet - test the carbide grpc client
func TestCarbideSubnet(t *testing.T) {
	TestInitElektra(t)
	grpcClient := testElektra.manager.API.Carbide.GetGRPCClient()

	// Send a request to the GRPC server
	gw := "10.0.1.1"
	mtu := int32(1600)

	createRequest := &wflows.NetworkSegment{
		Id:          &wflows.NetworkSegmentId{Value: uuid.NewString()},
		Name:        "test-subnet",
		VpcId:       &wflows.VpcId{Value: uuid.NewString()},
		SubdomainId: &wflows.DomainId{Value: uuid.NewString()},
		Mtu:         &mtu,
		Prefixes: []*wflows.NetworkPrefix{
			{Prefix: "10.0.1.0/16",
				Gateway:      &gw,
				ReserveFirst: 1,
			},
		},
	}

	var created *wflows.NetworkSegment

	tcs := []struct {
		descr     string
		expectErr bool
		index     int
	}{
		{
			descr:     "create",
			expectErr: false,
			index:     0,
		}, {
			descr:     "get",
			expectErr: false,
			index:     0,
		}, {
			descr:     "delete",
			expectErr: false,
			index:     0,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.descr, func(t *testing.T) {
			switch tc.descr {
			case "create":
				v := &wflows.CreateSubnetRequest{
					Name:        createRequest.Name,
					VpcId:       &wflows.UUID{Value: createRequest.VpcId.Value},
					Mtu:         createRequest.Mtu,
					SubdomainId: &wflows.UUID{Value: createRequest.SubdomainId.Value},
					NetworkPrefixes: []*wflows.NetworkPrefixInfo{{Prefix: createRequest.Prefixes[0].Prefix,
						Gateway:      createRequest.Prefixes[0].Gateway,
						ReserveFirst: createRequest.Prefixes[0].ReserveFirst},
					},
				}
				ctx := context.Background()
				ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideTest-CreateNetworkSegment")
				response, err := grpcClient.Networks().CreateNetworkSegment(ctx, v)
				span.End()
				if err != nil {
					t.Log(err.Error())
				}
				assert.Nil(t, err)
				assert.NotNil(t, response)
				created = response
				t.Log("GRPCResponse", response)
			case "get":
				ctx := context.Background()
				ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideTest-GetNetworkSegment")
				response, err := grpcClient.Networks().GetNetworkSegment(ctx, &wflows.UUID{Value: created.Id.Value})
				span.End()
				if err != nil {
					t.Log(err.Error())
				}
				assert.Nil(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, created.Name, response.Name)
				assert.Equal(t, created.Id.Value, response.Id.Value)
				assert.Equal(t, created.Mtu, response.Mtu)
				assert.Equal(t, created.SubdomainId.Value, response.SubdomainId.Value)
				assert.Equal(t, created.VpcId.Value, response.VpcId.Value)
				assert.Equal(t, len(created.Prefixes), len(response.Prefixes))
				assert.Equal(t, created.Prefixes[0].Prefix, response.Prefixes[0].Prefix)
				assert.Equal(t, created.Prefixes[0].Gateway, response.Prefixes[0].Gateway)
				assert.Equal(t, created.Prefixes[0].ReserveFirst, response.Prefixes[0].ReserveFirst)
				t.Log("GRPCResponse", response)
			case "delete":
				v := &wflows.DeleteSubnetRequest{
					NetworkSegmentId: &wflows.UUID{Value: created.Id.Value},
				}
				ctx := context.Background()
				ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideTest-DeleteNetworkSegment")
				response, err := grpcClient.Networks().DeleteNetworkSegment(ctx, v)
				span.End()
				if err != nil {
					t.Log(err.Error())
				}
				assert.Nil(t, err)
				assert.NotNil(t, response)
				t.Log("GRPCResponse", response)
			default:
				panic("invalid operation name")
			}
		})
	}
}
