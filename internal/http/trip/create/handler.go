package create

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	api "github.com/verdex4/go-course-avito/internal/generated"
	"github.com/verdex4/go-course-avito/internal/http/domainerror"
	"github.com/verdex4/go-course-avito/internal/model"
	"github.com/verdex4/go-course-avito/internal/usecase/trip/create"
)

type Handler struct {
	uc     tripUseCase
	logger *slog.Logger
}

func New(uc tripUseCase, logger *slog.Logger) *Handler {
	return &Handler{uc: uc, logger: logger}
}

// POST /api/v1/trips
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var req api.CreateTripJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		status, body := http.StatusBadRequest, domainerror.InvalidRequest("Failed to parse request body")
		writeProblem(w, status, body)
		return
	}

	var idempotencyKey *uuid.UUID
	if params.IdempotencyKey != nil {
		key := *params.IdempotencyKey
		idempotencyKey = &key
	}

	in := create.Input{
		IdempotencyKey: idempotencyKey,
		UserID:         req.UserId,
		DriverID:       req.DriverId,
		StartPoint:     model.Coordinates{Latitude: req.StartPoint.Latitude, Longitude: req.StartPoint.Longitude},
		EndPoint:       model.Coordinates{Latitude: req.EndPoint.Latitude, Longitude: req.EndPoint.Longitude},
		Price:          req.Price,
	}

	trip, isNew, err := h.uc.Execute(r.Context(), in)
	if err != nil {
		status, body := domainerror.Problem(err)
		writeProblem(w, status, body)
		return
	}

	if isNew {
		h.logger.Info("trip created", "trip_id", trip.ID.String())
	} else {
		h.logger.Info("idempotent request, returning existing trip", "trip_id", trip.ID.String())
	}

	w.Header().Set("Content-Type", "application/json")

	if isNew {
		w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	_ = json.NewEncoder(w).Encode(toAPI(trip))
}

func writeProblem(w http.ResponseWriter, status int, body api.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
