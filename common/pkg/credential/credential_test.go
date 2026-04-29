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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredentialEqual(t *testing.T) {
	tests := map[string]struct {
		a      *Credential
		b      *Credential
		expect bool
	}{
		"both nil": {
			a: nil, b: nil, expect: true,
		},
		"first nil": {
			a: nil, b: ptr(New("admin", "pass")), expect: false,
		},
		"second nil": {
			a: ptr(New("admin", "pass")), b: nil, expect: false,
		},
		"identical": {
			a: ptr(New("admin", "secret")), b: ptr(New("admin", "secret")), expect: true,
		},
		"different user": {
			a: ptr(New("admin", "secret")), b: ptr(New("root", "secret")), expect: false,
		},
		"different password": {
			a: ptr(New("admin", "secret")), b: ptr(New("admin", "other")), expect: false,
		},
		"both differ": {
			a: ptr(New("admin", "secret")), b: ptr(New("root", "other")), expect: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.a.Equal(tc.b))
		})
	}
}

func ptr(c Credential) *Credential {
	return &c
}
