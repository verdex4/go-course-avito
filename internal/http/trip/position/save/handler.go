package save

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/http/domainerror"
	"github.com/verdex4/go-course-avito/internal/model"
	saveUC "github.com/verdex4/go-course-avito/internal/usecase/trip/position/save"
)

type Handler struct {
	uc     positionUseCase
	logger *slog.Logger
}

func New(uc positionUseCase, logger *slog.Logger) *Handler {
	return &Handler{uc: uc, logger: logger}
}

// POST /api/v1/trips/{tripId}/positions
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) {
	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, domainerror.InvalidRequest("failed to parse request body"))
		return
	}

	in := saveUC.Input{
		TripID: tripID,
		Coordinates: model.Coordinates{
			Latitude:  req.Coordinates.Latitude,
			Longitude: req.Coordinates.Longitude,
		},
		RecordedAt: req.RecordedAt,
	}

	pos, err := h.uc.Execute(r.Context(), in)
	if err != nil {
		status, body := domainerror.Problem(err)
		writeProblem(w, status, body)
		return
	}

	h.logger.Info("position saved", "trip_id", tripID.String(), "position_id", pos.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toResponse(pos))
}

func writeProblem(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
