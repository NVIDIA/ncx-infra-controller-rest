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
package db

import (
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/credential"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate(t *testing.T) {
	sharedCredential := credential.New("admin", "password")

	tests := map[string]struct {
		config  Config
		wantErr bool
	}{
		"valid config": {
			config: Config{
				Host:       "localhost",
				Port:       5432,
				DBName:     "testdb",
				Credential: *sharedCredential,
			},
			wantErr: false,
		},
		"missing host": {
			config: Config{
				Port:       5432,
				DBName:     "testdb",
				Credential: *sharedCredential,
			},
			wantErr: true,
		},
		"invalid port (zero)": {
			config: Config{
				Host:       "localhost",
				Port:       0,
				DBName:     "testdb",
				Credential: *sharedCredential,
			},
			wantErr: true,
		},
		"missing database name": {
			config: Config{
				Host:       "localhost",
				Port:       5432,
				Credential: *sharedCredential,
			},
			wantErr: true,
		},
		"invalid credential": {
			config: Config{
				Host:   "localhost",
				Port:   5432,
				DBName: "testdb",
				Credential: credential.Credential{
					Password: sharedCredential.Password,
				},
			},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigBuildDSN(t *testing.T) {
	sharedCredential := credential.New("user", "password")

	tests := map[string]struct {
		config   Config
		expected string
	}{
		"with CA certificate": {
			config: Config{
				Host:              "localhost",
				Port:              5432,
				DBName:            "testdb",
				Credential:        *sharedCredential,
				CACertificatePath: "/path/to/ca.crt",
			},
			expected: "postgres://user:password@localhost:5432/testdb?sslmode=prefer&sslrootcert=/path/to/ca.crt",
		},
		"without CA certificate": {
			config: Config{
				Host:       "localhost",
				Port:       5432,
				DBName:     "testdb",
				Credential: *sharedCredential,
			},
			expected: "postgres://user:password@localhost:5432/testdb?sslmode=disable",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dsn := tt.config.BuildDSN()
			assert.Equal(t, tt.expected, dsn)
		})
	}
}
