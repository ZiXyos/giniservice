// Package grpc represents the gRPC server component.
package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/zixyos/httpservice/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

// Telemetry is the shutdown surface the grpc server holds onto so it can flush
// telemetry along with the rest of the lifecycle if the caller chooses.
type Telemetry interface {
	Shutdown(context.Context) error
}

// Server wraps *grpc.Server with logger, config, and lifecycle.
type Server struct {
	logger *slog.Logger
	cfg    *Config

	tel        Telemetry
	serverOpts []grpc.ServerOption

	srv *grpc.Server
	lis net.Listener
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

// WithTelemetry attaches an externally-built provider and registers the
// otelgrpc stats handler on the server. Passing nil is a no-op.
func WithTelemetry(p *telemetry.Provider) Options {
	return func(s *Server) error {
		if p == nil {
			return nil
		}
		s.tel = p
		s.serverOpts = append(s.serverOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
		return nil
	}
}

// WithServerOption is an escape hatch for passing raw grpc.ServerOption values
// (interceptors, TLS creds, max-msg-size, etc.) without us re-exporting them.
func WithServerOption(opts ...grpc.ServerOption) Options {
	return func(s *Server) error {
		s.serverOpts = append(s.serverOpts, opts...)
		return nil
	}
}

// NewServer creates a new Server. The underlying *grpc.Server is built after
// all options are applied so option order does not matter.
func NewServer(opts ...Options) *Server {
	s := &Server{}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			panic(err)
		}
	}

	s.srv = grpc.NewServer(s.serverOpts...)
	return s
}

// Server returns the underlying *grpc.Server so callers can register their
// generated pb services (e.g. pb.RegisterFooServiceServer(s.Server(), handler)).
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

// Shutdown gracefully stops the server and falls back to a hard Stop if the
// context expires before in-flight RPCs finish.
func (s *Server) Shutdown(ctx context.Context) error {
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

	if s.tel != nil {
		if err := s.tel.Shutdown(ctx); err != nil {
			s.logger.WarnContext(ctx, "failed to shutdown telemetry", "error", err)
		}
	}

	return shutdownErr
}
