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

package carbidecli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientDoRefreshesTokenOnUnauthorizedAndRetries(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, `{"message":"expired"}`, http.StatusUnauthorized)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer refreshed-token" {
			t.Fatalf("Authorization = %q, want Bearer refreshed-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	refreshes := 0
	client := NewClient(server.URL, "test-org", "stale-token", nil, false)
	client.TokenRefresh = func() (string, error) {
		refreshes++
		return "refreshed-token", nil
	}

	body, _, err := client.Do("GET", "/v2/org/{org}/carbide/test", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("body = %s", string(body))
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if refreshes != 1 {
		t.Fatalf("refreshes = %d, want 1", refreshes)
	}
}

func TestClientDoRetriesUnauthorizedAtMostThreeTimes(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		http.Error(w, `{"message":"expired"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	var events []AuthRetryEvent
	refreshes := 0
	client := NewClient(server.URL, "test-org", "stale-token", nil, false)
	client.TokenRefresh = func() (string, error) {
		refreshes++
		return "still-invalid-token", nil
	}
	client.AuthRetryNotify = func(event AuthRetryEvent) {
		events = append(events, event)
	}

	_, _, err := client.Do("GET", "/v2/org/{org}/carbide/test", nil, nil, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
	if requests != 4 {
		t.Fatalf("requests = %d, want 4", requests)
	}
	if refreshes != 3 {
		t.Fatalf("refreshes = %d, want 3", refreshes)
	}
	if len(events) != 6 {
		t.Fatalf("events = %d, want 6", len(events))
	}
	for i := 0; i < 3; i++ {
		login := events[i*2]
		retry := events[i*2+1]
		if login.Action != AuthRetryActionLogin || retry.Action != AuthRetryActionRetry {
			t.Fatalf("events[%d:%d] = %s/%s, want login/retry", i*2, i*2+1, login.Action, retry.Action)
		}
		if login.Attempt != i+1 || retry.Attempt != i+1 {
			t.Fatalf("attempt pair %d = %d/%d, want %d", i, login.Attempt, retry.Attempt, i+1)
		}
		if login.MaxAttempts != 3 || retry.MaxAttempts != 3 {
			t.Fatalf("max attempts = %d/%d, want 3", login.MaxAttempts, retry.MaxAttempts)
		}
	}
}

func TestClientDoReturnsUnauthorizedWhenNoRefreshFunc(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"expired"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-org", "stale-token", nil, false)
	_, _, err := client.Do("GET", "/v2/org/{org}/carbide/test", nil, nil, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
}

func TestClientDoDoesNotRefreshOnForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()

	refreshes := 0
	client := NewClient(server.URL, "test-org", "token", nil, false)
	client.TokenRefresh = func() (string, error) {
		refreshes++
		return "new-token", nil
	}

	_, _, err := client.Do("GET", "/v2/org/{org}/carbide/test", nil, nil, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusForbidden)
	}
	if refreshes != 0 {
		t.Fatalf("refreshes = %d, want 0", refreshes)
	}
}
