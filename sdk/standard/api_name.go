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

package standard

import (
	"net/http"
	"regexp"
	"strings"
)

const DefaultAPIName = "carbide"

var orgScopedAPIPathPattern = regexp.MustCompile(`(/v[0-9]+/org/[^/]+/)([^/]+)`)

type apiNameRewriteTransport struct {
	apiName string
	next    http.RoundTripper
}

// NewConfigurationWithAPIName returns a generated configuration with an
// additional API path override for deployments that use a non-default API name.
func NewConfigurationWithAPIName(apiName string) *Configuration {
	cfg := NewConfiguration()
	cfg.SetAPIName(apiName)
	return cfg
}

// SetAPIName configures an HTTP transport that rewrites the API path segment
// after /org/{org}/ before a request is sent. If you also provide a custom
// HTTP client, call SetAPIName after assigning that client to the configuration.
func (c *Configuration) SetAPIName(apiName string) {
	normalized := normalizeAPIName(apiName)
	if rewriter, ok := currentAPINameRewriteTransport(c.HTTPClient); ok {
		rewriter.apiName = normalized
		return
	}

	baseClient := c.HTTPClient
	if baseClient == nil {
		baseClient = http.DefaultClient
	}

	clientCopy := *baseClient
	clientCopy.Transport = &apiNameRewriteTransport{
		apiName: normalized,
		next:    baseClient.Transport,
	}
	c.HTTPClient = &clientCopy
}

// GetAPIName returns the configured API path segment. When unset, carbide is
// used to match the OpenAPI-generated paths.
func (c *Configuration) GetAPIName() string {
	if rewriter, ok := currentAPINameRewriteTransport(c.HTTPClient); ok {
		return rewriter.apiName
	}
	return DefaultAPIName
}

func currentAPINameRewriteTransport(client *http.Client) (*apiNameRewriteTransport, bool) {
	if client == nil {
		return nil, false
	}
	rewriter, ok := client.Transport.(*apiNameRewriteTransport)
	return rewriter, ok
}

func normalizeAPIName(apiName string) string {
	apiName = strings.TrimSpace(apiName)
	apiName = strings.Trim(apiName, "/")
	if apiName == "" {
		return DefaultAPIName
	}
	return apiName
}

func rewriteAPINamePath(path, apiName string) string {
	apiName = normalizeAPIName(apiName)
	if path == "" || apiName == DefaultAPIName {
		return path
	}
	return orgScopedAPIPathPattern.ReplaceAllString(path, "${1}"+apiName)
}

func (t *apiNameRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rewrittenReq := req
	if req != nil {
		rewrittenPath := rewriteAPINamePath(req.URL.Path, t.apiName)
		rewrittenRawPath := req.URL.RawPath
		if rewrittenRawPath != "" {
			rewrittenRawPath = rewriteAPINamePath(req.URL.RawPath, t.apiName)
		}
		if rewrittenPath != req.URL.Path || rewrittenRawPath != req.URL.RawPath {
			reqCopy := req.Clone(req.Context())
			urlCopy := *req.URL
			reqCopy.URL = &urlCopy
			reqCopy.URL.Path = rewrittenPath
			reqCopy.URL.RawPath = rewrittenRawPath
			rewrittenReq = reqCopy
		}
	}

	transport := t.next
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(rewrittenReq)
}
