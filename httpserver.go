package httpservice

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HTTPServerConfig represents the configuration for the HTTPServer component.
type HTTPServerConfig struct {}

// HTTPServer represents an HTTP server component.
type HTTPServer struct {
	logger *slog.Logger

	server *http.Server
	gin    *gin.Engine
}

type Options func(*HTTPServer) error

// WithLogger inject the logger to the HTTPServer.
func WithLogger(logger *slog.Logger) Options {
	return func(hs *HTTPServer) {
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
func WithHTTPServer(config *HTTPServerConfig) Options {
	return func(hs *HTTPServer) {
		if hs.engine == nil {
			return fmt.Errorf("engine is nil") //should impl errors const
		}

		hs.server = &http.Server{ //load from config or default val
			Addr:    ":8080",
			Handler: hs.engine,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
	}
}

// NewHTTPServer creates a new HTTPServer component.
func NewHTTPServer(opts ...Options) *HTTPServer {
	hs := &HTTPServer{}

	engine := gin.Default()
	withEngine(engine)(hs)

	for _, opt := range opts {
		opt(hs)
	}

	return hs
}

// Run starts the HTTPServer component.
func (h *HTTPServer) Run(ctx context.Context) error {
	h.logger.Info("starting http server")

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
		return h.logger.Warn("failed to shutdown http server", "error", err)
	}

	return nil
}
