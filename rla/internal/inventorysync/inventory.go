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

package inventorysync

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/carbideapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/config"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db/model"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db/postgres"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/psmapi"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

// RunInventory will loop and handle various inventory monitoring tasks
func RunInventory(ctx context.Context, dbConf *db.Config) {
	config := config.ReadConfig()
	if config.DisableInventory {
		log.Info().Msg("Inventory disabled by configuration")
		return
	}

	carbideClient, err := carbideapi.NewClient(config.GRPCTimeout)
	if err != nil {
		// Use whether CARBIDE_API_URL is set to determine if we're running in a production environment (fail hard) or not (just complain and do nothing)
		// Note that this doesn't actually create a connection immediately, so it won't fail just because carbide-api hasn't started yet.
		msg := fmt.Sprintf("Unable to create GRPC client (pre-connect): %v", err)
		if os.Getenv("CARBIDE_API_URL") == "" {
			log.Error().Msg(msg)
			return
		} else {
			log.Fatal().Msg(msg)
		}
	}

	psmClient, err := psmapi.NewClient(config.GRPCTimeout)
	if err != nil {
		log.Error().Msgf("Unable to create PSM GRPC client (PSM_API_URL: %v): %v", os.Getenv("PSM_API_URL"), err)
		return
	}

	if psmClient != nil {
		defer psmClient.Close()
	}

	pool, err := postgres.New(ctx, *dbConf)
	if err != nil {
		log.Fatal().Msgf("Unable to create database pool: %v", err)
	}

	log.Info().Msg("Starting inventory monitoring loop")

	for {
		runInventoryOne(ctx, &config, pool, carbideClient, psmClient)
	}
}

var lastUpdateMachineIDs time.Time

// runInventoryOne is a single iteration for RunInventory.
// It syncs each resource type against its external source, collects all drifts,
// and persists them in one shot.
func runInventoryOne(ctx context.Context, config *config.Config, pool *postgres.Postgres, carbideClient carbideapi.Client, psmClient psmapi.Client) {
	var allDrifts []model.ComponentDrift

	// Sync machines (compute nodes, NVSwitches, etc.) against Carbide
	machineDrifts := syncMachines(ctx, config, pool, carbideClient)
	allDrifts = append(allDrifts, machineDrifts...)

	// Sync powershelves against PSM
	powershelfDrifts := syncPowershelves(ctx, pool, carbideClient, psmClient)
	allDrifts = append(allDrifts, powershelfDrifts...)

	// Persist all drifts atomically (replace entire table)
	if err := pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return model.ReplaceAllDrifts(ctx, tx, allDrifts)
	}); err != nil {
		log.Error().Msgf("Unable to persist drift records: %v", err)
	} else {
		log.Info().Msgf("Drift detection complete: %d drift(s) detected", len(allDrifts))
	}

	time.Sleep(config.InventoryRunFrequency)
}

// ---------------------------------------------------------------------------
// syncMachines: sync non-PowerShelf components against Carbide
// ---------------------------------------------------------------------------
//
// Flow:
//  1. DB: get all non-PowerShelf components
//  2. Carbide GetMachines: match by serial → direct-write external_id
//  3. Carbide GetPowerStates: direct-write power_state
//  4. Carbide FindMachinesByIds + GetMachinePositionInfo: compare validation fields
//  5. Return drifts
//
// Validation fields (always compared): slot_id, tray_index, host_id, firmware_version, serial_number
// Direct-write fields (written, NOT compared): external_id, power_state
func syncMachines(ctx context.Context, config *config.Config, pool *postgres.Postgres, carbideClient carbideapi.Client) []model.ComponentDrift {
	log.Debug().Msg("Syncing machines...")

	// Step 1: Get all components from DB, filter out PowerShelves
	allComponents, err := model.GetAllComponents(ctx, pool.DB())
	if err != nil {
		log.Error().Msgf("Unable to retrieve components from db: %v", err)
		return nil
	}

	var components []model.Component
	for _, c := range allComponents {
		if c.Type != devicetypes.ComponentTypePowerShelf.String() {
			components = append(components, c)
		}
	}

	if len(components) == 0 {
		return nil
	}

	// Step 2: Direct-write external_id by serial matching (respecting frequency config)
	syncMachineIDs(ctx, config, pool, carbideClient, components)

	// Re-read components to pick up any external_id updates from step 2
	allComponents, err = model.GetAllComponents(ctx, pool.DB())
	if err != nil {
		log.Error().Msgf("Unable to re-read components from db after machine ID update: %v", err)
		return nil
	}
	components = components[:0]
	for _, c := range allComponents {
		if c.Type != devicetypes.ComponentTypePowerShelf.String() {
			components = append(components, c)
		}
	}

	// Collect machine IDs for Carbide queries
	var machineIDs []string
	componentsByExternalID := make(map[string]*model.Component)
	for i := range components {
		comp := &components[i]
		if comp.ComponentID != nil && *comp.ComponentID != "" {
			machineIDs = append(machineIDs, *comp.ComponentID)
			componentsByExternalID[*comp.ComponentID] = comp
		}
	}

	// Step 3: Direct-write power_state
	if len(machineIDs) > 0 {
		syncPowerStates(ctx, pool, carbideClient, machineIDs, componentsByExternalID)
	}

	// Step 4: Fetch machine details and positions for drift detection
	var machineDetails []carbideapi.MachineDetail
	var machinePositions []carbideapi.MachinePosition

	if len(machineIDs) > 0 {
		machineDetails, err = carbideClient.FindMachinesByIds(ctx, machineIDs)
		if err != nil {
			log.Error().Msgf("Unable to retrieve machine details from Carbide: %v", err)
			return nil
		}

		machinePositions, err = carbideClient.GetMachinePositionInfo(ctx, machineIDs)
		if err != nil {
			log.Error().Msgf("Unable to retrieve machine positions from Carbide: %v", err)
			return nil
		}
	}

	// Build lookup maps
	detailByID := make(map[string]carbideapi.MachineDetail)
	for _, d := range machineDetails {
		detailByID[d.MachineID] = d
	}
	positionByID := make(map[string]carbideapi.MachinePosition)
	for _, p := range machinePositions {
		positionByID[p.MachineID] = p
	}

	// Step 5: Compare and build drift records
	now := time.Now()
	var drifts []model.ComponentDrift

	for i := range components {
		comp := &components[i]

		if comp.ComponentID == nil || *comp.ComponentID == "" {
			// Component has no external_id — cannot look up in Carbide
			compID := comp.ID
			drifts = append(drifts, model.ComponentDrift{
				ComponentID: &compID,
				ExternalID:  nil,
				DriftType:   model.DriftTypeMissingInActual,
				Diffs:       []model.FieldDiff{},
				CheckedAt:   now,
			})
			continue
		}

		externalID := *comp.ComponentID
		detail, foundDetail := detailByID[externalID]
		position := positionByID[externalID] // zero value is fine if not found

		if !foundDetail {
			// Component has external_id but Carbide doesn't know about it
			compID := comp.ID
			drifts = append(drifts, model.ComponentDrift{
				ComponentID: &compID,
				ExternalID:  &externalID,
				DriftType:   model.DriftTypeMissingInActual,
				Diffs:       []model.FieldDiff{},
				CheckedAt:   now,
			})
			continue
		}

		// Compare validation fields
		fieldDiffs := compareMachineFieldsForDrift(comp, detail, position)
		if len(fieldDiffs) > 0 {
			compID := comp.ID
			drifts = append(drifts, model.ComponentDrift{
				ComponentID: &compID,
				ExternalID:  &externalID,
				DriftType:   model.DriftTypeMismatch,
				Diffs:       fieldDiffs,
				CheckedAt:   now,
			})
		}
	}

	// Detect missing_in_expected: machines in Carbide but not in local DB
	for _, detail := range machineDetails {
		if _, found := componentsByExternalID[detail.MachineID]; !found {
			extID := detail.MachineID
			drifts = append(drifts, model.ComponentDrift{
				ComponentID: nil,
				ExternalID:  &extID,
				DriftType:   model.DriftTypeMissingInExpected,
				Diffs:       []model.FieldDiff{},
				CheckedAt:   now,
			})
		}
	}

	log.Info().Msgf("Machine sync: %d drift(s) out of %d component(s)", len(drifts), len(components))
	return drifts
}

// syncMachineIDs matches components by serial number against Carbide machines
// and direct-writes the external_id. Respects UpdateMachineIDsFrequency config.
func syncMachineIDs(ctx context.Context, config *config.Config, pool *postgres.Postgres, carbideClient carbideapi.Client, components []model.Component) {
	shouldUpdate := false
	if config.UpdateMachineIDsFrequency == 0 {
		// A frequency of zero means to do it only once on startup
		if lastUpdateMachineIDs.IsZero() {
			shouldUpdate = true
		}
	} else {
		if lastUpdateMachineIDs.Before(time.Now().Add(-config.UpdateMachineIDsFrequency)) {
			shouldUpdate = true
		}
	}

	if !shouldUpdate {
		return
	}

	// If we already found everything, don't bother to recheck
	missingMachine := false
	for _, cur := range components {
		if cur.ComponentID == nil {
			missingMachine = true
			break
		}
	}
	if !missingMachine {
		lastUpdateMachineIDs = time.Now()
		return
	}

	machines, err := carbideClient.GetMachines(ctx)
	if err != nil {
		log.Error().Msgf("Unable to retrieve machines from carbide-api: %v", err)
		return
	}

	containersBySerial := make(map[string]model.Component)
	for _, cur := range components {
		containersBySerial[cur.SerialNumber] = cur
	}

	var toUpdate []model.Component
	for _, cur := range machines {
		if cur.ChassisSerial == nil {
			continue
		}
		if container, ok := containersBySerial[*cur.ChassisSerial]; ok {
			if container.ComponentID == nil || *container.ComponentID != cur.MachineID {
				componentID := cur.MachineID
				container.ComponentID = &componentID
				toUpdate = append(toUpdate, container)
			}
		}
	}

	if len(toUpdate) > 0 {
		if err := pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
			for _, cur := range toUpdate {
				if err := cur.SetComponentIDBySerial(ctx, tx); err != nil {
					return fmt.Errorf("Unable to update machine ID: %v", err)
				}
			}
			return nil
		}); err != nil {
			log.Error().Msgf("Unable to update components with serial: %v", err)
			return
		}

		log.Info().Msgf("Updated %d machine ID(s)", len(toUpdate))
	}

	// lastUpdateMachineIDs is the last time we ran successfully, not necessarily when we last actually changed something
	lastUpdateMachineIDs = time.Now()
}

// syncPowerStates fetches power states from Carbide and direct-writes to component table.
func syncPowerStates(ctx context.Context, pool *postgres.Postgres, carbideClient carbideapi.Client, machineIDs []string, componentsByExternalID map[string]*model.Component) {
	machines, err := carbideClient.GetPowerStates(ctx, machineIDs)
	if err != nil {
		log.Error().Msgf("Unable to retrieve power states from carbide-api: %v", err)
		return
	}

	var toUpdate []model.Component
	for _, cur := range machines {
		if comp, ok := componentsByExternalID[cur.MachineID]; ok {
			if comp.PowerState == nil || *comp.PowerState != cur.PowerState {
				powerState := cur.PowerState
				comp.PowerState = &powerState
				toUpdate = append(toUpdate, *comp)
			}
		}
	}

	if len(toUpdate) > 0 {
		if err := pool.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
			for _, cur := range toUpdate {
				if err := cur.SetPowerStateByComponentID(ctx, tx); err != nil {
					return fmt.Errorf("Unable to update power state: %v", err)
				}
			}
			return nil
		}); err != nil {
			log.Error().Msgf("Unable to update components with power state: %v", err)
		}
	}
}

// compareMachineFieldsForDrift compares validation fields between expected (DB) and actual (Carbide).
// Validation fields: slot_id, tray_index, host_id, firmware_version, serial_number.
func compareMachineFieldsForDrift(
	expected *model.Component,
	actual carbideapi.MachineDetail,
	position carbideapi.MachinePosition,
) []model.FieldDiff {
	var diffs []model.FieldDiff

	// Compare position.slot_id
	if position.PhysicalSlotNum != nil && expected.SlotID != int(*position.PhysicalSlotNum) {
		diffs = append(diffs, model.FieldDiff{
			FieldName:     "slot_id",
			ExpectedValue: fmt.Sprintf("%d", expected.SlotID),
			ActualValue:   fmt.Sprintf("%d", *position.PhysicalSlotNum),
		})
	}

	// Compare position.tray_index
	if position.ComputeTrayIndex != nil && expected.TrayIndex != int(*position.ComputeTrayIndex) {
		diffs = append(diffs, model.FieldDiff{
			FieldName:     "tray_index",
			ExpectedValue: fmt.Sprintf("%d", expected.TrayIndex),
			ActualValue:   fmt.Sprintf("%d", *position.ComputeTrayIndex),
		})
	}

	// Compare position.host_id
	if position.TopologyID != nil && expected.HostID != int(*position.TopologyID) {
		diffs = append(diffs, model.FieldDiff{
			FieldName:     "host_id",
			ExpectedValue: fmt.Sprintf("%d", expected.HostID),
			ActualValue:   fmt.Sprintf("%d", *position.TopologyID),
		})
	}

	// Compare firmware_version
	if actual.FirmwareVersion != "" && expected.FirmwareVersion != actual.FirmwareVersion {
		diffs = append(diffs, model.FieldDiff{
			FieldName:     "firmware_version",
			ExpectedValue: expected.FirmwareVersion,
			ActualValue:   actual.FirmwareVersion,
		})
	}

	// Compare serial_number (chassis_serial)
	if actual.ChassisSerial != nil && expected.SerialNumber != *actual.ChassisSerial {
		diffs = append(diffs, model.FieldDiff{
			FieldName:     "serial_number",
			ExpectedValue: expected.SerialNumber,
			ActualValue:   *actual.ChassisSerial,
		})
	}

	return diffs
}

// ---------------------------------------------------------------------------
// syncPowershelves: sync PowerShelf components against PSM
// ---------------------------------------------------------------------------
//
// Flow:
//  1. DB: get all PowerShelf components with BMCs
//  2. PSM GetPowershelves: get registered powershelves
//  3. Carbide FindInterfaces: check which PMCs have DHCPed
//  4. Direct-write: firmware_version, power_state (from PSM)
//  5. Register un-registered DHCPed powershelves with PSM
//  6. Return drifts (missing_in_actual for unregistered powershelves)

// Default factory credentials for powershelf BMCs
const (
	powershelfDefaultUsername = "root"
	powershelfDefaultPassword = "0penBmc"
)

func syncPowershelves(ctx context.Context, pool *postgres.Postgres, carbideClient carbideapi.Client, psmClient psmapi.Client) []model.ComponentDrift {
	if psmClient == nil {
		log.Debug().Msg("PSM client not available, skipping powershelf sync")
		return nil
	}

	log.Debug().Msg("Syncing powershelves...")

	// Step 1: Get all PowerShelf components with their PMCs
	expectedPowershelves, err := model.GetComponentsByType(ctx, pool.DB(), devicetypes.ComponentTypePowerShelf)
	if err != nil {
		log.Error().Msgf("Unable to retrieve powershelf components from db: %v", err)
		return nil
	}

	if len(expectedPowershelves) == 0 {
		return nil
	}

	// Build map from PMC MAC to component
	// Each powershelf should have exactly one PMC (BMC)
	expectedByPmcMac := make(map[string]*model.Component)
	for i := range expectedPowershelves {
		ps := &expectedPowershelves[i]
		if len(ps.BMCs) != 1 {
			log.Error().Msgf("Powershelf %s has %d BMCs, expected exactly 1; skipping", ps.SerialNumber, len(ps.BMCs))
			continue
		}

		// Validate PMC MAC address
		pmcMacAddr, err := net.ParseMAC(ps.BMCs[0].MacAddress)
		if err != nil || pmcMacAddr == nil {
			log.Error().Msgf("Powershelf %s has invalid BMC MAC address %s; skipping", ps.SerialNumber, ps.BMCs[0].MacAddress)
			continue
		}

		expectedByPmcMac[ps.BMCs[0].MacAddress] = ps
	}

	// Get list of expected PMC MACs
	expectedPmcMacs := make([]string, 0, len(expectedByPmcMac))
	for mac := range expectedByPmcMac {
		expectedPmcMacs = append(expectedPmcMacs, mac)
	}

	// Step 2: Get registered powershelves from PSM
	registeredPowershelves, err := psmClient.GetPowershelves(ctx, expectedPmcMacs)
	if err != nil {
		log.Error().Msgf("Unable to retrieve registered powershelves from PSM: %v", err)
		return nil
	}

	registeredByMac := make(map[string]psmapi.PowerShelf)
	for _, ps := range registeredPowershelves {
		registeredByMac[ps.PMC.MACAddress] = ps
	}

	// Step 3: Get machine interfaces from Carbide to check DHCP status
	interfacesByMac, err := carbideClient.FindInterfaces(ctx)
	if err != nil {
		log.Error().Msgf("Unable to retrieve interfaces from carbide-api: %v", err)
		return nil
	}

	// Steps 4 & 5: Process each expected powershelf
	now := time.Now()
	var drifts []model.ComponentDrift
	var toRegister []psmapi.RegisterPowershelfRequest

	for pmcMac, powershelf := range expectedByPmcMac {
		// Already registered with PSM — direct-write firmware_version + power_state
		if registeredPS, isRegistered := registeredByMac[pmcMac]; isRegistered {
			needsUpdate := false

			// Direct-write: firmware_version
			if registeredPS.PMC.FirmwareVersion != "" && powershelf.FirmwareVersion != registeredPS.PMC.FirmwareVersion {
				powershelf.FirmwareVersion = registeredPS.PMC.FirmwareVersion
				needsUpdate = true
				log.Info().Msgf("Updating firmware version for powershelf %s to %s", pmcMac, registeredPS.PMC.FirmwareVersion)
			}

			// Direct-write: power_state (derived from PSUs)
			// All on → On, All off → Off, Mix or no PSUs → Unknown
			allOn := len(registeredPS.PSUs) > 0
			allOff := len(registeredPS.PSUs) > 0
			for _, psu := range registeredPS.PSUs {
				if psu.PowerState {
					allOff = false
				} else {
					allOn = false
				}
			}
			psuPowerState := carbideapi.PowerStateUnknown
			if allOn {
				psuPowerState = carbideapi.PowerStateOn
			} else if allOff {
				psuPowerState = carbideapi.PowerStateOff
			}
			if powershelf.PowerState == nil || *powershelf.PowerState != psuPowerState {
				powershelf.PowerState = &psuPowerState
				needsUpdate = true
				log.Info().Msgf("Updating power state for powershelf %s to %v", pmcMac, psuPowerState)
			}

			if needsUpdate {
				if err := powershelf.Patch(ctx, pool.DB()); err != nil {
					log.Error().Msgf("Unable to update powershelf %s: %v", pmcMac, err)
				}
			}

			// TODO: add field-level drift detection for powershelves (serial_number, etc.)
			continue
		}

		// Not registered with PSM — check if DHCPed, register if possible
		iface, found := interfacesByMac[pmcMac]
		if !found || len(iface.Addresses) == 0 {
			// PMC hasn't DHCPed yet — record as missing_in_actual
			log.Warn().Msgf("Powershelf PMC %s has not DHCPed yet", pmcMac)
			compID := powershelf.ID
			drifts = append(drifts, model.ComponentDrift{
				ComponentID: &compID,
				ExternalID:  nil,
				DriftType:   model.DriftTypeMissingInActual,
				Diffs:       []model.FieldDiff{},
				CheckedAt:   now,
			})
			continue
		}

		// Check for unexpected multiple IP addresses
		if len(iface.Addresses) > 1 {
			log.Error().Msgf("Powershelf PMC %s has multiple IP addresses assigned (%v), skipping registration", pmcMac, iface.Addresses)
			continue
		}

		ipAddress := iface.Addresses[0]
		log.Info().Msgf("Powershelf PMC %s has DHCPed with IP %s, registering with PSM", pmcMac, ipAddress)

		toRegister = append(toRegister, psmapi.RegisterPowershelfRequest{
			PMCMACAddress:  pmcMac,
			PMCIPAddress:   ipAddress,
			PMCVendor:      psmapi.PMCVendorLiteon,
			PMCCredentials: psmapi.Credentials{Username: powershelfDefaultUsername, Password: powershelfDefaultPassword},
		})
	}

	// Register un-registered powershelves with PSM
	if len(toRegister) > 0 {
		responses, err := psmClient.RegisterPowershelves(ctx, toRegister)
		if err != nil {
			log.Error().Msgf("Unable to register powershelves with PSM: %v", err)
		} else {
			for _, resp := range responses {
				if resp.Status != psmapi.StatusSuccess {
					log.Error().Msgf("Failed to register powershelf %s with PSM: %s", resp.PMCMACAddress, resp.Error)
				} else if resp.IsNew {
					log.Info().Msgf("Successfully registered new powershelf %s with PSM", resp.PMCMACAddress)
				} else {
					log.Debug().Msgf("Powershelf %s was already registered with PSM", resp.PMCMACAddress)
				}
			}
		}
	}

	log.Info().Msgf("Powershelf sync: %d drift(s) out of %d expected", len(drifts), len(expectedPowershelves))
	return drifts
}
