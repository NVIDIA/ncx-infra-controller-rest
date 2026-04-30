/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package secretstring

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretString(t *testing.T) {
	s := New("secret")
	assert.Equal(t, "******", s.String())

	jsonData, err := json.Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, "\"******\"", string(jsonData))
	assert.False(t, s.IsEmpty())

	s.Value = " "
	assert.True(t, s.IsEmpty())

	s.Value = "not empty"
	assert.False(t, s.IsEmpty())

	assert.True(t, s.IsEqual(SecretString{Value: "not empty"}))
	assert.False(t, s.IsEqual(SecretString{Value: "different"}))
}
