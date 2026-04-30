/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package elektra

import (
	"testing"
	"time"
)

func TestCarbideClientReinitializationOnCertRenewal(t *testing.T) {
	// Initial setup with TestInitElektra which configures the Carbide client with initial certificates
	TestInitElektra(t)
	initialVersion := testElektra.manager.API.Carbide.GetGRPCClientVersion()

	// Regenerate and replace the certificates to simulate renewal
	SetupTestCerts(t, testElektraTypes.Conf.Carbide.ClientCertPath, testElektraTypes.Conf.Carbide.ClientKeyPath, testElektraTypes.Conf.Carbide.ServerCAPath)

	// Wait a few seconds to allow any background processes to complete
	time.Sleep(time.Second * 5)
	renewedVersion := testElektra.manager.API.Carbide.GetGRPCClientVersion()

	if renewedVersion > initialVersion {
		t.Logf("The Carbide client was successfully reinitialized from version %d to %d.", initialVersion, renewedVersion)
	} else {
		t.Errorf("The Carbide client was not reinitialized as expected. It remains at version %d.", initialVersion)
	}
}
