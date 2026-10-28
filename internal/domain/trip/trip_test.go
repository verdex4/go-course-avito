package trip_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"
)

func TestTrip_Finish(t *testing.T) {
	t.Run("success: active to completed", func(t *testing.T) {
		t.Parallel()
		tr := &model.Trip{Status: model.TripStatusActive}
		now := time.Now()
		err := trip.Finish(tr, now)

		require.NoError(t, err)
		require.Equal(t, model.TripStatusCompleted, tr.Status)
		require.Equal(t, now, *tr.FinishedAt)
	})

	t.Run("error: already completed", func(t *testing.T) {
		t.Parallel()
		tr := &model.Trip{Status: model.TripStatusCompleted}
		now := time.Now()
		err := trip.Finish(tr, now)

		require.ErrorIs(t, err, trip.ErrTripCompleted)
	})
}

func TestTrip_Validate(t *testing.T) {
	validUUID := uuid.New()
	validCoords := model.Coordinates{Latitude: 59.9, Longitude: 30.3}

	tests := []struct {
		name      string
		userID    uuid.UUID
		driverID  uuid.UUID
		start     model.Coordinates
		end       model.Coordinates
		price     int64
		wantError bool
	}{
		{"valid", validUUID, validUUID, validCoords, validCoords, 100, false},
		{"empty user_id", uuid.Nil, validUUID, validCoords, validCoords, 100, true},
		{"empty driver_id", validUUID, uuid.Nil, validCoords, validCoords, 100, true},
		{"invalid start lat", validUUID, validUUID, model.Coordinates{Latitude: 100, Longitude: 30.3}, validCoords, 100, true},
		{"invalid start lon", validUUID, validUUID, model.Coordinates{Latitude: 59.9, Longitude: 200}, validCoords, 100, true},
		{"invalid end lat", validUUID, validUUID, validCoords, model.Coordinates{Latitude: -100, Longitude: 30.3}, 100, true},
		{"invalid end lon", validUUID, validUUID, validCoords, model.Coordinates{Latitude: 59.9, Longitude: -200}, 100, true},
		{"negative price", validUUID, validUUID, validCoords, validCoords, -10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := trip.Validate(tt.userID, tt.driverID, tt.start, tt.end, tt.price)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
