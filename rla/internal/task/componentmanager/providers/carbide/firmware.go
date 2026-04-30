/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package carbide

import (
	pb "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/carbideapi/gen"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/operations"
)

// MapFirmwareState converts a Carbide protobuf FirmwareUpdateState into the
// corresponding operations.FirmwareUpdateState.
func MapFirmwareState(state pb.FirmwareUpdateState) operations.FirmwareUpdateState {
	switch state {
	case pb.FirmwareUpdateState_FW_STATE_QUEUED:
		return operations.FirmwareUpdateStateQueued
	case pb.FirmwareUpdateState_FW_STATE_IN_PROGRESS:
		return operations.FirmwareUpdateStateQueued // closest available state
	case pb.FirmwareUpdateState_FW_STATE_VERIFYING:
		return operations.FirmwareUpdateStateVerifying
	case pb.FirmwareUpdateState_FW_STATE_COMPLETED:
		return operations.FirmwareUpdateStateCompleted
	case pb.FirmwareUpdateState_FW_STATE_FAILED, pb.FirmwareUpdateState_FW_STATE_CANCELLED:
		return operations.FirmwareUpdateStateFailed
	default:
		return operations.FirmwareUpdateStateUnknown
	}
}
