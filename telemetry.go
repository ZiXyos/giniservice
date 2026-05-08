package httpservice

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)


type telemtry struct {
	exporter *otlptrace.Exporter
}

type telemetryOptions func(*telemtry) error

func (s *HTTPServer) WithTelemetry(ctx context.Context, opts ...telemetryOptions) Options {
	return func(hs *HTTPServer) error {
		exporter, err := otlptracegrpc.New(ctx) // need to map opts to otlp_opts
		if err != nil {
			s.logger.WarnContext(ctx, "failed to create new exporter", "error", err.Error())
			panic(err)
		}
		hs.tel.exporter = exporter

		return nil
	}
}

func (s *HTTPServer) initTracer(ctx context.Context) func(context.Context) error {

	return s.tel.exporter.Shutdown
}
