package oteltracer

import (
	"context"
	"regexp"
	"strings"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type customTracer struct {
	tracer pgx.QueryTracer
}

// Регулярное выражение для извлечения имени таблицы из FROM, INTO, UPDATE или JOIN
var tableRegex = regexp.MustCompile(`(?i)\b(?:FROM|INTO|UPDATE|JOIN)\s+([a-zA-Z0-9_]+)`)

func NewTracer() pgx.QueryTracer {
	return &customTracer{
		tracer: otelpgx.NewTracer(),
	}
}

func (t *customTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.tracer.TraceQueryStart(ctx, conn, data)

	if matches := tableRegex.FindStringSubmatch(data.SQL); len(matches) > 1 {
		tableName := strings.ToLower(matches[1])

		span := trace.SpanFromContext(ctx)
		if span.IsRecording() {
			span.SetAttributes(attribute.String("db.collection.name", tableName))
		}
	}

	return ctx
}

func (t *customTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	t.tracer.TraceQueryEnd(ctx, conn, data)
}
