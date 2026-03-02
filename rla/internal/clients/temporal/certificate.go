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

package temporal

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
)

const (
	caCertificateFileName     = "ca.crt"
	clientCertificateFileName = "tls.crt"
	clientKeyFileName         = "tls.key"
	caCertificateDirName      = "ca"
	clientCertificateDirName  = "client"
)

func loadClientCertificate(clientCertPath string) (tls.Certificate, error) {
	log.Info().Msgf("Loading client certificate from %s", clientCertPath)
	cert, err := tls.LoadX509KeyPair(
		clientCertPath+"/"+clientCertificateFileName,
		clientCertPath+"/"+clientKeyFileName,
	)

	if err != nil {
		log.Error().Msgf("Failed to load client certificate: %v", err)
		return tls.Certificate{}, err
	}

	log.Info().Msgf("Client certificate loaded successfully")
	return cert, nil
}

func loadCACertificate(caCertPath string) (*x509.CertPool, error) {
	log.Info().Msgf("Loading CA certificate from %s", caCertPath)
	caCert, err := os.ReadFile(caCertPath + "/" + caCertificateFileName)
	if err != nil {
		log.Error().Msgf("Failed to load CA certificate: %v", err)
		return nil, err
	}

	log.Info().Msgf("CA certificate loaded successfully")

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		log.Error().Msgf("Failed to append CA certificate to pool")
		return nil, fmt.Errorf("failed to append CA certificate to pool")
	}

	return certPool, nil
}

func buildTLSConfig(c Config) (*tls.Config, error) {
	if !c.EnableTLS {
		return nil, nil
	}

	caCertPath := c.Endpoint.CACertificatePath + "/" + caCertificateDirName
	caCert, err := loadCACertificate(caCertPath)
	if err != nil {
		return nil, err
	}

	clientCertPath := c.Endpoint.CACertificatePath + "/" + clientCertificateDirName
	clientCert, err := loadClientCertificate(clientCertPath)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		ServerName:   c.ServerName,
		RootCAs:      caCert,
	}, nil
}
