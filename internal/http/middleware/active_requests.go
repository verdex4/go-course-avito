package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func TrackActiveRequests(counter metric.Int64UpDownCounter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = r.URL.Path
			}

			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", routePattern),
			}

			counter.Add(r.Context(), 1, metric.WithAttributes(attrs...))

			defer counter.Add(r.Context(), -1, metric.WithAttributes(attrs...))

			next.ServeHTTP(w, r)
		})
	}
}
