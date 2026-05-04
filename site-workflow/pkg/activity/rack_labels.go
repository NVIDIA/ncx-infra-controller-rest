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

package activity

// Well-known label keys for Expected/Managed Rack metadata.
// These mirror the constants defined in api/pkg/api/model/util/rack_labels.go
// (and Core's api-model crate) so the site-workflow stays aligned with REST
// callers and Core on rack chassis and location labels. Redefined here to
// avoid pulling site-workflow into a dependency on the api package.

const (
	// Chassis identity labels — physically identifies the rack hardware.
	LabelChassisManufacturer = "chassis.manufacturer"
	LabelChassisSerialNumber = "chassis.serial-number"
	LabelChassisModel        = "chassis.model"

	// Physical location labels — identifies where the rack lives.
	LabelLocationRegion     = "location.region"
	LabelLocationDatacenter = "location.datacenter"
	LabelLocationRoom       = "location.room"
	LabelLocationPosition   = "location.position"
)
