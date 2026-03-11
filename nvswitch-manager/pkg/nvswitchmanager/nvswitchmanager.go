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

package nvswitchmanager

import (
	"context"
	"fmt"

	"github.com/nvidia/bare-metal-manager-rest/nvswitch-manager/pkg/credentials"
	"github.com/nvidia/bare-metal-manager-rest/nvswitch-manager/pkg/nvswitchregistry"
	"github.com/nvidia/bare-metal-manager-rest/nvswitch-manager/pkg/objects/nvswitch"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// NVSwitchManager coordinates registry and credential management for NV-Switch trays.
type NVSwitchManager struct {
	DataStoreType     DataStoreType
	Registry          nvswitchregistry.Registry
	CredentialManager credentials.CredentialManager
}

// New creates a new instance of NVSwitchManager.
func New(ctx context.Context, c Config) (*NVSwitchManager, error) {
	credentialManager, err := credentials.New(ctx, &c.CredentialConf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize credential manager: %v", err)
	}

	var registry nvswitchregistry.Registry
	switch c.DSType {
	case DatastoreTypeInMemory:
		log.Printf("Initializing NV-Switch Manager with in-memory registry")
		registry = nvswitchregistry.NewInMemoryRegistry()
	case DatastoreTypePersistent:
		if c.DB == nil {
			return nil, fmt.Errorf("database connection required for persistent registry")
		}
		log.Printf("Initializing NV-Switch Manager with persistent (PostgreSQL) registry")
		registry = nvswitchregistry.NewPostgresRegistry(c.DB)
	default:
		return nil, fmt.Errorf("unsupported datastore type: %v", c.DSType)
	}

	return &NVSwitchManager{
		DataStoreType:     c.DSType,
		Registry:          registry,
		CredentialManager: credentialManager,
	}, nil
}

// Start initializes the manager.
func (nm *NVSwitchManager) Start(ctx context.Context) error {
	if err := nm.Registry.Start(ctx); err != nil {
		return err
	}
	return nm.CredentialManager.Start(ctx)
}

// Stop shuts down the manager.
func (nm *NVSwitchManager) Stop(ctx context.Context) error {
	if err := nm.Registry.Stop(ctx); err != nil {
		return err
	}
	return nm.CredentialManager.Stop(ctx)
}

// Register registers a new NV-Switch tray and stores its credentials.
func (nm *NVSwitchManager) Register(ctx context.Context, tray *nvswitch.NVSwitchTray) (uuid.UUID, bool, error) {
	// Store credentials first
	if tray.BMC != nil && tray.BMC.Credential != nil {
		if err := nm.CredentialManager.PutBMC(ctx, tray.BMC.MAC, tray.BMC.Credential); err != nil {
			return uuid.Nil, false, fmt.Errorf("failed to store BMC credentials: %v", err)
		}
	}

	if tray.NVOS != nil && tray.NVOS.Credential != nil {
		// Use BMC MAC as the key for NVOS credentials (they're linked)
		if tray.BMC != nil {
			if err := nm.CredentialManager.PutNVOS(ctx, tray.BMC.MAC, tray.NVOS.Credential); err != nil {
				return uuid.Nil, false, fmt.Errorf("failed to store NVOS credentials: %v", err)
			}
		}
	}

	// Register in registry
	return nm.Registry.Register(ctx, tray)
}

// Get retrieves an NV-Switch by UUID and attaches credentials.
func (nm *NVSwitchManager) Get(ctx context.Context, id uuid.UUID) (*nvswitch.NVSwitchTray, error) {
	tray, err := nm.Registry.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Attach credentials
	if tray.BMC != nil {
		cred, err := nm.CredentialManager.GetBMC(ctx, tray.BMC.MAC)
		if err == nil {
			tray.BMC.Credential = cred
		}

		nvosCred, err := nm.CredentialManager.GetNVOS(ctx, tray.BMC.MAC)
		if err == nil && tray.NVOS != nil {
			tray.NVOS.Credential = nvosCred
		}
	}

	return tray, nil
}

// List returns all registered NV-Switches.
func (nm *NVSwitchManager) List(ctx context.Context) ([]*nvswitch.NVSwitchTray, error) {
	return nm.Registry.List(ctx)
}

// Delete removes an NV-Switch and its credentials.
func (nm *NVSwitchManager) Delete(ctx context.Context, id uuid.UUID) error {
	tray, err := nm.Registry.Get(ctx, id)
	if err != nil {
		return err
	}

	// Delete credentials
	if tray.BMC != nil {
		_ = nm.CredentialManager.DeleteBMC(ctx, tray.BMC.MAC)
		_ = nm.CredentialManager.DeleteNVOS(ctx, tray.BMC.MAC)
	}

	return nm.Registry.Delete(ctx, id)
}
