/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package common

import (
	"errors"
)

// Error Type
var (
	// ErrResourceStale Requested update of stale object to DB
	ErrResourceStale = errors.New("requested update of stale object to DB")
)

// Resource Type
var (
	// ResourceTypeVpc is VPC
	ResourceTypeVpc = "VPC"
	// ResourceTypeSubnet is Subnet
	ResourceTypeSubnet = "Subnet"
	// ResourceTypeInstance is Instance
	ResourceTypeInstance = "Instance"
	// ResourceTypeSSHKeyGroup is SSHKeyGroup
	ResourceTypeSSHKeyGroup = "SSHKeyGroup"
	// ResourceTypeInfiniBandPartition is InfiniBandPartition
	ResourceTypeInfiniBandPartition = "InfiniBandPartition"
	// ResourceTypeExpectedMachine is ExpectedMachine
	ResourceTypeExpectedMachine = "ExpectedMachine"
	// ResourceTypeSKU is SKU
	ResourceTypeSKU = "SKU"
	// ResourceTypeDpuExtensionService is DpuExtensionService
	ResourceTypeDpuExtensionService = "DpuExtensionService"
	// ResourceTypeNVLinkLogicalPartition is NVLinkLogicalPartition
	ResourceTypeNVLinkLogicalPartition = "NVLinkLogicalPartition"
)

// OpType is type of operation
type OpType int

const (
	// OpCreate is create operation
	OpCreate OpType = iota
	// OpUpdate is update request operation
	OpUpdate
	// OpDelete is delete operation
	OpDelete
	// No op
	OpNone
)
