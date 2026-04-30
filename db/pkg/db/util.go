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

package db

import (
	"hash/fnv"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

var queryORRegex = regexp.MustCompile(`.\s\|\s.`)
var queryANDRegex = regexp.MustCompile(`.\s\&\s.`)

// GetStrPtr returns a pointer for the provided string
func GetStrPtr(s string) *string {
	sp := s
	return &sp
}

// GetBoolPtr returns a pointer for the provided bool
func GetBoolPtr(b bool) *bool {
	bp := b
	return &bp
}

// GetUUIDPtr returns a pointer for the provided UUID
func GetUUIDPtr(u uuid.UUID) *uuid.UUID {
	up := u
	return &up
}

// GetIntPtr returns a pointer for the provided int
func GetIntPtr(i int) *int {
	ip := i
	return &ip
}

// GetTimePtr returns a pointer for the provided time
func GetTimePtr(t time.Time) *time.Time {
	tp := t
	return &tp
}

// GetCurTime returns the current time
func GetCurTime() time.Time {
	// Standardize time to match Postgres resolution
	return time.Now().UTC().Round(time.Microsecond)
}

// IsStrInSlice returns true if the provided string is in the provided slice
func IsStrInSlice(s string, sl []string) bool {
	for _, v := range sl {
		if v == s {
			return true
		}
	}
	return false
}

// GetStringToUint64Hash returns a uint64 hash of the input string
// this is used for advisory lock ids
func GetStringToUint64Hash(id string) uint64 {
	h := fnv.New64()
	h.Write([]byte(id))
	return h.Sum64()
}

// GetStringToTsQuery returns a string into a to_tsquery format from the input string
func GetStringToTsQuery(inputQuery string) string {

	if inputQuery == "" {
		return inputQuery
	}

	// make sure it doesn't have already " | " or " & "
	// becuase to_tsquery uses those format to search queries
	// by default we formatting " | " for all search text

	alreadyOr := queryORRegex.MatchString(inputQuery)
	alreadyAnd := queryANDRegex.MatchString(inputQuery)

	// skip if already containts " | " or " & "
	if alreadyOr || alreadyAnd {
		return inputQuery
	}

	tokens := strings.Fields(inputQuery)
	if len(tokens) == 0 {
		return inputQuery
	}

	return strings.Join(tokens, " | ")
}

// CompareStringSlicesIgnoreOrder compares two string slices ignoring order
func CompareStringSlicesIgnoreOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// Create sorted copies to compare
	aCopy := make([]string, len(a))
	bCopy := make([]string, len(b))
	copy(aCopy, a)
	copy(bCopy, b)
	slices.Sort(aCopy)
	slices.Sort(bCopy)
	return slices.Equal(aCopy, bCopy)
}
