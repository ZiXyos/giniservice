package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type provider struct {
	tp *sdktrace.TracerProvider
	lp *sdklog.LoggerProvider
}

var (
	initOnce sync.Once
	initErr  error
	inst     *provider

	shutOnce sync.Once
	shutErr  error
)

// Init installs the OTel SDK as the process-wide provider when telemetry is
// enabled. Idempotent: only the first caller's arguments take effect, so
// multiple components (http, grpc, ...) can call it safely.
func Init(ctx context.Context, serviceName, serviceVersion string, cfg Config) error {
	if !cfg.Enabled {
		return nil
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	initOnce.Do(func() {
		inst, initErr = newProvider(ctx, serviceName, serviceVersion, cfg)
	})
	return initErr
}

func newProvider(ctx context.Context, serviceName, serviceVersion string, cfg Config) (*provider, error) {
	var traceOpts []otlptracegrpc.Option
	var logOpts []otlploggrpc.Option
	if cfg.Endpoint != "" {
		traceOpts = append(traceOpts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
		logOpts = append(logOpts, otlploggrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
		logOpts = append(logOpts, otlploggrpc.WithInsecure())
	}

	res, err := sdkresource.New(
		ctx,
		sdkresource.WithFromEnv(),
		sdkresource.WithProcess(),
		sdkresource.WithHost(),
		sdkresource.WithTelemetrySDK(),
		sdkresource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("init resource: %w", err)
	}

	traceExp, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return nil, fmt.Errorf("init otlp trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExp),
	)

	logExp, err := otlploggrpc.New(ctx, logOpts...)
	if err != nil {
		_ = traceExp.Shutdown(ctx)
		return nil, fmt.Errorf("init otlp log exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	otellog.SetLoggerProvider(lp)

	return &provider{tp: tp, lp: lp}, nil
}

// LogHandler returns a slog.Handler bound to the global OTel logger provider.
// Returns nil if Init has not installed a provider yet.
func LogHandler(serviceName string) slog.Handler {
	if inst == nil || inst.lp == nil {
		return nil
	}
	return otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(inst.lp))
}

// Shutdown flushes pending spans and logs. Idempotent: safe to call from each
// component's own shutdown without coordination.
func Shutdown(ctx context.Context) error {
	if inst == nil {
		return nil
	}
	shutOnce.Do(func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if inst.tp != nil {
			if err := inst.tp.Shutdown(ctx); err != nil {
				shutErr = err
			}
		}
		if inst.lp != nil {
			if err := inst.lp.Shutdown(ctx); err != nil && shutErr == nil {
				shutErr = err
			}
		}
	})
	return shutErr
}
