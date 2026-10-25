package create

import (
	"encoding/json"
	"net/http"

	api "github.com/verdex4/go-course-avito/internal/generated"
	"github.com/verdex4/go-course-avito/internal/http/domainerror"
	"github.com/verdex4/go-course-avito/internal/usecase/trip/create"

	"github.com/verdex4/go-course-avito/internal/model"
)

type Handler struct {
	uc tripUseCase
}

func New(uc tripUseCase) *Handler {
	return &Handler{uc: uc}
}

// POST /api/v1/trips
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var req api.CreateTripJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		status, body := http.StatusBadRequest, domainerror.InvalidRequest("Failed to parse request body")
		writeProblem(w, status, body)
		return
	}

	in := create.Input{
		UserID:     req.UserId,
		DriverID:   req.DriverId,
		StartPoint: model.Coordinates{Latitude: req.StartPoint.Latitude, Longitude: req.StartPoint.Longitude},
		EndPoint:   model.Coordinates{Latitude: req.EndPoint.Latitude, Longitude: req.EndPoint.Longitude},
		Price:      req.Price,
	}

	trip, err := h.uc.Execute(r.Context(), in)
	if err != nil {
		status, body := domainerror.Problem(err)
		writeProblem(w, status, body)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPI(trip))
}

func writeProblem(w http.ResponseWriter, status int, body api.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
