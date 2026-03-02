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
package credentials

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/credential"
	"strings"

	vault "github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
)

// The mount path for the secrets engine
const mountPath = "secrets"

// The path for storing PMC credentials
const credentialPath = mountPath + "/data/pmc"

// VaultConfig configures access to Vault (address and token). The token should be scoped minimally for KV operations.
type VaultConfig struct {
	Address string
	Token   string
}

// String returns the canonical string form of the version.
func (c VaultConfig) String() string {
	return fmt.Sprintf("Vault Address: %s; Vault Token: %s", c.Address, c.Token)
}

// Validate ensures required Vault fields are provided.
func (c *VaultConfig) Validate() error {
	if strings.TrimSpace(c.Address) == "" {
		return errors.New("invalid vault address specified")
	}

	if strings.TrimSpace(c.Token) == "" {
		return errors.New("invalid vault token specified")
	}

	return nil
}

// VaultCredentialManager implements the CredentialManager interface with a Vault store.
type VaultCredentialManager struct {
	client *vault.Client
}

// NewManager initializes a Vault client with the configured address and token.
// TLS verification is skipped to handle self-signed certificates in Kubernetes environments.
func (c *VaultConfig) NewManager() (*VaultCredentialManager, error) {
	config := &vault.Config{
		Address: c.Address,
		HttpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, //nolint:gosec // Skip TLS verify for internal K8s services
				},
			},
		},
	}
	client, err := vault.NewClient(config)
	if err != nil {
		return nil, err
	}

	client.SetToken(c.Token)

	return &VaultCredentialManager{
		client: client,
	}, nil
}

func (m *VaultCredentialManager) pathExists(path string) (bool, error) {
	mounts, err := m.client.Sys().ListMounts()
	if err != nil {
		return false, err
	}

	for mountPath := range mounts {
		if mountPath == path || mountPath == path+"/" {
			return true, nil
		}
	}
	return false, nil
}

func (m *VaultCredentialManager) configureVault() error {
	exists, err := m.pathExists(mountPath)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	data := map[string]any{
		"type": "kv-v2",
	}
	_, err = m.client.Logical().Write(fmt.Sprintf("/sys/mounts/%s", mountPath), data)
	return err
}

// Start ensures the Vault engine is mounted at the configured path.
func (m *VaultCredentialManager) Start(ctx context.Context) error {
	log.Printf("Starting Vault credential manager")
	return m.configureVault()
}

// Stop performs no cleanup.
func (m *VaultCredentialManager) Stop(ctx context.Context) error {
	log.Printf("Stopping Vault credential manager")
	return nil
}

func (m *VaultCredentialManager) getCredentialKey(mac net.HardwareAddr) string {
	return fmt.Sprintf("%s/%s", credentialPath, mac.String())
}

// Get retrieves and validates credentials for the given MAC from Vault.
func (m *VaultCredentialManager) Get(ctx context.Context, mac net.HardwareAddr) (*credential.Credential, error) {
	key := m.getCredentialKey(mac)
	secret, err := m.client.Logical().Read(key)
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Data == nil {
		return nil, errors.New("credential not found")
	}

	credData, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected secret data format")
	}

	credential, err := credential.FromMap(credData)
	if err != nil {
		return nil, err
	}

	if credential == nil || !credential.IsValid() {
		return nil, fmt.Errorf("retrieved invalid credential from vault")
	}

	return credential, nil
}

// Put writes the credentials of a given PMC (specified by MAC) to Vault.
func (m *VaultCredentialManager) Put(ctx context.Context, mac net.HardwareAddr, cred *credential.Credential) error {
	if cred == nil || !cred.IsValid() {
		return fmt.Errorf("valid credential not specified to Vault Manager")
	}

	payload := map[string]any{
		"data": cred.ToMap(),
	}

	key := m.getCredentialKey(mac)
	_, err := m.client.Logical().Write(key, payload)
	return err
}

// Patch replaces the PMC's credentials in Vault (equivalent to Put).
func (m *VaultCredentialManager) Patch(ctx context.Context, mac net.HardwareAddr, cred *credential.Credential) error {
	// Assuming Patch is similar to Put for simplicity
	return m.Put(ctx, mac, cred)
}

// Delete removes the credential specified by the PMC mac (if it exists) from Vault.
func (m *VaultCredentialManager) Delete(ctx context.Context, mac net.HardwareAddr) error {
	key := m.getCredentialKey(mac)
	_, err := m.client.Logical().Delete(key)
	return err
}

// Keys returns a list of PMC MACs for which credential manager has secrets for.
func (m *VaultCredentialManager) Keys(ctx context.Context) ([]net.HardwareAddr, error) {
	secret, err := m.client.Logical().List(credentialPath)
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Data == nil {
		return nil, errors.New("no credentials found")
	}

	keys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return nil, errors.New("unexpected data format")
	}

	macs := make([]net.HardwareAddr, 0, len(keys))
	for _, key := range keys {
		keyStr, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected key format: %v", key)
		}

		mac, err := net.ParseMAC(keyStr)
		if err != nil {
			return nil, err
		}

		if mac != nil {
			macs = append(macs, mac)
		}
	}

	return macs, nil
}
