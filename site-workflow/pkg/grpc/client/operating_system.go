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

package client

import (
	"context"
	"errors"
	"os"

	wflows "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/schema/site-agent/workflows/v1"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
)

// OperatingSystemInterface is the interface for OsImage related Carbide client operations
type OperatingSystemInterface interface {
	CreateOsImage(ctx context.Context, request *wflows.OsImageAttributes) (*wflows.OsImage, error)
	UpdateOsImage(ctx context.Context, request *wflows.OsImageAttributes) (*wflows.OsImage, error)
	DeleteOsImage(ctx context.Context, request *wflows.DeleteOsImageRequest) (*wflows.DeleteOsImageResponse, error)
	GetOsImage(ctx context.Context, request *wflows.UUID) (response *wflows.OsImage, err error)
	ListOsImage(ctx context.Context, request *wflows.ListOsImageRequest) (*wflows.ListOsImageResponse, error)
}

// CreateOsImage creates a new OS image
func (osi *compute) CreateOsImage(ctx context.Context, request *wflows.OsImageAttributes) (*wflows.OsImage, error) {
	log.Info().Interface("request", request).Msg("CreateOsImage: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-CreateOsImage")
	defer span.End()

	// Validate the request
	if request == nil {
		err := errors.New("CreateOsImage: invalid request")
		log.Error().Err(err).Msg("CreateOsImage: invalid request")
		return nil, err
	}

	response, err := osi.carbide.CreateOsImage(ctx, request)
	return response, err
}

// UpdateOsImage updates an existing OS image
func (osi *compute) UpdateOsImage(ctx context.Context, request *wflows.OsImageAttributes) (*wflows.OsImage, error) {
	log.Info().Interface("request", request).Msg("UpdateOsImage: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-UpdateOsImage")
	defer span.End()

	// Validate the request
	if request == nil {
		err := errors.New("UpdateOsImage: invalid request")
		log.Error().Err(err).Msg("UpdateOsImage: invalid request")
		return nil, err
	}

	response, err := osi.carbide.UpdateOsImage(ctx, request)
	return response, err
}

// DeleteOsImage deletes an existing OS image
func (osi *compute) DeleteOsImage(ctx context.Context, request *wflows.DeleteOsImageRequest) (*wflows.DeleteOsImageResponse, error) {
	log.Info().Interface("request", request).Msg("DeleteOsImage: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-DeleteOsImage")
	defer span.End()

	// Validate the request
	if request == nil {
		err := errors.New("DeleteOsImage: invalid request")
		log.Error().Err(err).Msg("DeleteOsImage: invalid request")
		return nil, err
	}

	response, err := osi.carbide.DeleteOsImage(ctx, request)
	return response, err
}

// GetOsImage retrieves an existing OS image
func (osi *compute) GetOsImage(ctx context.Context, request *wflows.UUID) (response *wflows.OsImage, err error) {
	log.Info().Interface("request", request).Msg("GetOsImage: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-GetOsImage")
	defer span.End()

	// Validate the request
	if request == nil {
		err = errors.New("GetOsImage: invalid request")
		log.Error().Err(err).Msg("GetOsImage: invalid request")
		return nil, err
	}

	response, err = osi.carbide.GetOsImage(ctx, request)
	return
}

// ListOsImage retrieves a list of OS images
func (osi *compute) ListOsImage(ctx context.Context, request *wflows.ListOsImageRequest) (*wflows.ListOsImageResponse, error) {
	log.Info().Interface("request", request).Msg("ListOsImage: received request")
	ctx, span := otel.Tracer(os.Getenv("LS_SERVICE_NAME")).Start(ctx, "CarbideClient-ListOsImage")
	defer span.End()

	// Validate the request
	if request == nil {
		err := errors.New("ListOsImage: invalid request")
		log.Error().Err(err).Msg("ListOsImage: invalid request")
		return nil, err
	}

	response, err := osi.carbide.ListOsImage(ctx, request)
	return response, err
}
