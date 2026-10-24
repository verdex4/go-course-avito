package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/verdex4/go-course-avito/internal/domain"
	api "github.com/verdex4/go-course-avito/internal/generated"
)

// writeProblem отправляет ошибку в формате RFC 9457 (application/problem+json)
func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	prob := api.Problem{
		Type:     "https://tripgo.example/problems/" + code,
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Instance: &r.URL.Path,
		Code:     code,
	}

	_ = json.NewEncoder(w).Encode(prob)
}

// handleDomainError мапит доменные ошибки на HTTP-коды и коды из контракта
func handleDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case err == nil:
		return
	case err == domain.ErrTripNotFound:
		writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "Trip with given id was not found")
	case err == domain.ErrTripCompleted:
		writeProblem(w, r, http.StatusConflict, "trip_completed", "Trip completed", "Operation is not allowed for a completed trip")
	case err == domain.ErrDriverBusy:
		writeProblem(w, r, http.StatusConflict, "driver_busy", "Driver busy", "Driver already has an active trip")
	default:
		slog.Error("internal server error", "error", err, "path", r.URL.Path)
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
	}
}

// toAPITrip преобразует доменную модель в сгенерированную API-модель
func toAPITrip(d *domain.Trip) *api.Trip {
	return &api.Trip{
		Id:             uuid.UUID(d.ID),
		UserId:         uuid.UUID(d.UserID),
		DriverId:       uuid.UUID(d.DriverID),
		StartPoint:     api.Coordinates{Latitude: d.StartPoint.Latitude, Longitude: d.StartPoint.Longitude},
		EndPoint:       api.Coordinates{Latitude: d.EndPoint.Latitude, Longitude: d.EndPoint.Longitude},
		Price:          d.Price,
		Status:         api.TripStatus(d.Status),
		StartedAt:      d.StartedAt,
		FinishedAt:     d.FinishedAt,
		LastPositionAt: nil,
	}
}
