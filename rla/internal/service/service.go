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
	"errors"
	"fmt"
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/certs"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/db/migrations"
	inventorymanager "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/inventory/manager"
	inventorystore "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/inventory/store"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/inventorysync"
	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/leakdetection"
	taskmanager "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/manager"
	taskstore "github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/task/store"
	pb "github.com/NVIDIA/ncx-infra-controller-rest/rla/pkg/proto/v1"
)

// Service is the top-level RLA service. It owns the gRPC server, database
// session, inventory manager, and task manager and coordinates their lifecycles.
type Service struct {
	conf             Config
	grpcServer       *grpc.Server
	session          *cdb.Session
	inventoryManager inventorymanager.Manager
	taskStore        taskstore.Store
	taskManager      *taskmanager.Manager
}

// New creates and initialises a Service from the provided Config. It opens the
// database connection, runs pending migrations, and wires up the inventory and
// task managers. The returned service is ready to Start.
func New(ctx context.Context, c Config) (*Service, error) {
	// 1. Create shared PostgreSQL connection
	session, err := cdb.NewSessionFromConfig(ctx, c.DBConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection: %w", err)
	}

	// Run migrations
	if err := migrations.MigrateWithDB(ctx, session.DB); err != nil {
		session.Close()

		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// 2. Create stores (Storage Layer)
	invStore := inventorystore.NewPostgres(session)
	tskStore := taskstore.NewPostgres(session)

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
		return nil, fmt.Errorf("failed to create task manager: %w", err)
	}

	return &Service{
		conf:             c,
		session:          session,
		inventoryManager: invManager,
		taskStore:        tskStore,
		taskManager:      taskManager,
	}, nil
}

// Start starts the inventory manager, task manager, and inventory sync
// goroutine, then begins serving gRPC requests on the configured port.
// It blocks until the gRPC server stops.
func (s *Service) Start(ctx context.Context) error {
	log.Logger = log.With().Caller().Logger()

	// Rule resolver is ready immediately (queries DB for rules)
	log.Info().Msg("Rule resolver ready (will query DB for operation rules)")

	if err := s.inventoryManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start inventory manager: %w", err)
	}

	log.Info().Msg("Inventory manager started")

	if s.taskManager != nil {
		if err := s.taskManager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start task manager: %w", err)
		}

		log.Info().Msg("Task manager started")
	}

	go inventorysync.RunInventory(ctx, &s.conf.DBConf)

	go leakdetection.RunLeakDetection(ctx)

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

// Stop gracefully shuts down the gRPC server, task manager, inventory manager,
// and database session.
func (s *Service) Stop(ctx context.Context) {
	log.Info().Msg("Starting graceful shutdown now...")

	s.grpcServer.GracefulStop()
	if s.taskManager != nil {
		s.taskManager.Stop(ctx)
	}
	s.inventoryManager.Stop(ctx)
	// Rule resolver has no cleanup needed (cache is GC'd automatically)
	if s.session != nil {
		s.session.Close()
	}
}

// certOption resolves the TLS configuration for the gRPC server listener.
// If explicit certificate paths are set in the config they take precedence;
// otherwise CERTDIR / the k8s SPIFFE default is used. Returns an empty
// ServerOption (no TLS) when no certificates are found.
func (s *Service) certOption() grpc.ServerOption {
	tlsConfig, source, err := certs.ResolveServer(s.conf.CertConfig)
	if err != nil {
		if errors.Is(err, certs.ErrNotPresent) {
			log.Info().Msg("Certs not present, using non-mTLS")
			return grpc.EmptyServerOption{}
		}
		log.Fatal().Msg(err.Error())
	}

	log.Info().Msgf("Using certificates from %s", source)
	return grpc.Creds(credentials.NewTLS(tlsConfig))
}
