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

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.temporal.io/sdk/worker"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/carbideapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/nsmapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/psmapi"
	svc "github.com/nvidia/bare-metal-manager-rest/rla/internal/service"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager"
	computecarbide "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/compute/carbide"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/mock"
	nvlswitchcarbide "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/nvlswitch/carbide"
	nvlswitchnsm "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/nvlswitch/nvswitchmanager"
	powershelfpsm "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/powershelf/psm"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/providers/carbide"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/providers/nvswitchmanager"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager/providers/psm"
	temporalmanager "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/manager"
)

const (
	defaultServicePort    = 50051
	componentMgrCfgEnvVar = "COMPONENT_MANAGER_CONFIG"
)

var (
	port               int
	componentMgrConfig string

	// serveCmd represents the serve command
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the RLA gPRC server",
		Long:  `Start the gRPC server to allow other services to manage the racks`,
		Run: func(cmd *cobra.Command, args []string) {
			doServe()
		},
	}
)

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().IntVarP(&port, "port", "p", defaultServicePort, "Port for the gRPC server") //nolint
	// Component manager config: priority is CLI flag > env var > default prod config
	serveCmd.Flags().StringVarP(&componentMgrConfig, "component-config", "c", "", "Path to component manager config file (YAML)") //nolint
}

// providerClients holds the API clients extracted from providers for use by the service.
type providerClients struct {
	carbide carbideapi.Client
	psm     psmapi.Client
	nsm     nsmapi.Client
}

// initProviderRegistry creates and initializes the provider registry based on configuration.
// It returns the registry and the underlying clients for use by the service layer.
func initProviderRegistry(config componentmanager.Config) (*componentmanager.ProviderRegistry, providerClients, error) {
	providerRegistry := componentmanager.NewProviderRegistry()
	var clients providerClients

	// Initialize Carbide provider if configured
	if config.Providers.Carbide != nil {
		carbideProvider, err := carbide.New(*config.Providers.Carbide)
		if err != nil {
			log.Warn().Err(err).Msg("Unable to create Carbide GRPC client (power control may not work)")
		} else {
			clients.carbide = carbideProvider.Client()
			providerRegistry.Register(carbideProvider)
			log.Info().
				Dur("timeout", config.Providers.Carbide.Timeout).
				Msg("Initialized Carbide provider")
		}
	}

	// Initialize PSM provider if configured
	if config.Providers.PSM != nil {
		psmProvider, err := psm.New(*config.Providers.PSM)
		if err != nil {
			log.Warn().Err(err).Msg("Unable to create PSM client (powershelf operations may not work)")
		} else {
			clients.psm = psmProvider.Client()
			providerRegistry.Register(psmProvider)
			log.Info().
				Dur("timeout", config.Providers.PSM.Timeout).
				Msg("Initialized PSM provider")
		}
	}

	// Initialize NV-Switch Manager provider if configured
	if config.Providers.NVSwitchManager != nil {
		nsmProvider, err := nvswitchmanager.New(*config.Providers.NVSwitchManager)
		if err != nil {
			log.Warn().Err(err).Msg("Unable to create NV-Switch Manager client (NVLSwitch operations may not work)")
		} else {
			clients.nsm = nsmProvider.Client()
			providerRegistry.Register(nsmProvider)
			log.Info().
				Dur("timeout", config.Providers.NVSwitchManager.Timeout).
				Msg("Initialized NV-Switch Manager provider")
		}
	}

	// Log all registered providers
	registeredProviders := providerRegistry.List()
	log.Info().
		Strs("providers", registeredProviders).
		Msg("Provider registry initialized")

	return providerRegistry, clients, nil
}

// initComponentManagerRegistry creates and initializes the component manager registry.
func initComponentManagerRegistry(config componentmanager.Config, providerRegistry *componentmanager.ProviderRegistry) (*componentmanager.Registry, error) {
	registry := componentmanager.NewRegistry()

	// Register all available component manager factories
	var computePowerDelay time.Duration
	if config.Providers.Carbide != nil {
		computePowerDelay = config.Providers.Carbide.ComputePowerDelay
	}
	computecarbide.Register(registry, computePowerDelay)
	nvlswitchcarbide.Register(registry)
	nvlswitchnsm.Register(registry)
	powershelfpsm.Register(registry)
	mock.RegisterAll(registry)

	// Initialize registry with the config and providers
	if err := registry.Initialize(config, providerRegistry); err != nil {
		return nil, fmt.Errorf("failed to initialize component managers: %w", err)
	}

	// Log registered implementations
	impls := registry.ListRegisteredImplementations()
	for compType, names := range impls {
		log.Debug().
			Str("component_type", compType.String()).
			Strs("implementations", names).
			Msg("Registered component manager implementations")
	}

	return registry, nil
}

// loadComponentManagerConfig loads the component manager configuration with the following priority:
//
//  1. CLI flag: --component-config / -c <path>
//     Example: ./rla serve -c /etc/rla/custom.yaml
//
//  2. Environment variable: COMPONENT_MANAGER_CONFIG=<path>
//     Example: COMPONENT_MANAGER_CONFIG=/etc/rla/componentmanager.yaml
//
//  3. Embedded default: componentmanager.DefaultProdConfig()
//     Used when no config file is provided. The primary production path.
//     Uses real implementations (Carbide for compute/nvlswitch, PSM for powershelf).
//
// The config specifies:
//   - Which component manager implementations to use (carbide, psm, mock)
//   - Provider settings (timeouts, endpoints)
func loadComponentManagerConfig() (componentmanager.Config, error) {
	// Priority 1: CLI flag
	configPath := componentMgrConfig

	// Priority 2: Environment variable
	if configPath == "" {
		configPath = os.Getenv(componentMgrCfgEnvVar)
	}

	// Load from file if a path was specified
	if configPath != "" {
		log.Info().Str("config_path", configPath).Msg("Loading component manager config from file")
		return componentmanager.LoadConfig(configPath)
	}

	// Priority 3: Embedded production config
	log.Info().Msg("Using embedded production config (carbide + psm)")
	return componentmanager.DefaultProdConfig(), nil
}

// createOperationRulesLoader creates a rule loader from configuration file.
// Returns a loader that will be used by the resolver to load rules during Start().
func doServe() {
	dbConf, err := db.BuildDBConfigFromEnv()
	if err != nil {
		log.Fatal().Msgf("failed to retrieve DB conn information: %v", err)
	}

	temporalConf, err := svc.BuildTemporalConfigFromEnv()
	if err != nil {
		log.Fatal().Msgf("failed to retrieve Temporal conn information: %v", err)
	}

	// Load component manager configuration
	cmConfig, err := loadComponentManagerConfig()
	if err != nil {
		log.Fatal().Msgf("failed to load component manager config: %v", err)
	}

	// Initialize provider registry (creates API clients based on config)
	providerRegistry, clients, err := initProviderRegistry(cmConfig)
	if err != nil {
		log.Fatal().Msgf("failed to initialize provider registry: %v", err)
	}

	// Initialize component manager registry
	cmRegistry, err := initComponentManagerRegistry(cmConfig, providerRegistry)
	if err != nil {
		log.Fatal().Msgf("failed to initialize component manager registry: %v", err)
	}

	temporalManagerConf := temporalmanager.Config{
		ClientConf: *temporalConf,
		WorkerOptions: map[string]worker.Options{
			temporalmanager.WorkflowQueue: {},
		},
		ComponentManagerRegistry: cmRegistry,
	}

	ctx := context.Background()

	if os.Getenv("REPORT_CARBIDE_API_VERSION") != "" {
		// Do some basic carbide-api requests, mainly for early testing; this code can be removed when we're doing actual communication
		go func() {
			client, err := carbideapi.NewClient(time.Minute)
			if err != nil {
				log.Fatal().Msgf("Unable to create GRPC client: %v", err)
			}
			for {
				time.Sleep(time.Second * 10)
				if version, err := client.Version(ctx); err != nil {
					log.Error().Msgf("Unable to retrieve version from carbide-api: %v", err)
					continue
				} else {
					log.Info().Msgf("carbide-api version: %s", version)
					break
				}
			}
			for {
				time.Sleep(time.Second * 10)
				if machines, err := client.GetMachines(ctx); err != nil {
					log.Error().Msgf("Unable to retrieve machines from carbide-api: %v", err)
					continue
				} else {
					log.Info().Msgf("carbide-api machines: %v", machines)
					break
				}
			}
		}()
	}

	service, err := svc.New(
		ctx,
		svc.Config{
			Port:          port,
			DBConf:        dbConf,
			ExecutorConf:  &temporalManagerConf,
			CarbideClient: clients.carbide,
			PSMClient:     clients.psm,
		},
	)

	if err != nil {
		log.Fatal().Msgf("failed to create the new gRPC server: %v", err)
	}

	log.Info().Msg("New RLA service is created\n")
	log.Info().Msgf("DB config: %+v", dbConf)
	log.Info().Msgf("Temporal config: %+v", temporalManagerConf)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs // Block execution until signal from terminal gets triggered here.
		service.Stop(ctx)
	}()

	if err := service.Start(ctx); err != nil {
		log.Fatal().Msgf("failed to start the service: %v\n", err)
	}
}
