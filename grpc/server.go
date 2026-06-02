// Package grpc represents the gRPC server component.
package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/zixyos/giniservice/telemetry"
	serviceloader "github.com/zixyos/goloader/service"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

// Server wraps *grpc.Server with logger, config, and lifecycle.
type Server struct {
	logger *slog.Logger
	cfg    *Config

	serverOpts []grpc.ServerOption

	serviceID   serviceloader.UUID
	serviceName string

	srv *grpc.Server
	lis net.Listener

	mu sync.RWMutex
}

// Options is the functional-option type for the Server.
type Options func(*Server) error

// WithLogger injects the logger into the Server.
func WithLogger(logger *slog.Logger) Options {
	return func(s *Server) error {
		s.logger = logger
		return nil
	}
}

// WithConfig injects the Config into the Server.
func WithConfig(cfg *Config) Options {
	return func(s *Server) error {
		if cfg == nil {
			return fmt.Errorf("config is nil")
		}
		s.cfg = cfg
		return nil
	}
}

// WithServerOption is an escape hatch for passing raw grpc.ServerOption values.
func WithServerOption(opts ...grpc.ServerOption) Options {
	return func(s *Server) error {
		s.serverOpts = append(s.serverOpts, opts...)
		return nil
	}
}

// WithServiceName inject service name.
func WithServiceName(serviceName string) Options {
	return func(s *Server) error {
		s.serviceName = serviceName
		return nil
	}
}

// NewServer creates a new Server.
func NewServer(opts ...Options) (*Server, error) {
	s := &Server{}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	var telemetryErr error
	if s.cfg != nil && s.cfg.Telemetry.Enabled {
		if err := telemetry.Init(context.Background(), s.cfg.ServiceName, s.cfg.ServiceVersion, s.cfg.Telemetry); err != nil {
			logger := s.logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Error("failed to initialize telemetry for grpc server", "error", err)
			telemetryErr = err
		} else {
			s.serverOpts = append(s.serverOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
		}
	}

	if s.cfg != nil && s.cfg.Telemetry.Enabled {
		if err := telemetry.Init(context.Background(), s.cfg.ServiceName, s.cfg.ServiceVersion, s.cfg.Telemetry); err != nil {
			panic(err)
		}
		s.serverOpts = append(s.serverOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}

	s.srv = grpc.NewServer(s.serverOpts...)
	return s, telemetryErr
}

// Server returns the underlying *grpc.Server so callers can register their pb.
func (s *Server) Server() *grpc.Server {
	return s.srv
}

// Run binds the TCP listener and starts serving in a background goroutine.
func (s *Server) Run(ctx context.Context) error {
	if s.cfg == nil {
		return fmt.Errorf("no configuration provided")
	}

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	s.lis = lis

	s.logger.InfoContext(ctx, "starting grpc server", "addr", addr)

	go func() {
		if err := s.srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			s.logger.WarnContext(ctx, "grpc serve stopped", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the server and falls back to a hard Stop if the ctx T/O.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.InfoContext(ctx, "shutting down grpc server")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.srv.GracefulStop()
		close(done)
	}()

	var shutdownErr error
	select {
	case <-done:
		shutdownErr = nil
	case <-ctx.Done():
		s.srv.Stop()
		shutdownErr = ctx.Err()
	}

	telCtx, telCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer telCancel()
	if err := telemetry.Shutdown(telCtx); err != nil {
		s.logger.WarnContext(telCtx, "failed to shutdown telemetry", "error", err)
	}

	return shutdownErr
}

// Name return the service name.
func (s *Server) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.serviceName
}

// SetServiceID set the service ID from the application handler.
func (s *Server) SetServiceID(serviceID serviceloader.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.serviceID = serviceID
}
