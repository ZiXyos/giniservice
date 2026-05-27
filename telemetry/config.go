package telemetry

import (
	"fmt"
	"os"
)

// Config holds the OTLP transport configuration for the telemetry provider.
// Service identity (name, version) is passed separately to NewProvider as it
// belongs to the resource, not the exporter.
type Config struct {
	Enabled  bool   `koanf:"enabled"`
	Endpoint string `koanf:"endpoint"`
	Insecure bool   `koanf:"insecure"`
}

// Validate returns an error if telemetry is enabled but no endpoint can be
// resolved (neither in config nor in OTEL_EXPORTER_OTLP_ENDPOINT).
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.Endpoint == "" && os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return fmt.Errorf("telemetry enabled but no endpoint set: OTEL_EXPORTER_OTLP_ENDPOINT is required")
	}

	return nil
}
