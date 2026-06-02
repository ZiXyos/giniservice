package grpc

import "github.com/zixyos/giniservice/telemetry"

// Config represents the configuration for the gRPC server component.
type Config struct {
	Port           int              `koanf:"port"`
	ServiceName    string           `koanf:"service_name"`
	ServiceVersion string           `koanf:"service_version"`
	Telemetry      telemetry.Config `koanf:"telemetry"`
}
