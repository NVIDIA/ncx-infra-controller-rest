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
