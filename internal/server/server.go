// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package server serves aperio on runtime-go's HybridServer: every service
// on gRPC, the REST gateway and MCP, from the loaded configuration. It
// follows freebusy's internal/server.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/the-protobuf-project/runtime-go/grpc"
	"github.com/the-protobuf-project/runtime-go/grpc/options"
)

// Server wraps a runtime HybridServer and provides lifecycle helpers
// (Start, Stop, Wait) for the aperio service.
type Server struct {
	// options holds the gRPC/HTTP/MCP configuration for this server.
	options options.Options
	// hybridServer is the running runtime server instance.
	hybridServer *grpc.HybridServer
	// services are the assembled services; freebusy is their engine's
	// connection, closed on Stop.
	services *Services
	freebusy *engine.Client
	// ctx and cancel control the server's lifecycle context.
	ctx    context.Context
	cancel context.CancelFunc
}

// NewServer creates a Server configured with name, version, and optional
// overrides. When no options are provided the defaults from config are used.
// It assembles the services (opening the database and dialing freebusy), so
// it returns an error when that fails. Call Start() to begin serving.
func NewServer(name, version string, opts ...options.Options) (*Server, error) {
	var serverOpts options.Options
	if len(opts) == 0 {
		serverOpts = config.GetGRPCOptions()
	} else {
		serverOpts = opts[0]
	}
	serverOpts.ServiceName = name
	serverOpts.Version = version

	services, fb, err := newServices(nil)
	if err != nil {
		return nil, fmt.Errorf("assemble services: %w", err)
	}
	hybridOpts, err := serviceOptions(services)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		options:      serverOpts,
		hybridServer: grpc.NewHybridServer(serverOpts, hybridOpts...),
		services:     services,
		freebusy:     fb,
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

// serviceOptions builds the hybrid-server options: the services' interceptors,
// then protovalidate request validation (enforced by runtime-go across gRPC,
// the HTTP gateway and every MCP tool call; last, so a caller is
// authenticated before a request is judged), the registrations, and CORS and
// TLS when configured. The TLS pair is checked up front, so a bad path
// returns a clear error instead of the panic grpc.WithCertificates raises.
func serviceOptions(s *Services) ([]grpc.Option, error) {
	cfg := config.Get().Server
	opts := append([]grpc.Option{
		grpc.WithUnaryInterceptors(s.Interceptors()...),
		grpc.WithStreamInterceptors(s.StreamInterceptors()...),
		grpc.WithValidationLast(),
		// Every MCP service together, as one server an agent connects to once.
		// The runtime lifts each MCP request's Authorization header into the
		// incoming metadata the interceptors read, there and on each service's
		// own path (mcp.DefaultHeaderMappings).
		grpc.WithMCPEndpoint(cfg.MCP.Path),
		grpc.WithShutdownTimeout(cfg.ShutdownTimeout()),
	}, s.Options()...)
	if cfg.MCP.Stateless {
		opts = append(opts, grpc.WithStatelessMCP())
	}
	cors, err := corsOption(cfg.CORS)
	if err != nil {
		return nil, err
	}
	if cors != nil {
		opts = append(opts, cors)
	}
	if cfg.TLS.Enabled() {
		if _, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil {
			return nil, fmt.Errorf("load TLS certificate (%s, %s): %w", cfg.TLS.CertFile, cfg.TLS.KeyFile, err)
		}
		opts = append(opts, grpc.WithCertificates(cfg.TLS.CertFile, cfg.TLS.KeyFile))
	}
	return opts, nil
}

// Start begins serving gRPC, HTTP and MCP traffic. Non-blocking.
func (s *Server) Start() error {
	if err := s.hybridServer.Start(); err != nil {
		return fmt.Errorf("failed to start hybrid server: %w", err)
	}
	go s.services.Prime(s.ctx)
	s.services.Start(s.ctx, s.services.Background())
	return nil
}

// Stop gracefully drains active connections, shuts the server down, flushes
// the OpenSearch indexers, and closes the freebusy client and the database
// pool.
func (s *Server) Stop() error {
	s.cancel()
	if err := s.hybridServer.Stop(); err != nil {
		return fmt.Errorf("error stopping hybrid server: %w", err)
	}
	if err := s.hybridServer.Close(); err != nil {
		return fmt.Errorf("error closing hybrid server: %w", err)
	}
	s.services.waitNotify()
	s.services.Stays.WaitNotes()
	return errors.Join(s.services.CloseSearch(context.Background()), s.freebusy.Close(), s.services.Conn.Close())
}

// Wait blocks until a SIGINT or SIGTERM signal is received, or until the
// server's context is cancelled programmatically.
func (s *Server) Wait() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
	case <-s.ctx.Done():
	}
}
