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
	"errors"
	"os"
	"strconv"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/carbideapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/clients/temporal"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/psmapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/endpoint"
)

const (
	DefaultPort = 50051
)

// Config holds the service configuration.
// It uses interfaces to abstract implementation details:
//   - ExecutorConfig: abstracts the task executor (e.g., Temporal)
//   - CarbideClient: abstracts the hardware management API client
//   - PSMClient: abstracts the powershelf manager API client
type Config struct {
	Port          int
	DBConf        db.Config
	ExecutorConf  executor.ExecutorConfig
	CarbideClient carbideapi.Client
	PSMClient     psmapi.Client
}

func BuildTemporalConfigFromEnv() (*temporal.Config, error) {
	host := os.Getenv("TEMPORAL_HOST")
	if host == "" {
		return nil, errors.New("TEMPORAL_HOST is not set")
	}

	port, err := strconv.Atoi(os.Getenv("TEMPORAL_PORT"))
	if err != nil {
		return nil, errors.New("fail to retrieve port")
	}

	namespace := os.Getenv("TEMPORAL_NAMESPACE")
	if namespace == "" {
		return nil, errors.New("TEMPORAL_NAMESPACE is not set")
	}

	return &temporal.Config{
		Endpoint: endpoint.Config{
			Host:              host,
			Port:              port,
			CACertificatePath: os.Getenv("TEMPORAL_CERT_PATH"),
		},
		EnableTLS:  os.Getenv("TEMPORAL_ENABLE_TLS") == "true",
		Namespace:  namespace,
		ServerName: os.Getenv("TEMPORAL_SERVER_NAME"),
	}, nil
}
