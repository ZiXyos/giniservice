package httpservice

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Telemetry interface {
	Shutdown(context.Context) error
}

// HTTPServer represents an HTTP server component.
type HTTPServer struct {
	logger *slog.Logger

	server *http.Server
	engine *gin.Engine

	cfg *Config

	tel Telemetry
	mu  sync.RWMutex
}

type Options func(*HTTPServer) error

// WithLogger inject the logger to the HTTPServer.
func WithLogger(logger *slog.Logger) Options {
	return func(hs *HTTPServer) error {
		hs.logger = logger
		return nil
	}
}

func withEngine(engine *gin.Engine) Options {
	return func(h *HTTPServer) error {
		h.engine = engine
		return nil
	}
}

// WithHTTPServer inject the HTTP Server to the HTTPServer.
func WithHTTPServer(config *Config) Options {
	return func(hs *HTTPServer) error {
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
func (h *HTTPServer) Engine() *gin.Engine {
	return h.engine
}

// NewHTTPServer creates a new HTTPServer component.
func NewHTTPServer(opts ...Options) *HTTPServer {
	hs := &HTTPServer{}

	engine := gin.Default()
	if err := withEngine(engine)(hs); err != nil {
		panic(err)
	}

	for _, opt := range opts {
		if err := opt(hs); err != nil {
			panic(err)
		}
	}

	return hs
}

// Run starts the HTTPServer component.
func (h *HTTPServer) Run(ctx context.Context) error {
	h.logger.Info("starting http server")

	go func() {
		if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.logger.Warn("failed to start http server", "error", err)
		}
	}()

	return nil
}

// Shutdown gracefully Shutdown HTTPServer component.
func (h *HTTPServer) Shutdown(ctx context.Context) error {
	h.logger.Info("shutting down http server")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.server.Shutdown(ctx); err != nil {
		h.logger.WarnContext(ctx, "failed to shutdown http server", "error", err)
		return err
	}

	if err := h.tel.Shutdown(ctx); err != nil {
		h.logger.WarnContext(ctx, "failed to shutdown telemetry", "error", err)
	}

	return nil
}

func (h *HTTPServer) Name() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.cfg.ServiceName
}

func WithTelemetry(ctx context.Context) Options {
	return func(hs *HTTPServer) error {
		if hs.cfg == nil {
			return fmt.Errorf("no configuration provided")
		}

		if !hs.cfg.TelemetryConfig.Enabled {
			return nil
		}

		return hs.initTelemetry(ctx)
	}
}

func (h *HTTPServer) initTelemetry(ctx context.Context) error {
	t, err := newTelemetry(ctx, h.cfg)
	if err != nil {
		return err
	}

	h.tel = t
	h.engine.Use(otelgin.Middleware(h.Name()))
	return nil
}

func (h *HTTPServer) initTracer(ctx context.Context) func(context.Context) error {
	return nil
}
