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
	"os"
	"strconv"

	"github.com/nvidia/bare-metal-manager-rest/nvswitch-manager/pkg/common/credential"
)

// Config represents the configuration needed to connect to a database.
type Config struct {
	Host              string
	Port              int
	DBName            string
	Credential        credential.Credential
	CACertificatePath string
}

// Validate checks if the Config fields are set correctly.
func (c *Config) Validate() error {
	if c.Host == "" {
		return errors.New("host is required")
	}

	if c.Port <= 0 || c.Port > 65535 {
		return errors.New("port must be between (0, 65535]")
	}

	if c.DBName == "" {
		return errors.New("database name is required")
	}

	if !c.Credential.IsValid() {
		return errors.New("valid credential is required")
	}

	return nil
}

// BuildDSN builds the Data Source Name (DSN) string for connecting to
// the database.
func (c *Config) BuildDSN() string {
	dsn := fmt.Sprintf(
		"postgres://%v:%v@%v:%v/%v?sslmode=",
		c.Credential.User,
		c.Credential.Password.Value,
		c.Host,
		c.Port,
		c.DBName,
	)

	if len(c.CACertificatePath) > 0 {
		// Use sslmode=prefer (like RLA) instead of verify-full to avoid issues with expired server certs
		dsn += fmt.Sprintf("prefer&sslrootcert=%v", c.CACertificatePath)
	} else {
		dsn += "disable"
	}

	return dsn
}

// BuildDBConfigFromEnv builds a Config from environment variables.
// Required env vars: DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD
// Optional: DB_CA_CERT_PATH
func BuildDBConfigFromEnv() (Config, error) {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}

	portStr := os.Getenv("DB_PORT")
	if portStr == "" {
		return Config{}, errors.New("DB_PORT environment variable is required")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid DB_PORT: %v", err)
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "nvswitch_manager"
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}

	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "postgres"
	}

	caCertPath := os.Getenv("DB_CA_CERT_PATH")

	config := Config{
		Host:              host,
		Port:              port,
		DBName:            dbName,
		CACertificatePath: caCertPath,
	}
	config.Credential.Update(&user, &password)

	return config, nil
}
