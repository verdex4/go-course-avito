package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			duration := time.Since(start).Milliseconds()

			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = r.URL.Path
			}

			level := slog.LevelInfo
			if ww.Status() >= 500 {
				level = slog.LevelError
			}

			logger.Log(r.Context(), level, "request handled",
				"method", r.Method,
				"route", routePattern,
				"status", ww.Status(),
				"duration_ms", duration,
			)
		})
	}
}
