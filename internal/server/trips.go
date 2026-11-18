package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/verdex4/go-course-avito/internal/domain"
	api "github.com/verdex4/go-course-avito/internal/generated"
)

// CreateTrip — POST /api/v1/trips
func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var req api.CreateTripJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Failed to parse request body")
		return
	}

	trip := &domain.Trip{
		ID:       uuid.New(),
		UserID:   uuid.UUID(req.UserId),
		DriverID: uuid.UUID(req.DriverId),
		StartPoint: domain.Coordinates{
			Latitude:  req.StartPoint.Latitude,
			Longitude: req.StartPoint.Longitude,
		},
		EndPoint: domain.Coordinates{
			Latitude:  req.EndPoint.Latitude,
			Longitude: req.EndPoint.Longitude,
		},
		Price: req.Price,
	}

	err := s.txManager.Do(r.Context(), func(ctx context.Context) error {
		return s.tripRepo.CreateTrip(ctx, trip)
	})

	if err != nil {
		handleDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPITrip(trip))
}

// GetTrip — GET /api/v1/trips/{tripId}
func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	// tripId уже валидирован как UUID кодогенератором
	trip, err := s.tripRepo.GetTrip(r.Context(), uuid.UUID(tripId))
	if err != nil {
		handleDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(trip))
}

// FinishTrip — POST /api/v1/trips/{tripId}/finish
func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	var updatedTrip *domain.Trip

	err := s.txManager.Do(r.Context(), func(ctx context.Context) error {
		var err error
		updatedTrip, err = s.tripRepo.FinishTrip(ctx, uuid.UUID(tripId))
		return err
	})

	if err != nil {
		handleDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(updatedTrip))
}
