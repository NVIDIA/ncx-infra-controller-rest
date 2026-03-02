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
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/executor/temporalworkflow/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

// ComponentManager defines the interface for managing various types of
// components. Implementations handle component-specific operations like
// power control, firmware management, and status monitoring.
type ComponentManager interface {
	Type() devicetypes.ComponentType
	InjectExpectation(ctx context.Context, target common.Target, info operations.InjectExpectationTaskInfo) error //nolint
	PowerControl(ctx context.Context, target common.Target, info operations.PowerControlTaskInfo) error           //nolint
	GetPowerStatus(ctx context.Context, target common.Target) (map[string]operations.PowerStatus, error)          //nolint
	FirmwareControl(ctx context.Context, target common.Target, info operations.FirmwareControlTaskInfo) error     //nolint

	// StartFirmwareUpdate initiates firmware update without waiting for completion.
	// Returns immediately after the update request is accepted.
	StartFirmwareUpdate(ctx context.Context, target common.Target, info operations.FirmwareControlTaskInfo) error //nolint

	// GetFirmwareUpdateStatus returns the current status of firmware updates for the target components.
	// Returns a map of component ID to FirmwareUpdateStatus.
	GetFirmwareUpdateStatus(ctx context.Context, target common.Target) (map[string]operations.FirmwareUpdateStatus, error) //nolint

	// AllowBringUpAndPowerOn opens the power-on gate for the target components via Carbide.
	// Only applicable to compute and NVSwitch components managed by Carbide.
	AllowBringUpAndPowerOn(ctx context.Context, target common.Target) error //nolint

	// GetBringUpState returns the bring-up state for each target component.
	// Returns a map of component ID to MachineBringUpState.
	GetBringUpState(ctx context.Context, target common.Target) (map[string]operations.MachineBringUpState, error) //nolint
}

// ManagerFactory is a function that creates a ComponentManager instance.
// It receives a ProviderRegistry from which it can retrieve the providers it needs.
type ManagerFactory func(providers *ProviderRegistry) (ComponentManager, error)

// Registry maintains a collection of component manager factories and active managers.
// It allows dynamic registration and selection of implementations per component type.
type Registry struct {
	mu        sync.RWMutex
	factories map[devicetypes.ComponentType]map[string]ManagerFactory // type -> impl_name -> factory
	active    map[devicetypes.ComponentType]ComponentManager          // type -> active manager
}

// NewRegistry creates a new Registry instance.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[devicetypes.ComponentType]map[string]ManagerFactory),
		active:    make(map[devicetypes.ComponentType]ComponentManager),
	}
}

// RegisterFactory registers a factory for a specific component type and implementation name.
// Returns false if a factory with the same type and name already exists.
func (r *Registry) RegisterFactory(
	componentType devicetypes.ComponentType,
	implName string,
	factory ManagerFactory,
) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.factories[componentType]; !ok {
		r.factories[componentType] = make(map[string]ManagerFactory)
	}

	if _, exists := r.factories[componentType][implName]; exists {
		log.Warn().
			Str("component_type", devicetypes.ComponentTypeToString(componentType)).
			Str("impl_name", implName).
			Msg("Factory already registered, skipping")
		return false
	}

	r.factories[componentType][implName] = factory
	log.Debug().
		Str("component_type", devicetypes.ComponentTypeToString(componentType)).
		Str("impl_name", implName).
		Msg("Registered component manager factory")
	return true
}

// Initialize creates and activates component managers based on the provided configuration.
// The config maps component types to implementation names.
func (r *Registry) Initialize(config Config, providers *ProviderRegistry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for componentType, implName := range config.ComponentManagers {
		factories, ok := r.factories[componentType]
		if !ok {
			return fmt.Errorf(
				"no factories registered for component type: %s",
				devicetypes.ComponentTypeToString(componentType),
			)
		}

		factory, ok := factories[implName]
		if !ok {
			available := make([]string, 0, len(factories))
			for name := range factories {
				available = append(available, name)
			}
			return fmt.Errorf(
				"unknown implementation '%s' for component type %s, available: %v",
				implName,
				devicetypes.ComponentTypeToString(componentType),
				available,
			)
		}

		manager, err := factory(providers)
		if err != nil {
			return fmt.Errorf(
				"failed to create manager for component type %s with implementation '%s': %w",
				devicetypes.ComponentTypeToString(componentType),
				implName,
				err,
			)
		}

		r.active[componentType] = manager
		log.Info().
			Str("component_type", devicetypes.ComponentTypeToString(componentType)).
			Str("impl_name", implName).
			Msg("Initialized component manager")
	}

	return nil
}

// GetManager returns the active manager for the specified component type.
// Returns nil if no manager is active for the type.
func (r *Registry) GetManager(componentType devicetypes.ComponentType) ComponentManager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active[componentType]
}

// GetAllManagers returns all active managers.
func (r *Registry) GetAllManagers() []ComponentManager {
	r.mu.RLock()
	defer r.mu.RUnlock()

	managers := make([]ComponentManager, 0, len(r.active))
	for _, manager := range r.active {
		managers = append(managers, manager)
	}
	return managers
}

// ListRegisteredImplementations returns a map of component types to their registered implementation names.
func (r *Registry) ListRegisteredImplementations() map[devicetypes.ComponentType][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[devicetypes.ComponentType][]string)
	for componentType, factories := range r.factories {
		names := make([]string, 0, len(factories))
		for name := range factories {
			names = append(names, name)
		}
		result[componentType] = names
	}
	return result
}
