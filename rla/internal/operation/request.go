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

package operation

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	taskcommon "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	identifier "github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/Identifier"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/common/devicetypes"
)

// Wrapper wraps the operation type and its serialized information.
type Wrapper struct {
	Type taskcommon.TaskType
	Code string          // Operation code string (e.g., "power_on", "upgrade")
	Info json.RawMessage // Serialized operation details
}

// TargetSpec contains either rack targets or component targets, but not both.
// This enforces single-type targeting at the type level.
type TargetSpec struct {
	Racks      []RackTarget      // Set if targeting racks (mutually exclusive with Components)
	Components []ComponentTarget // Set if targeting components (mutually exclusive with Racks)
}

// IsRackTargeting returns true if this spec targets racks.
func (ts *TargetSpec) IsRackTargeting() bool {
	return len(ts.Racks) > 0
}

// IsComponentTargeting returns true if this spec targets components.
func (ts *TargetSpec) IsComponentTargeting() bool {
	return len(ts.Components) > 0
}

// Validate validates the target specification
func (ts *TargetSpec) Validate() error {
	if ts == nil {
		return fmt.Errorf("target spec is nil")
	}

	if ts.IsRackTargeting() {
		if ts.IsComponentTargeting() {
			return fmt.Errorf("target_spec cannot have both racks and components set")
		}

		for _, rt := range ts.Racks {
			if err := rt.Validate(); err != nil {
				return fmt.Errorf("invalid rack target: %w", err)
			}
		}
	} else {
		if !ts.IsComponentTargeting() {
			return fmt.Errorf("target_spec must have either racks or components set")
		}

		for _, ct := range ts.Components {
			if err := ct.Validate(); err != nil {
				return fmt.Errorf("invalid component target: %w", err)
			}
		}
	}

	return nil
}

// RackTarget identifies a rack with optional component type filtering.
type RackTarget struct {
	Identifier     identifier.Identifier       // Rack identifier (ID or Name, at least one must be set)
	ComponentTypes []devicetypes.ComponentType // Optional: empty = ALL component types in rack
}

func (rt *RackTarget) Validate() error {
	if rt == nil {
		return fmt.Errorf("rack target is nil")
	}

	if !rt.Identifier.ValidateAtLeastOne() {
		return fmt.Errorf("rack target must have either id or name set")
	}

	for _, ctype := range rt.ComponentTypes {
		if ctype == devicetypes.ComponentTypeUnknown {
			return fmt.Errorf("unknown component type")
		}
	}

	return nil
}

// ComponentTarget identifies a specific component.
// Either UUID or External must be set, but not both.
type ComponentTarget struct {
	UUID     uuid.UUID    // RLA internal UUID (one of UUID or External must be set)
	External *ExternalRef // External system reference (one of UUID or External must be set)
}

func (ct *ComponentTarget) Validate() error {
	if ct == nil {
		return fmt.Errorf("component target is nil")
	}

	if ct.UUID != uuid.Nil {
		if ct.External != nil {
			return fmt.Errorf("component target cannot have both uuid and external set")
		}
	} else {
		if err := ct.External.Validate(); err != nil {
			return fmt.Errorf("invalid external ref: %w", err)
		}
	}

	return nil
}

// ExternalRef identifies a component by its external system ID.
// The component type determines which external system to query
// (e.g., Compute -> Carbide, PowerShelf -> PSM)
type ExternalRef struct {
	Type devicetypes.ComponentType // Component type determines the source system
	ID   string                    // ID in that system (e.g., Carbide machine_id, PSM PMC MAC)
}

func (er *ExternalRef) Validate() error {
	if er == nil {
		return fmt.Errorf("external ref is nil")
	}

	if er.Type == devicetypes.ComponentTypeUnknown {
		return fmt.Errorf("external ref must have a valid component type")
	}

	if er.ID == "" {
		return fmt.Errorf("external ref must have an id")
	}

	return nil
}

// Request represents the specification of an operation submitted by the user.
// This is a simple translation of the gRPC input, can contain multiple racks/components.
// Task Manager will resolve and split by rack, creating one Task per rack.
// -- Operation: The operation to be performed.
// -- TargetSpec: Either rack targets or component targets (single-type targeting enforced).
// -- Description: Optional task description.
type Request struct {
	Operation   Wrapper
	TargetSpec  TargetSpec // Either racks or components, not both
	Description string
}

func (r *Request) Validate() error {
	if r == nil {
		return fmt.Errorf("request is nil")
	}

	if !r.Operation.Type.IsValid() {
		return fmt.Errorf("unknown task type")
	}

	if err := r.TargetSpec.Validate(); err != nil {
		return fmt.Errorf("invalid target spec: %w", err)
	}

	return nil
}
