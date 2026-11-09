package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// SetHTTPRoute добавляет атрибут http.route в активный otel-спан, используя шаблон chi.
func SetHTTPRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = r.URL.Path
		}

		span := trace.SpanFromContext(r.Context())
		if span.IsRecording() {
			span.SetAttributes(semconv.HTTPRoute(routePattern))
		}

		next.ServeHTTP(w, r)
	})
}
