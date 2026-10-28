package get_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/verdex4/go-course-avito/internal/domain/trip"
	api "github.com/verdex4/go-course-avito/internal/generated"
	"github.com/verdex4/go-course-avito/internal/http/trip/get"
	"github.com/verdex4/go-course-avito/internal/model"
	"go.uber.org/mock/gomock"
)

func TestHandler_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := NewMocktripUseCase(ctrl)
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := get.New(uc, testLogger)

	tripID := api.TripId(uuid.New())

	t.Run("success: 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String(), nil)
		w := httptest.NewRecorder()

		uc.EXPECT().Execute(gomock.Any(), tripID).Return(&model.Trip{ID: uuid.UUID(tripID), Status: model.TripStatusActive}, nil)

		h.Handle(w, req, tripID)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	})

	t.Run("error: trip not found -> 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String(), nil)
		w := httptest.NewRecorder()

		uc.EXPECT().Execute(gomock.Any(), tripID).Return(nil, trip.ErrTripNotFound)

		h.Handle(w, req, tripID)

		require.Equal(t, http.StatusNotFound, w.Code)
		require.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

		var prob api.Problem
		err := json.Unmarshal(w.Body.Bytes(), &prob)
		require.NoError(t, err)
		require.Equal(t, "trip_not_found", prob.Code)
	})
}
