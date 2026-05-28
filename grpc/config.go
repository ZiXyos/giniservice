package grpc

// Config represents the configuration for the gRPC server component.
type Config struct {
	Port int `koanf:"port"`
}
