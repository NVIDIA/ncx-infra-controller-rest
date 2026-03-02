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
package credential

import (
	"errors"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/secretstring"
	"os"
	"strings"
)

// Credential holds authentication information with password protection
type Credential struct {
	User     string                    `json:"user"`     // User name
	Password secretstring.SecretString `json:"password"` // Password (masked in JSON/logs)
}

// New creates a Credential with the given user and password.
func New(user string, password string) *Credential {
	return &Credential{
		User:     user,
		Password: secretstring.New(password),
	}
}

// NewFromEnv creates a Credential from environment variables as-is
func NewFromEnv(userEnv string, passwordEnv string) Credential {
	return Credential{
		User:     os.Getenv(userEnv),
		Password: secretstring.New(os.Getenv(passwordEnv)),
	}
}

// Patch updates the credential with non-empty values from the given
// credential. It returns true if any field was updated.
func (cred *Credential) Patch(nc *Credential) bool {
	if cred == nil || nc == nil {
		return false
	}

	patched := false

	if strings.TrimSpace(nc.User) != "" && cred.User != nc.User {
		cred.User = nc.User
		patched = true
	}

	if !nc.Password.IsEmpty() && !cred.Password.IsEqual(nc.Password) {
		cred.Password = nc.Password
		patched = true
	}

	return patched
}

// IsValid returns true if the credential has a non-empty username
// Note: Password validation is intentionally not included for flexibility
func (cred *Credential) IsValid() bool {
	return strings.TrimSpace(cred.User) != ""
}

// Update modifies credential fields if the provided pointers are not nil
func (cred *Credential) Update(user *string, password *string) {
	if user != nil {
		cred.User = *user
	}

	if password != nil {
		cred.Password.Value = *password
	}
}

// Retrieve returns pointers to user and password if credential is valid
// Returns nil pointers if credential is invalid
func (cred *Credential) Retrieve() (*string, *string) {
	if !cred.IsValid() {
		return nil, nil
	}

	// Create a copy to avoid exposing internal state
	c := *cred
	return &c.User, &c.Password.Value
}

// ToMap converts a Credential to a map[string]interface{} suitable for Vault storage
func (c Credential) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"username": c.User,
		"password": c.Password.Value, // Store the actual password value in Vault
	}
}

// FromMap converts a map[string]interface{} from Vault storage to a Credential
func FromMap(data map[string]interface{}) (*Credential, error) {
	user, ok := data["username"].(string)
	if !ok {
		return nil, errors.New("invalid username value")
	}

	password, ok := data["password"].(string)
	if !ok {
		return nil, errors.New("invalid password value")
	}

	return New(user, password), nil
}
