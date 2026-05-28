package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	http2 "github.com/zixyos/httpservice/http"
	"github.com/zixyos/httpservice/telemetry"
)

func main() {
	cfg := &http2.Config{
		ServiceName: "giniservice-demo",
	}
	cfg.HTTPServer.Port = 8081
	cfg.HTTPServer.ReadTimeout = 5 * time.Second
	cfg.HTTPServer.WriteTimeout = 10 * time.Second
	cfg.Telemetry.Enabled = true
	cfg.Telemetry.Insecure = true
	cfg.Telemetry.Endpoint = "localhost:4317"

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Telemetry must exist before the logger so its slog handler can be wired in.
	tel, err := telemetry.NewProvider(ctx, cfg.ServiceName, cfg.ServiceVersion, cfg.Telemetry)
	if err != nil {
		slog.Error("telemetry init failed", "error", err)
		os.Exit(1)
	}

	logger := slog.New(multi(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		tel.LogHandler(cfg.ServiceName),
	))

	hs := http2.NewHTTPServer(
		http2.WithLogger(logger),
		http2.WithHTTPServer(cfg),
		http2.WithTelemetry(tel),
	)

	hs.Engine().GET("/ping", func(c *gin.Context) {
		logger.InfoContext(c.Request.Context(), "ping handled", "response", "pong")
		c.JSON(http.StatusOK, gin.H{"msg": "pong"})
	})

	hs.Engine().GET("/hello/:name", func(c *gin.Context) {
		name := c.Param("name")
		logger.InfoContext(c.Request.Context(), "name processed", "name", name)

		c.JSON(http.StatusOK, gin.H{"hello": name})
	})

	hs.Engine().GET("/boom", func(c *gin.Context) {
		logger.ErrorContext(c.Request.Context(), "boom failed handled", "error", errors.New("boom"))
		c.JSON(http.StatusInternalServerError, gin.H{"err": "boom"})
	})

	if err := hs.Run(ctx); err != nil {
		logger.Error("run failed", "error", err)
		os.Exit(1)
	}

	<-ctx.Done()
	logger.Info("signal received, shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := hs.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
	if err := tel.Shutdown(shutdownCtx); err != nil {
		logger.Error("telemetry shutdown failed", "error", err)
	}
}

// multiHandler fans an slog record out to every non-nil sub-handler.
type multiHandler []slog.Handler

func multi(handlers ...slog.Handler) slog.Handler {
	out := make(multiHandler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			out = append(out, h)
		}
	}
	return out
}

func (h multiHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	for _, sub := range h {
		if sub.Enabled(ctx, lvl) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, sub := range h {
		if !sub.Enabled(ctx, r.Level) {
			continue
		}
		if err := sub.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(multiHandler, len(h))
	for i, sub := range h {
		out[i] = sub.WithAttrs(attrs)
	}
	return out
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	out := make(multiHandler, len(h))
	for i, sub := range h {
		out[i] = sub.WithGroup(name)
	}
	return out
}
