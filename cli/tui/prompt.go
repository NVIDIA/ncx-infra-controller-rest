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
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PromptText displays a label and reads a line of text input.
func PromptText(label string, required bool) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s: ", Bold(label))
		if !scanner.Scan() {
			return "", fmt.Errorf("input cancelled")
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" && required {
			fmt.Println(Red("  (required)"))
			continue
		}
		return text, nil
	}
}

// PromptConfirm displays a y/N confirmation prompt.
func PromptConfirm(label string) (bool, error) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("%s [y/N] ", Bold(label))
	if !scanner.Scan() {
		return false, fmt.Errorf("input cancelled")
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}
