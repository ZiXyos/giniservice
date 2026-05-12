package httpservice

import (
	"fmt"
	"os"
	"time"
)

// Config represents the configuration for the HTTPServer component.
type Config struct {
	HTTPServer struct {
		Port         int           `koanf:"port"`
		ReadTimeout  time.Duration `koanf:"read_timeout"`
		WriteTimeout time.Duration `koanf:"write_timeout"`
	} `koanf:"http_server"`
	TelemetryConfig struct {
		Enabled  bool   `koanf:"enabled"`
		Endpoint string `koanf:"endpoint"`
	} `koanf:"telemetry"`
}

func (c Config) ValidateTelemetry() error {
	if !c.TelemetryConfig.Enabled {
		return nil
	}

	if c.TelemetryConfig.Endpoint == "" && os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return fmt.Errorf("telemetry enabled but no endpoint set: OTEL_EXPORTER_OTLP_ENDPOINT is required")
	}

	return nil
}
