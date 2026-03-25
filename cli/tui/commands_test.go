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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestReadyMachineItemsForSite_FiltersByStatusAndSite(t *testing.T) {
	machines := []NamedItem{
		{Name: "m1", ID: "1", Status: "Ready", Extra: map[string]string{"siteId": "site-a"}},
		{Name: "m2", ID: "2", Status: "ready", Extra: map[string]string{"siteId": "site-a"}},
		{Name: "m3", ID: "3", Status: "NotReady", Extra: map[string]string{"siteId": "site-a"}},
		{Name: "m4", ID: "4", Status: "Ready", Extra: map[string]string{"siteId": "site-b"}},
	}

	got := readyMachineItemsForSite(machines, "site-a")
	require.Len(t, got, 2)
	assert.Equal(t, "1", got[0].ID)
	assert.Equal(t, "2", got[1].ID)
}

func TestSetSiteScopeFromID_UpdatesScopeAndInvalidatesFiltered(t *testing.T) {
	c := NewCache()
	c.Set("site", []NamedItem{{Name: "Site Two", ID: "site-2"}})
	c.Set("machine", []NamedItem{{Name: "m1", ID: "1"}})
	s := &Session{
		Scope:    Scope{SiteID: "site-1", SiteName: "Site One", VpcID: "vpc-1", VpcName: "VPC One"},
		Cache:    c,
		Resolver: NewResolver(c),
	}

	setSiteScopeFromID(s, "site-2")

	assert.Equal(t, "site-2", s.Scope.SiteID)
	assert.Equal(t, "Site Two", s.Scope.SiteName)
	assert.Empty(t, s.Scope.VpcID, "VPC scope must be cleared when site changes")
	assert.Empty(t, s.Scope.VpcName, "VPC name must be cleared when site changes")
	assert.Nil(t, c.Get("machine"), "filtered cache must be invalidated")
}

func TestSetSiteScopeFromID_NoChangeKeepsFilteredCache(t *testing.T) {
	c := NewCache()
	c.Set("machine", []NamedItem{{Name: "m1", ID: "1"}})
	s := &Session{
		Scope:    Scope{SiteID: "site-1", SiteName: "Site One", VpcID: "vpc-1"},
		Cache:    c,
		Resolver: NewResolver(c),
	}

	setSiteScopeFromID(s, "site-1")

	assert.NotNil(t, c.Get("machine"), "machine cache should remain when scope site does not change")
	assert.Equal(t, "vpc-1", s.Scope.VpcID, "VPC scope should remain when site does not change")
}

// --- Label support tests ---

func TestExtractLabels(t *testing.T) {
	t.Run("valid map", func(t *testing.T) {
		m := map[string]interface{}{
			"labels": map[string]interface{}{"env": "prod", "rack": "A3"},
		}
		got := extractLabels(m)
		require.Len(t, got, 2)
		assert.Equal(t, "prod", got["env"])
		assert.Equal(t, "A3", got["rack"])
	})
	t.Run("nil labels", func(t *testing.T) {
		m := map[string]interface{}{"name": "test"}
		assert.Nil(t, extractLabels(m))
	})
	t.Run("non-string values ignored", func(t *testing.T) {
		m := map[string]interface{}{
			"labels": map[string]interface{}{"env": "prod", "count": 42},
		}
		got := extractLabels(m)
		require.Len(t, got, 1)
		assert.Equal(t, "prod", got["env"])
	})
	t.Run("empty map", func(t *testing.T) {
		m := map[string]interface{}{
			"labels": map[string]interface{}{},
		}
		assert.Nil(t, extractLabels(m))
	})
}

func TestFormatLabels(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, "", formatLabels(nil, 60))
	})
	t.Run("single", func(t *testing.T) {
		assert.Equal(t, "env=prod", formatLabels(map[string]string{"env": "prod"}, 60))
	})
	t.Run("multiple sorted", func(t *testing.T) {
		assert.Equal(t, "env=prod, rack=A3", formatLabels(map[string]string{"rack": "A3", "env": "prod"}, 60))
	})
	t.Run("truncation", func(t *testing.T) {
		got := formatLabels(map[string]string{"env": "production", "rack": "A3"}, 15)
		assert.LessOrEqual(t, len(got), 15)
		assert.True(t, strings.HasSuffix(got, "..."), "expected truncation suffix, got %q", got)
	})
	t.Run("no truncation when fits", func(t *testing.T) {
		got := formatLabels(map[string]string{"a": "b"}, 60)
		assert.False(t, strings.HasSuffix(got, "..."), "should not truncate short label: %q", got)
	})
}

func TestFilterByLabels(t *testing.T) {
	items := []NamedItem{
		{Name: "a", Labels: map[string]string{"env": "prod", "rack": "A3"}},
		{Name: "b", Labels: map[string]string{"env": "dev"}},
		{Name: "c", Labels: nil},
		{Name: "d", Labels: map[string]string{"env": "prod", "rack": "B1"}},
	}

	t.Run("no filters", func(t *testing.T) {
		assert.Len(t, filterByLabels(items, nil), 4)
	})
	t.Run("single match", func(t *testing.T) {
		got := filterByLabels(items, map[string]string{"env": "dev"})
		require.Len(t, got, 1)
		assert.Equal(t, "b", got[0].Name)
	})
	t.Run("multi-key AND", func(t *testing.T) {
		got := filterByLabels(items, map[string]string{"env": "prod", "rack": "A3"})
		require.Len(t, got, 1)
		assert.Equal(t, "a", got[0].Name)
	})
	t.Run("no match", func(t *testing.T) {
		assert.Empty(t, filterByLabels(items, map[string]string{"env": "staging"}))
	})
	t.Run("nil labels handled", func(t *testing.T) {
		got := filterByLabels(items, map[string]string{"env": "prod"})
		for _, item := range got {
			assert.NotNil(t, item.Labels, "nil-label item should not pass filter")
		}
	})
}

func TestSortByLabelKey(t *testing.T) {
	t.Run("ascending sort", func(t *testing.T) {
		items := []NamedItem{
			{Name: "c", Labels: map[string]string{"rack": "C1"}},
			{Name: "a", Labels: map[string]string{"rack": "A1"}},
			{Name: "b", Labels: map[string]string{"rack": "B1"}},
		}
		sorted := sortByLabelKey(items, "rack")
		require.Len(t, sorted, 3)
		assert.Equal(t, "a", sorted[0].Name)
		assert.Equal(t, "b", sorted[1].Name)
		assert.Equal(t, "c", sorted[2].Name)
		assert.Equal(t, "c", items[0].Name, "sortByLabelKey must not mutate the original slice")
	})
	t.Run("missing keys sort last", func(t *testing.T) {
		items := []NamedItem{
			{Name: "no-label", Labels: nil},
			{Name: "has-label", Labels: map[string]string{"rack": "A1"}},
		}
		sorted := sortByLabelKey(items, "rack")
		assert.Equal(t, "has-label", sorted[0].Name)
		assert.Equal(t, "no-label", sorted[1].Name)
	})
	t.Run("stable order for equal values", func(t *testing.T) {
		items := []NamedItem{
			{Name: "first", Labels: map[string]string{"rack": "A1"}},
			{Name: "second", Labels: map[string]string{"rack": "A1"}},
		}
		sorted := sortByLabelKey(items, "rack")
		assert.Equal(t, "first", sorted[0].Name)
		assert.Equal(t, "second", sorted[1].Name)
	})
}

func TestParseLabelArgs(t *testing.T) {
	t.Run("label and sort-label", func(t *testing.T) {
		remaining, labels, sortKey, err := parseLabelArgs([]string{"--label", "env=prod", "--sort-label", "rack", "extra"})
		require.NoError(t, err)
		assert.Equal(t, []string{"extra"}, remaining)
		assert.Equal(t, "prod", labels["env"])
		assert.Equal(t, "rack", sortKey)
	})
	t.Run("no label args", func(t *testing.T) {
		remaining, labels, sortKey, err := parseLabelArgs([]string{"foo", "bar"})
		require.NoError(t, err)
		assert.Len(t, remaining, 2)
		assert.Empty(t, labels)
		assert.Empty(t, sortKey)
	})
	t.Run("multiple labels AND", func(t *testing.T) {
		_, labels, _, err := parseLabelArgs([]string{"--label", "env=prod", "--label", "rack=A3"})
		require.NoError(t, err)
		require.Len(t, labels, 2)
		assert.Equal(t, "prod", labels["env"])
		assert.Equal(t, "A3", labels["rack"])
	})
	t.Run("label without equals", func(t *testing.T) {
		_, _, _, err := parseLabelArgs([]string{"--label", "env"})
		assert.Error(t, err)
	})
	t.Run("dangling sort-label", func(t *testing.T) {
		_, _, _, err := parseLabelArgs([]string{"--sort-label"})
		assert.Error(t, err)
	})
	t.Run("dangling label flag", func(t *testing.T) {
		_, _, _, err := parseLabelArgs([]string{"--label"})
		assert.Error(t, err)
	})
	t.Run("conflicting same-key labels", func(t *testing.T) {
		_, _, _, err := parseLabelArgs([]string{"--label", "env=prod", "--label", "env=dev"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting")
	})
	t.Run("duplicate same-value labels accepted", func(t *testing.T) {
		_, labels, _, err := parseLabelArgs([]string{"--label", "env=prod", "--label", "env=prod"})
		require.NoError(t, err)
		assert.Equal(t, "prod", labels["env"])
	})
}

func TestMergeLabels(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		got, err := mergeLabels(nil, nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("conflicting scope and cmd", func(t *testing.T) {
		scope := map[string]string{"env": "dev"}
		cmd := map[string]string{"env": "prod"}
		_, err := mergeLabels(scope, cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicts")
	})
	t.Run("same value allowed", func(t *testing.T) {
		scope := map[string]string{"env": "prod"}
		cmd := map[string]string{"env": "prod"}
		got, err := mergeLabels(scope, cmd)
		require.NoError(t, err)
		assert.Equal(t, "prod", got["env"])
	})
	t.Run("combines unique keys", func(t *testing.T) {
		scope := map[string]string{"env": "prod"}
		cmd := map[string]string{"rack": "A3"}
		got, err := mergeLabels(scope, cmd)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "prod", got["env"])
		assert.Equal(t, "A3", got["rack"])
	})
}

func TestInvalidateFilteredIncludesInstanceType(t *testing.T) {
	c := NewCache()
	c.Set("instance-type", []NamedItem{{Name: "it1", ID: "1"}})
	c.InvalidateFiltered()
	assert.Nil(t, c.Get("instance-type"), "instance-type cache should be invalidated by InvalidateFiltered")
}

func TestAppendScopeFlagsIncludesInstanceType(t *testing.T) {
	s := &Session{
		Scope: Scope{SiteID: "site-1"},
		Cache: NewCache(),
	}
	s.Resolver = NewResolver(s.Cache)
	got := appendScopeFlags(s, []string{"instance-type", "list"})
	assert.True(t, contains(got, "--site-id"), "instance-type should receive --site-id scope flag")
}

func TestVPCFilteringDoesNotMutateCachedSlice(t *testing.T) {
	original := []NamedItem{
		{Name: "m1", ID: "1"},
		{Name: "m2", ID: "2"},
		{Name: "m3", ID: "3"},
	}
	cached := make([]NamedItem, len(original))
	copy(cached, original)

	vpcMembers := map[string]string{"1": "vpc-a"}
	filtered := make([]NamedItem, 0, len(cached))
	for _, item := range cached {
		if _, ok := vpcMembers[item.ID]; ok {
			filtered = append(filtered, item)
		}
	}

	require.Len(t, filtered, 1)
	assert.Equal(t, "m1", filtered[0].Name)
	require.Len(t, cached, 3, "cached slice must not be truncated by filtering")
	assert.Equal(t, "m1", cached[0].Name)
	assert.Equal(t, "m2", cached[1].Name)
	assert.Equal(t, "m3", cached[2].Name)
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
