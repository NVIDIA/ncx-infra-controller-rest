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
	"net"
	"testing"

	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/credential"

	"github.com/stretchr/testify/assert"
)

func parseMAC(t *testing.T, s string) net.HardwareAddr {
	t.Helper()
	m, err := net.ParseMAC(s)
	assert.NoError(t, err, "failed to parse MAC %q", s)
	return m
}

func TestInMemoryStartStop(t *testing.T) {
	testCases := map[string]struct {
		setup func() *InMemoryCredentialManager
	}{
		"start and stop return nil": {
			setup: func() *InMemoryCredentialManager {
				return NewInMemoryCredentialManager()
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mgr := tc.setup()
			assert.NoError(t, mgr.Start(context.Background()))
			assert.NoError(t, mgr.Stop(context.Background()))
		})
	}
}

func TestInMemoryPutGet(t *testing.T) {
	testCases := map[string]struct {
		initialPut bool
		putMAC     string
		putCred    *credential.Credential
		getMAC     string
		wantErr    bool
		wantUser   string
		wantPass   string
		samePtr    bool
	}{
		"get existing valid credential": {
			initialPut: true,
			putMAC:     "00:11:22:33:44:55",
			putCred:    credential.New("admin", "secret"),
			getMAC:     "00:11:22:33:44:55",
			wantErr:    false,
			wantUser:   "admin",
			wantPass:   "secret",
			samePtr:    true,
		},
		"get existing invalid credential (empty user) returns not found": {
			initialPut: true,
			putMAC:     "00:11:22:33:44:66",
			putCred:    credential.New("", "nopass"),
			getMAC:     "00:11:22:33:44:66",
			wantErr:    true,
		},
		"get missing credential returns not found": {
			initialPut: false,
			getMAC:     "66:77:88:99:00:11",
			wantErr:    true,
		},
		"put overwrites existing value": {
			initialPut: true,
			putMAC:     "aa:bb:cc:dd:ee:ff",
			putCred:    credential.New("user1", "p1"),
			getMAC:     "aa:bb:cc:dd:ee:ff",
			wantErr:    false,
			wantUser:   "user2",
			wantPass:   "p2",
			samePtr:    true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			mgr := NewInMemoryCredentialManager()

			// Optional initial put
			if tc.initialPut {
				mac := parseMAC(t, tc.putMAC)
				assert.NoError(t, mgr.Put(ctx, mac, tc.putCred))
				// For the overwrite scenario, put a second credential to same MAC
				if name == "put overwrites existing value" {
					assert.NoError(t, mgr.Put(ctx, mac, credential.New("user2", "p2")))
				}
			}

			// Get flow
			got, err := mgr.Get(ctx, parseMAC(t, tc.getMAC))
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tc.wantUser, got.User)
			assert.Equal(t, tc.wantPass, got.Password.Value)

			if tc.samePtr && tc.initialPut && name != "put overwrites existing value" {
				// For non-overwrite cases, ensure returned pointer equals the one stored
				assert.Same(t, tc.putCred, got)
			}
		})
	}
}

func TestInMemoryPatch(t *testing.T) {
	testCases := map[string]struct {
		setupMAC      string
		setupCred     *credential.Credential
		patchMAC      string
		patchCred     *credential.Credential
		wantErr       bool
		wantUser      string
		wantPass      string
		expectSamePtr bool
	}{
		"patch existing replaces value": {
			setupMAC:      "00:11:22:33:44:55",
			setupCred:     credential.New("admin", "old"),
			patchMAC:      "00:11:22:33:44:55",
			patchCred:     credential.New("root", "new"),
			wantErr:       false,
			wantUser:      "root",
			wantPass:      "new",
			expectSamePtr: true,
		},
		"patch missing returns error": {
			setupMAC:  "aa:bb:cc:dd:ee:ff",
			setupCred: credential.New("user", "pass"),
			patchMAC:  "66:77:88:99:00:11",
			patchCred: credential.New("root", "new"),
			wantErr:   true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			mgr := NewInMemoryCredentialManager()

			// Put initial
			assert.NoError(t, mgr.Put(ctx, parseMAC(t, tc.setupMAC), tc.setupCred))

			// Patch
			err := mgr.Patch(ctx, parseMAC(t, tc.patchMAC), tc.patchCred)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Verify Get returns updated credential
			got, err := mgr.Get(ctx, parseMAC(t, tc.patchMAC))
			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tc.wantUser, got.User)
			assert.Equal(t, tc.wantPass, got.Password.Value)
			if tc.expectSamePtr {
				assert.Same(t, tc.patchCred, got)
			}
		})
	}
}

func TestInMemoryDelete(t *testing.T) {
	testCases := map[string]struct {
		putMAC       string
		putCred      *credential.Credential
		delMAC       string
		expectErrGet bool
	}{
		"delete existing removes entry": {
			putMAC:       "00:11:22:33:44:55",
			putCred:      credential.New("admin", "secret"),
			delMAC:       "00:11:22:33:44:55",
			expectErrGet: true,
		},
		"delete missing returns nil": {
			putMAC:       "aa:bb:cc:dd:ee:ff",
			putCred:      credential.New("user", "p"),
			delMAC:       "66:77:88:99:00:11",
			expectErrGet: false, // original still present
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			mgr := NewInMemoryCredentialManager()

			// Put initial
			assert.NoError(t, mgr.Put(ctx, parseMAC(t, tc.putMAC), tc.putCred))

			// Delete target
			assert.NoError(t, mgr.Delete(ctx, parseMAC(t, tc.delMAC)))

			// Verify
			_, err := mgr.Get(ctx, parseMAC(t, tc.delMAC))
			if tc.expectErrGet {
				assert.Error(t, err)
			} else {
				// Ensure original entry remains when deleting a different MAC
				got, err2 := mgr.Get(ctx, parseMAC(t, tc.putMAC))
				assert.NoError(t, err2)
				assert.NotNil(t, got)
				assert.Equal(t, tc.putCred.User, got.User)
				assert.Equal(t, tc.putCred.Password.Value, got.Password.Value)
			}
		})
	}
}

func TestInMemoryKeys(t *testing.T) {
	testCases := map[string]struct {
		putPairs    [][2]interface{} // [mac string, *credential.Credential]
		expectCount int
		expectSet   map[string]bool
	}{
		"no entries returns empty": {
			putPairs:    nil,
			expectCount: 0,
			expectSet:   map[string]bool{},
		},
		"one entry returns that MAC": {
			putPairs: [][2]interface{}{
				{"00:11:22:33:44:55", credential.New("admin", "secret")},
			},
			expectCount: 1,
			expectSet:   map[string]bool{"00:11:22:33:44:55": true},
		},
		"multiple entries return all MACs": {
			putPairs: [][2]interface{}{
				{"00:11:22:33:44:55", credential.New("admin", "a")},
				{"66:77:88:99:00:11", credential.New("root", "r")},
				{"aa:bb:cc:dd:ee:ff", credential.New("user", "u")},
			},
			expectCount: 3,
			expectSet: map[string]bool{
				"00:11:22:33:44:55": true,
				"66:77:88:99:00:11": true,
				"aa:bb:cc:dd:ee:ff": true,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			mgr := NewInMemoryCredentialManager()

			// Populate
			for _, pair := range tc.putPairs {
				macStr := pair[0].(string)
				cred := pair[1].(*credential.Credential)
				assert.NoError(t, mgr.Put(ctx, parseMAC(t, macStr), cred))
			}

			// Keys
			keys, err := mgr.Keys(ctx)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectCount, len(keys))

			// Build set from returned keys for lookup
			gotSet := make(map[string]bool, len(keys))
			for _, mac := range keys {
				gotSet[mac.String()] = true
			}
			assert.Equal(t, tc.expectSet, gotSet)
		})
	}
}
