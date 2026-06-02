package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zixyos/giniservice/telemetry"
	serviceloader "github.com/zixyos/goloader/service"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Server represents an HTTP Server component.
type Server struct {
	logger *slog.Logger

	server *http.Server
	engine *gin.Engine

	cfg *Config

	serviceID   serviceloader.UUID
	serviceName string

	mu sync.RWMutex
}

type Options func(*Server) error

// WithLogger inject the logger to the HTTPServer.
func WithLogger(logger *slog.Logger) Options {
	return func(hs *Server) error {
		hs.logger = logger
		return nil
	}
}

func WithServiceName(serviceName string) Options {
	return func(hs *Server) error {
		hs.serviceName = serviceName
		return nil
	}
}

func withEngine(engine *gin.Engine) Options {
	return func(h *Server) error {
		h.engine = engine
		return nil
	}
}

// WithHTTPServer inject the HTTP Server to the HTTPServer.
func WithHTTPServer(config *Config) Options {
	return func(hs *Server) error {
		if hs.engine == nil {
			return fmt.Errorf("engine is nil") //should impl errors const
		}
		if config == nil {
			return fmt.Errorf("config is nil")
		}

		hs.cfg = config

		addr := ":8080"
		if config.HTTPServer.Port != 0 {
			addr = fmt.Sprintf(":%d", config.HTTPServer.Port)
		}
		readTimeout := config.HTTPServer.ReadTimeout
		if readTimeout == 0 {
			readTimeout = 5 * time.Second
		}
		writeTimeout := config.HTTPServer.WriteTimeout
		if writeTimeout == 0 {
			writeTimeout = 10 * time.Second
		}

		hs.server = &http.Server{
			Addr:         addr,
			Handler:      hs.engine,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		}

		return nil
	}
}

// Engine exposes the underlying gin engine so handlers can be registered.
func (h *Server) Engine() *gin.Engine {
	return h.engine
}

// NewHTTPServer creates a new HTTPServer component.
func NewHTTPServer(opts ...Options) (*Server, error) {
	hs := &Server{}

	engine := gin.Default()
	if err := withEngine(engine)(hs); err != nil {
		return nil, err
	}

	for _, opt := range opts {
		if err := opt(hs); err != nil {
			return nil, err
		}
	}

	var telemetryErr error
	if hs.cfg != nil && hs.cfg.Telemetry.Enabled {
		if err := telemetry.Init(context.Background(), hs.cfg.ServiceName, hs.cfg.ServiceVersion, hs.cfg.Telemetry); err != nil {
			logger := hs.logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Error("failed to initialize telemetry for http server", "error", err)
			telemetryErr = err
		} else {
			hs.engine.Use(otelgin.Middleware(hs.cfg.ServiceName))
		}
	}

	if hs.cfg != nil && hs.cfg.Telemetry.Enabled {
		if err := telemetry.Init(context.Background(), hs.cfg.ServiceName, hs.cfg.ServiceVersion, hs.cfg.Telemetry); err != nil {
			panic(err)
		}
		hs.engine.Use(otelgin.Middleware(hs.cfg.ServiceName))
	}

	return hs, telemetryErr
}

// Run starts the HTTPServer component.
func (h *Server) Run(ctx context.Context) error {
	h.logger.Info("starting http server")

	go func() {
		if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.logger.Warn("failed to start http server", "error", err)
		}
	}()

	return nil
}

// Stop gracefully Shutdown HTTPServer component.
func (h *Server) Stop(ctx context.Context) error {
	h.logger.Info("shutting down http server")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.server.Shutdown(ctx); err != nil {
		h.logger.WarnContext(ctx, "failed to shutdown http server", "error", err)
		return err
	}

	if err := telemetry.Shutdown(ctx); err != nil {
		h.logger.WarnContext(ctx, "failed to shutdown telemetry", "error", err)
	}

	return nil
}

// SetServiceID set the service id from the application handler.
func (h *Server) SetServiceID(serviceID serviceloader.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.serviceID = serviceID
}

// Name return the service name.
func (h *Server) Name() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.cfg.ServiceName
}
