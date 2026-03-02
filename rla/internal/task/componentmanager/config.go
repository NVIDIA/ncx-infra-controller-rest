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

package componentmanager

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/providers/carbide"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/providers/psm"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

// ProviderConfig holds the configuration for API providers.
// A nil pointer means the provider is not enabled.
type ProviderConfig struct {
	// Carbide holds Carbide-specific configuration. Nil means disabled.
	Carbide *carbide.Config

	// PSM holds PSM-specific configuration. Nil means disabled.
	PSM *psm.Config
}

// Config holds the component manager configuration.
type Config struct {
	// ComponentManagers maps component types to their implementation names.
	ComponentManagers map[devicetypes.ComponentType]string

	// Providers holds provider-specific configuration.
	Providers ProviderConfig
}

// rawConfig is the raw YAML structure before conversion.
type rawConfig struct {
	ComponentManagers map[string]string `yaml:"component_managers"`
	Providers         rawProviderConfig `yaml:"providers"`
}

// rawProviderConfig is the raw YAML structure for provider configuration.
type rawProviderConfig struct {
	Carbide *rawCarbideConfig `yaml:"carbide"`
	PSM     *rawPSMConfig     `yaml:"psm"`
}

// rawCarbideConfig is the raw YAML structure for Carbide configuration.
type rawCarbideConfig struct {
	Timeout           string `yaml:"timeout"`
	ComputePowerDelay string `yaml:"compute_power_delay"`
}

// rawPSMConfig is the raw YAML structure for PSM configuration.
type rawPSMConfig struct {
	Timeout string `yaml:"timeout"`
}

// LoadConfig loads the component manager configuration from a YAML file.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	return ParseConfig(data)
}

// ParseConfig parses the component manager configuration from YAML data.
func ParseConfig(data []byte) (Config, error) {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("failed to parse config: %w", err)
	}

	config := Config{
		ComponentManagers: make(map[devicetypes.ComponentType]string),
	}

	// Parse component managers
	for typeStr, implName := range raw.ComponentManagers {
		componentType := devicetypes.ComponentTypeFromString(typeStr)
		if componentType == devicetypes.ComponentTypeUnknown {
			return Config{}, fmt.Errorf("unknown component type: %s", typeStr)
		}
		config.ComponentManagers[componentType] = strings.TrimSpace(implName)
	}

	// Parse Carbide config if present in YAML
	if raw.Providers.Carbide != nil {
		carbideConfig := &carbide.Config{
			Timeout:           carbide.DefaultTimeout,
			ComputePowerDelay: carbide.DefaultComputePowerDelay,
		}
		if raw.Providers.Carbide.Timeout != "" {
			timeout, err := time.ParseDuration(raw.Providers.Carbide.Timeout)
			if err != nil {
				return Config{}, fmt.Errorf("invalid carbide timeout: %w", err)
			}
			carbideConfig.Timeout = timeout
		}
		if raw.Providers.Carbide.ComputePowerDelay != "" {
			delay, err := time.ParseDuration(raw.Providers.Carbide.ComputePowerDelay)
			if err != nil {
				return Config{}, fmt.Errorf(
					"invalid carbide compute_power_delay: %w", err,
				)
			}
			carbideConfig.ComputePowerDelay = delay
		}
		config.Providers.Carbide = carbideConfig
	}

	// Parse PSM config if present in YAML
	if raw.Providers.PSM != nil {
		psmConfig := &psm.Config{
			Timeout: psm.DefaultTimeout,
		}
		if raw.Providers.PSM.Timeout != "" {
			timeout, err := time.ParseDuration(raw.Providers.PSM.Timeout)
			if err != nil {
				return Config{}, fmt.Errorf("invalid psm timeout: %w", err)
			}
			psmConfig.Timeout = timeout
		}
		config.Providers.PSM = psmConfig
	}

	// If no providers are explicitly configured, derive from component manager implementations
	if config.Providers.Carbide == nil && config.Providers.PSM == nil {
		deriveProviders(&config)
	}

	return config, nil
}

// deriveProviders enables providers based on the component manager implementations configured.
func deriveProviders(config *Config) {
	for _, implName := range config.ComponentManagers {
		switch implName {
		case carbide.ProviderName:
			if config.Providers.Carbide == nil {
				config.Providers.Carbide = &carbide.Config{
					Timeout:           carbide.DefaultTimeout,
					ComputePowerDelay: carbide.DefaultComputePowerDelay,
				}
			}
		case psm.ProviderName:
			if config.Providers.PSM == nil {
				config.Providers.PSM = &psm.Config{
					Timeout: psm.DefaultTimeout,
				}
			}
			// mock implementations don't require any providers
		}
	}
}

// HasProvider checks if a provider is enabled in the configuration.
func (c *Config) HasProvider(name string) bool {
	switch name {
	case carbide.ProviderName:
		return c.Providers.Carbide != nil
	case psm.ProviderName:
		return c.Providers.PSM != nil
	}
	return false
}

// DefaultTestConfig returns the default configuration for testing/development.
// Uses mock implementations that don't require external services.
func DefaultTestConfig() Config {
	return Config{
		ComponentManagers: map[devicetypes.ComponentType]string{
			devicetypes.ComponentTypeCompute:    "mock",
			devicetypes.ComponentTypeNVLSwitch:  "mock",
			devicetypes.ComponentTypePowerShelf: "mock",
		},
		Providers: ProviderConfig{
			// No providers needed for mock - both nil
		},
	}
}

// DefaultProdConfig returns the embedded production configuration.
// Used when no config file is specified. Connects to external services
// (Carbide for compute/nvlswitch, PSM for powershelf).
// Timing parameters for operations are configured per-rule via action parameters.
func DefaultProdConfig() Config {
	return Config{
		ComponentManagers: map[devicetypes.ComponentType]string{
			devicetypes.ComponentTypeCompute:    carbide.ProviderName,
			devicetypes.ComponentTypeNVLSwitch:  carbide.ProviderName,
			devicetypes.ComponentTypePowerShelf: psm.ProviderName,
		},
		Providers: ProviderConfig{
			Carbide: &carbide.Config{
				Timeout:           carbide.DefaultTimeout,
				ComputePowerDelay: carbide.DefaultComputePowerDelay,
			},
			PSM: &psm.Config{
				Timeout: psm.DefaultTimeout,
			},
		},
	}
}
