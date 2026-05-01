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

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/term"

	carbidecli "github.com/NVIDIA/ncx-infra-controller-rest/cli/pkg"
)

// ChooseConfigFile scans ~/.carbide for config*.yaml files and shows an interactive
// selector if multiple configs exist. Returns the chosen path, or empty string
// if only one config exists (use default) or no terminal is available.
func ChooseConfigFile(explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return "", nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}

	configDir := filepath.Join(home, ".carbide")
	entries, err := os.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading config directory: %w", err)
	}

	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "config") {
			continue
		}
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		candidates = append(candidates, filepath.Join(configDir, name))
	}

	if len(candidates) <= 1 {
		return "", nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		left := filepath.Base(candidates[i])
		right := filepath.Base(candidates[j])
		leftDefault := left == "config.yaml" || left == "config.yml"
		rightDefault := right == "config.yaml" || right == "config.yml"
		if leftDefault != rightDefault {
			return leftDefault
		}
		return left < right
	})

	items := make([]SelectItem, len(candidates))
	for i, path := range candidates {
		display := path
		if strings.HasPrefix(path, home+string(os.PathSeparator)) {
			display = "~/" + strings.TrimPrefix(path, home+string(os.PathSeparator))
		}
		items[i] = SelectItem{Label: display, ID: path}
	}

	fmt.Println()
	selected, err := Select("Choose config for this session", items)
	if err != nil {
		return "", err
	}
	fmt.Printf("Using config: %s\n\n", selected.Label)
	return selected.ID, nil
}

// RunTUI is the entry point for carbidecli tui. It handles config selection,
// authentication, and starts the REPL.
func RunTUI(explicitConfig string) error {
	configPath, err := ChooseConfigFile(explicitConfig)
	if err != nil {
		return fmt.Errorf("choosing config: %w", err)
	}

	var cfg *carbidecli.ConfigFile
	if configPath != "" {
		cfg, err = carbidecli.LoadConfigFromPath(configPath)
	} else {
		cfg, err = carbidecli.LoadConfig()
		configPath = carbidecli.ConfigPath()
	}
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	org := cfg.API.Org
	if org == "" {
		return fmt.Errorf("api.org is required in config %s", configPath)
	}

	baseURL := cfg.API.Base
	apiName := cfg.API.Name
	if apiName == "" {
		apiName = "carbide"
	}

	token, _ := carbidecli.AutoRefreshTokenToPath(cfg, configPath)
	if token == "" {
		token = carbidecli.GetAuthToken(cfg)
	}
	if token == "" && carbidecli.HasTokenCommandConfig(cfg) {
		token, err = carbidecli.LoginWithTokenCommand(cfg, configPath, cfg.Auth.TokenCommand)
		if err != nil {
			return fmt.Errorf("running auth script: %w", err)
		}
	}

	client := carbidecli.NewClient(baseURL, org, token, nil, false)
	client.APIName = apiName

	session := NewSession(client, org, configPath)
	session.Token = token

	if carbidecli.HasTokenCommandConfig(cfg) || carbidecli.HasOIDCConfig(cfg) || carbidecli.HasAPIKeyConfig(cfg) {
		loginFn := func() (string, error) {
			return carbidecli.LoginFromConfig(cfg, configPath)
		}
		session.LoginFn = loginFn
		client.TokenRefresh = func() (string, error) {
			token, err := loginFn()
			if err == nil && token != "" {
				session.RefreshClient(token)
			}
			return token, err
		}
		client.AuthRetryNotify = func(event carbidecli.AuthRetryEvent) {
			switch event.Action {
			case carbidecli.AuthRetryActionLogin:
				fmt.Fprintf(os.Stderr, "%s API returned %d; running configured login (%d/%d).\n",
					Yellow("Auth:"), event.StatusCode, event.Attempt, event.MaxAttempts)
			case carbidecli.AuthRetryActionRetry:
				fmt.Fprintf(os.Stderr, "%s Retrying API request with refreshed token (%d/%d).\n",
					Yellow("Auth:"), event.Attempt, event.MaxAttempts)
			}
		}
	}

	if token == "" {
		fmt.Fprintf(os.Stderr, "%s No auth token found. Type %s to authenticate.\n\n",
			Yellow("Warning:"), Bold("login"))
	}

	return RunREPL(session)
}
