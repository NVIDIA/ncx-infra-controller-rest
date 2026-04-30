/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"fmt"
	"os"

	carbidecli "github.com/NVIDIA/ncx-infra-controller-rest/cli/pkg"
	"github.com/NVIDIA/ncx-infra-controller-rest/cli/tui"
	"github.com/NVIDIA/ncx-infra-controller-rest/openapi"
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
