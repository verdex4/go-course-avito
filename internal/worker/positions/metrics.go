package positions

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	telemetry "github.com/verdex4/go-course-avito/internal/infra/telemetry"
)

type metricsAdapter struct {
	m     *telemetry.Metrics
	meter metric.Meter
}

func NewMetricsAdapter(m *telemetry.Metrics, meter metric.Meter) poolMetrics {
	return &metricsAdapter{m: m, meter: meter}
}

func (a *metricsAdapter) AddStaleCheck(ctx context.Context, result string) {
	a.m.StaleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (a *metricsAdapter) AddPositionRequest(ctx context.Context, result string) {
	a.m.PositionRequests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (a *metricsAdapter) RecordPositionRequestDuration(ctx context.Context, seconds float64, result string) {
	a.m.PositionRequestsDuration.Record(ctx, seconds, metric.WithAttributes(attribute.String("result", result)))
}

func (a *metricsAdapter) IncWorkersActive(ctx context.Context) {
	a.m.PositionRequestsWorkersActive.Add(ctx, 1)
}

func (a *metricsAdapter) DecWorkersActive(ctx context.Context) {
	a.m.PositionRequestsWorkersActive.Add(ctx, -1)
}

func (a *metricsAdapter) RegisterQueueSizeGauge(queueLen func() int64) {
	_, err := a.meter.Int64ObservableGauge(
		"tripgo.position_requests.queue.size",
		metric.WithUnit("{task}"),
		metric.WithDescription("Current number of tasks in the position requests queue"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(queueLen())
			return nil
		}),
	)
	if err != nil {
		slog.Error("failed to register queue size gauge", "error", err)
	}
}
