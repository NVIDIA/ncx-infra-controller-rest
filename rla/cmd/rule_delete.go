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
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/client"
)

var ruleDeleteCmd = &cobra.Command{
	Use:   "delete <rule-id>",
	Short: "Delete an operation rule",
	Long:  `Delete an operation rule by ID. This will also remove all rack associations for this rule.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRuleDelete,
}

var (
	ruleDeleteHost string
	ruleDeletePort int
)

func init() {
	ruleCmd.AddCommand(ruleDeleteCmd)

	ruleDeleteCmd.Flags().StringVar(&ruleDeleteHost, "host", "localhost", "RLA service host")
	ruleDeleteCmd.Flags().IntVar(&ruleDeletePort, "port", 50051, "RLA service port")
}

func runRuleDelete(cmd *cobra.Command, args []string) error {
	ruleIDStr := args[0]

	ruleID, err := uuid.Parse(ruleIDStr)
	if err != nil {
		return fmt.Errorf("invalid rule ID: %w", err)
	}

	rlaClient, err := client.New(client.Config{
		Host: ruleDeleteHost,
		Port: ruleDeletePort,
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer rlaClient.Close()

	err = rlaClient.DeleteOperationRule(context.Background(), ruleID)
	if err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	fmt.Printf("Successfully deleted rule %s\n", ruleIDStr)
	return nil
}
