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
	"fmt"

	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/endpoint"
)

// Config represents the configuration needed to connect to a database.
type Config struct {
	Endpoint endpoint.Config
	DBName   string
}

// Validate checks if the Config fields are set correctly.
func (c *Config) Validate() error {
	if err := c.Endpoint.Validate(); err != nil {
		return err
	}

	if c.DBName == "" {
		return errors.New("database name is required")
	}

	return nil
}

// BuildDSN builds the Data Source Name (DSN) string for connecting to
// the database.
func (c *Config) BuildDSN() string {
	dsn := fmt.Sprintf(
		"postgres://%v:%v@%s/%v?sslmode=",
		c.Endpoint.Credential.User,
		c.Endpoint.Credential.Password.Value,
		c.Endpoint.Target(),
		c.DBName,
	)

	if len(c.Endpoint.CACertificatePath) > 0 {
		dsn += fmt.Sprintf(
			"prefer&sslrootcert=%v",
			c.Endpoint.CACertificatePath,
		)
	} else {
		dsn += "disable"
	}

	return dsn
}
