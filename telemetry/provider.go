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
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// Provider owns the OTel SDK state and is the single point of shutdown.
type Provider struct {
	exporter *otlptrace.Exporter
	tp       *sdktrace.TracerProvider
	lp       *sdklog.LoggerProvider

	once sync.Once
	err  error
}

// NewProvider builds the OTel SDK and installs it as the global provider.
func NewProvider(ctx context.Context, serviceName, serviceVersion string, cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

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

	return &Provider{exporter: traceExp, tp: tp, lp: lp}, nil
}

func (p *Provider) LogHandler(serviceName string) slog.Handler {
	if p == nil || p.lp == nil {
		return nil
	}
	return otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(p.lp))
}

// Shutdown flushes pending spans and logs; safe to call more than once.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}

	p.once.Do(func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if p.tp != nil {
			if err := p.tp.Shutdown(ctx); err != nil {
				p.err = err
			}
		}
		if p.lp != nil {
			if err := p.lp.Shutdown(ctx); err != nil && p.err == nil {
				p.err = err
			}
		}
	})

	return p.err
}
