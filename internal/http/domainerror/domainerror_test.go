package domainerror_test

import (
	"errors"
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
	}{
		{
			name:       "trip not found",
			err:        trip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "trip already completed",
			err:        trip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
		{
			name:       "driver busy",
			err:        trip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
		},
		{
			name:       "internal error",
			err:        errors.New("pq: syntax error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, body := domainerror.Problem(tt.err)

			require.Equal(t, tt.wantStatus, status)
			require.Equal(t, int32(tt.wantStatus), body.Status)
			require.Equal(t, tt.wantCode, body.Code)
			require.NotEmpty(t, body.Type)
			require.NotEmpty(t, body.Title)
		})
	}
}

func TestInvalidRequest(t *testing.T) {
	detail := "Failed to parse request body"
	body := domainerror.InvalidRequest(detail)

	require.Equal(t, int32(http.StatusBadRequest), body.Status)
	require.Equal(t, "invalid_request", body.Code)
	require.Equal(t, detail, *body.Detail)
}
