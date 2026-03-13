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

package tui

import (
	"bytes"
	"io"
	"os"
	"sort"
	"strings"
	"testing"
)

// --- Upstream tests ---

func TestAppendScopeFlags_NoSession(t *testing.T) {
	got := appendScopeFlags(nil, []string{"machine", "list"})
	want := []string{"machine", "list"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAppendScopeFlags_SiteScope_MachineList(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "site-123", SiteName: "pdx-dev3"}}
	got := appendScopeFlags(s, []string{"machine", "list"})
	want := []string{"machine", "list", "--site-id", "site-123"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAppendScopeFlags_SiteScope_VPCList(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "site-123"}}
	got := appendScopeFlags(s, []string{"vpc", "list"})
	want := []string{"vpc", "list", "--site-id", "site-123"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAppendScopeFlags_BothScopes_SubnetList(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "site-123", VpcID: "vpc-456"}}
	got := appendScopeFlags(s, []string{"subnet", "list"})
	want := []string{"subnet", "list", "--site-id", "site-123", "--vpc-id", "vpc-456"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAppendScopeFlags_BothScopes_InstanceList(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "site-123", VpcID: "vpc-456"}}
	got := appendScopeFlags(s, []string{"instance", "list"})
	want := []string{"instance", "list", "--site-id", "site-123", "--vpc-id", "vpc-456"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAppendScopeFlags_NonListAction_Ignored(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "site-123"}}
	got := appendScopeFlags(s, []string{"machine", "get"})
	want := []string{"machine", "get"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAppendScopeFlags_UnknownResource_NoFlags(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "site-123"}}
	got := appendScopeFlags(s, []string{"site", "list"})
	want := []string{"site", "list"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAppendScopeFlags_SinglePart_NoFlags(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "site-123"}}
	got := appendScopeFlags(s, []string{"help"})
	want := []string{"help"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAppendScopeFlags_VpcOnlyScope_SubnetList(t *testing.T) {
	s := &Session{Scope: Scope{VpcID: "vpc-456"}}
	got := appendScopeFlags(s, []string{"subnet", "list"})
	want := []string{"subnet", "list", "--vpc-id", "vpc-456"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLogCmd_IncludesScopeFlags(t *testing.T) {
	s := &Session{
		ConfigPath: "/tmp/config.yaml",
		Scope:      Scope{SiteID: "site-123"},
	}
	output := captureStdout(func() {
		LogCmd(s, "machine", "list")
	})
	if !strings.Contains(output, "--site-id site-123") {
		t.Errorf("LogCmd output missing --site-id flag: %q", output)
	}
	if !strings.Contains(output, "--config /tmp/config.yaml") {
		t.Errorf("LogCmd output missing --config flag: %q", output)
	}
	if !strings.Contains(output, "carbidecli") {
		t.Errorf("LogCmd output missing carbidecli: %q", output)
	}
}

func TestLogCmd_NoScope(t *testing.T) {
	s := &Session{}
	output := captureStdout(func() {
		LogCmd(s, "machine", "list")
	})
	if strings.Contains(output, "--site-id") {
		t.Errorf("LogCmd output should not contain --site-id when no scope set: %q", output)
	}
}

// --- VPC scope coverage tests ---

func TestAppendScopeFlags_SiteOnly(t *testing.T) {
	siteOnlyResources := []string{
		"vpc", "allocation", "ip-block", "operating-system", "ssh-key-group",
		"network-security-group", "sku", "rack", "expected-machine",
		"dpu-extension-service", "infiniband-partition", "nvlink-logical-partition",
	}

	s := &Session{Scope: Scope{SiteID: "site-1", VpcID: "vpc-1"}}

	for _, resource := range siteOnlyResources {
		got := appendScopeFlags(s, []string{resource, "list"})
		if !contains(got, "--site-id") {
			t.Errorf("%s list: expected --site-id flag", resource)
		}
		if contains(got, "--vpc-id") {
			t.Errorf("%s list: should not include --vpc-id flag", resource)
		}
	}
}

func TestAppendScopeFlags_SiteAndVPC(t *testing.T) {
	vpcResources := []string{"subnet", "vpc-prefix", "instance", "machine"}

	s := &Session{Scope: Scope{SiteID: "site-1", VpcID: "vpc-1"}}

	for _, resource := range vpcResources {
		got := appendScopeFlags(s, []string{resource, "list"})
		if !contains(got, "--site-id") {
			t.Errorf("%s list: expected --site-id flag", resource)
		}
		if !contains(got, "--vpc-id") {
			t.Errorf("%s list: expected --vpc-id flag", resource)
		}
	}
}

func TestAppendScopeFlags_NoScope(t *testing.T) {
	s := &Session{Scope: Scope{}}

	got := appendScopeFlags(s, []string{"machine", "list"})
	if contains(got, "--site-id") || contains(got, "--vpc-id") {
		t.Error("empty scope should not produce any flags")
	}
}

func TestAppendScopeFlags_VPCOnlyScope(t *testing.T) {
	s := &Session{Scope: Scope{VpcID: "vpc-1"}}

	got := appendScopeFlags(s, []string{"instance", "list"})
	if contains(got, "--site-id") {
		t.Error("should not include --site-id when SiteID is empty")
	}
	if !contains(got, "--vpc-id") {
		t.Error("expected --vpc-id flag")
	}
}

func TestAppendScopeFlags_NonListAction(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "site-1", VpcID: "vpc-1"}}

	got := appendScopeFlags(s, []string{"machine", "get"})
	if contains(got, "--site-id") || contains(got, "--vpc-id") {
		t.Error("get actions should not have scope flags appended")
	}
}

func TestAppendScopeFlags_UnscopedResources(t *testing.T) {
	unscopedResources := []string{"site", "audit", "ssh-key", "tenant-account"}

	s := &Session{Scope: Scope{SiteID: "site-1", VpcID: "vpc-1"}}

	for _, resource := range unscopedResources {
		got := appendScopeFlags(s, []string{resource, "list"})
		if contains(got, "--site-id") || contains(got, "--vpc-id") {
			t.Errorf("%s list: unscoped resource should not have scope flags", resource)
		}
	}
}

func TestAppendScopeFlags_CoversAllRegisteredFetchers(t *testing.T) {
	scopeFilteredFetchers := []string{
		"vpc", "subnet", "instance", "machine",
		"allocation", "ip-block", "operating-system",
		"ssh-key-group", "network-security-group",
		"sku", "rack", "expected-machine", "vpc-prefix",
		"dpu-extension-service", "infiniband-partition", "nvlink-logical-partition",
	}

	s := &Session{Scope: Scope{SiteID: "site-1", VpcID: "vpc-1"}}

	for _, resource := range scopeFilteredFetchers {
		got := appendScopeFlags(s, []string{resource, "list"})
		if !contains(got, "--site-id") {
			t.Errorf("%s list: scope-filtered fetcher missing from appendScopeFlags", resource)
		}
	}
}

func TestInvalidateFiltered_MatchesScopeFilteredFetchers(t *testing.T) {
	scopeFilteredFetchers := []string{
		"vpc", "subnet", "instance",
		"allocation", "machine", "ip-block", "operating-system",
		"ssh-key-group", "network-security-group",
		"vpc-prefix", "rack", "expected-machine", "sku",
		"dpu-extension-service", "infiniband-partition", "nvlink-logical-partition",
	}

	c := NewCache()
	for _, rt := range scopeFilteredFetchers {
		c.Set(rt, []NamedItem{{Name: rt, ID: rt}})
	}
	c.Set("site", []NamedItem{{Name: "site", ID: "site"}})
	c.Set("audit", []NamedItem{{Name: "audit", ID: "audit"}})

	c.InvalidateFiltered()

	for _, rt := range scopeFilteredFetchers {
		if got := c.Get(rt); got != nil {
			t.Errorf("InvalidateFiltered did not clear scope-filtered type %q", rt)
		}
	}
	if c.Get("site") == nil {
		t.Error("InvalidateFiltered should not clear unscoped type site")
	}
	if c.Get("audit") == nil {
		t.Error("InvalidateFiltered should not clear unscoped type audit")
	}
}

func TestAppendScopeFlags_ScopeFlagCategories_Consistent(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "s", VpcID: "v"}}

	vpcFilteredInFetchers := map[string]bool{
		"subnet": true, "instance": true, "vpc-prefix": true, "machine": true,
	}

	allScoped := []string{
		"vpc", "subnet", "instance", "machine",
		"allocation", "ip-block", "operating-system",
		"ssh-key-group", "network-security-group",
		"sku", "rack", "expected-machine", "vpc-prefix",
		"dpu-extension-service", "infiniband-partition", "nvlink-logical-partition",
	}

	for _, resource := range allScoped {
		got := appendScopeFlags(s, []string{resource, "list"})
		hasVpc := contains(got, "--vpc-id")
		expectVpc := vpcFilteredInFetchers[resource]
		if hasVpc != expectVpc {
			t.Errorf("%s: appendScopeFlags vpc-id=%v but fetcher expects vpc=%v", resource, hasVpc, expectVpc)
		}
	}
}

func TestAllCommands_HaveUniqueNames(t *testing.T) {
	commands := AllCommands()
	seen := map[string]bool{}
	for _, cmd := range commands {
		if seen[cmd.Name] {
			t.Errorf("duplicate command name: %s", cmd.Name)
		}
		seen[cmd.Name] = true
	}
}

func TestInvalidateFiltered_ListMatchesAppendScopeFlags(t *testing.T) {
	s := &Session{Scope: Scope{SiteID: "s"}}

	c := NewCache()
	allTypes := []string{
		"vpc", "subnet", "instance",
		"allocation", "machine", "ip-block", "operating-system",
		"ssh-key-group", "network-security-group",
		"vpc-prefix", "rack", "expected-machine", "sku",
		"dpu-extension-service", "infiniband-partition", "nvlink-logical-partition",
		"site", "audit", "ssh-key", "tenant-account",
	}
	for _, rt := range allTypes {
		c.Set(rt, []NamedItem{{Name: rt}})
	}
	c.InvalidateFiltered()

	var invalidated, preserved []string
	for _, rt := range allTypes {
		if c.Get(rt) == nil {
			invalidated = append(invalidated, rt)
		} else {
			preserved = append(preserved, rt)
		}
	}

	for _, rt := range invalidated {
		got := appendScopeFlags(s, []string{rt, "list"})
		if !contains(got, "--site-id") {
			t.Errorf("type %q is invalidated by InvalidateFiltered but not handled by appendScopeFlags", rt)
		}
	}

	for _, rt := range preserved {
		got := appendScopeFlags(s, []string{rt, "list"})
		if contains(got, "--site-id") || contains(got, "--vpc-id") {
			t.Errorf("type %q is preserved by InvalidateFiltered but has scope flags in appendScopeFlags", rt)
		}
	}
}

// --- Helpers ---

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(ss []string, target string) bool {
	i := sort.SearchStrings(ss, target)
	if i < len(ss) && ss[i] == target {
		return true
	}
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
