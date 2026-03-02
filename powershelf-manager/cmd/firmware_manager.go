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
	"net"
	svc "github.com/nvidia/bare-metal-manager-rest/powershelf-manager/internal/service"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/credential"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/secretstring"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/vendor"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/credentials"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/objects/pmc"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/objects/powershelf"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/powershelfmanager"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var vendorStr string
var fw_action string
var dryRun bool
var versionTo string

// serveCmd represents the serve command
var fwCmd = &cobra.Command{
	Use:   "fw",
	Short: "print embedded fws",
	Long:  `print embedded fws`,
	Run: func(cmd *cobra.Command, args []string) {
		doFw()
	},
}

type FwManagerAction string

const (
	Summary    FwManagerAction = "summary"
	CanUpgrade FwManagerAction = "can_upgrade"
	Upgrade    FwManagerAction = "upgrade"
)

var fwManagerActions = []FwManagerAction{
	Summary,
	CanUpgrade,
	Upgrade,
}

func getAvailableFwActions() string {
	actionStrings := make([]string, len(fwManagerActions))
	for i, action := range fwManagerActions {
		actionStrings[i] = string(action)
	}
	return strings.Join(actionStrings, ", ")
}

func init() {
	rootCmd.AddCommand(fwCmd)

	fwCmd.Flags().StringVarP(&vendorStr, "vendor", "v", "", "Vendor")
	fwCmd.Flags().StringVarP(&fw_action, "action", "a", "", "Action to perform: "+getAvailableFwActions())
	fwCmd.Flags().BoolVarP(&dryRun, "dry", "d", true, "dry run (default true)")
	fwCmd.Flags().StringVarP(&pmcIP, "ip", "i", "", "PMC IP address")
	fwCmd.Flags().StringVarP(&pmcUsername, "user", "u", "root", "Username")
	fwCmd.Flags().StringVarP(&pmcPassword, "pass", "p", "0penBmc", "Password")
	fwCmd.Flags().StringVar(&versionTo, "version", "0penBmc", "Target Version to upgrade to")
}

func doFw() {
	ip := net.ParseIP(pmcIP)
	if ip == nil {
		log.Fatalf("invalid IP address: %s", pmcIP)
	}

	vendor := vendor.StringToVendor(vendorStr)

	if err := vendor.IsSupported(); err != nil {
		log.Fatalf("unsupported vendor: %v\n", err)
	}

	pmc := &pmc.PMC{
		IP:     ip,
		Vendor: vendor,
		Credential: &credential.Credential{
			User:     pmcUsername,
			Password: secretstring.New(pmcPassword),
		},
	}

	svcConfig := svc.Config{
		Port:          port,
		DataStoreType: powershelfmanager.DataStoreType(datastoreType),
		VaultConf: credentials.VaultConfig{
			Address: vaultAddress,
			Token:   vaultToken,
		},
		DBConf: db.Config{
			Host:              dbHostName,
			Port:              dbPort,
			DBName:            dbName,
			Credential:        *credential.New(dbUser, dbPassword),
			CACertificatePath: "",
		},
	}

	psmConfig, err := svcConfig.ToPsmConf()
	if err != nil {
		log.Fatalf("failed to convert to psm conf: %v\n", err)
	}

	psm, err := powershelfmanager.New(context.Background(), *psmConfig)
	if err != nil {
		log.Fatalf("failed to init powershelf manager: %v\n", err)
	}

	fw_manager := psm.FirmwareManager

	switch FwManagerAction(fw_action) {
	case Summary:
		summary, err := fw_manager.Summary()
		if err != nil {
			log.Fatalf("failed to get fw repo summary for %v: %v\n", vendor, err)
		}

		fmt.Println(summary)
	case CanUpgrade:
		ip := net.ParseIP(pmcIP)
		if ip == nil {
			log.Fatalf("invalid IP address: %s", pmcIP)
		}

		supported, err := fw_manager.CanUpdate(context.Background(), pmc, powershelf.PMC, versionTo)
		if err != nil {
			log.Fatalf("failed to upgrade fw for %v: %v\n", vendor, err)
		}

		fmt.Printf("%v\n", supported)
	case Upgrade:
		fmt.Printf("Upgrading fw for %v (ip %s user: %s password: %s)\n", vendor, pmcIP, pmcUsername, pmcPassword)
		err := fw_manager.Upgrade(context.Background(), pmc, powershelf.PMC, versionTo)
		if err != nil {
			log.Fatalf("failed to upgrade fw for %v: %v\n", vendor, err)
		}
	default:
		log.Fatalf("unsupported action: %s\n", fw_action)
	}
}
