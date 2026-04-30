/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package service

import (
	"context"
	"testing"

	pb "github.com/NVIDIA/ncx-infra-controller-rest/nvswitch-manager/internal/proto/v1"
	"github.com/NVIDIA/ncx-infra-controller-rest/nvswitch-manager/pkg/redfish"

	"github.com/stretchr/testify/assert"
)

func TestResetTarget_InvalidIP(t *testing.T) {
	tests := map[string]struct {
		ip string
	}{
		"empty":   {ip: ""},
		"garbage": {ip: "bmc-bad-addr"},
		"partial": {ip: "172.16.0"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			target := &pb.PowerTarget{
				BmcIp: tc.ip,
				BmcCredentials: &pb.Credentials{
					Username: "bmcAdmin",
					Password: "bmcSecret",
				},
			}

			resp := resetTarget(context.Background(), target, redfish.ResetOn)

			assert.Equal(t, pb.StatusCode_INVALID_ARGUMENT, resp.Status)
			assert.Equal(t, tc.ip, resp.BmcIp)
			assert.Contains(t, resp.Error, "invalid BMC IP")
		})
	}
}

func TestResetTarget_NilCredentials(t *testing.T) {
	target := &pb.PowerTarget{
		BmcIp:          "172.16.0.10",
		BmcCredentials: nil,
	}

	resp := resetTarget(context.Background(), target, redfish.ResetOn)

	assert.Equal(t, pb.StatusCode_INVALID_ARGUMENT, resp.Status)
	assert.Equal(t, "172.16.0.10", resp.BmcIp)
	assert.Contains(t, resp.Error, "bmc_credentials are required")
}

func TestResetTarget_EmptyCredentials(t *testing.T) {
	tests := map[string]struct {
		username string
		password string
	}{
		"empty username": {username: "", password: "bmcSecret"},
		"empty password": {username: "bmcAdmin", password: ""},
		"both empty":     {username: "", password: ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			target := &pb.PowerTarget{
				BmcIp: "172.16.0.10",
				BmcCredentials: &pb.Credentials{
					Username: tc.username,
					Password: tc.password,
				},
			}

			resp := resetTarget(context.Background(), target, redfish.ResetOn)

			assert.Equal(t, pb.StatusCode_INVALID_ARGUMENT, resp.Status)
			assert.Contains(t, resp.Error, "must not be empty")
		})
	}
}
