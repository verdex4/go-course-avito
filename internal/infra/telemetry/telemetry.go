package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type Telemetry struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	Logger         *slog.Logger
	Metrics        *Metrics
}

type Metrics struct {
	TripsCreated  metric.Int64Counter
	TripsFinished metric.Int64Counter
}

func New(ctx context.Context, endpoint, protocol, samplerStr string) (*Telemetry, error) {
	res, err := sdkresource.New(ctx,
		sdkresource.WithFromEnv(),
		sdkresource.WithAttributes(
			semconv.ServiceName("trip-service"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	e := strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(e),
		otlptracehttp.WithURLPath("/v1/traces"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	var sampler sdktrace.Sampler
	if samplerStr == "parentbased_always_on" || samplerStr == "always_on" {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.NeverSample()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(e),
		otlpmetrichttp.WithURLPath("/v1/metrics"),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(10*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	meter := mp.Meter("trip-service")
	tripsCreated, _ := meter.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"), metric.WithDescription("Created trips"))
	TripsFinished, _ := meter.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"), metric.WithDescription("Completed trips"))

	level := slog.LevelInfo
	logger := slog.New(&otelContextHandler{
		Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	})

	return &Telemetry{
		TracerProvider: tp,
		MeterProvider:  mp,
		Logger:         logger,
		Metrics: &Metrics{
			TripsCreated:  tripsCreated,
			TripsFinished: TripsFinished,
		},
	}, nil
}

func (m *Metrics) AddTripCreated(ctx context.Context, n int64) {
	m.TripsCreated.Add(ctx, n)
}

func (m *Metrics) AddTripFinished(ctx context.Context, n int64) {
	m.TripsFinished.Add(ctx, n)
}

type otelContextHandler struct {
	slog.Handler
}

func (h *otelContextHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		sc := span.SpanContext()
		if sc.HasTraceID() {
			r.AddAttrs(slog.String("trace_id", sc.TraceID().String()))
		}
		if sc.HasSpanID() {
			r.AddAttrs(slog.String("span_id", sc.SpanID().String()))
		}
	}
	r.AddAttrs(slog.String("service", "trip-service"))

	return h.Handler.Handle(ctx, r)
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	if err := t.TracerProvider.Shutdown(ctx); err != nil {
		return err
	}
	if err := t.MeterProvider.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
