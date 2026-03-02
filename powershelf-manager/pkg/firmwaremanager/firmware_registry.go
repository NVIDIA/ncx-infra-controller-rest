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
package firmwaremanager

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
	"github.com/nvidia/bare-metal-manager-rest/powershelf-manager/pkg/objects/powershelf"

	log "github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
)

type Registry struct {
	pg *postgres.Postgres
}

// newRegistry initializes connectivity to Postgres and runs any pending migrations.
func newRegistry(ctx context.Context, c db.Config) (*Registry, error) {
	pg, err := postgres.New(ctx, c)
	if err != nil {
		return nil, err
	}

	// Run migrations automatically at startup to ensure schema is up to date
	if err := migrations.Migrate(ctx, pg); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	return &Registry{pg}, nil
}

// Start starts the PostgresStore instance. Currently, it is no-op.
func (ps *Registry) Start(ctx context.Context) error {
	log.Printf("Starting PostgresQL FW Register")
	return nil
}

// Stop stops the PostgresStore instance by closing the PostgresQL connection.
func (ps *Registry) Stop(ctx context.Context) error {
	log.Printf("Stopping PostgresQL FW Register")
	return ps.pg.Close(ctx)
}

func (ps *Registry) runInTx(
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

// QueueFwUpdate creates a FirmwareUpdate row, mapping domain → DAO.
func (ps *Registry) QueueFwUpdate(ctx context.Context, fwUpdate *powershelf.FirmwareUpdate) error {
	operation := func(ctx context.Context, tx bun.Tx) error {
		fwUpdateDao, err := dao.FirmwareUpdateTo(fwUpdate)
		if err != nil {
			log.Printf("failed to convert fw update: %v", err)
			return errors.GRPCErrorInternal(err.Error())
		}

		if err := fwUpdateDao.Create(ctx, tx); err != nil {
			log.Printf("failed to create fw update entry: %s", fwUpdateDao.PmcMacAddress.String())
			return errors.GRPCErrorInternal(err.Error())
		}

		return nil
	}

	return ps.runInTx(ctx, operation)
}

func (ps *Registry) GetFwUpdate(
	ctx context.Context,
	mac net.HardwareAddr,
	component powershelf.Component,
) (*powershelf.FirmwareUpdate, error) {
	fwUpdate, err := model.GetFirmwareUpdate(ctx, ps.pg.DB(), mac, component)
	if err != nil {
		return nil, err
	}

	return dao.FirmwareUpdateFrom(fwUpdate), nil
}
