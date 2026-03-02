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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredential(t *testing.T) {
	cred := New("testuser", "testpassword")

	assert.True(t, cred.IsValid())

	patched := cred.Patch(nil)
	assert.False(t, patched)

	nc := New("newuser", "newpassword")
	patched = cred.Patch(nc)
	assert.True(t, patched)
	assert.Equal(t, "newuser", cred.User)
	assert.Equal(t, "newpassword", cred.Password.Value)

	newUser := "updateduser"
	newPassword := "updatedpassword"
	cred.Update(&newUser, &newPassword)
	assert.Equal(t, "updateduser", cred.User)
	assert.Equal(t, "updatedpassword", cred.Password.Value)

	user, password := cred.Retrieve()
	assert.NotNil(t, user)
	assert.NotNil(t, password)
	assert.Equal(t, "updateduser", *user)
	assert.Equal(t, "updatedpassword", *password)
}

func TestNewCredentialFromEnv(t *testing.T) {
	os.Setenv("TEST_USER", "testuser")
	os.Setenv("TEST_PASSWORD", "testpassword")

	cred := NewFromEnv("TEST_USER", "TEST_PASSWORD")
	assert.Equal(t, "testuser", cred.User)
	assert.Equal(t, "testpassword", cred.Password.Value)
}
