package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

const (
	LicenseHeader = `
// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
`

	goPackageOption = `option go_package = "github.com/NVIDIA/ncx-infra-controller-rest/workflow-schema/proto";`
)

func normalizeProtoFile(protoFile string) {
	protoFileContent, err := os.ReadFile(protoFile)
	if err != nil {
		log.Err(err).Str("protoFile", protoFile).Msg("Failed to read proto file")
		return
	}

	log.Info().Str("ProtoFile", protoFile).Int("ContentLength", len(protoFileContent)).Msg("Normalizing proto file")

	content := string(protoFileContent)
	content = addOrReplaceLicenseHeader(content)
	content = addGoPackageOption(content)
	content = updateImports(content)

	baseName := filepath.Base(protoFile)
	switch baseName {
	case "site_explorer_carbide.proto":
		content = normalizeSiteExplorer(content)
	case "dns_carbide.proto":
		content = normalizeDns(content)
	case "forge_carbide.proto":
		content = normalizeForge(content)
	}

	content = trimWhitespace(content)

	if err := os.WriteFile(protoFile, []byte(content), 0644); err != nil {
		log.Err(err).Str("protoFile", protoFile).Msg("Failed to write normalized proto file")
	}
}

// addOrReplaceLicenseHeader strips any existing comment/blank-line preamble
// before the first proto directive (e.g. `syntax`) and prepends LicenseHeader.
func addOrReplaceLicenseHeader(content string) string {
	lines := strings.Split(content, "\n")
	idx := 0
	for idx < len(lines) {
		trimmed := strings.TrimSpace(lines[idx])
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			idx++
			continue
		}
		break
	}
	return strings.TrimSpace(LicenseHeader) + "\n\n" + strings.Join(lines[idx:], "\n")
}

func addGoPackageOption(content string) string {
	if strings.Contains(content, "go_package") {
		return content
	}
	re := regexp.MustCompile(`(?m)(^package\s+\w+;)`)
	return re.ReplaceAllString(content, "${1}\n\n"+goPackageOption)
}

// updateImports rewrites local proto imports (those without a path separator)
// to use the _carbide.proto suffix, leaving google/protobuf imports untouched.
func updateImports(content string) string {
	re := regexp.MustCompile(`import "([^"]+)\.proto"`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		if strings.Contains(match, "google/") || strings.Contains(match, "_carbide.proto") {
			return match
		}
		return strings.Replace(match, `.proto"`, `_carbide.proto"`, 1)
	})
}

func normalizeSiteExplorer(content string) string {
	re := regexp.MustCompile(`\bPowerState\b`)
	content = re.ReplaceAllString(content, "ComputerSystemPowerState")

	warning := "// WARNING: This enum conflicts with PowerState in forge_carbide.proto and must be renamed to ComputerSystemPowerState\n"
	content = strings.Replace(content, "enum ComputerSystemPowerState {", warning+"enum ComputerSystemPowerState {", 1)

	return content
}

func normalizeDns(content string) string {
	re := regexp.MustCompile(`\bMetadata\b`)
	content = re.ReplaceAllString(content, "DomainMetadata")

	warning := "// WARNING: This type conflicts with Metadata in forge_carbide.proto and must be renamed to DomainMetadata\n"
	content = strings.Replace(content, "message DomainMetadata {", warning+"message DomainMetadata {", 1)

	return content
}

func normalizeForge(content string) string {
	content = forgeRenameMachineInventory(content)
	content = forgeUpdateInterfaceFunctionType(content)
	content = forgeMoveValidationEnums(content)
	content = forgeRemoveDomainTypes(content)
	content = forgeUpdatePxeDomain(content)
	return content
}

func forgeRenameMachineInventory(content string) string {
	re := regexp.MustCompile(`\bMachineInventory\b`)
	content = re.ReplaceAllString(content, "MachineComponentInventory")

	warning := "// WARNING: This type conflicts with MachineInventory in forge_carbide.proto and must be renamed to MachineComponentInventory\n"
	content = strings.Replace(content, "message MachineComponentInventory {", warning+"message MachineComponentInventory {", 1)

	return content
}

func forgeUpdateInterfaceFunctionType(content string) string {
	warning := "// WARNING: This enum was changed in a non-backwards compatible way in forge_carbide.proto to drop _FUNCTION suffix\n"
	content = strings.Replace(content, "enum InterfaceFunctionType {", warning+"enum InterfaceFunctionType {", 1)
	content = strings.Replace(content, "  PHYSICAL = 0;", "  PHYSICAL_FUNCTION = 0;", 1)
	content = strings.Replace(content, "  VIRTUAL = 1;", "  VIRTUAL_FUNCTION = 1;", 1)
	return content
}

// forgeMoveValidationEnums extracts the three enums nested inside
// MachineValidationStatus and places them at the top level immediately
// before the message so proto3 can compile them.
func forgeMoveValidationEnums(content string) string {
	warning := "// WARNING: Site proto declares these enums inside `MachineValidationStatus`. This is not compilable to protobuf so we move the enums to the top level"

	enumNames := []string{"MachineValidationStarted", "MachineValidationInProgress", "MachineValidationCompleted"}
	var extractedEnums strings.Builder

	for _, name := range enumNames {
		re := regexp.MustCompile(`\n\s*enum\s+` + name + `\s*\{[^}]*\}`)
		match := re.FindString(content)
		if match != "" {
			content = strings.Replace(content, match, "", 1)
			extractedEnums.WriteString(warning + "\n")
			extractedEnums.WriteString(dedent(match) + "\n\n")
		}
	}

	content = strings.Replace(content, "message MachineValidationStatus {", extractedEnums.String()+"message MachineValidationStatus {", 1)

	return content
}

// trimWhitespace removes trailing whitespace from every line and ensures the
// file ends with exactly one newline.
func trimWhitespace(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

// dedent strips the leading/trailing whitespace from s and removes one level
// of 2-space indentation from each line (the nesting from the parent message).
func dedent(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, "  ")
	}
	return strings.Join(lines, "\n")
}

func forgeRemoveDomainTypes(content string) string {
	typesToRemove := []string{"DomainSearchQuery", "DomainDeletionResult", "DomainDeletion", "DomainList", "Domain"}

	for _, typeName := range typesToRemove {
		re := regexp.MustCompile(`(?m)^message ` + typeName + `\s*\{[^}]*\}\n*`)
		content = re.ReplaceAllString(content, "")
	}

	return content
}

func forgeUpdatePxeDomain(content string) string {
	warning := "    // WARNING: Updated to correct legacy type\n"
	content = strings.Replace(content, "    Domain legacy_domain = 2;", warning+"    DomainLegacy legacy_domain = 2;", 1)
	return content
}

func main() {
	workflowsDir := filepath.Join("..", "site-agent", "workflows", "v1")
	carbideProtoFiles := filepath.Join(workflowsDir, "*_carbide.proto")
	protoFiles, err := filepath.Glob(carbideProtoFiles)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to get list of carbide proto files")
	}
	for _, protoFile := range protoFiles {
		normalizeProtoFile(protoFile)
	}
}
