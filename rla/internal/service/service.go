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

package service

import (
	"context"
	"fmt"
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/certs"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db/migrations"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/db/postgres"
	inventorymanager "github.com/nvidia/bare-metal-manager-rest/rla/internal/inventory/manager"
	inventorystore "github.com/nvidia/bare-metal-manager-rest/rla/internal/inventory/store"
	"github.com/nvidia/bare-metal-manager-rest/rla/internal/inventorysync"
	taskmanager "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/manager"
	taskstore "github.com/nvidia/bare-metal-manager-rest/rla/internal/task/store"
	pb "github.com/nvidia/bare-metal-manager-rest/rla/pkg/proto/v1"
)

type Service struct {
	conf             Config
	grpcServer       *grpc.Server
	pg               *postgres.Postgres
	inventoryManager inventorymanager.Manager
	taskStore        taskstore.Store
	taskManager      *taskmanager.Manager
}

func New(ctx context.Context, c Config) (*Service, error) {
	// 1. Create shared PostgreSQL connection
	pg, err := postgres.New(ctx, c.DBConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection: %v", err)
	}

	// Run migrations
	if err := migrations.Migrate(ctx, pg); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	// 2. Create stores (Storage Layer)
	invStore := inventorystore.NewPostgres(pg)
	tskStore := taskstore.NewPostgres(pg)

	// 3. Create InventoryManager (Business Logic Layer)
	invManager := inventorymanager.New(invStore)

	// 4. Create TaskManager (Business Logic Layer)
	// Note: Task manager creates its own rule resolver internally
	taskManager, err := taskmanager.New(
		ctx,
		&taskmanager.Config{
			InventoryStore: invStore,
			TaskStore:      tskStore,
			ExecutorConfig: c.ExecutorConf,
		},
	)
	if err != nil {
		// XXX -- ignore the error for now until we have access to the temporal server
		// in the real deployment.
		log.Error().Err(err).Msg("failed to create task manager (ignore the error for now)")
		taskManager = nil
	}

	return &Service{
		conf:             c,
		pg:               pg,
		inventoryManager: invManager,
		taskStore:        tskStore,
		taskManager:      taskManager,
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	log.Logger = log.With().Caller().Logger()

	// Rule resolver is ready immediately (queries DB for rules)
	log.Info().Msg("Rule resolver ready (will query DB for operation rules)")

	if err := s.inventoryManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start inventory manager: %v", err)
	}

	log.Info().Msg("Inventory manager started")

	if s.taskManager != nil {
		if err := s.taskManager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start task manager: %v", err)
		}

		log.Info().Msg("Task manager started")
	}

	go inventorysync.RunInventory(ctx, &s.conf.DBConf)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", s.conf.Port))
	if err != nil {
		return err
	}

	serverImpl, err := newServerImplementation(
		s.inventoryManager,
		s.taskManager,
		s.taskStore,
		s.conf.CarbideClient,
		s.conf.PSMClient,
	)
	if err != nil {
		return err
	}

	s.grpcServer = grpc.NewServer(s.certOption())

	log.Info().Msg("gRPC server is running")

	// Block the main runtime loop for accepting and processing gRPC requests.
	pb.RegisterRLAServer(s.grpcServer, serverImpl)
	reflection.Register(s.grpcServer)

	if err := s.grpcServer.Serve(lis); err != nil {
		return err
	}

	return nil
}

func (s *Service) Stop(ctx context.Context) {
	log.Info().Msg("Starting graceful shutdown now...")

	s.grpcServer.GracefulStop()
	if s.taskManager != nil {
		s.taskManager.Stop(ctx)
	}
	s.inventoryManager.Stop(ctx)
	// Rule resolver has no cleanup needed (cache is GC'd automatically)
	if s.pg != nil {
		s.pg.Close(ctx)
	}
}

func (s *Service) certOption() grpc.ServerOption {
	tlsConfig, certDir, err := certs.TLSConfig()
	if err != nil {
		if err == certs.ErrNotPresent {
			log.Info().Msg("Certs not present, using non-mTLS")
			return grpc.EmptyServerOption{}
		}
		log.Fatal().Msg(err.Error())
	}
	log.Info().Msgf("Using certificates from %s", certDir)
	return grpc.Creds(credentials.NewTLS(tlsConfig))
}
