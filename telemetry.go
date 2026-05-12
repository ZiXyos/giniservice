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

func (h *HTTPServer) initTelemetry(ctx context.Context) error {
	var opts []otlptracegrpc.Option
	if ep := h.cfg.TelemetryConfig.Endpoint; ep != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(ep), otlptracegrpc.WithInsecure())
	}

	exp, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return err
	}

	h.tel.exporter = exp
	return nil
}
func (h *HTTPServer) initTracer(ctx context.Context) func(context.Context) error {
	return nil
}
