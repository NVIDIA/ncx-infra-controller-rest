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

package operations

import (
	"testing"

	taskcommon "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
)

func TestPowerOperationCodeString(t *testing.T) {
	tests := []struct {
		name         string
		operation    PowerOperation
		expectedCode string
	}{
		{"PowerOn", PowerOperationPowerOn, taskcommon.OpCodePowerControlPowerOn},
		{"ForcePowerOn", PowerOperationForcePowerOn, taskcommon.OpCodePowerControlForcePowerOn},
		{"PowerOff", PowerOperationPowerOff, taskcommon.OpCodePowerControlPowerOff},
		{"ForcePowerOff", PowerOperationForcePowerOff, taskcommon.OpCodePowerControlForcePowerOff},
		{"Restart", PowerOperationRestart, taskcommon.OpCodePowerControlRestart},
		{"ForceRestart", PowerOperationForceRestart, taskcommon.OpCodePowerControlForceRestart},
		{"WarmReset", PowerOperationWarmReset, taskcommon.OpCodePowerControlWarmReset},
		{"ColdReset", PowerOperationColdReset, taskcommon.OpCodePowerControlColdReset},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.operation.CodeString()
			if got != tt.expectedCode {
				t.Errorf("PowerOperation.CodeString() = %v, want %v", got, tt.expectedCode)
			}
		})
	}
}

func TestFirmwareOperationCodeString(t *testing.T) {
	tests := []struct {
		name         string
		operation    FirmwareOperation
		expectedCode string
	}{
		{"Upgrade", FirmwareOperationUpgrade, taskcommon.OpCodeFirmwareControlUpgrade},
		{"Downgrade", FirmwareOperationDowngrade, taskcommon.OpCodeFirmwareControlDowngrade},
		{"Rollback", FirmwareOperationRollback, taskcommon.OpCodeFirmwareControlRollback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.operation.CodeString()
			if got != tt.expectedCode {
				t.Errorf("FirmwareOperation.CodeString() = %v, want %v", got, tt.expectedCode)
			}
		})
	}
}
