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

package util

import (
	"context"

	"github.com/google/uuid"
	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"

	cwssaws "github.com/nvidia/bare-metal-manager-rest/workflow-schema/schema/site-agent/workflows/v1"
)

// GetNVLinkLogicalPartitionStatus returns the NVLinkLogicalPartition status and message from Controller NVLinkLogicalPartition state
func GetNVLinkLogicalPartitionStatus(controllerNVLinkLogicalPartitionTenantState cwssaws.TenantState) (*string, *string) {
	switch controllerNVLinkLogicalPartitionTenantState {
	case cwssaws.TenantState_PROVISIONING:
		return cdb.GetStrPtr(cdbm.NVLinkLogicalPartitionStatusProvisioning), cdb.GetStrPtr("NVLink Logical Partition is being provisioned on Site")
	case cwssaws.TenantState_CONFIGURING:
		return cdb.GetStrPtr(cdbm.NVLinkLogicalPartitionStatusConfiguring), cdb.GetStrPtr("NVLink Logical Partition is being configured on Site")
	case cwssaws.TenantState_READY:
		return cdb.GetStrPtr(cdbm.NVLinkLogicalPartitionStatusReady), cdb.GetStrPtr("NVLink Logical Partition is ready for use")
	case cwssaws.TenantState_FAILED:
		return cdb.GetStrPtr(cdbm.NVLinkLogicalPartitionStatusError), cdb.GetStrPtr("NVLink Logical Partition is in error state")
	default:
		return nil, nil
	}
}

// UpdateNVLinkLogicalPartitionStatusInDB updates the NVLinkLogicalPartition status in the DB and creates a new StatusDetail
func UpdateNVLinkLogicalPartitionStatusInDB(ctx context.Context, tx *cdb.Tx, dbSession *cdb.Session, nvlinklogicalpartitionID uuid.UUID, status *string, statusMessage *string) (*cdbm.NVLinkLogicalPartition, *cdbm.StatusDetail, error) {
	var updatedNVLinkLogicalPartition *cdbm.NVLinkLogicalPartition
	var err error
	var newSSD *cdbm.StatusDetail
	if status != nil {
		nvlinklogicalpartitionDAO := cdbm.NewNVLinkLogicalPartitionDAO(dbSession)
		updatedNVLinkLogicalPartition, err = nvlinklogicalpartitionDAO.Update(
			ctx,
			tx,
			cdbm.NVLinkLogicalPartitionUpdateInput{
				NVLinkLogicalPartitionID: nvlinklogicalpartitionID,
				Status:                   status,
			},
		)
		if err != nil {
			return updatedNVLinkLogicalPartition, newSSD, err
		}

		statusDetailDAO := cdbm.NewStatusDetailDAO(dbSession)
		newSSD, err = statusDetailDAO.CreateFromParams(ctx, tx, nvlinklogicalpartitionID.String(), *status, statusMessage)
		if err != nil {
			return updatedNVLinkLogicalPartition, newSSD, err
		}
	}
	return updatedNVLinkLogicalPartition, newSSD, err
}
