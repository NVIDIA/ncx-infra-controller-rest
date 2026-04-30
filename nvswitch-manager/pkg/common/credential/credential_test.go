/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
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
