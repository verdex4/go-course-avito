package server

import (
	"context"
	"net/http"
	"time"

	api "github.com/verdex4/go-course-avito/internal/generated"
)

// Health — liveness. Не ходит в БД.
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

// Ready — readiness. Проверяет доступность БД.
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := s.pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}

	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}
