/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package model

import "google.golang.org/protobuf/encoding/protojson"

var protoJsonUnmarshalOptions = protojson.UnmarshalOptions{
	AllowPartial:   true,
	DiscardUnknown: true,
}
