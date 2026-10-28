package create_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/verdex4/go-course-avito/internal/domain/trip"
	api "github.com/verdex4/go-course-avito/internal/generated"
	"github.com/verdex4/go-course-avito/internal/http/trip/create"
	"github.com/verdex4/go-course-avito/internal/model"
	"go.uber.org/mock/gomock"
)

func TestHandler_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := NewMocktripUseCase(ctrl)
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := create.New(uc, testLogger)

	validBody := `{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.4},"price":100}`

	t.Run("success: 201 Created", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody))
		w := httptest.NewRecorder()

		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(&model.Trip{ID: uuid.New(), Status: model.TripStatusActive}, nil)

		h.Handle(w, req, api.CreateTripParams{})

		require.Equal(t, http.StatusCreated, w.Code)
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
		require.NotEmpty(t, w.Header().Get("Location"))
	})

	t.Run("error: invalid json -> 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(`{invalid}`))
		w := httptest.NewRecorder()

		h.Handle(w, req, api.CreateTripParams{})

		require.Equal(t, http.StatusBadRequest, w.Code)

		var prob api.Problem
		json.Unmarshal(w.Body.Bytes(), &prob)
		require.Equal(t, "invalid_request", prob.Code)
	})

	t.Run("error: driver busy -> 409", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody))
		w := httptest.NewRecorder()

		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, trip.ErrDriverBusy)

		h.Handle(w, req, api.CreateTripParams{})

		require.Equal(t, http.StatusConflict, w.Code)
		var prob api.Problem
		json.Unmarshal(w.Body.Bytes(), &prob)
		require.Equal(t, "driver_busy", prob.Code)
	})

	t.Run("error: internal repo error -> 500 without details", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody))
		w := httptest.NewRecorder()

		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, errors.New("pq: syntax error at or near"))

		h.Handle(w, req, api.CreateTripParams{})

		require.Equal(t, http.StatusInternalServerError, w.Code)
		var prob api.Problem
		json.Unmarshal(w.Body.Bytes(), &prob)
		require.Equal(t, "internal_error", prob.Code)
		// Проверяем, что детали SQL-ошибки не утекли в ответ
		require.NotContains(t, w.Body.String(), "pq:")
		require.NotContains(t, w.Body.String(), "syntax")
	})
}
