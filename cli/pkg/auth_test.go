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
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	cli "github.com/urfave/cli/v2"
)

func TestExtractNGCToken(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "token field",
			body: `{"token": "abc123"}`,
			want: "abc123",
		},
		{
			name: "access_token field",
			body: `{"access_token": "xyz789"}`,
			want: "xyz789",
		},
		{
			name: "token takes precedence over access_token",
			body: `{"token": "primary", "access_token": "secondary"}`,
			want: "primary",
		},
		{
			name: "empty response",
			body: `{}`,
			want: "",
		},
		{
			name: "invalid json",
			body: `not json`,
			want: "",
		},
		{
			name: "empty token values",
			body: `{"token": "", "access_token": ""}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNGCToken([]byte(tt.body))
			if got != tt.want {
				t.Errorf("extractNGCToken(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestLoginWithTokenCommandSavesTokenAndCommand(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	cfg := &ConfigFile{}

	token, err := LoginWithTokenCommand(cfg, configPath, "printf script-token")
	if err != nil {
		t.Fatal(err)
	}
	if token != "script-token" {
		t.Fatalf("token = %q, want script-token", token)
	}

	loaded, err := LoadConfigFromPath(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Auth.Token != "script-token" {
		t.Errorf("saved token = %q, want script-token", loaded.Auth.Token)
	}
	if loaded.Auth.TokenCommand != "printf script-token" {
		t.Errorf("saved token command = %q", loaded.Auth.TokenCommand)
	}
}

func TestLoginWithTokenCommandRejectsEmptyOutput(t *testing.T) {
	cfg := &ConfigFile{}
	_, err := LoginWithTokenCommand(cfg, filepath.Join(t.TempDir(), "config.yaml"), "printf ''")
	if err == nil {
		t.Fatal("expected empty token error")
	}
}

func TestAutoRefreshTokenToPathSavesSelectedConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			t.Fatalf("grant_type = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-token","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	defaultPath := filepath.Join(dir, "default.yaml")
	selectedPath := filepath.Join(dir, "selected.yaml")
	SetConfigPath(defaultPath)
	defer SetConfigPath("")

	cfg := &ConfigFile{
		Auth: ConfigAuth{
			OIDC: &ConfigOIDC{
				TokenURL:     server.URL,
				ClientID:     "client-id",
				Token:        "old-token",
				RefreshToken: "old-refresh",
				ExpiresAt:    time.Now().Add(-time.Hour).Format(time.RFC3339),
			},
		},
	}

	token, err := AutoRefreshTokenToPath(cfg, selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if token != "new-token" {
		t.Fatalf("token = %q, want new-token", token)
	}

	selected, err := LoadConfigFromPath(selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Auth.OIDC.Token != "new-token" {
		t.Fatalf("selected config token = %q", selected.Auth.OIDC.Token)
	}
	if _, err := os.Stat(defaultPath); !os.IsNotExist(err) {
		t.Fatalf("default config should not be written, stat err=%v", err)
	}
}

func TestLoginCommandExplicitAPIKeyWinsOverOIDCFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "ApiKey explicit-key" {
			t.Fatalf("Authorization = %q, want ApiKey explicit-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"api-token"}`))
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &ConfigFile{
		Auth: ConfigAuth{
			OIDC: &ConfigOIDC{
				TokenURL: "https://oidc.example.invalid/token",
				ClientID: "client-id",
			},
			APIKey: &ConfigAPIKey{
				AuthnURL: server.URL,
			},
		},
	}
	if err := SaveConfigToPath(cfg, configPath); err != nil {
		t.Fatal(err)
	}
	SetConfigPath(configPath)
	defer SetConfigPath("")

	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	for _, name := range []string{"api-key", "authn-url", "token-url", "keycloak-url", "keycloak-realm", "client-id", "client-secret", "username", "password", "token-command"} {
		flags.String(name, "", "")
	}
	if err := flags.Set("api-key", "explicit-key"); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("token-url", "https://oidc.example.invalid/token"); err != nil {
		t.Fatal(err)
	}

	ctx := cli.NewContext(cli.NewApp(), flags, nil)
	if err := LoginCommand().Action(ctx); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfigFromPath(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Auth.APIKey.Token != "api-token" {
		t.Fatalf("api key token = %q, want api-token", loaded.Auth.APIKey.Token)
	}
}
