/*
 * SPDX-FileCopyrightText: Copyright (c) 2021-2023 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"github.com/NVIDIA/ncx-infra-controller-rest/nvswitch-manager/cmd"
	"github.com/NVIDIA/ncx-infra-controller-rest/nvswitch-manager/pkg/common/log"
)

func main() {
	log.Init()
	cmd.Execute()
}
