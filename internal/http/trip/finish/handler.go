package finish

import (
	"encoding/json"
	"net/http"

	api "github.com/verdex4/go-course-avito/internal/generated"
	"github.com/verdex4/go-course-avito/internal/http/domainerror"
)

type Handler struct {
	uc tripUseCase
}

func New(uc tripUseCase) *Handler {
	return &Handler{uc: uc}
}

// POST /api/v1/trips/{tripId}/finish
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	trip, err := h.uc.Execute(r.Context(), tripId)
	if err != nil {
		status, body := domainerror.Problem(err)
		writeProblem(w, status, body)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPI(trip))
}

func writeProblem(w http.ResponseWriter, status int, body api.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
