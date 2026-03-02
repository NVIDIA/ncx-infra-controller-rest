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
package service

import (
	"context"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	pb "github.com/nvidia/bare-metal-manager-rest/powershelf-manager/internal/proto/v1"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/credential"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/converter/protobuf"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/objects/pmc"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/powershelfmanager"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PowershelfManagerServerImpl implements the v1.PowershelfManager gRPC service by delegating to a PowershelfManager instance.
type PowershelfManagerServerImpl struct {
	psm *powershelfmanager.PowershelfManager
	pb.UnimplementedPowershelfManagerServer
}

func newServerImplementation(psm *powershelfmanager.PowershelfManager) (*PowershelfManagerServerImpl, error) {
	return &PowershelfManagerServerImpl{
		psm: psm,
	}, nil
}

// registerPowershelf registers a powershelf by its PMC MAC/IP/Vendor and persists its credentials. Returns creation timestamp on success.
func (s *PowershelfManagerServerImpl) registerPowershelf(
	ctx context.Context,
	req *pb.RegisterPowershelfRequest,
) *pb.RegisterPowershelfResponse {
	pmc, err := pmc.New(req.PmcMacAddress, req.PmcIpAddress, protobuf.PMCVendorFrom(req.PmcVendor), credential.New(req.PmcCredentials.Username, req.PmcCredentials.Password))
	if err != nil {
		return &pb.RegisterPowershelfResponse{
			PmcMacAddress: req.PmcMacAddress,
			IsNew:         true,
			Created:       timestamppb.New(time.Now()),
			Status:        pb.StatusCode_INVALID_ARGUMENT,
			Error:         err.Error(),
		}
	}

	err = s.psm.RegisterPmc(ctx, pmc)
	if err != nil {
		return &pb.RegisterPowershelfResponse{
			PmcMacAddress: req.PmcMacAddress,
			IsNew:         true,
			Created:       timestamppb.New(time.Now()),
			Status:        pb.StatusCode_INTERNAL_ERROR,
			Error:         err.Error(),
		}
	}

	log.Printf("Successfully registered PMC %v with IP %v", req.PmcMacAddress, req.PmcIpAddress)

	return &pb.RegisterPowershelfResponse{
		PmcMacAddress: req.PmcMacAddress,
		IsNew:         true,
		Created:       timestamppb.New(time.Now()),
		Status:        pb.StatusCode_SUCCESS,
		Error:         "",
	}
}

// registerPowershelf registers a powershelf by its PMC MAC/IP/Vendor and persists its credentials. Returns creation timestamp on success.
func (s *PowershelfManagerServerImpl) RegisterPowershelves(
	ctx context.Context,
	req *pb.RegisterPowershelvesRequest,
) (*pb.RegisterPowershelvesResponse, error) {
	responses := make([]*pb.RegisterPowershelfResponse, 0, len(req.RegistrationRequests))
	for _, req := range req.RegistrationRequests {
		response := s.registerPowershelf(ctx, req)
		responses = append(responses, response)
	}

	return &pb.RegisterPowershelvesResponse{
		Responses: responses,
	}, nil
}

func (s *PowershelfManagerServerImpl) GetPowershelves(ctx context.Context, req *pb.PowershelfRequest) (*pb.GetPowershelvesResponse, error) {
	responses := make([]*pb.PowerShelf, 0, len(req.PmcMacs))
	if len(req.PmcMacs) == 0 {
		powershelves, err := s.psm.GetAllPowershelves(ctx)
		if err != nil {
			return nil, err
		}

		for _, powershelf := range powershelves {
			responses = append(responses, protobuf.PowershelfTo(powershelf))
		}
	} else {
		pmcs := make([]net.HardwareAddr, 0, len(req.PmcMacs))
		for _, mac := range req.PmcMacs {
			pmc, err := net.ParseMAC(mac)
			if err != nil {
				return nil, err
			}
			pmcs = append(pmcs, pmc)
		}

		powershelves, err := s.psm.GetPowershelves(ctx, pmcs)
		if err != nil {
			return nil, err
		}

		for _, powershelf := range powershelves {
			responses = append(responses, protobuf.PowershelfTo(powershelf))
		}
	}

	return &pb.GetPowershelvesResponse{
		Powershelves: responses,
	}, nil
}

func (s *PowershelfManagerServerImpl) ListAvailableFirmware(ctx context.Context, req *pb.PowershelfRequest) (*pb.ListAvailableFirmwareResponse, error) {
	responses := make([]*pb.AvailableFirmware, 0, len(req.PmcMacs))
	for _, mac := range req.PmcMacs {
		response, err := s.listAvailableFirmware(ctx, mac)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}

	return &pb.ListAvailableFirmwareResponse{
		Upgrades: responses,
	}, nil
}

// CanUpdateFirmware returns whether a firmware upgrade is supported for the PMC’s current version given vendor rules and available artifacts.
func (s *PowershelfManagerServerImpl) listAvailableFirmware(ctx context.Context, pmc_mac string) (*pb.AvailableFirmware, error) {
	mac, err := net.ParseMAC(pmc_mac)
	if err != nil {
		return nil, err
	}

	upgrades, err := s.psm.ListAvailableFirmware(ctx, mac)
	if err != nil {
		return nil, err
	}

	protoUpgrades := make([]*pb.FirmwareVersion, 0, len(upgrades))
	for _, upgrade := range upgrades {
		protoUpgrades = append(protoUpgrades, &pb.FirmwareVersion{
			Version: upgrade.UpgradeTo().String(),
		})
	}

	pmcComponent := &pb.ComponentFirmwareUpgrades{
		Component: pb.PowershelfComponent_PMC,
		Upgrades:  protoUpgrades,
	}

	componentUpgrades := []*pb.ComponentFirmwareUpgrades{pmcComponent}
	return &pb.AvailableFirmware{
		PmcMacAddress: pmc_mac,
		Upgrades:      componentUpgrades,
	}, nil
}

func (s *PowershelfManagerServerImpl) UpdateFirmware(ctx context.Context, req *pb.UpdateFirmwareRequest) (*pb.UpdateFirmwareResponse, error) {
	responses := make([]*pb.UpdatePowershelfFirmwareResponse, 0, len(req.Upgrades))
	for _, powershelf := range req.Upgrades {
		pmc_mac := powershelf.PmcMacAddress
		componentUpgradeResponses := make([]*pb.UpdateComponentFirmwareResponse, 0, len(powershelf.Components))
		for _, component := range powershelf.Components {
			componentUpgradeResponse := s.updateFirmware(ctx, pmc_mac, component.Component, component.UpgradeTo.Version)
			componentUpgradeResponses = append(componentUpgradeResponses, componentUpgradeResponse)
		}
		responses = append(responses, &pb.UpdatePowershelfFirmwareResponse{
			PmcMacAddress: pmc_mac,
			Components:    componentUpgradeResponses,
		})
	}

	return &pb.UpdateFirmwareResponse{
		Responses: responses,
	}, nil
}

// UpdateFirmware triggers a firmware upgrade for the PMC. If dry_run is true, it resolves artifacts and simulates the update without uploading.
func (s *PowershelfManagerServerImpl) updateFirmware(ctx context.Context, pmc_mac string, pbComponent pb.PowershelfComponent, targetFwVersion string) *pb.UpdateComponentFirmwareResponse {
	// TODO: support upgrading components other than the PMC
	if pbComponent != pb.PowershelfComponent_PMC {
		return &pb.UpdateComponentFirmwareResponse{
			Status: pb.StatusCode_INVALID_ARGUMENT,
			Error:  fmt.Sprintf("PSM does not support upgrading %s component", pbComponent.String()),
		}
	}

	mac, err := net.ParseMAC(pmc_mac)
	if err != nil {
		return &pb.UpdateComponentFirmwareResponse{
			Status: pb.StatusCode_INVALID_ARGUMENT,
			Error:  err.Error(),
		}
	}

	component, err := protobuf.ComponentTypeFromMap(pbComponent)
	if err != nil {
		return &pb.UpdateComponentFirmwareResponse{
			Status: pb.StatusCode_INVALID_ARGUMENT,
			Error:  err.Error(),
		}
	}

	if err := s.psm.UpgradeFirmware(ctx, mac, component, targetFwVersion); err != nil {
		return &pb.UpdateComponentFirmwareResponse{
			Status: pb.StatusCode_INTERNAL_ERROR,
			Error:  err.Error(),
		}
	}

	return &pb.UpdateComponentFirmwareResponse{
		Status: pb.StatusCode_SUCCESS,
		Error:  "",
	}
}

// GetFirmwareUpdateStatus returns the status of firmware updates for the specified PMC(s) and component(s).
func (s *PowershelfManagerServerImpl) GetFirmwareUpdateStatus(ctx context.Context, req *pb.GetFirmwareUpdateStatusRequest) (*pb.GetFirmwareUpdateStatusResponse, error) {
	statuses := make([]*pb.FirmwareUpdateStatus, 0, len(req.Queries))

	for _, query := range req.Queries {
		status := s.getFirmwareUpdateStatus(ctx, query.PmcMacAddress, query.Component)
		statuses = append(statuses, status)
	}

	return &pb.GetFirmwareUpdateStatusResponse{
		Statuses: statuses,
	}, nil
}

// getFirmwareUpdateStatus returns the status of a single firmware update.
func (s *PowershelfManagerServerImpl) getFirmwareUpdateStatus(ctx context.Context, pmcMac string, pbComponent pb.PowershelfComponent) *pb.FirmwareUpdateStatus {
	mac, err := net.ParseMAC(pmcMac)
	if err != nil {
		return &pb.FirmwareUpdateStatus{
			PmcMacAddress: pmcMac,
			Component:     pbComponent,
			Status:        pb.StatusCode_INVALID_ARGUMENT,
			Error:         fmt.Sprintf("invalid MAC address: %v", err),
		}
	}

	component, err := protobuf.ComponentTypeFromMap(pbComponent)
	if err != nil {
		return &pb.FirmwareUpdateStatus{
			PmcMacAddress: pmcMac,
			Component:     pbComponent,
			Status:        pb.StatusCode_INVALID_ARGUMENT,
			Error:         err.Error(),
		}
	}

	update, err := s.psm.GetFirmwareUpdateStatus(ctx, mac, component)
	if err != nil {
		return &pb.FirmwareUpdateStatus{
			PmcMacAddress: pmcMac,
			Component:     pbComponent,
			Status:        pb.StatusCode_INTERNAL_ERROR,
			Error:         err.Error(),
		}
	}

	return protobuf.FirmwareUpdateStatusTo(update, pbComponent)
}

// PowerOff issues a Redfish chassis off action for the PMC's powershelf.
func (s *PowershelfManagerServerImpl) powerOff(ctx context.Context, pmc_mac string) *pb.PowershelfResponse {
	mac, err := net.ParseMAC(pmc_mac)
	if err != nil {
		return &pb.PowershelfResponse{
			PmcMacAddress: pmc_mac,
			Status:        pb.StatusCode_INVALID_ARGUMENT,
			Error:         err.Error(),
		}
	}

	if err := s.psm.PowerOff(ctx, mac); err != nil {
		return &pb.PowershelfResponse{
			PmcMacAddress: pmc_mac,
			Status:        pb.StatusCode_INTERNAL_ERROR,
			Error:         err.Error(),
		}
	}

	return &pb.PowershelfResponse{
		PmcMacAddress: pmc_mac,
		Status:        pb.StatusCode_SUCCESS,
		Error:         "",
	}
}

// PowerOn issues a Redfish chassis on action for the PMC’s powershelf.
func (s *PowershelfManagerServerImpl) powerOn(ctx context.Context, pmc_mac string) *pb.PowershelfResponse {
	mac, err := net.ParseMAC(pmc_mac)
	if err != nil {
		return &pb.PowershelfResponse{
			PmcMacAddress: pmc_mac,
			Status:        pb.StatusCode_INVALID_ARGUMENT,
			Error:         err.Error(),
		}
	}

	if err := s.psm.PowerOn(ctx, mac); err != nil {
		return &pb.PowershelfResponse{
			PmcMacAddress: pmc_mac,
			Status:        pb.StatusCode_INTERNAL_ERROR,
			Error:         err.Error(),
		}
	}

	return &pb.PowershelfResponse{
		PmcMacAddress: pmc_mac,
		Status:        pb.StatusCode_SUCCESS,
		Error:         "",
	}
}

func (s *PowershelfManagerServerImpl) PowerOff(ctx context.Context, req *pb.PowershelfRequest) (*pb.PowerControlResponse, error) {
	responses := make([]*pb.PowershelfResponse, 0, len(req.PmcMacs))
	for _, mac := range req.PmcMacs {
		response := s.powerOff(ctx, mac)
		responses = append(responses, response)
	}

	return &pb.PowerControlResponse{
		Responses: responses,
	}, nil
}

func (s *PowershelfManagerServerImpl) PowerOn(ctx context.Context, req *pb.PowershelfRequest) (*pb.PowerControlResponse, error) {
	responses := make([]*pb.PowershelfResponse, 0, len(req.PmcMacs))
	for _, mac := range req.PmcMacs {
		response := s.powerOn(ctx, mac)
		responses = append(responses, response)
	}

	return &pb.PowerControlResponse{
		Responses: responses,
	}, nil
}

// registerPowershelf registers a powershelf by its PMC MAC/IP/Vendor and persists its credentials. Returns creation timestamp on success.
func (s *PowershelfManagerServerImpl) SetDryRun(
	ctx context.Context,
	req *pb.SetDryRunRequest,
) (*emptypb.Empty, error) {
	to := req.DryRun
	log.Printf("SetDryRun to %v", to)
	s.psm.FirmwareManager.SetDryRun(to)
	return &emptypb.Empty{}, nil
}
