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

package main

import (
	"fmt"
	"os"

	carbidecli "github.com/nvidia/bare-metal-manager-rest/cli/pkg"
	"github.com/nvidia/bare-metal-manager-rest/cli/tui"
	"github.com/nvidia/bare-metal-manager-rest/openapi"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app, err := carbidecli.NewApp(openapi.Spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
	app.Commands = append(app.Commands, &cli.Command{
		Name:    "tui",
		Aliases: []string{"i"},
		Usage:   "Start interactive TUI mode with config selector",
		Action: func(c *cli.Context) error {
			return tui.RunTUI(c.String("config"))
		},
	})
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
