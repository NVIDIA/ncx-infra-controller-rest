/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package error

import (
	"go.temporal.io/sdk/temporal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrTypeInvalidRequest is returned when the request is invalid
	ErrTypeInvalidRequest            = "InvalidRequest"
	ErrTypeCarbideObjectNotFound     = "CarbideObjectNotFound"
	ErrTypeCarbideUnimplemented      = "CarbideUnimplemented"
	ErrTypeCarbideUnavailable        = "CarbideUnavailable"
	ErrTypeCarbideDenied             = "CarbideDenied"
	ErrTypeCarbideAlreadyExists      = "CarbideAlreadyExists"
	ErrTypeCarbideFailedPrecondition = "CarbideFailedPrecondition"
	ErrTypeCarbideInvalidArgument    = "CarbideInvalidArgument"
)

// WrapError accepts an error and checks if it
// can be converted to a gRPC status.
//
// If the error can be converted and the status code matches a
// set of specific codes, it will be "wrapped" in a
// Temporal NewNonRetryableApplicationError.
//
// Otherwise, it returns the original error.
func WrapErr(err error) error {
	status, hasGrpcStatus := status.FromError(err)
	if hasGrpcStatus {
		switch status.Code() {
		case codes.NotFound:
			// If this is a 404 back from Carbide, we'll bubble that back up as a custom temporal error.
			return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeCarbideObjectNotFound, err)
		case codes.Unimplemented:
			return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeCarbideUnimplemented, err)
		case codes.Unavailable:
			return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeCarbideUnavailable, err)
		case codes.PermissionDenied:
			return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeCarbideDenied, err)
		case codes.AlreadyExists:
			return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeCarbideAlreadyExists, err)
		case codes.FailedPrecondition:
			return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeCarbideFailedPrecondition, err)
		case codes.InvalidArgument:
			return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeCarbideInvalidArgument, err)
		}
	}
	return err
}
