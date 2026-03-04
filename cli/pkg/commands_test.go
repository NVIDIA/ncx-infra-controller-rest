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

package carbidecli

import (
	"testing"
)

func TestToKebab(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Space-separated (tag names)
		{"Site", "site"},
		{"IP Block", "ip-block"},
		{"SSH Key Group", "ssh-key-group"},
		{"DPU Extension Service", "dpu-extension-service"},
		{"Infrastructure Provider", "infrastructure-provider"},
		{"NVLink Logical Partition", "nvlink-logical-partition"},

		// CamelCase (parameter and field names)
		{"siteId", "site-id"},
		{"pageNumber", "page-number"},
		{"pageSize", "page-size"},
		{"infrastructureProviderId", "infrastructure-provider-id"},
		{"networkSecurityGroupId", "network-security-group-id"},
		{"serialConsoleHostname", "serial-console-hostname"},

		// Acronym handling
		{"NVLinkLogicalPartition", "nvlink-logical-partition"},
		{"isNVLinkPartitionEnabled", "is-nvlink-partition-enabled"},
		{"dpuExtensionServiceId", "dpu-extension-service-id"},

		// Already lowercase
		{"site", "site"},
		{"org", "org"},

		// Single word uppercase
		{"ID", "id"},
		{"VPC", "vpc"},
		{"SKU", "sku"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toKebab(tt.input)
			if got != tt.want {
				t.Errorf("toKebab(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOperationAction(t *testing.T) {
	tests := []struct {
		opID string
		want string
	}{
		{"get-all-site", "list"},
		{"get-all-instance", "list"},
		{"get-all-allocation-constraint", "list"},
		{"get-current-infrastructure-provider", "get"},
		{"get-current-tenant", "get"},
		{"get-current-service-account", "get"},
		{"create-site", "create"},
		{"create-allocation-constraint", "create"},
		{"update-site", "update"},
		{"delete-site", "delete"},
		{"get-site", "get"},
		{"get-allocation", "get"},
		{"get-site-status-history", "status-history"},
		{"get-instance-status-history", "status-history"},
		{"get-machine-status-history", "status-history"},
		// get-current-* matches before -stats suffix check
		{"get-current-infrastructure-provider-stats", "get"},
		{"get-current-tenant-stats", "get"},
		{"batch-create-instance", "batch-create"},
		{"batch-create-expected-machines", "batch-create"},
		{"batch-update-expected-machines", "batch-update"},
		{"get-metadata", "get"},
		{"get-user", "get"},
	}

	for _, tt := range tests {
		t.Run(tt.opID, func(t *testing.T) {
			got := operationAction(tt.opID)
			if got != tt.want {
				t.Errorf("operationAction(%q) = %q, want %q", tt.opID, got, tt.want)
			}
		})
	}
}

func TestExtractResourceSuffix(t *testing.T) {
	tests := []struct {
		opID string
		want string
	}{
		{"get-all-site", "site"},
		{"create-site", "site"},
		{"get-site", "site"},
		{"delete-site", "site"},
		{"update-site", "site"},
		{"get-all-allocation-constraint", "allocation-constraint"},
		{"get-current-infrastructure-provider", "infrastructure-provider"},
		{"batch-create-expected-machines", "expected-machines"},
		{"batch-update-expected-machines", "expected-machines"},
		{"get-site-status-history", "site-status-history"},
		{"get-instance-status-history", "instance-status-history"},
	}

	for _, tt := range tests {
		t.Run(tt.opID, func(t *testing.T) {
			got := extractResourceSuffix(tt.opID)
			if got != tt.want {
				t.Errorf("extractResourceSuffix(%q) = %q, want %q", tt.opID, got, tt.want)
			}
		})
	}
}

func TestSubResourceName(t *testing.T) {
	tests := []struct {
		suffix  string
		primary string
		want    string
	}{
		// Exact match → primary
		{"site", "site", ""},
		{"allocation", "allocation", ""},
		{"instance", "instance", ""},

		// Plural match → primary
		{"expected-machines", "expected-machine", ""},

		// Primary as prefix → sub-resource
		{"allocation-constraint", "allocation", "constraint"},
		{"dpu-extension-service-version", "dpu-extension-service", "version"},
		{"instance-type-machine-association", "instance-type", "machine-association"},

		// Action modifiers → primary (not sub-resource)
		{"site-status-history", "site", ""},
		{"instance-status-history", "instance", ""},
		{"infrastructure-provider-stats", "infrastructure-provider", ""},

		// Primary as suffix → sub-resource
		{"derived-ipblock", "ipblock", "derived"},

		// No overlap → sub-resource (full suffix)
		{"interface", "instance", "interface"},
		{"infiniband-interface", "instance", "infiniband-interface"},
		{"nvlink-interface", "nvlink-logical-partition", "nvlink-interface"},
	}

	for _, tt := range tests {
		name := tt.suffix + "_primary_" + tt.primary
		t.Run(name, func(t *testing.T) {
			got := subResourceName(tt.suffix, tt.primary)
			if got != tt.want {
				t.Errorf("subResourceName(%q, %q) = %q, want %q", tt.suffix, tt.primary, got, tt.want)
			}
		})
	}
}

func TestDetectPrimaryResource(t *testing.T) {
	tests := []struct {
		name  string
		opIDs []string
		want  string
	}{
		{
			name: "site is primary",
			opIDs: []string{
				"get-all-site", "create-site", "get-site", "update-site", "delete-site",
				"get-site-status-history",
			},
			want: "site",
		},
		{
			name: "allocation wins over allocation-constraint by shorter length on tie",
			opIDs: []string{
				"get-all-allocation", "create-allocation", "get-allocation", "update-allocation", "delete-allocation",
				"get-all-allocation-constraint", "create-allocation-constraint", "get-allocation-constraint", "update-allocation-constraint", "delete-allocation-constraint",
			},
			want: "allocation",
		},
		{
			name: "instance wins with more operations",
			opIDs: []string{
				"get-all-instance", "create-instance", "get-instance", "update-instance", "delete-instance",
				"batch-create-instance", "get-instance-status-history",
				"get-all-interface",
				"get-all-infiniband-interface",
			},
			want: "instance",
		},
		{
			name: "expected-machine with plural batch ops",
			opIDs: []string{
				"create-expected-machine", "get-all-expected-machine", "get-expected-machine",
				"update-expected-machine", "delete-expected-machine",
				"batch-create-expected-machines", "batch-update-expected-machines",
			},
			want: "expected-machine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := make([]resolvedOp, len(tt.opIDs))
			for i, opID := range tt.opIDs {
				ops[i] = resolvedOp{
					op: &Operation{OperationID: opID},
				}
			}
			got := detectPrimaryResource(ops)
			if got != tt.want {
				t.Errorf("detectPrimaryResource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCoerceValue(t *testing.T) {
	tests := []struct {
		value      string
		schemaType SchemaType
		want       interface{}
		wantErr    bool
	}{
		// Integers
		{"42", "integer", 42, false},
		{"0", "integer", 0, false},
		{"-1", "integer", -1, false},
		{"abc", "integer", nil, true},

		// Booleans
		{"true", "boolean", true, false},
		{"false", "boolean", false, false},
		{"1", "boolean", true, false},
		{"0", "boolean", false, false},
		{"yes", "boolean", nil, true},

		// Numbers (float)
		{"3.14", "number", 3.14, false},
		{"0", "number", float64(0), false},
		{"abc", "number", nil, true},

		// Strings (passthrough)
		{"hello", "string", "hello", false},
		{"", "string", "", false},
		{"123", "string", "123", false},
	}

	for _, tt := range tests {
		name := string(tt.schemaType) + "_" + tt.value
		t.Run(name, func(t *testing.T) {
			got, err := coerceValue(tt.value, tt.schemaType)
			if (err != nil) != tt.wantErr {
				t.Errorf("coerceValue(%q, %q) error = %v, wantErr %v", tt.value, tt.schemaType, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("coerceValue(%q, %q) = %v (%T), want %v (%T)", tt.value, tt.schemaType, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestIsListAction(t *testing.T) {
	tests := []struct {
		action string
		want   bool
	}{
		{"list", true},
		{"list-interfaces", true},
		{"list-infiniband-interfaces", true},
		{"get", false},
		{"create", false},
		{"delete", false},
		{"listing", false},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := isListAction(tt.action)
			if got != tt.want {
				t.Errorf("isListAction(%q) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestIsActionModifier(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"status-history", true},
		{"stats", true},
		{"constraint", false},
		{"version", false},
		{"virtualization", false},
		{"machine-association", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isActionModifier(tt.input)
			if got != tt.want {
				t.Errorf("isActionModifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
