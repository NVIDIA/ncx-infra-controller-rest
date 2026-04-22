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
	"testing"

	pb "github.com/NVIDIA/ncx-infra-controller-rest/powershelf-manager/internal/proto/v1"

	"github.com/stretchr/testify/assert"
)

func newTestServer() *PowershelfManagerServerImpl {
	return &PowershelfManagerServerImpl{}
}

func TestPowerTarget_InvalidIP(t *testing.T) {
	tests := map[string]struct {
		ip string
	}{
		"empty":   {ip: ""},
		"garbage": {ip: "pmc-bad-addr"},
		"partial": {ip: "10.20.30"},
	}

	s := newTestServer()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			target := &pb.PowerTarget{
				PmcIp: tc.ip,
				PmcCredentials: &pb.Credentials{
					Username: "pmcUser",
					Password: "pmcPass",
				},
			}

			resp := s.powerTarget(context.Background(), target, true)

			assert.Equal(t, pb.StatusCode_INVALID_ARGUMENT, resp.Status)
			assert.Equal(t, tc.ip, resp.PmcIp)
			assert.Contains(t, resp.Error, "invalid PMC IP")
		})
	}
}

func TestPowerTarget_NilCredentials(t *testing.T) {
	s := newTestServer()
	target := &pb.PowerTarget{
		PmcIp:          "10.20.30.40",
		PmcCredentials: nil,
	}

	resp := s.powerTarget(context.Background(), target, true)

	assert.Equal(t, pb.StatusCode_INVALID_ARGUMENT, resp.Status)
	assert.Equal(t, "10.20.30.40", resp.PmcIp)
	assert.Contains(t, resp.Error, "credentials are required")
}

func TestPowerTarget_EmptyCredentials(t *testing.T) {
	tests := map[string]struct {
		username string
		password string
	}{
		"empty username": {username: "", password: "pmcPass"},
		"empty password": {username: "pmcUser", password: ""},
		"both empty":     {username: "", password: ""},
	}

	s := newTestServer()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			target := &pb.PowerTarget{
				PmcIp: "10.20.30.40",
				PmcCredentials: &pb.Credentials{
					Username: tc.username,
					Password: tc.password,
				},
			}

			resp := s.powerTarget(context.Background(), target, true)

			assert.Equal(t, pb.StatusCode_INVALID_ARGUMENT, resp.Status)
			assert.Contains(t, resp.Error, "must not be empty")
		})
	}
}
