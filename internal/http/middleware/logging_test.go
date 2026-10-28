package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		requestPath   string
		registerRoute string
		handlerStatus int
		expectedLevel string
		expectedRoute string
	}{
		{
			name:          "200 OK with route pattern",
			requestPath:   "/api/v1/trips/123e4567-e89b-12d3-a456-426614174000",
			registerRoute: "/api/v1/trips/{tripId}",
			handlerStatus: http.StatusOK,
			expectedLevel: "INFO",
			expectedRoute: "/api/v1/trips/{tripId}",
		},
		{
			name:          "500 Internal Server Error triggers ERROR level",
			requestPath:   "/api/v1/trips",
			registerRoute: "/api/v1/trips",
			handlerStatus: http.StatusInternalServerError,
			expectedLevel: "ERROR",
			expectedRoute: "/api/v1/trips",
		},
		{
			name:          "404 Not Found (fallback to URL path)",
			requestPath:   "/unknown-path",
			registerRoute: "",
			handlerStatus: http.StatusNotFound,
			expectedLevel: "INFO",
			expectedRoute: "/unknown-path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{}))

			r := chi.NewRouter()
			r.Use(Logging(logger))

			if tt.registerRoute != "" {
				r.Get(tt.registerRoute, func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.handlerStatus)
					_, _ = w.Write([]byte("response body"))
				})
			} else {
				r.Get("/non-existing-route", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			}
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			require.Equal(t, tt.handlerStatus, rr.Code)

			var logEntry map[string]any
			err := json.Unmarshal(buf.Bytes(), &logEntry)
			require.NoError(t, err, "Log output should be valid JSON")

			require.Equal(t, tt.expectedLevel, logEntry["level"], "Log level mismatch")
			require.Equal(t, "request handled", logEntry["msg"], "Log message mismatch")
			require.Equal(t, http.MethodGet, logEntry["method"], "HTTP method mismatch")

			require.Equal(t, float64(tt.handlerStatus), logEntry["status"], "Status code mismatch")
			require.Equal(t, tt.expectedRoute, logEntry["route"], "Route pattern mismatch")

			require.Contains(t, logEntry, "duration_ms", "duration_ms should be present")
			require.IsType(t, float64(0), logEntry["duration_ms"], "duration_ms should be a number")
		})
	}
}
