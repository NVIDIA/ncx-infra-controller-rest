/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package managertypes

import (
	bootstraptypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/bootstrap"
	carbidetypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/carbide"
	rlatypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/rla"
	workflowtypes "github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/datatypes/managertypes/workflow"
)

// Managers - manager ds
type Managers struct {
	Version string
	// All the datastructures of Managers below
	Workflow  *workflowtypes.Workflow
	Carbide   *carbidetypes.Carbide
	RLA       *rlatypes.RLA
	Bootstrap *bootstraptypes.Bootstrap
}

// NewManagerType - get new type of all managers
func NewManagerType() *Managers {
	return &Managers{
		Version: "0.0.1",
		// All the managers below
		Workflow:  workflowtypes.NewWorkflowInstance(),
		Carbide:   carbidetypes.NewCarbideInstance(),
		RLA:       rlatypes.NewRLAInstance(),
		Bootstrap: bootstraptypes.NewBootstrapInstance(),
	}
}
