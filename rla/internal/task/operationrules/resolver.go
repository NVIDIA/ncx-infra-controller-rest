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

package operationrules

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/task/operations"
)

// RuleStore defines the interface for operation rule persistence.
// This is a subset of the full Store interface focusing on rule operations.
type RuleStore interface {
	GetRuleByOperationAndRack(ctx context.Context, opType common.TaskType, operation string, rackID *uuid.UUID) (*OperationRule, error)
}

// Resolver resolves operation rules following priority hierarchy:
// 1. Database rack association (rack-specific rule via association table)
// 2. Database default rule (is_default=true for operation type + operation)
// 3. Hardcoded fallback
type Resolver struct {
	store RuleStore
}

// NewResolver creates a new rule resolver.
func NewResolver(store RuleStore) *Resolver {
	return &Resolver{
		store: store,
	}
}

// ResolveRule resolves the operation rule for a given operation type, operation, and rack.
// It always returns a rule (never nil) or an error. The resolution follows this priority:
// 1. Database rule (rack-specific or default)
// 2. Hardcoded default rule
func (r *Resolver) ResolveRule(
	ctx context.Context,
	operationType common.TaskType,
	operation string,
	rackID uuid.UUID,
) (*OperationRule, error) {
	if r == nil {
		// If resolver is nil, return hardcoded default
		if rule := getHardcodedDefaultRule(operationType, operation); rule != nil {
			return rule, nil
		}
		return nil, fmt.Errorf("resolver is nil and no hardcoded default found for %s/%s", operationType, operation)
	}

	// Query the database for the rule
	dbRule, err := r.store.GetRuleByOperationAndRack(ctx, operationType, operation, &rackID)
	if err != nil {
		log.Warn().Err(err).
			Str("operation_type", string(operationType)).
			Str("operation", operation).
			Str("rack_id", rackID.String()).
			Msg("Failed to query database for operation rule")
	}

	if dbRule != nil {
		log.Debug().
			Str("operation_type", string(operationType)).
			Str("operation", operation).
			Str("rack_id", rackID.String()).
			Str("rule_name", dbRule.Name).
			Msg("Using database operation rule")
		return dbRule, nil
	}

	// Fall back to hardcoded default rule
	log.Info().
		Str("operation_type", string(operationType)).
		Str("operation", operation).
		Msg("No rule found in database, using hardcoded default")

	hardcoded := getHardcodedDefaultRule(operationType, operation)
	if hardcoded != nil {
		return hardcoded, nil
	}

	// This should never happen since hardcoded defaults cover all operations
	return nil, fmt.Errorf("no rule or hardcoded default found for %s/%s", operationType, operation)
}

// getHardcodedDefaultRule returns a pre-built hardcoded default rule for a specific operation.
// Rules are pre-built in init() in resolver_defaults.go for efficiency.
func getHardcodedDefaultRule(operationType common.TaskType, operation string) *OperationRule {
	// Look up pre-built rule from map
	key := ruleKey(operationType, operation)
	if rule := hardcodedRuleMap[key]; rule != nil {
		return rule
	}

	// No hardcoded rule found - return minimal rule for unsupported operations
	return getMinimalDefaultRule(operationType, operation)
}

// getMinimalDefaultRule returns a minimal rule for unknown operations
func getMinimalDefaultRule(operationType common.TaskType, operation string) *OperationRule {
	return &OperationRule{
		Name:          fmt.Sprintf("Minimal Default Rule for %s.%s", operationType, operation),
		Description:   "Minimal fallback rule",
		OperationType: operationType,
		OperationCode: operation,
		RuleDefinition: RuleDefinition{
			Version: CurrentRuleDefinitionVersion,
			Steps:   []SequenceStep{},
		},
	}
}

// GetSequenceNameForOperation maps operation-specific types to sequence names
func GetSequenceNameForOperation(operationType common.TaskType, operation any) string {
	switch operationType {
	case common.TaskTypePowerControl:
		if powerOp, ok := operation.(operations.PowerOperation); ok {
			return powerOperationToSequenceName(powerOp)
		}
	case common.TaskTypeFirmwareControl:
		if firmwareOp, ok := operation.(operations.FirmwareOperation); ok {
			return firmwareOperationToSequenceName(firmwareOp)
		}
	}
	return "default"
}

// powerOperationToSequenceName converts PowerOperation to sequence name
func powerOperationToSequenceName(op operations.PowerOperation) string {
	switch op {
	case operations.PowerOperationPowerOn:
		return SequencePowerOn
	case operations.PowerOperationForcePowerOn:
		return SequenceForcePowerOn
	case operations.PowerOperationPowerOff:
		return SequencePowerOff
	case operations.PowerOperationForcePowerOff:
		return SequenceForcePowerOff
	case operations.PowerOperationRestart:
		return SequenceRestart
	case operations.PowerOperationForceRestart:
		return SequenceForceRestart
	case operations.PowerOperationWarmReset:
		return SequenceWarmReset
	case operations.PowerOperationColdReset:
		return SequenceColdReset
	default:
		return SequencePowerOn // Default fallback
	}
}

// firmwareOperationToSequenceName converts FirmwareOperation to sequence name
func firmwareOperationToSequenceName(op operations.FirmwareOperation) string {
	switch op {
	case operations.FirmwareOperationUpgrade:
		return SequenceUpgrade
	case operations.FirmwareOperationDowngrade:
		return SequenceDowngrade
	case operations.FirmwareOperationRollback:
		return SequenceRollback
	default:
		return SequenceUpgrade // Default fallback
	}
}
