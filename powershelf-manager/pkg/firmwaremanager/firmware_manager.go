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
package firmwaremanager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/runner"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/vendor"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db/model"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/objects/pmc"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/objects/powershelf"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/pmcmanager"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/redfish"
)

const redfishTimeout = time.Minute * 1
const dbTimeout = time.Second * 30
const waiterSleep = time.Second * 30

// Manager aggregates per-vendor FirmwareUpdater instances and exposes a vendor-agnostic API for firmware operations.
type Manager struct {
	firmwareUpdater  map[vendor.Vendor]*FirmwareUpdater
	fwUpdateRegistry *Registry
	pmcManager       *pmcmanager.PmcManager
	runner           *runner.Runner
	dryRun           bool
}

// New constructs a Manager by creating updaters for all supported vendors.
func New(ctx context.Context, c db.Config, pmcManager *pmcmanager.PmcManager, dryRun bool) (*Manager, error) {
	registry, err := newRegistry(ctx, c)
	if err != nil {
		return nil, err
	}

	manager := Manager{
		firmwareUpdater:  make(map[vendor.Vendor]*FirmwareUpdater),
		fwUpdateRegistry: registry,
		pmcManager:       pmcManager,
		dryRun:           dryRun,
	}

	for v := range vendor.VendorCodeMax {
		vendor := vendor.CodeToVendor(v)
		if err := vendor.IsSupported(); err != nil {
			continue
		}

		updater, err := newFirmwareUpdater(vendor)
		if err != nil {
			return nil, err
		}

		manager.firmwareUpdater[vendor] = updater
	}

	manager.runner = runner.New("firmware manager", func() interface{} { return &manager }, fwWaiter, fwRunner)

	return &manager, nil
}

func (manager *Manager) SetDryRun(dryRun bool) {
	manager.dryRun = dryRun
}

// GetFirmwareUpdate returns the status of a firmware update for the specified PMC MAC and component.
func (manager *Manager) GetFirmwareUpdate(ctx context.Context, mac net.HardwareAddr, component powershelf.Component) (*powershelf.FirmwareUpdate, error) {
	return manager.fwUpdateRegistry.GetFwUpdate(ctx, mac, component)
}

// Summary returns a human-readable summary of supported ranges and rules for all vendors.
func (manager *Manager) Summary() (string, error) {
	sb := strings.Builder{}
	for _, updater := range manager.firmwareUpdater {
		sb.WriteString("Firmware Manager Summary:\n")
		updaterSummary, err := updater.Summary()
		if err != nil {
			return "", err
		}

		sb.WriteString(updaterSummary)
	}

	return sb.String(), nil
}

func (manager *Manager) getUpdater(pmc *pmc.PMC) (*FirmwareUpdater, error) {
	updater, ok := manager.firmwareUpdater[pmc.Vendor]
	if !ok {
		return nil, fmt.Errorf("could not find a firmware updater for %v", pmc.Vendor)
	}

	return updater, nil
}

// Upgrade upgrades firmware for a PMC, honoring vendor rules and dry-run.
func (manager *Manager) Upgrade(ctx context.Context, pmc *pmc.PMC, component powershelf.Component, targetVersion string) error {
	currentFwVersion, err := getFwVersion(ctx, pmc, component)
	if err != nil {
		return err
	}

	canUpdate, err := manager.CanUpdate(ctx, pmc, component, targetVersion)
	if err != nil {
		return err
	}

	if !canUpdate {
		return fmt.Errorf("cannot update %v for %v from %v to %v", component, pmc, currentFwVersion.String(), targetVersion)
	}

	_, err = model.NewFirmwareUpdate(ctx, manager.fwUpdateRegistry.pg.DB(), pmc.MAC, component, currentFwVersion.String(), targetVersion)
	return err
}

// CanUpdate returns whether a PMC’s current firmware is within the supported range.
func (manager *Manager) CanUpdate(ctx context.Context, pmc *pmc.PMC, component powershelf.Component, targetVersion string) (bool, error) {
	update, err := model.GetFirmwareUpdate(ctx, manager.fwUpdateRegistry.pg.DB(), pmc.MAC, component)
	if err != nil {
		// if there isnt a pendinging firmware update, we can proceed
		if !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
	}

	if update != nil && !update.IsTerminal() {
		return false, nil
	}

	updater, err := manager.getUpdater(pmc)
	if err != nil {
		return false, err
	}

	targetFwVersion, err := fwVersionFromStr(targetVersion)
	if err != nil {
		return false, err
	}

	return updater.canUpdatePmc(ctx, pmc, targetFwVersion)
}

// CanUpdate returns whether a PMC’s current firmware is within the supported range.
func (manager *Manager) ListAvailableFirmware(ctx context.Context, pmc *pmc.PMC) ([]FirmwareUpgrade, error) {
	updater, err := manager.getUpdater(pmc)
	if err != nil {
		return nil, err
	}

	return updater.repo.upgrades, nil
}

func getFwVersion(ctx context.Context, pmc *pmc.PMC, component powershelf.Component) (firmwareVersion, error) {
	client, err := redfish.New(ctx, pmc, true)
	if err != nil {
		return firmwareVersion{
			major: 0,
			minor: 0,
			patch: 0,
		}, err
	}
	defer client.Logout()

	switch component {
	case powershelf.PMC:
		manager, err := client.QueryManager()
		if err != nil {
			return firmwareVersion{
				major: 0,
				minor: 0,
				patch: 0,
			}, err
		}

		return fwVersionFromStr(manager.FirmwareVersion)
	default:
		return firmwareVersion{
			major: 0,
			minor: 0,
			patch: 0,
		}, fmt.Errorf("TODO: implement querying %v firmware version", component)
	}
}

func (manager *Manager) getPendingFwUpdates(ctx context.Context) ([]model.FirmwareUpdate, error) {
	dbCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	return model.GetAllPendingFirmwareUpdates(dbCtx, manager.fwUpdateRegistry.pg.DB())
}

func (manager *Manager) SetUpdateState(ctx context.Context, update model.FirmwareUpdate, newState powershelf.FirmwareState, errMsg string) error {
	dbCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	return update.UpdateFirmwareUpdateState(dbCtx, manager.fwUpdateRegistry.pg.DB(), newState, errMsg)
}

func (manager *Manager) handleOnePmcUpdate(ctx context.Context, pmc *pmc.PMC, update model.FirmwareUpdate) (powershelf.FirmwareState, error) {
	ctx, cancel := context.WithTimeout(ctx, redfishTimeout)
	defer cancel()

	switch update.State {
	case powershelf.FirmwareStateQueued:
		updater, err := manager.getUpdater(pmc)
		if err != nil {
			return powershelf.FirmwareStateFailed, err
		}

		version, err := fwVersionFromStr(update.VersionTo)
		if err != nil {
			return powershelf.FirmwareStateFailed, err
		}

		response, err := updater.upgrade(ctx, pmc, version, manager.dryRun)
		if err != nil {
			return powershelf.FirmwareStateFailed, fmt.Errorf("failed initiate  MAC %v for fw update %v", update.PmcMacAddress, update)
		} else {
			log.Printf("successfully initiated firmware update of component %v for powershelf with PMC MAC %v from %v to %v: %v\n", update.Component, pmc, update.VersionFrom, update.VersionTo, response)
			return powershelf.FirmwareStateVerifying, nil
		}
	case powershelf.FirmwareStateVerifying:
		currentFwVersion, err := getFwVersion(ctx, pmc, update.Component)
		if err != nil {
			// Do not transition to a failed state here b/c this may be transient. instead, wait for the timeout to handle updating that transition
			return powershelf.FirmwareStateVerifying, fmt.Errorf("failed to query fw version of %v on PMC %v: %v", update.Component, pmc, err)
		} else {
			if currentFwVersion.String() == update.VersionTo {
				log.Printf("successfully updated firmware of of component %v for powershelf with PMC MAC %v from %v to %v", update.Component, pmc, update.VersionFrom, update.VersionTo)
				return powershelf.FirmwareStateCompleted, nil
			} else if currentFwVersion.String() == update.VersionFrom {
				// Do not transition to a failed state here b/c this may be transient.. instead, wait for the timeout to handle updating that transition
				return powershelf.FirmwareStateVerifying, fmt.Errorf("waiting for the completion of firmware update of component %v for powershelf with PMC MAC %v from %v to %v", update.Component, pmc, update.VersionFrom, update.VersionTo)
			} else {
				return powershelf.FirmwareStateFailed, fmt.Errorf("found unexpected version %v while trying to do a firmware update of component %v for powershelf with PMC MAC %v from %v to %v", currentFwVersion.String(), update.Component, pmc, update.VersionFrom, update.VersionTo)
			}
		}
	default:
		return powershelf.FirmwareStateFailed, fmt.Errorf("fw manager does not support handling unexpected update state %v", update.State)
	}
}

func (manager *Manager) handleOneUpdate(ctx context.Context, update model.FirmwareUpdate) {
	var nextState powershelf.FirmwareState
	var err error
	timeSincelastStateTransition := time.Since(update.LastTransitionTime)
	timeSincelastUpdate := time.Since(update.UpdatedAt)
	mac := update.PmcMacAddress.HardwareAddr()
	log.Printf("Handling update of %v in powershelf with PMC MAC %v at state %v from version %v to version %v (Time Since Last State Transition: %v; Last Update: %v)", update.Component, mac, update.State, update.VersionFrom, update.VersionTo, timeSincelastStateTransition, timeSincelastUpdate)

	pmc, err := manager.pmcManager.GetPmc(context.Background(), mac)
	if err != nil {
		log.Printf("failed to query PMC for MAC %v for fw update %v\n", mac, update)
		return
	}

	switch update.Component {
	case powershelf.PMC:
		nextState, err = manager.handleOnePmcUpdate(ctx, pmc, update)
	default:
		nextState, err = powershelf.FirmwareStateFailed, fmt.Errorf("fw manager does not support upgraded powershelf component %v", update.Component)
	}

	// Timeout handling: this update has been pending for an hour without any progress
	if nextState == update.State && timeSincelastStateTransition > time.Hour {
		if err == nil {
			err = fmt.Errorf("timeout")
		}
		nextState = powershelf.FirmwareStateFailed
	}

	var errMsg string
	if err != nil {
		log.Printf("Failure in handling update of %v in powershelf with PMC MAC %v at state %v from version %v to version %v (Time Since Last State Transition: %v; Last Update: %v): %v", update.Component, update.PmcMacAddress, update.State, update.VersionFrom, update.VersionTo, timeSincelastStateTransition, timeSincelastUpdate, err)
		errMsg = err.Error()
	}

	if nextState != update.State {
		log.Printf("Updating state of fw update %v in powershelf with PMC MAC %v from %v to %v with err %v", update.Component, update.PmcMacAddress, update.State, nextState, err)
		if updateErr := manager.SetUpdateState(ctx, update, nextState, errMsg); updateErr != nil {
			log.Printf("failed to update firmware update state in DB of component %v in powershelf with PMC MAC %v for %v from %v to %v: %v", update.Component, update.PmcMacAddress, update.PmcMacAddress, update.State, nextState, updateErr)
		}
	}
}

func fwWaiter(ctx interface{}) interface{} {
	log.Println("Firmware Manager: Waiter")
	time.Sleep(waiterSleep)
	return nil
}

func fwRunner(ctx interface{}, task interface{}) {
	start := time.Now()
	log.Println("Firmware Manager: Runner")
	fwManager := ctx.(*Manager)

	runnerCtx := context.Background()
	updates, err := fwManager.getPendingFwUpdates(runnerCtx)
	if err != nil {
		log.Printf("failed to query pending firmware updates: %v\n", err)
		return
	}

	for _, update := range updates {
		fwManager.handleOneUpdate(runnerCtx, update)
	}

	log.Printf("Firmare Runner: finished handling %v pending updates in %s", len(updates), time.Since(start))
}
