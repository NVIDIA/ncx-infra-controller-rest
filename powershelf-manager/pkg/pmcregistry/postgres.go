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
package pmcregistry

import (
	"context"
	"fmt"
	"net"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/common/errors"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/converter/dao"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db/migrations"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db/model"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/db/postgres"
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/objects/pmc"

	log "github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
)

// PostgresPmcRegistry implements the PmcRegister interface and it uses PostgresQL
// as the datastore.
type PostgresPmcRegistry struct {
	pg *postgres.Postgres
}

// newPostgresRegistry initializes connectivity to Postgres and runs any pending migrations.
func newPostgresRegistry(ctx context.Context, c db.Config) (*PostgresPmcRegistry, error) {
	pg, err := postgres.New(ctx, c)
	if err != nil {
		return nil, err
	}

	// Run migrations automatically at startup to ensure schema is up to date
	if err := migrations.Migrate(ctx, pg); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	return &PostgresPmcRegistry{pg}, nil
}

// NewPostgresRegistryFromDB creates a PostgresPmcRegistry from an existing database connection.
// This is useful for tests where migrations have already been applied.
func NewPostgresRegistryFromDB(pg *postgres.Postgres) *PostgresPmcRegistry {
	return &PostgresPmcRegistry{pg: pg}
}

// Start starts the PostgresStore instance. Currently, it is no-op.
func (ps *PostgresPmcRegistry) Start(ctx context.Context) error {
	log.Printf("Starting PostgresQL PMC Register")
	return nil
}

// Stop stops the PostgresStore instance by closing the PostgresQL connection.
func (ps *PostgresPmcRegistry) Stop(ctx context.Context) error {
	log.Printf("Stopping PostgresQL PMC Register")
	return ps.pg.Close(ctx)
}

func (ps *PostgresPmcRegistry) runInTx(
	ctx context.Context,
	operation func(ctx context.Context, tx bun.Tx) error,
) error {
	if err := ps.pg.RunInTx(ctx, operation); err != nil {
		if !errors.IsGRPCError(err) {
			err = errors.GRPCErrorInternal(err.Error())
		}

		return err
	}

	return nil
}

// RegisterPmc creates a PMC row, mapping domain → DAO.
func (ps *PostgresPmcRegistry) RegisterPmc(ctx context.Context, pmc *pmc.PMC) error {
	operation := func(ctx context.Context, tx bun.Tx) error {
		pmcDao := dao.PmcTo(pmc)

		if err := pmcDao.Create(ctx, tx); err != nil {
			log.Printf("failed to create bmc entry: %s", pmcDao.MacAddress)
			return errors.GRPCErrorInternal(err.Error())
		}

		return nil
	}

	return ps.runInTx(ctx, operation)
}

// GetPmc queries by MAC and maps DAO → domain.
func (ps *PostgresPmcRegistry) GetPmc(
	ctx context.Context,
	mac net.HardwareAddr,
) (*pmc.PMC, error) {
	pmcModel := &model.PMC{
		MacAddress: model.MacAddr(mac),
	}

	cur, err := pmcModel.Get(ctx, ps.pg.DB())
	if err != nil {
		return nil, err
	}

	return dao.PmcFrom(cur)
}

// IsPmcRegistered returns true if a PMC exists.
func (ps *PostgresPmcRegistry) IsPmcRegistered(ctx context.Context, mac net.HardwareAddr) (bool, error) {
	pmc, err := ps.GetPmc(ctx, mac)
	if err != nil {
		return false, err
	}

	return pmc != nil, nil

}

// GetAllPmcs returns all PMCs mapped to domain objects.
func (ps *PostgresPmcRegistry) GetAllPmcs(ctx context.Context) ([]*pmc.PMC, error) {
	pmcDaos := make([]model.PMC, 0)
	err := ps.pg.DB().NewSelect().Model(&pmcDaos).Scan(ctx)
	if err != nil {
		return nil, errors.GRPCErrorInternal(err.Error())
	}

	pmcs := make([]*pmc.PMC, 0, len(pmcDaos))
	for _, pmcDao := range pmcDaos {
		pmc, err := dao.PmcFrom(&pmcDao)
		if err != nil {
			return nil, errors.GRPCErrorInternal(err.Error())
		}
		pmcs = append(pmcs, pmc)
	}

	return pmcs, nil
}
