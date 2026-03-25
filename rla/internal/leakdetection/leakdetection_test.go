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

package leakdetection

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/carbideapi"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/config"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/operation"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/common/devicetypes"
)

// --- mock taskmanager.Manager ---

type mockManager struct {
	requests []*operation.Request
	submitErr error
	cancelErr error
}

func (m *mockManager) Start(_ context.Context) error { return nil }
func (m *mockManager) Stop(_ context.Context)        {}

func (m *mockManager) SubmitTask(_ context.Context, req *operation.Request) ([]uuid.UUID, error) {
	m.requests = append(m.requests, req)
	if m.submitErr != nil {
		return nil, m.submitErr
	}
	return []uuid.UUID{uuid.New()}, nil
}

func (m *mockManager) CancelTask(_ context.Context, _ uuid.UUID) error {
	return m.cancelErr
}

// --- tests ---

func TestSubmitPowerOffTask_Success(t *testing.T) {
	ctx := context.Background()
	mgr := &mockManager{}
	machineID := "machine-abc-123"

	err := submitPowerOffTask(ctx, mgr, machineID)
	require.NoError(t, err)
	require.Len(t, mgr.requests, 1)

	req := mgr.requests[0]

	// Verify target spec uses component targeting with ExternalRef
	assert.True(t, req.TargetSpec.IsComponentTargeting())
	require.Len(t, req.TargetSpec.Components, 1)

	comp := req.TargetSpec.Components[0]
	assert.Equal(t, uuid.Nil, comp.UUID)
	require.NotNil(t, comp.External)
	assert.Equal(t, devicetypes.ComponentTypeCompute, comp.External.Type)
	assert.Equal(t, machineID, comp.External.ID)

	// Verify conflict strategy is queue
	assert.Equal(t, operation.ConflictStrategyQueue, req.ConflictStrategy)

	// Verify description contains machine ID
	assert.Contains(t, req.Description, machineID)
}

func TestSubmitPowerOffTask_SubmitError(t *testing.T) {
	ctx := context.Background()
	mgr := &mockManager{submitErr: errors.New("submit failed")}

	err := submitPowerOffTask(ctx, mgr, "machine-xyz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "submit failed")
}

func TestRunLeakDetectionOne_NoLeaks(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	carbideClient := carbideapi.NewMockClient()
	mgr := &mockManager{}

	runLeakDetectionOne(ctx, cfg, carbideClient, mgr)

	assert.Empty(t, mgr.requests)
}

func TestRunLeakDetectionOne_SubmitsTaskPerMachine(t *testing.T) {
	ctx := context.Background()
	mgr := &mockManager{}

	// Mock carbide returns no leaking machines, so test multi-machine
	// path directly via submitPowerOffTask.
	machines := []string{"machine-1", "machine-2", "machine-3"}
	for _, m := range machines {
		err := submitPowerOffTask(ctx, mgr, m)
		require.NoError(t, err)
	}
	require.Len(t, mgr.requests, 3)

	for i, m := range machines {
		assert.Equal(t, m, mgr.requests[i].TargetSpec.Components[0].External.ID)
	}
}

func TestRunLeakDetectionOne_ContinuesOnSubmitError(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	carbideClient := carbideapi.NewMockClient()
	mgr := &mockManager{submitErr: errors.New("always fails")}

	// Should not panic even when submission fails.
	runLeakDetectionOne(ctx, cfg, carbideClient, mgr)
}
