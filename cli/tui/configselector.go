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

	carbidecli "github.com/nvidia/bare-metal-manager-rest/cli/pkg"
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

	token, _ := carbidecli.AutoRefreshToken(cfg)
	if token == "" {
		token = carbidecli.GetAuthToken(cfg)
	}

	client := carbidecli.NewClient(baseURL, org, token, nil, false)
	client.APIName = apiName

	session := NewSession(client, org, configPath)
	session.Token = token

	if carbidecli.HasOIDCConfig(cfg) || carbidecli.HasAPIKeyConfig(cfg) {
		session.LoginFn = func() (string, error) {
			return loginFromConfig(cfg, configPath)
		}
	}

	if token == "" {
		fmt.Fprintf(os.Stderr, "%s No auth token found. Type %s to authenticate.\n\n",
			Yellow("Warning:"), Bold("login"))
	}

	return RunREPL(session)
}

// loginFromConfig performs a fresh login using the config's auth method.
func loginFromConfig(cfg *carbidecli.ConfigFile, configPath string) (string, error) {
	if carbidecli.HasOIDCConfig(cfg) {
		newToken, err := carbidecli.AutoRefreshToken(cfg)
		if err != nil || newToken == "" {
			return "", fmt.Errorf("OIDC token refresh failed: %w", err)
		}
		if saveErr := carbidecli.SaveConfigToPath(cfg, configPath); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save refreshed token: %v\n", saveErr)
		}
		return newToken, nil
	}
	if carbidecli.HasAPIKeyConfig(cfg) {
		return carbidecli.ExchangeAPIKey(cfg, configPath)
	}
	return "", fmt.Errorf("no auth method configured")
}
