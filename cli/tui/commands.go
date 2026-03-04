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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Command represents a registered interactive command.
type Command struct {
	Name        string
	Description string
	Run         func(s *Session, args []string) error
}

// AllCommands returns all available commands.
func AllCommands() []Command {
	return []Command{
		{Name: "site list", Description: "List all sites", Run: cmdSiteList},
		{Name: "site get", Description: "Get site details", Run: cmdSiteGet},

		{Name: "vpc list", Description: "List all VPCs", Run: cmdVPCList},
		{Name: "vpc get", Description: "Get VPC details", Run: cmdVPCGet},
		{Name: "vpc create", Description: "Create a VPC", Run: cmdVPCCreate},
		{Name: "vpc delete", Description: "Delete a VPC", Run: cmdVPCDelete},

		{Name: "subnet list", Description: "List all subnets", Run: cmdSubnetList},
		{Name: "subnet get", Description: "Get subnet details", Run: cmdSubnetGet},

		{Name: "instance list", Description: "List all instances", Run: cmdInstanceList},
		{Name: "instance get", Description: "Get instance details", Run: cmdInstanceGet},

		{Name: "machine list", Description: "List machines", Run: cmdMachineList},
		{Name: "machine get", Description: "Get machine details", Run: cmdMachineGet},

		{Name: "operating-system list", Description: "List operating systems", Run: cmdOSList},
		{Name: "operating-system get", Description: "Get operating system details", Run: cmdOSGet},

		{Name: "ssh-key-group list", Description: "List SSH key groups", Run: cmdSSHKeyGroupList},
		{Name: "ssh-key-group get", Description: "Get SSH key group details", Run: cmdSSHKeyGroupGet},

		{Name: "ssh-key list", Description: "List SSH keys", Run: cmdSSHKeyList},
		{Name: "ssh-key get", Description: "Get SSH key details", Run: cmdSSHKeyGet},

		{Name: "allocation list", Description: "List allocations", Run: cmdAllocationList},
		{Name: "allocation get", Description: "Get allocation details", Run: cmdAllocationGet},
		{Name: "allocation delete", Description: "Delete an allocation", Run: cmdAllocationDelete},

		{Name: "ip-block list", Description: "List IP blocks", Run: cmdIPBlockList},
		{Name: "ip-block get", Description: "Get IP block details", Run: cmdIPBlockGet},
		{Name: "ip-block create", Description: "Create an IP block", Run: cmdIPBlockCreate},
		{Name: "ip-block delete", Description: "Delete an IP block", Run: cmdIPBlockDelete},

		{Name: "network-security-group list", Description: "List network security groups", Run: cmdNSGList},
		{Name: "network-security-group get", Description: "Get network security group details", Run: cmdNSGGet},

		{Name: "sku list", Description: "List SKUs", Run: cmdSKUList},
		{Name: "sku get", Description: "Get SKU details", Run: cmdSKUGet},

		{Name: "rack list", Description: "List racks", Run: cmdRackList},
		{Name: "rack get", Description: "Get rack details", Run: cmdRackGet},

		{Name: "vpc-prefix list", Description: "List VPC prefixes", Run: cmdVPCPrefixList},
		{Name: "vpc-prefix get", Description: "Get VPC prefix details", Run: cmdVPCPrefixGet},

		{Name: "tenant-account list", Description: "List tenant accounts", Run: cmdTenantAccountList},
		{Name: "tenant-account get", Description: "Get tenant account details", Run: cmdTenantAccountGet},

		{Name: "expected-machine list", Description: "List expected machines", Run: cmdExpectedMachineList},
		{Name: "expected-machine get", Description: "Get expected machine details", Run: cmdExpectedMachineGet},

		{Name: "infiniband-partition list", Description: "List InfiniBand partitions", Run: cmdInfiniBandPartitionList},
		{Name: "infiniband-partition get", Description: "Get InfiniBand partition details", Run: cmdInfiniBandPartitionGet},

		{Name: "nvlink-logical-partition list", Description: "List NVLink logical partitions", Run: cmdNVLinkLogicalPartitionList},
		{Name: "nvlink-logical-partition get", Description: "Get NVLink logical partition details", Run: cmdNVLinkLogicalPartitionGet},

		{Name: "dpu-extension-service list", Description: "List DPU extension services", Run: cmdDPUExtensionServiceList},
		{Name: "dpu-extension-service get", Description: "Get DPU extension service details", Run: cmdDPUExtensionServiceGet},

		{Name: "audit list", Description: "List audit log entries", Run: cmdAuditList},
		{Name: "audit get", Description: "Get audit log entry details", Run: cmdAuditGet},

		{Name: "metadata get", Description: "Get API metadata", Run: cmdMetadataGet},
		{Name: "user current", Description: "Get current user", Run: cmdUserCurrent},
		{Name: "tenant current", Description: "Get current tenant", Run: cmdTenantCurrent},
		{Name: "tenant stats", Description: "Get tenant stats", Run: cmdTenantStats},
		{Name: "infrastructure-provider current", Description: "Get current infrastructure provider", Run: cmdInfraProviderCurrent},
		{Name: "infrastructure-provider stats", Description: "Get infrastructure provider stats", Run: cmdInfraProviderStats},

		{Name: "login", Description: "Login / refresh auth token", Run: cmdLogin},
		{Name: "help", Description: "Show available commands", Run: cmdHelp},
	}
}

// LogCmd prints the equivalent carbidecli one-liner for reference.
func LogCmd(s *Session, parts ...string) {
	cmdParts := []string{"carbidecli"}
	if s != nil && strings.TrimSpace(s.ConfigPath) != "" {
		cmdParts = append(cmdParts, "--config", s.ConfigPath)
	}
	cmdParts = append(cmdParts, appendScopeFlags(s, parts)...)
	fmt.Printf("%s %s\n", Dim("INFO:"), strings.Join(cmdParts, " "))
}

func appendScopeFlags(s *Session, parts []string) []string {
	out := append([]string(nil), parts...)
	if s == nil || len(parts) < 2 {
		return out
	}
	resource := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	if action != "list" {
		return out
	}
	scopeSiteID := strings.TrimSpace(s.Scope.SiteID)
	scopeVpcID := strings.TrimSpace(s.Scope.VpcID)
	switch resource {
	case "vpc", "allocation", "ip-block", "operating-system", "ssh-key-group",
		"network-security-group", "sku", "rack", "expected-machine",
		"dpu-extension-service", "infiniband-partition", "nvlink-logical-partition", "machine":
		if scopeSiteID != "" {
			out = append(out, "--site-id", scopeSiteID)
		}
	case "subnet", "vpc-prefix":
		if scopeSiteID != "" {
			out = append(out, "--site-id", scopeSiteID)
		}
		if scopeVpcID != "" {
			out = append(out, "--vpc-id", scopeVpcID)
		}
	case "instance":
		if scopeSiteID != "" {
			out = append(out, "--site-id", scopeSiteID)
		}
		if scopeVpcID != "" {
			out = append(out, "--vpc-id", scopeVpcID)
		}
	}
	return out
}

// -- List commands --

func cmdSiteList(s *Session, _ []string) error {
	LogCmd(s, "site", "list")
	items, err := s.Resolver.Fetch(context.Background(), "site")
	if err != nil {
		return err
	}
	return printResourceTable(os.Stdout, "NAME", "STATUS", "ID", items)
}

func cmdVPCList(s *Session, _ []string) error {
	LogCmd(s, "vpc", "list")
	items, err := s.Resolver.Fetch(context.Background(), "vpc")
	if err != nil {
		return err
	}
	siteNameByID := map[string]string{}
	if sites, err := s.Resolver.Fetch(context.Background(), "site"); err == nil {
		for _, site := range sites {
			siteNameByID[site.ID] = site.Name
		}
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tSITE\tID")
	for _, item := range items {
		siteID := item.Extra["siteId"]
		siteName := strings.TrimSpace(siteNameByID[siteID])
		if siteName == "" {
			siteName = siteID
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Name, item.Status, siteName, item.ID)
	}
	return tw.Flush()
}

func cmdVPCCreate(s *Session, _ []string) error {
	site, err := s.Resolver.Resolve(context.Background(), "site", "Site")
	if err != nil {
		return err
	}
	name, err := PromptText("VPC name", true)
	if err != nil {
		return err
	}
	desc, err := PromptText("Description (optional)", false)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"name": name, "siteId": site.ID}
	if strings.TrimSpace(desc) != "" {
		body["description"] = desc
	}
	LogCmd(s, "vpc", "create", "--name", name, "--site-id", site.ID)
	bodyJSON, _ := json.Marshal(body)
	resp, _, err := s.Client.Do("POST", "/v2/org/{org}/carbide/vpc", nil, nil, bodyJSON)
	if err != nil {
		return fmt.Errorf("creating VPC: %w", err)
	}
	s.Cache.Invalidate("vpc")
	var created map[string]interface{}
	json.Unmarshal(resp, &created)
	fmt.Printf("%s VPC created: %s (%s)\n", Green("OK"), str(created, "name"), str(created, "id"))
	return nil
}

func cmdVPCDelete(s *Session, args []string) error {
	vpc, err := s.Resolver.ResolveWithArgs(context.Background(), "vpc", "VPC to delete", args)
	if err != nil {
		return err
	}
	ok, err := PromptConfirm(fmt.Sprintf("Delete VPC %s (%s)?", vpc.Name, vpc.ID))
	if err != nil || !ok {
		return err
	}
	LogCmd(s, "vpc", "delete", vpc.ID)
	_, _, err = s.Client.Do("DELETE", "/v2/org/{org}/carbide/vpc/{id}", map[string]string{"id": vpc.ID}, nil, nil)
	if err != nil {
		return fmt.Errorf("deleting VPC: %w", err)
	}
	s.Cache.Invalidate("vpc")
	fmt.Printf("%s VPC deleted: %s\n", Green("OK"), vpc.Name)
	return nil
}

func cmdSubnetList(s *Session, _ []string) error {
	LogCmd(s, "subnet", "list")
	items, err := s.Resolver.Fetch(context.Background(), "subnet")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tVPC\tID")
	for _, item := range items {
		vpcName := s.Resolver.ResolveID("vpc", item.Extra["vpcId"])
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Name, item.Status, vpcName, item.ID)
	}
	return tw.Flush()
}

func cmdInstanceList(s *Session, _ []string) error {
	LogCmd(s, "instance", "list")
	// Warm VPC and site caches so IDs resolve to names.
	_, _ = s.Resolver.Fetch(context.Background(), "vpc")
	_, _ = s.Resolver.Fetch(context.Background(), "site")
	items, err := s.Resolver.Fetch(context.Background(), "instance")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tVPC\tSITE\tID")
	for _, item := range items {
		vpcName := s.Resolver.ResolveID("vpc", item.Extra["vpcId"])
		siteName := s.Resolver.ResolveID("site", item.Extra["siteId"])
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", item.Name, item.Status, vpcName, siteName, item.ID)
	}
	return tw.Flush()
}

func cmdMachineList(s *Session, _ []string) error {
	LogCmd(s, "machine", "list")
	items, err := s.Resolver.Fetch(context.Background(), "machine")
	if err != nil {
		if s.Scope.SiteID == "" && strings.Contains(err.Error(), "400") {
			fmt.Printf("%s Machine listing requires a site filter. Select a site.\n", Dim("Note:"))
			site, resolveErr := s.Resolver.Resolve(context.Background(), "site", "Site")
			if resolveErr != nil {
				return resolveErr
			}
			s.Scope.SiteID = site.ID
			s.Scope.SiteName = site.Name
			s.Cache.InvalidateFiltered()
			items, err = s.Resolver.Fetch(context.Background(), "machine")
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Warm VPC cache so names resolve, then build machine→vpc map via instances.
	_, _ = s.Resolver.Fetch(context.Background(), "vpc")
	vpcNamesByMachineID := s.buildMachineVPCNames(context.Background())

	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tSITE\tVPC\tID")
	for _, item := range items {
		siteName := s.Resolver.ResolveID("site", item.Extra["siteId"])
		vpcNames := strings.TrimSpace(vpcNamesByMachineID[item.ID])
		if vpcNames == "" {
			vpcNames = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", item.Name, item.Status, siteName, vpcNames, item.ID)
	}
	return tw.Flush()
}

func cmdOSList(s *Session, _ []string) error {
	LogCmd(s, "operating-system", "list")
	items, err := s.Resolver.Fetch(context.Background(), "operating-system")
	if err != nil {
		return err
	}
	return printResourceTable(os.Stdout, "NAME", "STATUS", "ID", items)
}

func cmdSSHKeyGroupList(s *Session, _ []string) error {
	LogCmd(s, "ssh-key-group", "list")
	items, err := s.Resolver.Fetch(context.Background(), "ssh-key-group")
	if err != nil {
		return err
	}
	return printResourceTable(os.Stdout, "NAME", "STATUS", "ID", items)
}

func cmdSSHKeyList(s *Session, _ []string) error {
	LogCmd(s, "ssh-key", "list")
	items, err := s.Resolver.Fetch(context.Background(), "ssh-key")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tFINGERPRINT\tID")
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", item.Name, item.Extra["fingerprint"], item.ID)
	}
	return tw.Flush()
}

func cmdAllocationList(s *Session, _ []string) error {
	LogCmd(s, "allocation", "list")
	items, err := s.Resolver.Fetch(context.Background(), "allocation")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tSITE\tID")
	for _, item := range items {
		siteName := s.Resolver.ResolveID("site", item.Extra["siteId"])
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Name, item.Status, siteName, item.ID)
	}
	return tw.Flush()
}

func cmdIPBlockList(s *Session, _ []string) error {
	LogCmd(s, "ip-block", "list")
	items, err := s.Resolver.Fetch(context.Background(), "ip-block")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tSITE\tID")
	for _, item := range items {
		siteName := s.Resolver.ResolveID("site", item.Extra["siteId"])
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Name, item.Status, siteName, item.ID)
	}
	return tw.Flush()
}

func cmdIPBlockCreate(s *Session, _ []string) error {
	site, err := s.Resolver.Resolve(context.Background(), "site", "Site")
	if err != nil {
		return err
	}
	name, err := PromptText("IP block name", true)
	if err != nil {
		return err
	}
	prefix, err := PromptText("Prefix (e.g. 10.0.0.0)", true)
	if err != nil {
		return err
	}
	prefixLen, err := PromptText("Prefix length (e.g. 16)", true)
	if err != nil {
		return err
	}
	var pl int
	fmt.Sscanf(prefixLen, "%d", &pl)
	if pl < 1 || pl > 32 {
		return fmt.Errorf("prefix length must be between 1 and 32")
	}
	LogCmd(s, "ip-block", "create", "--name", name, "--site-id", site.ID, "--prefix", prefix, "--prefix-length", prefixLen)
	body := map[string]interface{}{
		"name":         name,
		"siteId":       site.ID,
		"ipVersion":    "IPv4",
		"usageType":    "DatacenterOnly",
		"prefix":       prefix,
		"prefixLength": pl,
	}
	bodyJSON, _ := json.Marshal(body)
	resp, _, err := s.Client.Do("POST", "/v2/org/{org}/carbide/ip-block", nil, nil, bodyJSON)
	if err != nil {
		return fmt.Errorf("creating IP block: %w", err)
	}
	s.Cache.Invalidate("ip-block")
	var created map[string]interface{}
	json.Unmarshal(resp, &created)
	fmt.Printf("%s IP block created: %s (%s)\n", Green("OK"), str(created, "name"), str(created, "id"))
	return nil
}

func cmdIPBlockDelete(s *Session, args []string) error {
	block, err := s.Resolver.ResolveWithArgs(context.Background(), "ip-block", "IP Block to delete", args)
	if err != nil {
		return err
	}
	ok, err := PromptConfirm(fmt.Sprintf("Delete IP block %s (%s)?", block.Name, block.ID))
	if err != nil || !ok {
		return err
	}
	LogCmd(s, "ip-block", "delete", block.ID)
	_, _, err = s.Client.Do("DELETE", "/v2/org/{org}/carbide/ip-block/{id}", map[string]string{"id": block.ID}, nil, nil)
	if err != nil {
		return fmt.Errorf("deleting IP block: %w", err)
	}
	s.Cache.Invalidate("ip-block")
	fmt.Printf("%s IP block deleted: %s\n", Green("OK"), block.Name)
	return nil
}

func cmdNSGList(s *Session, _ []string) error {
	LogCmd(s, "network-security-group", "list")
	items, err := s.Resolver.Fetch(context.Background(), "network-security-group")
	if err != nil {
		return err
	}
	return printResourceTable(os.Stdout, "NAME", "STATUS", "ID", items)
}

func cmdSKUList(s *Session, _ []string) error {
	LogCmd(s, "sku", "list")
	items, err := s.Resolver.Fetch(context.Background(), "sku")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "DEVICE TYPE\tSITE\tID")
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", item.Extra["deviceType"], item.Extra["siteId"], item.ID)
	}
	return tw.Flush()
}

func cmdRackList(s *Session, _ []string) error {
	LogCmd(s, "rack", "list")
	items, err := s.Resolver.Fetch(context.Background(), "rack")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tMANUFACTURER\tMODEL\tID")
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Name, item.Extra["manufacturer"], item.Extra["model"], item.ID)
	}
	return tw.Flush()
}

func cmdVPCPrefixList(s *Session, _ []string) error {
	LogCmd(s, "vpc-prefix", "list")
	items, err := s.Resolver.Fetch(context.Background(), "vpc-prefix")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tVPC\tID")
	for _, item := range items {
		vpcName := s.Resolver.ResolveID("vpc", item.Extra["vpcId"])
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Name, item.Status, vpcName, item.ID)
	}
	return tw.Flush()
}

func cmdTenantAccountList(s *Session, _ []string) error {
	LogCmd(s, "tenant-account", "list")
	items, err := s.Resolver.Fetch(context.Background(), "tenant-account")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "TENANT ORG\tSTATUS\tINFRA PROVIDER ID\tID")
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Name, item.Status, item.Extra["infrastructureProviderId"], item.ID)
	}
	return tw.Flush()
}

func cmdExpectedMachineList(s *Session, _ []string) error {
	LogCmd(s, "expected-machine", "list")
	items, err := s.Resolver.Fetch(context.Background(), "expected-machine")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "SITE ID\tBMC MAC\tCHASSIS SN\tID")
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Extra["siteId"], item.Extra["bmcMacAddress"], item.Extra["chassisSerialNumber"], item.ID)
	}
	return tw.Flush()
}

func cmdInfiniBandPartitionList(s *Session, _ []string) error {
	LogCmd(s, "infiniband-partition", "list")
	items, err := s.Resolver.Fetch(context.Background(), "infiniband-partition")
	if err != nil {
		return err
	}
	return printResourceTable(os.Stdout, "NAME", "STATUS", "ID", items)
}

func cmdNVLinkLogicalPartitionList(s *Session, _ []string) error {
	LogCmd(s, "nvlink-logical-partition", "list")
	items, err := s.Resolver.Fetch(context.Background(), "nvlink-logical-partition")
	if err != nil {
		return err
	}
	return printResourceTable(os.Stdout, "NAME", "STATUS", "ID", items)
}

func cmdDPUExtensionServiceList(s *Session, _ []string) error {
	LogCmd(s, "dpu-extension-service", "list")
	items, err := s.Resolver.Fetch(context.Background(), "dpu-extension-service")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tSITE ID\tID")
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Name, item.Extra["serviceType"], item.Extra["siteId"], item.ID)
	}
	return tw.Flush()
}

func cmdAuditList(s *Session, _ []string) error {
	LogCmd(s, "audit", "list")
	items, err := s.Resolver.Fetch(context.Background(), "audit")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "METHOD\tENDPOINT\tSTATUS CODE\tID")
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", item.Extra["method"], item.Extra["endpoint"], item.Status, item.ID)
	}
	return tw.Flush()
}

// -- Get commands (raw JSON detail) --

func cmdSiteGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "site", "Site", args)
	if err != nil {
		return err
	}
	LogCmd(s, "site", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/site/{id}", item.ID)
}

func cmdVPCGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "vpc", "VPC", args)
	if err != nil {
		return err
	}
	LogCmd(s, "vpc", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/vpc/{id}", item.ID)
}

func cmdSubnetGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "subnet", "Subnet", args)
	if err != nil {
		return err
	}
	LogCmd(s, "subnet", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/subnet/{id}", item.ID)
}

func cmdInstanceGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "instance", "Instance", args)
	if err != nil {
		return err
	}
	LogCmd(s, "instance", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/instance/{id}", item.ID)
}

func cmdMachineGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "machine", "Machine", args)
	if err != nil {
		return err
	}
	LogCmd(s, "machine", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/machine/{id}", item.ID)
}

func cmdOSGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "operating-system", "Operating System", args)
	if err != nil {
		return err
	}
	LogCmd(s, "operating-system", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/operating-system/{id}", item.ID)
}

func cmdSSHKeyGroupGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "ssh-key-group", "SSH Key Group", args)
	if err != nil {
		return err
	}
	LogCmd(s, "ssh-key-group", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/ssh-key-group/{id}", item.ID)
}

func cmdSSHKeyGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "ssh-key", "SSH Key", args)
	if err != nil {
		return err
	}
	LogCmd(s, "ssh-key", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/ssh-key/{id}", item.ID)
}

func cmdAllocationGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "allocation", "Allocation", args)
	if err != nil {
		return err
	}
	LogCmd(s, "allocation", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/allocation/{id}", item.ID)
}

func cmdAllocationDelete(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "allocation", "Allocation to delete", args)
	if err != nil {
		return err
	}
	ok, err := PromptConfirm(fmt.Sprintf("Delete allocation %s (%s)?", item.Name, item.ID))
	if err != nil || !ok {
		return err
	}
	LogCmd(s, "allocation", "delete", item.ID)
	_, _, err = s.Client.Do("DELETE", "/v2/org/{org}/carbide/allocation/{id}", map[string]string{"id": item.ID}, nil, nil)
	if err != nil {
		return fmt.Errorf("deleting allocation: %w", err)
	}
	s.Cache.Invalidate("allocation")
	fmt.Printf("%s Allocation deleted: %s\n", Green("OK"), item.Name)
	return nil
}

func cmdIPBlockGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "ip-block", "IP Block", args)
	if err != nil {
		return err
	}
	LogCmd(s, "ip-block", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/ip-block/{id}", item.ID)
}

func cmdNSGGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "network-security-group", "Network Security Group", args)
	if err != nil {
		return err
	}
	LogCmd(s, "network-security-group", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/network-security-group/{id}", item.ID)
}

func cmdSKUGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "sku", "SKU", args)
	if err != nil {
		return err
	}
	LogCmd(s, "sku", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/sku/{id}", item.ID)
}

func cmdRackGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "rack", "Rack", args)
	if err != nil {
		return err
	}
	LogCmd(s, "rack", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/rack/{id}", item.ID)
}

func cmdVPCPrefixGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "vpc-prefix", "VPC Prefix", args)
	if err != nil {
		return err
	}
	LogCmd(s, "vpc-prefix", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/vpc-prefix/{id}", item.ID)
}

func cmdTenantAccountGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "tenant-account", "Tenant Account", args)
	if err != nil {
		return err
	}
	LogCmd(s, "tenant-account", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/tenant-account/{id}", item.ID)
}

func cmdExpectedMachineGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "expected-machine", "Expected Machine", args)
	if err != nil {
		return err
	}
	LogCmd(s, "expected-machine", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/expected-machine/{id}", item.ID)
}

func cmdInfiniBandPartitionGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "infiniband-partition", "InfiniBand Partition", args)
	if err != nil {
		return err
	}
	LogCmd(s, "infiniband-partition", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/infiniband-partition/{id}", item.ID)
}

func cmdNVLinkLogicalPartitionGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "nvlink-logical-partition", "NVLink Logical Partition", args)
	if err != nil {
		return err
	}
	LogCmd(s, "nvlink-logical-partition", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/nvlink-logical-partition/{id}", item.ID)
}

func cmdDPUExtensionServiceGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "dpu-extension-service", "DPU Extension Service", args)
	if err != nil {
		return err
	}
	LogCmd(s, "dpu-extension-service", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/dpu-extension-service/{id}", item.ID)
}

func cmdAuditGet(s *Session, args []string) error {
	item, err := s.Resolver.ResolveWithArgs(context.Background(), "audit", "Audit Entry", args)
	if err != nil {
		return err
	}
	LogCmd(s, "audit", "get", item.ID)
	return getAndPrint(s, "/v2/org/{org}/carbide/audit/{id}", item.ID)
}

// -- Singleton / info commands --

func cmdMetadataGet(s *Session, _ []string) error {
	LogCmd(s, "metadata", "get")
	body, _, err := s.Client.Do("GET", "/v2/org/{org}/carbide/metadata", nil, nil, nil)
	if err != nil {
		return fmt.Errorf("getting metadata: %w", err)
	}
	return printDetailJSON(os.Stdout, body)
}

func cmdUserCurrent(s *Session, _ []string) error {
	LogCmd(s, "user", "current")
	body, _, err := s.Client.Do("GET", "/v2/org/{org}/carbide/user/current", nil, nil, nil)
	if err != nil {
		return fmt.Errorf("getting current user: %w", err)
	}
	return printDetailJSON(os.Stdout, body)
}

func cmdTenantCurrent(s *Session, _ []string) error {
	LogCmd(s, "tenant", "current")
	body, _, err := s.Client.Do("GET", "/v2/org/{org}/carbide/tenant/current", nil, nil, nil)
	if err != nil {
		return fmt.Errorf("getting current tenant: %w", err)
	}
	return printDetailJSON(os.Stdout, body)
}

func cmdTenantStats(s *Session, _ []string) error {
	LogCmd(s, "tenant", "stats")
	body, _, err := s.Client.Do("GET", "/v2/org/{org}/carbide/tenant/current/stats", nil, nil, nil)
	if err != nil {
		return fmt.Errorf("getting tenant stats: %w", err)
	}
	return printDetailJSON(os.Stdout, body)
}

func cmdInfraProviderCurrent(s *Session, _ []string) error {
	LogCmd(s, "infrastructure-provider", "current")
	body, _, err := s.Client.Do("GET", "/v2/org/{org}/carbide/infrastructure-provider/current", nil, nil, nil)
	if err != nil {
		return fmt.Errorf("getting infrastructure provider: %w", err)
	}
	return printDetailJSON(os.Stdout, body)
}

func cmdInfraProviderStats(s *Session, _ []string) error {
	LogCmd(s, "infrastructure-provider", "stats")
	body, _, err := s.Client.Do("GET", "/v2/org/{org}/carbide/infrastructure-provider/current/stats", nil, nil, nil)
	if err != nil {
		return fmt.Errorf("getting infrastructure provider stats: %w", err)
	}
	return printDetailJSON(os.Stdout, body)
}

func cmdLogin(s *Session, _ []string) error {
	if s.LoginFn == nil {
		return fmt.Errorf("login not available (no auth method configured)")
	}
	fmt.Println("Logging in...")
	token, err := s.LoginFn()
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	s.RefreshClient(token)
	fmt.Printf("%s Logged in successfully.\n", Green("OK"))
	return nil
}

func cmdHelp(_ *Session, _ []string) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "\nCOMMAND\tDESCRIPTION")
	fmt.Fprintln(tw, "-------\t-----------")
	for _, cmd := range AllCommands() {
		fmt.Fprintf(tw, "%s\t%s\n", cmd.Name, cmd.Description)
	}
	fmt.Fprintln(tw, "org\tShow current org")
	fmt.Fprintln(tw, "org list\tList available orgs (from JWT claims)")
	fmt.Fprintln(tw, "org set <name>\tSwitch to a different org")
	fmt.Fprintln(tw, "scope\tShow current scope filters")
	fmt.Fprintln(tw, "scope site [name]\tSet site scope (filters lists)")
	fmt.Fprintln(tw, "scope vpc [name]\tSet VPC scope (filters lists)")
	fmt.Fprintln(tw, "scope clear\tClear all scope filters")
	fmt.Fprintln(tw, "exit\tExit interactive mode")
	tw.Flush()
	fmt.Println()
	return nil
}

// -- Helpers --

func getAndPrint(s *Session, path, id string) error {
	body, _, err := s.Client.Do("GET", path, map[string]string{"id": id}, nil, nil)
	if err != nil {
		return err
	}
	return printDetailJSON(os.Stdout, body)
}

func printDetailJSON(w io.Writer, data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Fprintln(w, string(data))
		return nil
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(w, string(data))
		return nil
	}
	fmt.Fprintln(w, string(pretty))
	return nil
}

func printResourceTable(w io.Writer, col1, col2, col3 string, items []NamedItem) error {
	fmt.Fprintf(os.Stderr, "%d items\n", len(items))
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "%s\t%s\t%s\n", col1, col2, col3)
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", item.Name, item.Status, item.ID)
	}
	return tw.Flush()
}
