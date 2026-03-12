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
	"strings"
	"testing"
)

func TestAppendScopeFlags_NoSession(t *testing.T) {
	got := appendScopeFlags(nil, []string{"machine", "list"})
	want := []string{"machine", "list"}
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAppendScopeFlags_NoScope(t *testing.T) {
	s := &Session{}
	got := appendScopeFlags(s, []string{"machine", "list"})
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
