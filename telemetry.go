package httpservice

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type telemetry struct {
	exporter *otlptrace.Exporter
	tp       *sdktrace.TracerProvider

	once sync.Once
	err  error
}

func WithTelemetry(ctx context.Context) Options {
	return func(hs *HTTPServer) error {
		if hs.cfg == nil || !hs.cfg.TelemetryConfig.Enabled {
			return nil
		}
		if hs.engine == nil {
			return fmt.Errorf("telemetry: engine must be initialized before WithTelemetry")
		}
		hs.tel = &telemetry{}
		return hs.initTelemetry(ctx)
	}
}

func (h *HTTPServer) initTelemetry(ctx context.Context) error {
	var opts []otlptracegrpc.Option
	if ep := h.cfg.TelemetryConfig.Endpoint; ep != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(ep), otlptracegrpc.WithInsecure())
	}

	exp, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return err
	}

	res, err := sdkresource.New(
		ctx,
		sdkresource.WithFromEnv(),
		sdkresource.WithProcess(),
		sdkresource.WithHost(),
		sdkresource.WithTelemetrySDK(),
	)
	if err != nil {
		return fmt.Errorf("unable to initialize resource: %w", err)
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)

	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	h.tel.tp = traceProvider
	h.tel.exporter = exp
	h.engine.Use(otelgin.Middleware(h.Name()))
	return nil
}

func (h *HTTPServer) initTracer(ctx context.Context) func(context.Context) error {
	return nil
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
