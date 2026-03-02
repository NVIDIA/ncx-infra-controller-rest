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
	"embed"
	"fmt"
	"io/fs"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/vendor"
	"strings"

	log "github.com/sirupsen/logrus"
)

//go:embed firmware/*
var firmware embed.FS

const firmware_path = "firmware"
const pmc_path = "pmc"

// FirmwareFetcher provides read-only access to embedded firmware assets organized as firmware/<vendor>/pmc.
type FirmwareFetcher struct {
	fs embed.FS
}

// FirmwareEntry identifies a firmware artifact by name and embedded FS path.
type FirmwareEntry struct {
	name string
	path string
}

func newFirmwareFetcher() *FirmwareFetcher {
	return &FirmwareFetcher{
		fs: firmware,
	}
}

// getVendorDirectories lists vendor directories under the embedded firmware root.
func (ff *FirmwareFetcher) getVendorDirectories() ([]fs.DirEntry, error) {
	return fs.ReadDir(ff.fs, firmware_path)
}

// getPmcFirmwareEntries returns all PMC firmware files for a vendor; entries are non-empty .tar files.
func (ff *FirmwareFetcher) getPmcFirmwareEntries(v vendor.Vendor) ([]FirmwareEntry, error) {
	vendors, err := ff.getVendorDirectories()
	if err != nil {
		return nil, err
	}

	expectedVendorName := strings.ToLower(v.Name)

	for _, vendor := range vendors {
		if vendor.IsDir() {
			vendorName := vendor.Name()
			if vendorName == expectedVendorName {
				path := fmt.Sprintf("%s/%s/%s", firmware_path, vendorName, pmc_path)
				entries, err := fs.ReadDir(firmware, path)
				if err != nil {
					return nil, err
				}

				fwEntries := make([]FirmwareEntry, 0, len(entries))
				for _, entry := range entries {
					if entry.IsDir() {
						log.Printf("found unexpected dir entry in {%s}: {%s}\n", path, entry.Name())
						continue
					}

					name := entry.Name()
					info, err := entry.Info()
					if err != nil {
						log.Printf("failed to get info for entry in {%s}: {%s}, err: %v\n", path, entry.Name(), err)
						continue
					}

					size := info.Size()
					if size == 0 {
						log.Printf("Vendor %s: skipping empty firmware file in {%s}: {%s}\n", vendorName, path, entry.Name())
						continue
					}

					fw_path := fmt.Sprintf("%s/%s", path, name)
					//log.Printf("Vendor %s: adding fw {%s} at %s (size: %d bytes)\n", vendorName, name, fw_path, size)
					fwEntries = append(fwEntries, FirmwareEntry{
						name: name,
						path: fw_path,
					})

				}

				return fwEntries, nil
			}
		}
	}

	return nil, fmt.Errorf("no firmware found for vendor %s out of vendors %v", v.Name, vendors)
}

// open opens a file by embedded path.
func (ff *FirmwareFetcher) open(path string) (fs.File, error) {
	return ff.fs.Open(path)
}
