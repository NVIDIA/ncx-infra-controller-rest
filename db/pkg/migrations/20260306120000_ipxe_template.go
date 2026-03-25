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

	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		tx, terr := db.BeginTx(ctx, &sql.TxOptions{})
		if terr != nil {
			handlePanic(terr, "failed to begin transaction")
		}

		// create ipxe_template table
		_, err := tx.NewCreateTable().Model((*model.IpxeTemplate)(nil)).IfNotExists().Exec(ctx)
		handleError(tx, err)

		// template column
		_, err = tx.Exec("ALTER TABLE ipxe_template ADD COLUMN IF NOT EXISTS template TEXT NOT NULL DEFAULT ''")
		handleError(tx, err)

		// unique constraint to prevent duplicate templates per site
		_, err = tx.Exec("ALTER TABLE ipxe_template DROP CONSTRAINT IF EXISTS ipxe_template_site_id_name_key")
		handleError(tx, err)
		_, err = tx.Exec("ALTER TABLE ipxe_template ADD CONSTRAINT ipxe_template_site_id_name_key UNIQUE (site_id, name)")
		handleError(tx, err)

		// index for site_id
		_, err = tx.Exec("DROP INDEX IF EXISTS ipxe_template_site_id_idx")
		handleError(tx, err)
		_, err = tx.Exec("CREATE INDEX ipxe_template_site_id_idx ON ipxe_template(site_id)")
		handleError(tx, err)

		// index for name
		_, err = tx.Exec("DROP INDEX IF EXISTS ipxe_template_name_idx")
		handleError(tx, err)
		_, err = tx.Exec("CREATE INDEX ipxe_template_name_idx ON ipxe_template(name)")
		handleError(tx, err)

		// index for scope
		_, err = tx.Exec("DROP INDEX IF EXISTS ipxe_template_scope_idx")
		handleError(tx, err)
		_, err = tx.Exec("CREATE INDEX ipxe_template_scope_idx ON ipxe_template(scope)")
		handleError(tx, err)

		// index for created
		_, err = tx.Exec("DROP INDEX IF EXISTS ipxe_template_created_idx")
		handleError(tx, err)
		_, err = tx.Exec("CREATE INDEX ipxe_template_created_idx ON ipxe_template(created)")
		handleError(tx, err)

		// index for updated
		_, err = tx.Exec("DROP INDEX IF EXISTS ipxe_template_updated_idx")
		handleError(tx, err)
		_, err = tx.Exec("CREATE INDEX ipxe_template_updated_idx ON ipxe_template(updated)")
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
