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

package activity

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/protobuf/types/known/emptypb"

	swe "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/error"
	cclient "github.com/NVIDIA/ncx-infra-controller-rest/site-workflow/pkg/grpc/client"
	rlav1 "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/rla/protobuf/v1"
	cwssaws "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
)

// ManageExpectedRack is an activity wrapper for Expected Rack management
type ManageExpectedRack struct {
	CarbideAtomicClient *cclient.CarbideAtomicClient
	RlaAtomicClient     *cclient.RlaAtomicClient
}

// NewManageExpectedRack returns a new ManageExpectedRack client
func NewManageExpectedRack(carbideClient *cclient.CarbideAtomicClient, rlaClient *cclient.RlaAtomicClient) ManageExpectedRack {
	return ManageExpectedRack{
		CarbideAtomicClient: carbideClient,
		RlaAtomicClient:     rlaClient,
	}
}

// CreateExpectedRackOnSite creates Expected Rack with Carbide
func (mer *ManageExpectedRack) CreateExpectedRackOnSite(ctx context.Context, request *cwssaws.ExpectedRack) error {
	logger := log.With().Str("Activity", "CreateExpectedRackOnSite").Logger()

	logger.Info().Msg("Starting activity")

	var err error

	// Validate request
	if request == nil {
		err = errors.New("received empty create Expected Rack request")
	} else if request.GetRackId().GetId() == "" {
		err = errors.New("received create Expected Rack request without required rack_id field")
	} else if request.GetRackProfileId().GetId() == "" {
		err = errors.New("received create Expected Rack request without required rack_profile_id field")
	}

	if err != nil {
		return temporal.NewNonRetryableApplicationError(err.Error(), swe.ErrTypeInvalidRequest, err)
	}

	// Call Site Controller gRPC endpoint
	carbideClient := mer.CarbideAtomicClient.GetClient()
	forgeClient := carbideClient.Carbide()

	_, err = forgeClient.AddExpectedRack(ctx, request)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create Expected Rack using Site Controller API")
		return swe.WrapErr(err)
	}

	logger.Info().Msg("Completed activity")

	return nil
}

// UpdateExpectedRackOnSite updates Expected Rack on Carbide
func (mer *ManageExpectedRack) UpdateExpectedRackOnSite(ctx context.Context, request *cwssaws.ExpectedRack) error {
	logger := log.With().Str("Activity", "UpdateExpectedRackOnSite").Logger()

	logger.Info().Msg("Starting activity")

	var err error

	// Validate request
	if request == nil {
		err = errors.New("received empty update Expected Rack request")
	} else if request.GetRackId().GetId() == "" {
		err = errors.New("received update Expected Rack request without required rack_id field")
	} else if request.GetRackProfileId().GetId() == "" {
		err = errors.New("received update Expected Rack request without required rack_profile_id field")
	}

	if err != nil {
		return temporal.NewNonRetryableApplicationError(err.Error(), swe.ErrTypeInvalidRequest, err)
	}

	// Call Site Controller gRPC endpoint
	carbideClient := mer.CarbideAtomicClient.GetClient()
	forgeClient := carbideClient.Carbide()

	_, err = forgeClient.UpdateExpectedRack(ctx, request)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to update Expected Rack using Site Controller API")
		return swe.WrapErr(err)
	}

	logger.Info().Msg("Completed activity")

	return nil
}

// DeleteExpectedRackOnSite deletes Expected Rack on Carbide
func (mer *ManageExpectedRack) DeleteExpectedRackOnSite(ctx context.Context, request *cwssaws.ExpectedRackRequest) error {
	logger := log.With().Str("Activity", "DeleteExpectedRackOnSite").Logger()

	logger.Info().Msg("Starting activity")

	var err error

	// Validate request
	if request == nil {
		err = errors.New("received empty delete Expected Rack request")
	} else if request.GetRackId() == "" {
		err = errors.New("received delete Expected Rack request without required rack_id field")
	}

	if err != nil {
		return temporal.NewNonRetryableApplicationError(err.Error(), swe.ErrTypeInvalidRequest, err)
	}

	// Call Site Controller gRPC endpoint
	carbideClient := mer.CarbideAtomicClient.GetClient()
	forgeClient := carbideClient.Carbide()

	_, err = forgeClient.DeleteExpectedRack(ctx, request)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to delete Expected Rack using Site Controller API")
		return swe.WrapErr(err)
	}

	logger.Info().Msg("Completed activity")

	return nil
}

// ReplaceAllExpectedRacksOnSite replaces all Expected Racks on Carbide with the supplied list
func (mer *ManageExpectedRack) ReplaceAllExpectedRacksOnSite(ctx context.Context, request *cwssaws.ExpectedRackList) error {
	logger := log.With().Str("Activity", "ReplaceAllExpectedRacksOnSite").Logger()

	logger.Info().Msg("Starting activity")

	// Validate request
	if request == nil {
		err := errors.New("received empty replace Expected Rack list request")
		return temporal.NewNonRetryableApplicationError(err.Error(), swe.ErrTypeInvalidRequest, err)
	}

	// Validate each entry has required ids
	for i, rack := range request.GetExpectedRacks() {
		if rack.GetRackId().GetId() == "" {
			err := errors.New("received replace Expected Rack request with entry missing rack_id field")
			logger.Warn().Int("index", i).Msg(err.Error())
			return temporal.NewNonRetryableApplicationError(err.Error(), swe.ErrTypeInvalidRequest, err)
		}
		if rack.GetRackProfileId().GetId() == "" {
			err := errors.New("received replace Expected Rack request with entry missing rack_profile_id field")
			logger.Warn().Int("index", i).Msg(err.Error())
			return temporal.NewNonRetryableApplicationError(err.Error(), swe.ErrTypeInvalidRequest, err)
		}
	}

	// Call Site Controller gRPC endpoint
	carbideClient := mer.CarbideAtomicClient.GetClient()
	forgeClient := carbideClient.Carbide()

	_, err := forgeClient.ReplaceAllExpectedRacks(ctx, request)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to replace all Expected Racks using Site Controller API")
		return swe.WrapErr(err)
	}

	logger.Info().Msg("Completed activity")

	return nil
}

// DeleteAllExpectedRacksOnSite deletes all Expected Racks on Carbide
func (mer *ManageExpectedRack) DeleteAllExpectedRacksOnSite(ctx context.Context) error {
	logger := log.With().Str("Activity", "DeleteAllExpectedRacksOnSite").Logger()

	logger.Info().Msg("Starting activity")

	// Call Site Controller gRPC endpoint
	carbideClient := mer.CarbideAtomicClient.GetClient()
	forgeClient := carbideClient.Carbide()

	_, err := forgeClient.DeleteAllExpectedRacks(ctx, &emptypb.Empty{})
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to delete all Expected Racks using Site Controller API")
		return swe.WrapErr(err)
	}

	logger.Info().Msg("Completed activity")

	return nil
}

// CreateExpectedRackOnRLA creates an Expected Rack in RLA via CreateExpectedRack.
// Best-effort: if the RLA client is not configured, the activity logs and returns nil
// so the workflow can continue. RPC failures are surfaced as errors so the workflow
// can decide how to handle them (typically log and ignore).
func (mer *ManageExpectedRack) CreateExpectedRackOnRLA(ctx context.Context, request *cwssaws.ExpectedRack) error {
	logger := log.With().Str("Activity", "CreateExpectedRackOnRLA").Logger()

	logger.Info().Msg("Starting activity")

	// Validate request
	if request == nil {
		return temporal.NewNonRetryableApplicationError("received empty create Expected Rack request for RLA", swe.ErrTypeInvalidRequest, errors.New("nil request"))
	}

	// If RLA client is not configured, skip gracefully
	if mer.RlaAtomicClient == nil {
		logger.Warn().Msg("RLA client not configured, skipping RLA expected rack creation")
		return nil
	}

	rlaClient := mer.RlaAtomicClient.GetClient()
	if rlaClient == nil {
		logger.Warn().Msg("RLA client not connected, skipping RLA expected rack creation")
		return nil
	}

	rack := expectedRackToRLARack(request)
	_, err := rlaClient.Rla().CreateExpectedRack(ctx, &rlav1.CreateExpectedRackRequest{Rack: rack})
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create Expected Rack on RLA")
		return swe.WrapErr(err)
	}

	logger.Info().Msg("Completed activity")
	return nil
}

// labelValue extracts the value for a label key from a metadata label slice. Returns
// empty string if the key is not present or the value is nil.
func labelValue(labels []*cwssaws.Label, key string) string {
	for _, l := range labels {
		if l == nil {
			continue
		}
		if l.GetKey() == key {
			// Label.Value is *string; GetValue() handles nil safely.
			return l.GetValue()
		}
	}
	return ""
}

// expectedRackToRLARack converts a Forge ExpectedRack proto to an RLA Rack proto.
// Chassis identity (manufacturer/serial/model) and physical location (region/datacenter/
// room/position) are read from well-known label keys defined in this package
// (see rack_labels.go). Missing labels are tolerated and rendered as empty strings
// on the RLA side.
func expectedRackToRLARack(rack *cwssaws.ExpectedRack) *rlav1.Rack {
	labels := rack.GetMetadata().GetLabels()

	manufacturer := labelValue(labels, LabelChassisManufacturer)
	serialNumber := labelValue(labels, LabelChassisSerialNumber)
	model := labelValue(labels, LabelChassisModel)

	region := labelValue(labels, LabelLocationRegion)
	datacenter := labelValue(labels, LabelLocationDatacenter)
	room := labelValue(labels, LabelLocationRoom)
	position := labelValue(labels, LabelLocationPosition)

	deviceInfo := &rlav1.DeviceInfo{
		Id:           &rlav1.UUID{Id: rack.GetRackId().GetId()},
		Name:         rack.GetMetadata().GetName(),
		Manufacturer: manufacturer,
		SerialNumber: serialNumber,
	}

	if model != "" {
		deviceInfo.Model = &model
	}

	if description := rack.GetMetadata().GetDescription(); description != "" {
		deviceInfo.Description = &description
	}

	location := &rlav1.Location{
		Region:     region,
		Datacenter: datacenter,
		Room:       room,
		Position:   position,
	}

	return &rlav1.Rack{
		Info:     deviceInfo,
		Location: location,
	}
}
