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

package elektra

import (
	"testing"
	"time"
)

func TestNicoClientReinitializationOnCertRenewal(t *testing.T) {
	// Initial setup with TestInitElektra which configures the Nico client with initial certificates
	TestInitElektra(t)
	initialVersion := testElektra.manager.API.Nico.GetGRPCClientVersion()

	// Regenerate and replace the certificates to simulate renewal
	SetupTestCerts(t, testElektraTypes.Conf.Nico.ClientCertPath, testElektraTypes.Conf.Nico.ClientKeyPath, testElektraTypes.Conf.Nico.ServerCAPath)

	// Wait a few seconds to allow any background processes to complete
	time.Sleep(time.Second * 5)
	renewedVersion := testElektra.manager.API.Nico.GetGRPCClientVersion()

	if renewedVersion > initialVersion {
		t.Logf("The Nico client was successfully reinitialized from version %d to %d.", initialVersion, renewedVersion)
	} else {
		t.Errorf("The Nico client was not reinitialized as expected. It remains at version %d.", initialVersion)
	}
}
