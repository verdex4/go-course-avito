package domainerror_test

import (
	"errors"
	"math"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name:       "trip not found",
			err:        trip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantDetail: "Trip was not found",
		},
		{
			name:       "trip already completed",
			err:        trip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
			wantDetail: "Operation is not allowed for a completed trip",
		},
		{
			name:       "driver busy",
			err:        trip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
			wantDetail: "Driver already has an active trip",
		},
		{
			name:       "validation error",
			err:        &trip.ValidationError{Field: "price", Message: "price must be non-negative"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantDetail: "validation error: price: price must be non-negative",
		},
		{
			name:       "internal error",
			err:        errors.New("pq: syntax error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantDetail: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, body := domainerror.Problem(tt.err)

			require.Equal(t, tt.wantStatus, status)
			s, err := toInt32(tt.wantStatus)
			require.NoError(t, err)
			require.Equal(t, s, body.Status)
			require.Equal(t, tt.wantCode, body.Code)
			require.NotEmpty(t, body.Type)
			require.NotEmpty(t, body.Title)
			require.Equal(t, tt.wantDetail, *body.Detail)
		})
	}
}

func TestInvalidRequest(t *testing.T) {
	detail := "Failed to parse request body"
	body := domainerror.InvalidRequest(detail)

	s, err := toInt32(http.StatusBadRequest)
	require.NoError(t, err)
	require.Equal(t, s, body.Status)
	require.Equal(t, "invalid_request", body.Code)
	require.Equal(t, detail, *body.Detail)
	require.Equal(t, "https://tripgo.example/problems/invalid-request", body.Type)
	require.Equal(t, "Invalid request", body.Title)
}

func toInt32(value int) (int32, error) {
	if value > math.MaxInt32 || value < math.MinInt32 {
		return 0, errors.New("integer overflow: value out of int32 range")
	}

	return int32(value), nil
}
