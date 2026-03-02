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

import (
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/componentmanager"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

var cmRegistry *componentmanager.Registry

// SetComponentManagerRegistry sets the component manager registry to use for activities.
// This must be called before using GetComponentManager.
func SetComponentManagerRegistry(r *componentmanager.Registry) {
	cmRegistry = r
}

// GetComponentManager returns the component manager for the specified type.
// Returns nil if the registry is not set or no manager is registered for the type.
func GetComponentManager(
	typ devicetypes.ComponentType,
) componentmanager.ComponentManager {
	if cmRegistry == nil {
		return nil
	}
	return cmRegistry.GetManager(typ)
}
