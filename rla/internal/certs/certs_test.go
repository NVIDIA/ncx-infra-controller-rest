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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgcerts "github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/certs"
)

// generateTestCerts creates a self-signed CA and a client cert/key in a temp
// directory using the standard file names (ca.crt, tls.crt, tls.key).
// Returns the directory path.
func generateTestCerts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Test Client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	require.NoError(t, err)

	clientKeyDER, err := x509.MarshalECPrivateKey(clientKey)
	require.NoError(t, err)

	writePEM(t, filepath.Join(dir, defaultCACert), "CERTIFICATE", caDER)
	writePEM(t, filepath.Join(dir, defaultCertFile), "CERTIFICATE", clientDER)
	writePEM(t, filepath.Join(dir, defaultKeyFile), "EC PRIVATE KEY", clientKeyDER)

	return dir
}

func writePEM(t *testing.T, path, pemType string, der []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, pem.Encode(f, &pem.Block{Type: pemType, Bytes: der}))
}

func TestTLSConfig(t *testing.T) {
	t.Run("CERTDIR set with valid certs", func(t *testing.T) {
		dir := generateTestCerts(t)
		t.Setenv("CERTDIR", dir)

		tlsConfig, source, err := TLSConfig()
		require.NoError(t, err)
		assert.Equal(t, dir, source)
		assert.NotNil(t, tlsConfig)
	})

	t.Run("CERTDIR set but no certs present", func(t *testing.T) {
		t.Setenv("CERTDIR", t.TempDir())

		tlsConfig, source, err := TLSConfig()
		assert.ErrorIs(t, err, ErrNotPresent)
		assert.NotEmpty(t, source)
		assert.Nil(t, tlsConfig)
	})

	t.Run("CERTDIR not set falls back to default path", func(t *testing.T) {
		t.Setenv("CERTDIR", "")

		tlsConfig, source, err := TLSConfig()
		assert.ErrorIs(t, err, ErrNotPresent)
		assert.Equal(t, defaultCertDir, source)
		assert.Nil(t, tlsConfig)
	})
}

func TestServerTLSConfig(t *testing.T) {
	t.Run("CERTDIR set with valid certs", func(t *testing.T) {
		dir := generateTestCerts(t)
		t.Setenv("CERTDIR", dir)

		tlsConfig, source, err := ServerTLSConfig()
		require.NoError(t, err)
		assert.Equal(t, dir, source)
		assert.NotEmpty(t, tlsConfig.Certificates)
		assert.NotNil(t, tlsConfig.ClientCAs)
	})

	t.Run("CERTDIR set but no certs present", func(t *testing.T) {
		t.Setenv("CERTDIR", t.TempDir())

		tlsConfig, source, err := ServerTLSConfig()
		assert.ErrorIs(t, err, ErrNotPresent)
		assert.NotEmpty(t, source)
		assert.Nil(t, tlsConfig)
	})
}

func TestResolveServer(t *testing.T) {
	t.Run("explicit paths used when set returns server config", func(t *testing.T) {
		dir := generateTestCerts(t)
		c := pkgcerts.Config{
			CACert:  filepath.Join(dir, defaultCACert),
			TLSCert: filepath.Join(dir, defaultCertFile),
			TLSKey:  filepath.Join(dir, defaultKeyFile),
		}

		tlsConfig, source, err := ResolveServer(c)
		require.NoError(t, err)
		assert.Equal(t, c.CACert, source)
		assert.NotEmpty(t, tlsConfig.Certificates) // server config
		assert.NotNil(t, tlsConfig.ClientCAs)      // server config
		assert.Nil(t, tlsConfig.RootCAs)           // not client config
	})

	t.Run("empty config falls back to env/default", func(t *testing.T) {
		t.Setenv("CERTDIR", t.TempDir()) // empty dir → ErrNotPresent

		_, _, err := ResolveServer(pkgcerts.Config{})
		assert.ErrorIs(t, err, ErrNotPresent)
	})

	t.Run("partial config returns validation error", func(t *testing.T) {
		c := pkgcerts.Config{CACert: "ca.crt"} // missing tls-cert and tls-key

		_, _, err := ResolveServer(c)
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrNotPresent)
	})
}
