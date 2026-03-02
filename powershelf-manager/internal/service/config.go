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
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/credential"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/credentials"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/pmcregistry"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/powershelfmanager"
	"os"
	"strconv"
)

// Config captures runtime settings for running the gRPC service, including the public port,
// the datastore mode (Persistent or InMemory), and concrete configurations for the PMC registry and credential manager backends.
type Config struct {
	Port          int
	DataStoreType powershelfmanager.DataStoreType
	VaultConf     credentials.VaultConfig
	DBConf        db.Config
}

// toCredentialManagerConf converts the public service Config into a pmcregistry.Config,
// mapping datastore selection to a concrete registry backend.
func (c *Config) toCredentialManagerConf() (*credentials.Config, error) {
	var dataStoreType credentials.DataStoreType
	var vaultConfig *credentials.VaultConfig
	switch c.DataStoreType {
	case powershelfmanager.DatastoreTypePersistent:
		dataStoreType = credentials.DatastoreTypeVault
		vaultConfig = &credentials.VaultConfig{Address: c.VaultConf.Address, Token: c.VaultConf.Token}
	case powershelfmanager.DatastoreTypeInMemory:
		dataStoreType = credentials.DatastoreTypeInMemory
	}
	return &credentials.Config{
		DataStoreType: dataStoreType,
		VaultConfig:   vaultConfig,
	}, nil
}

// toDataStoreConf converts the public service Config into a pmcregistry.Config,
// mapping datastore selection to a concrete credential backend.
func (c *Config) toDataStoreConf() (*pmcregistry.Config, error) {
	var pmcRegistryType pmcregistry.PmcRegisterType
	switch c.DataStoreType {
	case powershelfmanager.DatastoreTypePersistent:
		pmcRegistryType = pmcregistry.RegisterTypePostgres
	case powershelfmanager.DatastoreTypeInMemory:
		pmcRegistryType = pmcregistry.RegisterTypeInMemory
	}
	return &pmcregistry.Config{
		DSType: pmcRegistryType,
		DSConf: c.DBConf,
	}, nil
}

// ToPsmConf converts the public service Config into a powershelfmanager.Config,
// mapping datastore selection to concrete registry and credential backends.
func (c *Config) ToPsmConf() (*powershelfmanager.Config, error) {
	credentialManagerConf, err := c.toCredentialManagerConf()
	if err != nil {
		return nil, err
	}

	dataStoreConf, err := c.toDataStoreConf()
	if err != nil {
		return nil, err
	}

	psmConf := powershelfmanager.Config{
		DSType:          c.DataStoreType,
		CredentialConf:  *credentialManagerConf,
		PmcRegistryConf: *dataStoreConf,
	}

	return &psmConf, nil
}

// BuildDBConfigFromEnv builds db.Config from environment variables (DB_ADDR, DB_PORT, DB_USER, DB_PASSWORD, DB_DATABASE).
// Returns an error if the port is not a valid integer.
func BuildDBConfigFromEnv() (*db.Config, error) {
	port, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		return nil, errors.New("fail to retrieve port")
	}

	dbConf := db.Config{
		Host:       os.Getenv("DB_ADDR"),
		Port:       port,
		Credential: credential.NewFromEnv("DB_USER", "DB_PASSWORD"),
		DBName:     os.Getenv("DB_DATABASE"),
	}

	return &dbConf, nil
}

// BuildVaultConfigFromEnv builds credentials.VaultConfig from environment variables (VAULT_ADDR, VAULT_TOKEN).
func BuildVaultConfigFromEnv() (*credentials.VaultConfig, error) {
	return &credentials.VaultConfig{
		Address: os.Getenv("VAULT_ADDR"),
		Token:   os.Getenv("VAULT_TOKEN"),
	}, nil
}
