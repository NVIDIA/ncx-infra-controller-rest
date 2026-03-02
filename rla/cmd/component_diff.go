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

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/client"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/types"
)

var (
	diffCmd = &cobra.Command{
		Use:   "diff",
		Short: "Compare expected (local DB) vs actual (source system) components",
		Long: `Compare expected components from local database against actual components from source systems.

Each component type queries its own source system (e.g., Carbide for Compute, PSM for PowerShelf).
Currently only supports Compute component type.

Specify exactly ONE of the following options:
  --rack-ids      : Comma-separated list of rack UUIDs
  --rack-names    : Comma-separated list of rack names
  --component-ids : Comma-separated list of component IDs

Component types (required):
  --type compute     : Compute nodes (currently the only supported type)

Output formats:
  --output json      : JSON format (default)
  --output table     : Table format

Examples:
  # Compare compute components from racks by name
  rla component diff --rack-names "rack-1,rack-2" --type compute

  # Compare components from rack by ID
  rla component diff --rack-ids "uuid-1,uuid-2" --type compute

  # Compare by component IDs
  rla component diff --component-ids "machine-1,machine-2" --type compute

  # Output as table
  rla component diff --rack-names "rack-1" --type compute --output table
`,
		Run: func(cmd *cobra.Command, args []string) {
			doDiffComponents()
		},
	}

	diffRackIDs       string
	diffRackNames     string
	diffComponentIDs  string
	diffComponentType string
	diffOutput        string
	diffHost          string
	diffPort          int
)

func init() {
	componentCmd.AddCommand(diffCmd)

	diffCmd.Flags().StringVar(&diffRackIDs, "rack-ids", "", "Comma-separated list of rack UUIDs")
	diffCmd.Flags().StringVar(&diffRackNames, "rack-names", "", "Comma-separated list of rack names")
	diffCmd.Flags().StringVar(&diffComponentIDs, "component-ids", "", "Comma-separated list of component IDs")
	diffCmd.Flags().StringVarP(&diffComponentType, "type", "t", "", "Component type (required): compute")
	diffCmd.Flags().StringVarP(&diffOutput, "output", "o", "json", "Output format: json, table")
	diffCmd.Flags().StringVar(&diffHost, "host", "localhost", "RLA server host")
	diffCmd.Flags().IntVar(&diffPort, "port", 50051, "RLA server port")
}

func doDiffComponents() {
	// Validate input - exactly one of rack-ids, rack-names, or component-ids must be provided
	optionCount := 0
	if diffRackIDs != "" {
		optionCount++
	}
	if diffRackNames != "" {
		optionCount++
	}
	if diffComponentIDs != "" {
		optionCount++
	}

	if optionCount == 0 {
		log.Fatal().Msg("One of --rack-ids, --rack-names, or --component-ids must be specified")
	}
	if optionCount > 1 {
		log.Fatal().Msg("Only one of --rack-ids, --rack-names, or --component-ids can be specified")
	}

	// Component type is required
	if diffComponentType == "" {
		log.Fatal().Msg("--type is required (currently only 'compute' is supported)")
	}

	// Parse and validate component type
	componentType := parseComponentTypeToTypes(diffComponentType)
	if componentType != types.ComponentTypeCompute {
		log.Fatal().Msg("Only 'compute' component type is supported for diff")
	}

	// Create client
	c, err := client.New(client.Config{
		Host: diffHost,
		Port: diffPort,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create client")
	}
	defer c.Close()

	ctx := context.Background()
	var result *client.ValidateComponentsResult

	// Call the appropriate client method based on input
	if diffRackIDs != "" {
		rackIDStrs := strings.Split(diffRackIDs, ",")
		rackIDs := make([]uuid.UUID, 0, len(rackIDStrs))
		for _, idStr := range rackIDStrs {
			id, err := uuid.Parse(strings.TrimSpace(idStr))
			if err != nil {
				log.Fatal().Err(err).Str("id", idStr).Msg("Invalid rack UUID")
			}
			rackIDs = append(rackIDs, id)
		}
		result, err = c.ValidateComponentsByRackIDs(ctx, rackIDs, componentType)
	} else if diffRackNames != "" {
		rackNames := strings.Split(diffRackNames, ",")
		for i := range rackNames {
			rackNames[i] = strings.TrimSpace(rackNames[i])
		}
		result, err = c.ValidateComponentsByRackNames(ctx, rackNames, componentType)
	} else if diffComponentIDs != "" {
		componentIDs := strings.Split(diffComponentIDs, ",")
		for i := range componentIDs {
			componentIDs[i] = strings.TrimSpace(componentIDs[i])
		}
		result, err = c.ValidateComponentsByComponentIDs(ctx, componentIDs, componentType)
	}

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to compare components")
	}

	// Output results
	switch diffOutput {
	case "json":
		outputDiffJSON(result)
	case "table":
		outputDiffTable(result)
	default:
		log.Fatal().Str("format", diffOutput).Msg("Unknown output format")
	}
}

func outputDiffJSON(result *client.ValidateComponentsResult) {
	output := struct {
		TotalDiffs          int                    `json:"total_diffs"`
		OnlyInExpectedCount int                    `json:"only_in_expected_count"`
		OnlyInActualCount   int                    `json:"only_in_actual_count"`
		DriftCount          int                    `json:"drift_count"`
		MatchCount          int                    `json:"match_count"`
		Diffs               []*types.ComponentDiff `json:"diffs"`
	}{
		TotalDiffs:          result.TotalDiffs,
		OnlyInExpectedCount: result.OnlyInExpectedCount,
		OnlyInActualCount:   result.OnlyInActualCount,
		DriftCount:          result.DriftCount,
		MatchCount:          result.MatchCount,
		Diffs:               result.Diffs,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to marshal JSON")
	}
	fmt.Println(string(data))
}

func outputDiffTable(result *client.ValidateComponentsResult) {
	// Summary
	fmt.Println("Summary:")
	fmt.Printf("  Total compared: %d\n", result.TotalDiffs+result.MatchCount)
	fmt.Printf("  - Match: %d\n", result.MatchCount)
	fmt.Printf("  - Only in Expected (missing from source): %d\n", result.OnlyInExpectedCount)
	fmt.Printf("  - Only in Actual (unexpected in source): %d\n", result.OnlyInActualCount)
	fmt.Printf("  - Drift (field differences): %d\n", result.DriftCount)
	fmt.Println()

	if len(result.Diffs) == 0 {
		fmt.Println("No differences found.")
		return
	}

	// Differences table
	fmt.Println("Differences:")
	fmt.Println(strings.Repeat("-", 100))
	fmt.Printf("%-20s %-30s %s\n", "TYPE", "COMPONENT_ID", "DETAILS")
	fmt.Println(strings.Repeat("-", 100))

	for _, diff := range result.Diffs {
		diffType := ""
		details := ""

		switch diff.Type {
		case types.DiffTypeOnlyInExpected:
			diffType = "ONLY_IN_EXPECTED"
			details = "Missing from source system"
		case types.DiffTypeOnlyInActual:
			diffType = "ONLY_IN_ACTUAL"
			details = "Not in local DB"
		case types.DiffTypeDrift:
			diffType = "DRIFT"
			var fieldStrs []string
			for _, fd := range diff.FieldDiffs {
				fieldStrs = append(fieldStrs, fmt.Sprintf("%s: %s → %s",
					fd.FieldName, fd.ExpectedValue, fd.ActualValue))
			}
			details = strings.Join(fieldStrs, ", ")
		}

		fmt.Printf("%-20s %-30s %s\n", diffType, diff.ComponentID, details)
	}
	fmt.Println(strings.Repeat("-", 100))
}
