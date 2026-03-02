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

package certs

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

var ErrNotPresent = errors.New("Certificates are not present")

func TLSConfig() (tlsConfig *tls.Config, certDir string, err error) {
	certDir = os.Getenv("CERTDIR")
	if certDir == "" {
		// Cert directory in k8s
		certDir = "/var/run/secrets/spiffe.io"
	}

	caCert, err := os.ReadFile(certDir + "/ca.crt")
	if err != nil {
		return nil, certDir, ErrNotPresent
	}

	clientCert, err := tls.LoadX509KeyPair(certDir+"/tls.crt", certDir+"/tls.key")
	if err != nil {
		return nil, certDir, fmt.Errorf("Invalid certs present: %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, certDir, fmt.Errorf("Invalid CA cert present: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}, certDir, nil

}
