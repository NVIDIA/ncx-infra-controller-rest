/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: LicenseRef-NvidiaProprietary
 *
 * NVIDIA CORPORATION, its affiliates and licensors retain all intellectual
 * property and proprietary rights in and to this material, related
 * documentation and any modifications thereto. Any use, reproduction,
 * disclosure or distribution of this material and related documentation
 * without an express license agreement from NVIDIA CORPORATION or
 * its affiliates is strictly prohibited.
 */
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/nvidia/bare-metal-manager-rest/nvswitch-manager/internal/certs"
	pb "github.com/nvidia/bare-metal-manager-rest/nvswitch-manager/internal/proto/v1"
	"github.com/nvidia/bare-metal-manager-rest/nvswitch-manager/pkg/db/postgres"
	"github.com/nvidia/bare-metal-manager-rest/nvswitch-manager/pkg/firmwaremanager"
	"github.com/nvidia/bare-metal-manager-rest/nvswitch-manager/pkg/nvswitchmanager"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Service owns the gRPC server lifecycle and an NVSwitchManager orchestrator.
type Service struct {
	conf       Config
	grpcServer *grpc.Server
	db         *bun.DB
	nsm        *nvswitchmanager.NVSwitchManager
	fwm        *firmwaremanager.FirmwareManager
}

// New initializes an NVSwitchManager and constructs a Service from the Config.
func New(ctx context.Context, c Config) (*Service, error) {
	// Connect to database first if configured (needed for persistent registry)
	var db *bun.DB
	if c.DBConf.Host != "" {
		pg, err := postgres.New(ctx, c.DBConf)
		if err != nil {
			if c.DataStoreType == nvswitchmanager.DatastoreTypePersistent {
				return nil, fmt.Errorf("database connection required for persistent mode: %w", err)
			}
			log.Warnf("Failed to connect to database: %v (firmware manager will be disabled)", err)
		} else {
			db = pg.DB()
			log.Info("Connected to database")
		}
	}

	nsmConfig, err := c.ToNsmConf()
	if err != nil {
		return nil, err
	}

	// Set the DB connection for persistent registry
	nsmConfig.DB = db

	nsm, err := nvswitchmanager.New(ctx, *nsmConfig)
	if err != nil {
		return nil, err
	}

	return &Service{
		conf: c,
		db:   db,
		nsm:  nsm,
		// fwm is initialized in Start()
	}, nil
}

// Start begins the NVSwitchManager, binds the gRPC server on the configured port, and serves until the listener is closed.
func (s *Service) Start(ctx context.Context) error {
	err := s.nsm.Start(ctx)
	if err != nil {
		return err
	}

	// Initialize FirmwareManager if firmware config is present
	if s.conf.FirmwareConf.PackagesDir != "" {
		fwmConfig := s.conf.FirmwareConf.ToFirmwareManagerConfig()

		// Use PostgreSQL store if in persistent mode with database, otherwise use in-memory store
		var store firmwaremanager.UpdateStore
		if s.conf.DataStoreType == nvswitchmanager.DatastoreTypePersistent && s.db != nil {
			store = firmwaremanager.NewPostgresUpdateStore(s.db)
			log.Info("FirmwareManager using PostgreSQL store")
		} else {
			store = firmwaremanager.NewInMemoryUpdateStore()
			log.Info("FirmwareManager using in-memory store (updates will not persist across restarts)")
		}

		fwm, err := firmwaremanager.New(fwmConfig, store, s.nsm)
		if err != nil {
			log.Warnf("Failed to initialize FirmwareManager: %v", err)
		} else {
			s.fwm = fwm
			if err := s.fwm.Start(ctx); err != nil {
				log.Warnf("Failed to start FirmwareManager: %v", err)
				s.fwm = nil
			} else {
				log.Info("FirmwareManager initialized and started")
			}
		}
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", s.conf.Port))
	if err != nil {
		return err
	}

	serverImpl, err := newServerImplementation(s.nsm, s.fwm)
	if err != nil {
		return err
	}

	s.grpcServer = grpc.NewServer(
		s.certOption(),
		grpc.ChainUnaryInterceptor(
			loggingUnaryInterceptor(),
		),
	)

	log.Info("gRPC server starting with request/response logging enabled")

	// Block the main runtime loop for accepting and processing gRPC requests.
	pb.RegisterNVSwitchManagerServer(s.grpcServer, serverImpl)
	reflection.Register(s.grpcServer)

	if err := s.grpcServer.Serve(lis); err != nil {
		return err
	}

	return nil
}

// Stop gracefully shuts down the gRPC server and stops the NVSwitchManager.
func (s *Service) Stop(ctx context.Context) {
	log.Printf("Starting graceful shutdown now...")

	s.grpcServer.GracefulStop()

	// Stop FirmwareManager first (waits for active jobs to complete)
	if s.fwm != nil {
		s.fwm.Stop()
	}

	s.nsm.Stop(ctx)
}

// certOption returns the gRPC server option for TLS/mTLS if certificates are present.
// Falls back to plaintext if certificates are not found.
func (s *Service) certOption() grpc.ServerOption {
	tlsConfig, certDir, err := certs.TLSConfig()
	if err != nil {
		if err == certs.ErrNotPresent {
			log.Printf("Certs not present, using non-mTLS (plaintext)")
			return grpc.EmptyServerOption{}
		}
		log.Fatalf("Failed to load TLS certificates: %v", err)
	}
	log.Printf("Using certificates from %s (mTLS enabled)", certDir)
	return grpc.Creds(credentials.NewTLS(tlsConfig))
}

// loggingUnaryInterceptor returns a gRPC unary interceptor that logs request and response payloads.
func loggingUnaryInterceptor() grpc.UnaryServerInterceptor {
	jsonOpts := protojson.MarshalOptions{
		EmitUnpopulated: false,
		Indent:          "",
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		// Log request
		reqLog := log.WithField("grpc.method", info.FullMethod)

		if msg, ok := req.(proto.Message); ok {
			if jsonBytes, err := jsonOpts.Marshal(msg); err == nil {
				reqLog = reqLog.WithField("grpc.request", json.RawMessage(jsonBytes))
			}
		}
		reqLog.Info("gRPC request")

		// Call handler
		resp, err := handler(ctx, req)

		// Log response
		duration := time.Since(start)
		respLog := log.WithFields(log.Fields{
			"grpc.method":   info.FullMethod,
			"grpc.duration": duration.String(),
		})

		if err != nil {
			st, _ := status.FromError(err)
			respLog = respLog.WithFields(log.Fields{
				"grpc.code":  st.Code().String(),
				"grpc.error": st.Message(),
			})
			respLog.Error("gRPC response")
		} else {
			respLog = respLog.WithField("grpc.code", "OK")
			if msg, ok := resp.(proto.Message); ok {
				if jsonBytes, err := jsonOpts.Marshal(msg); err == nil {
					respLog = respLog.WithField("grpc.response", json.RawMessage(jsonBytes))
				}
			}
			respLog.Info("gRPC response")
		}

		return resp, err
	}
}
