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

	"github.com/NVIDIA/ncx-infra-controller-rest/common/pkg/endpoint"
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/carbideapi"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/clients/temporal"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/psmapi"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/executor"
	pkgcerts "github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/certs"
)

const (
	// DefaultPort is the default port the RLA gRPC server listens on.
	DefaultPort = 50051
)

// Config holds the service configuration.
// It uses interfaces to abstract implementation details:
//   - ExecutorConfig: abstracts the task executor (e.g., Temporal)
//   - CarbideClient: abstracts the hardware management API client
//   - PSMClient: abstracts the powershelf manager API client
type Config struct {
	Port          int
	DBConf        cdb.Config
	ExecutorConf  executor.ExecutorConfig
	CarbideClient carbideapi.Client
	PSMClient     psmapi.Client

	// CertConfig holds certificate file paths for the gRPC server listener.
	// When set, these take precedence over CERTDIR / the k8s default.
	// Either all three fields must be set or none.
	CertConfig pkgcerts.Config
}

// BuildTemporalConfigFromEnv builds a Temporal client configuration from
// environment variables: TEMPORAL_HOST, TEMPORAL_PORT, TEMPORAL_NAMESPACE,
// TEMPORAL_CERT_PATH, TEMPORAL_ENABLE_TLS, and TEMPORAL_SERVER_NAME.
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
