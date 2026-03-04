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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nvidia/bare-metal-manager-rest/sdk/simple"
)

func main() {
	// CARBIDE_SDK_BASE_URL, CARBIDE_SDK_ORG_NAME, and CARBIDE_SDK_TOKEN are required.
	// CARBIDE_SDK_MACHINE_ID is optional; if not set, a Ready machine is selected.
	// CARBIDE_SDK_SITE_ID and CARBIDE_SDK_VPC_ID are optional for testing.
	// See sdk/simple/README.md for local dev (kind) setup.
	client, err := simple.NewClientFromEnv()
	if err != nil {
		fmt.Println("Error creating client:", err)
		os.Exit(1)
	}
	ctx := context.Background()
	if siteID := os.Getenv("CARBIDE_SDK_SITE_ID"); siteID != "" {
		client.SetSiteID(siteID)
	}
	if vpcID := os.Getenv("CARBIDE_SDK_VPC_ID"); vpcID != "" {
		client.SetVpcID(vpcID)
	}
	if err := client.Authenticate(ctx); err != nil {
		fmt.Printf("Error authenticating: %v\n", err)
		os.Exit(1)
	}

	// Get machines to select one in Ready state
	machines, _, apiErr := client.GetMachines(ctx, nil)
	if apiErr != nil {
		fmt.Printf("Error getting machines: %s\n", apiErr.Message)
		os.Exit(1)
	}

	selectedMachineID := os.Getenv("CARBIDE_SDK_MACHINE_ID")
	if selectedMachineID == "" {
		for _, machine := range machines {
			if machine.Status == "Ready" {
				selectedMachineID = machine.ID
				break
			}
		}
	}
	if selectedMachineID == "" {
		fmt.Println("Could not find a suitable Machine to create an Instance. Set CARBIDE_SDK_MACHINE_ID or ensure a Ready machine exists.")
		os.Exit(1)
	}

	// Create an Instance
	userData := "#cloud-config\nnetwork:\n  version: 2\n  ethernets:\n    eth0:\n      set-name: eth0\n      dhcp4: true\n      optional: true\n      match:\n        name: en*np0\n\n#user-data\nusers:\n- name: forge\nlock_passwd: false\nshell: /bin/bash\nsudo: ALL=(ALL) NOPASSWD:ALL\ngroups: users, admin\npasswd: <replace-with-hashed-password>\n"
	instanceCreateRequest := simple.InstanceCreateRequest{
		Name:       "test-instance",
		MachineID:  selectedMachineID,
		IpxeScript: "chain ${base-url}/internal/x86_64/qcow-imager.efi loglevel=7 console=ttyS0,115200 console=tty0 console=ttyS1,115200 pci=realloc=off image_distro_name=ubuntu image_distro_version=22.04 ds=nocloud-net;s=${cloudinit-url}\nboot",
		UserData:   &userData,
		SSHKeys: []string{
			"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG97ZbT39B/6DhvY7Wsw1+pxK7L4MFQ37peSZx+jXPuL user3@test.com",
		},
		Labels: map[string]string{
			"test-key": "test-value",
		},
	}
	instance, apiErr := client.CreateInstance(ctx, instanceCreateRequest)
	if apiErr != nil {
		fmt.Printf("Error creating instance: %s\n", apiErr.Message)
		os.Exit(1)
	}
	fmt.Printf("Created instance: %s. Will delete in 10s.\n", instance.GetId())
	time.Sleep(10 * time.Second)

	// Delete the Instance
	apiErr = client.DeleteInstance(ctx, instance.GetId())
	if apiErr != nil {
		fmt.Printf("Error deleting instance: %s\n", apiErr.Message)
		os.Exit(1)
	}
	fmt.Printf("Deleted instance: %s\n", instance.GetId())
}
