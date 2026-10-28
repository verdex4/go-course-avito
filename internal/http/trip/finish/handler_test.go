package finish_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/verdex4/go-course-avito/internal/domain/trip"
	api "github.com/verdex4/go-course-avito/internal/generated"
	"github.com/verdex4/go-course-avito/internal/http/trip/finish"
	"github.com/verdex4/go-course-avito/internal/model"
	"go.uber.org/mock/gomock"
)

func TestHandler_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := NewMocktripUseCase(ctrl)
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := finish.New(uc, testLogger)

	tripID := api.TripId(uuid.New())

	t.Run("success: 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		w := httptest.NewRecorder()

		uc.EXPECT().Execute(gomock.Any(), tripID).Return(&model.Trip{ID: uuid.UUID(tripID), Status: model.TripStatusCompleted}, nil)

		h.Handle(w, req, tripID)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var res api.Trip
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)
		require.Equal(t, api.Completed, res.Status)
	})

	t.Run("error: trip completed -> 409", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		w := httptest.NewRecorder()

		uc.EXPECT().Execute(gomock.Any(), tripID).Return(nil, trip.ErrTripCompleted)

		h.Handle(w, req, tripID)

		require.Equal(t, http.StatusConflict, w.Code)
		require.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

		var prob api.Problem
		err := json.Unmarshal(w.Body.Bytes(), &prob)
		require.NoError(t, err)
		require.Equal(t, "trip_completed", prob.Code)
	})

	t.Run("error: internal error -> 500 without details", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		w := httptest.NewRecorder()

		uc.EXPECT().Execute(gomock.Any(), tripID).Return(nil, errors.New("pq: deadlock detected"))

		h.Handle(w, req, tripID)

		require.Equal(t, http.StatusInternalServerError, w.Code)

		var prob api.Problem
		err := json.Unmarshal(w.Body.Bytes(), &prob)
		require.NoError(t, err)
		require.Equal(t, "internal_error", prob.Code)
		// Проверяем, что детали ошибки БД не утекли в ответ
		require.NotContains(t, w.Body.String(), "pq:")
		require.NotContains(t, w.Body.String(), "deadlock")
	})
}
