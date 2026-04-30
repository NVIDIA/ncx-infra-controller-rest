/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package common

// QueryOverride provides values that override query params when delegating from
// path-scoped endpoints (e.g. instance/{instanceId}/interface, instance/{instanceId}/nvlink-interface) to more general endpoints.
// When set, error messages in general endpoints will be modulated
type QueryOverride struct {
	InstanceIDs   []string
	ValueFromPath bool
}
