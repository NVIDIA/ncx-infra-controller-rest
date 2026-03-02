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
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/credential"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/endpoint"
)

func CurTime() time.Time {
	return time.Now().UTC().Round(time.Microsecond)
}

type ErrorChecker interface {
	IsErrNoRows(err error) bool
	IsUniqueConstraintError(err error) bool
}

func BuildDBConfigFromEnv() (Config, error) {
	port, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		return Config{}, errors.New("fail to retrieve port")
	}

	credential := credential.NewFromEnv("DB_USER", "DB_PASSWORD")
	if !credential.IsValid() {
		return Config{}, errors.New("invalid credential")
	}

	dbConf := Config{
		Endpoint: endpoint.Config{
			Host:              os.Getenv("DB_ADDR"),
			Port:              port,
			Credential:        &credential,
			CACertificatePath: os.Getenv("DB_CERT_PATH"),
		},
		DBName: os.Getenv("DB_DATABASE"),
	}

	return dbConf, nil
}
