package domainerror

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/verdex4/go-course-avito/internal/domain/trip"
	api "github.com/verdex4/go-course-avito/internal/generated"
)

// Problem преобразует доменную ошибку в HTTP-статус и тело Problem
func Problem(err error) (status int, body api.Problem) {
	switch {
	case errors.Is(err, trip.ErrTripNotFound):
		return http.StatusNotFound, api.Problem{
			Type:   "https://tripgo.example/problems/trip-not-found",
			Title:  "Trip not found",
			Status: http.StatusNotFound,
			Detail: new("Trip was not found"),
			Code:   "trip_not_found",
		}

	case errors.Is(err, trip.ErrTripCompleted):
		return http.StatusConflict, api.Problem{
			Type:   "https://tripgo.example/problems/trip-completed",
			Title:  "Trip completed",
			Status: http.StatusConflict,
			Detail: new("Operation is not allowed for a completed trip"),
			Code:   "trip_completed",
		}

	case errors.Is(err, trip.ErrDriverBusy):
		return http.StatusConflict, api.Problem{
			Type:   "https://tripgo.example/problems/driver-busy",
			Title:  "Driver busy",
			Status: http.StatusConflict,
			Detail: new("Driver already has an active trip"),
			Code:   "driver_busy",
		}

	default:
		slog.Error("internal error", "error", err)
		return http.StatusInternalServerError, api.Problem{
			Type:   "https://tripgo.example/problems/internal-error",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: new("Internal server error"),
			Code:   "internal_error",
		}
	}
}

// InvalidRequest возвращает ошибку валидации запроса
func InvalidRequest(detail string) api.Problem {
	return api.Problem{
		Type:   "https://tripgo.example/problems/invalid-request",
		Title:  "Invalid request",
		Status: http.StatusBadRequest,
		Detail: new(detail),
		Code:   "invalid_request",
	}
}
