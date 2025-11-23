package httpservice

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

type HTTPServer struct {
	logger *slog.Logger

	server *http.Server
	gin    *gin.Engine
}

type Options func(*HTTPServer)

func WithLogger(logger *slog.Logger) Options {
	return func(hs *HTTPServer) {
		hs.logger = logger
	}
}

func NewHTTPServer(opts ...Options) *HTTPServer {
	hs := &HTTPServer{}

	for _, opt := range opts {
		opt(hs)
	}

	return hs
}
