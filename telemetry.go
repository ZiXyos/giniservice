package httpservice

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type telemetry struct {
	exporter *otlptrace.Exporter
	tp       *sdktrace.TracerProvider

	once sync.Once
	err  error
}

func newTelemetry(ctx context.Context, cfg *Config) (*telemetry, error) {
	tel := &telemetry{}

	var opts []otlptracegrpc.Option
	if ep := cfg.TelemetryConfig.Endpoint; ep != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(ep))
		if cfg.TelemetryConfig.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
	}

	res, err := sdkresource.New(
		ctx,
		sdkresource.WithFromEnv(),
		sdkresource.WithProcess(),
		sdkresource.WithHost(),
		sdkresource.WithTelemetrySDK(),
		sdkresource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize resource: %w", err)
	}

	exp, err := otlptracegrpc.New(ctx, opts...)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exp),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	tel.exporter = exp
	tel.tp = tp

	return tel, nil
}

func (t *telemetry) Shutdown(ctx context.Context) error {
	if t == nil || t.tp == nil {
		return nil
	}

	t.once.Do(func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		t.err = t.tp.Shutdown(ctx)
	})

	return t.err
}
