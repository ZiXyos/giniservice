package httpservice

import (
	"time"

	"github.com/zixyos/httpservice/telemetry"
)

// Config represents the configuration for the HTTPServer component.
type Config struct {
	ServiceName    string `koanf:"service_name"`
	ServiceVersion string `koanf:"service_version"`
	HTTPServer     struct {
		Port         int           `koanf:"port"`
		ReadTimeout  time.Duration `koanf:"read_timeout"`
		WriteTimeout time.Duration `koanf:"write_timeout"`
	} `koanf:"http_server"`
	Telemetry telemetry.Config `koanf:"telemetry"`
}
