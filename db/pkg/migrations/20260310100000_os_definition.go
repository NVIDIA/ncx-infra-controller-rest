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

package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		tx, terr := db.BeginTx(ctx, &sql.TxOptions{})
		if terr != nil {
			handlePanic(terr, "failed to begin transaction")
		}

		// Add iPXE definition columns to the operating_system table.
		// These fields are populated for OS definitions synced from carbide-core (type = iPXE).
		// The primary key (id) is shared between carbide-core and carbide-rest: both sides use
		// the same UUID, enforced by the inventory sync workflows.

		_, err := tx.Exec("ALTER TABLE operating_system ADD COLUMN IF NOT EXISTS ipxe_template_id TEXT NULL")
		handleError(tx, err)

		_, err = tx.Exec("ALTER TABLE operating_system ADD COLUMN IF NOT EXISTS ipxe_template_parameters JSONB NULL")
		handleError(tx, err)

		_, err = tx.Exec("ALTER TABLE operating_system ADD COLUMN IF NOT EXISTS ipxe_template_artifacts JSONB NULL")
		handleError(tx, err)

		_, err = tx.Exec("ALTER TABLE operating_system ADD COLUMN IF NOT EXISTS ipxe_template_definition_hash TEXT NULL")
		handleError(tx, err)

		// controller_operating_system_id is no longer needed: the primary key is the same on
		// both sides, so drop the column and its index if they still exist.
		_, err = tx.Exec("DROP INDEX IF EXISTS operating_system_controller_os_id_idx")
		handleError(tx, err)

		_, err = tx.Exec("ALTER TABLE operating_system DROP COLUMN IF EXISTS controller_operating_system_id")
		handleError(tx, err)

		terr = tx.Commit()
		if terr != nil {
			handlePanic(terr, "failed to commit transaction")
		}

		fmt.Print(" [up migration] ")
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] No action taken")
		return nil
	})
}
