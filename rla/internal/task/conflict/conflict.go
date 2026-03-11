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

// Package conflict provides data-driven task conflict detection for RLA.
//
// The core abstraction is ConflictRule, a declarative struct that defines which
// operation pairs cannot coexist and at what scope (dimension). A single
// EvaluateConflict function interprets the rule — no interface hierarchy needed.
//
// V1 default: exclusive access per rack (any active task blocks a new one).
// Future: per-pair scoping, configurable allowed-pairs, DB-backed rules.
package conflict

import (
	"context"

	"github.com/google/uuid"

	taskstore "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/store"
	taskdef "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/task"
)

// OperationSpec matches an operation by type and code.
// The wildcard value "*" matches any value in that field.
type OperationSpec struct {
	OperationType string // e.g. "power_control", "*"
	OperationCode string // e.g. "power_on", "*"
}

// Matches returns true if this spec matches the given operation.
func (s OperationSpec) Matches(op OperationSpec) bool {
	typeMatch := s.OperationType == "*" || s.OperationType == op.OperationType
	codeMatch := s.OperationCode == "*" || s.OperationCode == op.OperationCode
	return typeMatch && codeMatch
}

// ConflictDimension defines the scope for conflict checking via an extractor
// function. Two tasks are "in scope" only when their extracted key sets share
// at least one element.
type ConflictDimension struct {
	// Name is used for logging and configuration (e.g. "rack").
	Name string
	// ExtractKeys returns the set of scope keys for a task.
	// For DimensionRack this is always a single element (the rack UUID).
	// For DimensionComponentUUID it returns all targeted component UUIDs.
	ExtractKeys func(t *taskdef.Task) []string
}

// Overlaps returns true when tasks a and b share at least one scope key.
func (d *ConflictDimension) Overlaps(a, b *taskdef.Task) bool {
	keysA := d.ExtractKeys(a)
	keysB := d.ExtractKeys(b)
	setB := make(map[string]struct{}, len(keysB))
	for _, k := range keysB {
		setB[k] = struct{}{}
	}
	for _, k := range keysA {
		if _, ok := setB[k]; ok {
			return true
		}
	}
	return false
}

// Built-in dimensions.
var (
	// DimensionRack: tasks conflict when they target the same rack.
	// This is the default and most common scope for conflict detection.
	DimensionRack = &ConflictDimension{
		Name: "rack",
		ExtractKeys: func(t *taskdef.Task) []string {
			return []string{t.RackID.String()}
		},
	}

	// DimensionComponentUUID: tasks conflict when they target at least one
	// common component UUID. Requires ComponentUUIDs to be populated.
	DimensionComponentUUID = &ConflictDimension{
		Name: "component_uuid",
		ExtractKeys: func(t *taskdef.Task) []string {
			keys := make([]string, len(t.ComponentUUIDs))
			for i, id := range t.ComponentUUIDs {
				keys[i] = id.String()
			}
			return keys
		},
	}
)

// ConflictEntry is a symmetric pair of operations that cannot coexist
// when the scope condition is satisfied.
type ConflictEntry struct {
	A OperationSpec
	B OperationSpec
	// Scope overrides the rule-level RequireOverlapOn for this specific pair.
	// If nil, inherits RequireOverlapOn from the parent ConflictRule.
	Scope *ConflictDimension
}

// Matches returns true if (p, q) or (q, p) matches (A, B).
// ConflictEntry is symmetric — the order of A and B does not matter.
func (e ConflictEntry) Matches(p, q OperationSpec) bool {
	return (e.A.Matches(p) && e.B.Matches(q)) ||
		(e.A.Matches(q) && e.B.Matches(p))
}

// ConflictRule declaratively defines when operations conflict.
// Empty ConflictingPairs means exclusive mode: any active task is a conflict.
type ConflictRule struct {
	// RequireOverlapOn is the default dimension for scope checking.
	// Nil defaults to DimensionRack.
	RequireOverlapOn *ConflictDimension

	// ConflictingPairs lists operation pairs that cannot coexist within
	// the scope. Each entry is symmetric — the order of A and B does not
	// matter. Empty = all operations conflict (exclusive mode).
	ConflictingPairs []ConflictEntry

	// AtomicAcrossRacks: when true, conflict checking spans all racks in
	// the same task group. V2 feature — currently always false.
	AtomicAcrossRacks bool
}

// Conflicts returns true if incomingOp conflicts with any of the active tasks
// under this rule.
//
// Exclusive mode (empty ConflictingPairs): any active task is a conflict.
// Pair mode: conflict only when an active task's operation matches one of
// the listed ConflictEntry pairs.
func (r *ConflictRule) Conflicts(
	incomingOp OperationSpec,
	activeTasks []*taskdef.Task,
) bool {
	if len(r.ConflictingPairs) == 0 {
		return len(activeTasks) > 0
	}

	for _, activeTask := range activeTasks {
		activeOp := OperationSpec{
			OperationType: string(activeTask.Operation.Type),
			OperationCode: activeTask.Operation.Code,
		}
		for _, entry := range r.ConflictingPairs {
			if entry.Matches(incomingOp, activeOp) {
				return true
			}
		}
	}
	return false
}

// defaultConflictRule is the V1 default: exclusive access per rack.
// Any active task on the rack blocks a new one.
var defaultConflictRule = &ConflictRule{
	RequireOverlapOn: DimensionRack,
	// ConflictingPairs is nil → exclusive mode
}

// ConflictResolver determines whether an incoming operation conflicts with
// active tasks on a rack. It fetches the applicable rule and evaluates it.
// V1: always applies the hardcoded exclusive default rule.
// V2: will query DB for a rack-specific or global default rule.
type ConflictResolver struct {
	store taskstore.Store
}

// NewConflictResolver creates a new ConflictResolver backed by the given store.
func NewConflictResolver(store taskstore.Store) *ConflictResolver {
	return &ConflictResolver{store: store}
}

// ruleFor returns the conflict rule for the given rack.
// V1 always returns the exclusive default rule.
func (r *ConflictResolver) ruleFor(
	_ context.Context,
	_ uuid.UUID,
) (*ConflictRule, error) {
	return defaultConflictRule, nil
}

// HasConflict returns true if incomingOp would conflict with an existing
// active task on rackID under the applicable conflict rule.
func (r *ConflictResolver) HasConflict(
	ctx context.Context,
	incomingOp OperationSpec,
	rackID uuid.UUID,
) (bool, error) {
	rule, err := r.ruleFor(ctx, rackID)
	if err != nil {
		return false, err
	}

	activeTasks, err := r.store.ListActiveTasksForRack(ctx, rackID)
	if err != nil {
		return false, err
	}

	return rule.Conflicts(incomingOp, activeTasks), nil
}
