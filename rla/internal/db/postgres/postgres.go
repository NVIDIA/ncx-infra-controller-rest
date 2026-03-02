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

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db"
)

type Postgres struct {
	dbName       string
	db           *bun.DB
	errorChecker db.ErrorChecker
}

func New(ctx context.Context, c db.Config) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, c.BuildDSN())
	if err != nil {
		return nil, err
	}

	return &Postgres{
		dbName:       c.DBName,
		db:           bun.NewDB(stdlib.OpenDBFromPool(pool), pgdialect.New()),
		errorChecker: &PostgresErrorChecker{},
	}, nil
}

func (p *Postgres) Close(ctx context.Context) error {
	return p.db.Close()
}

func (p *Postgres) BeginTx(ctx context.Context) (bun.Tx, error) {
	return p.db.BeginTx(ctx, &sql.TxOptions{})
}

func (p *Postgres) RunInTx(
	ctx context.Context,
	fn func(ctx context.Context, tx bun.Tx) error,
) error {
	return p.db.RunInTx(ctx, &sql.TxOptions{}, fn)
}

func (p *Postgres) ErrorChecker() db.ErrorChecker {
	return p.errorChecker
}

func (p *Postgres) DB() *bun.DB {
	return p.db
}

type PostgresErrorChecker struct{}

func (checker *PostgresErrorChecker) IsErrNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func (checker *PostgresErrorChecker) IsUniqueConstraintError(err error) bool {
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				return true
			}
		}
	}

	return false
}

func UnitTest(ctx context.Context, t *testing.T, dbConf db.Config) (*Postgres, error) {
	dbInitial, err := pgxpool.New(ctx, dbConf.BuildDSN())
	if err != nil {
		return nil, err
	}

	testDBName := dbConf.DBName + "_unit_test_" + strings.ToLower(t.Name())
	log.Info().Msgf("Creating %v", testDBName)
	if _, err = dbInitial.Exec(ctx, "DROP DATABASE "+testDBName); err != nil {
		// Includes when it did not exist
		if !strings.Contains(err.Error(), "does not exist") {
			return nil, err
		}
	}
	if _, err = dbInitial.Exec(ctx, "CREATE DATABASE "+testDBName); err != nil {
		return nil, err
	}

	dbConfNew := dbConf
	dbConfNew.DBName = testDBName

	db, err := New(ctx, dbConfNew)
	if err != nil {
		log.Fatal().Msgf("failed to connect to DB: %v", err)
	}

	return db, nil
}
