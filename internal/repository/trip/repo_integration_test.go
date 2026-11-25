//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/verdex4/go-course-avito/internal/infra/postgres/transaction"
	"github.com/verdex4/go-course-avito/internal/model"
	repoTrip "github.com/verdex4/go-course-avito/internal/repository/trip"
)

func testDBURL(t *testing.T) string {
	url := os.Getenv("DATABASE_URL")
	require.NotEmpty(t, url, "DATABASE_URL must be set for integration tests")
	return url
}

func setupDB(t *testing.T) (*pgxpool.Pool, func()) {
	pool, err := pgxpool.New(context.Background(), testDBURL(t))
	require.NoError(t, err)
	_, _ = pool.Exec(context.Background(),
		"TRUNCATE TABLE trip_positions, trip_status_history, trips CASCADE")
	return pool, func() { pool.Close() }
}

func newTrip() *model.Trip {
	return &model.Trip{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 59.9, Longitude: 30.3},
		EndPoint:   model.Coordinates{Latitude: 59.8, Longitude: 30.4},
		Price:      100,
	}
}

func TestTripRepository_Integration(t *testing.T) {
	pool, teardown := setupDB(t)
	defer teardown()

	repo := repoTrip.New(pool)
	txManager := transaction.New(pool)

	t.Run("save and get success", func(t *testing.T) {
		tr := newTrip()
		err := txManager.Do(context.Background(), func(ctx context.Context) error {
			return repo.Save(ctx, tr)
		})
		require.NoError(t, err)

		fetched, err := repo.Get(context.Background(), tr.ID)
		require.NoError(t, err)
		require.Equal(t, tr.ID, fetched.ID)
		require.Equal(t, model.TripStatusActive, fetched.Status)
	})

	t.Run("rollback on error", func(t *testing.T) {
		tr := newTrip()
		mockErr := errors.New("simulated error")

		err := txManager.Do(context.Background(), func(ctx context.Context) error {
			if err := repo.Save(ctx, tr); err != nil {
				return err
			}
			return mockErr
		})
		require.ErrorIs(t, err, mockErr)

		var tripCount int64
		err = pool.QueryRow(context.Background(),
			"SELECT COUNT(*) FROM trips WHERE id = $1", tr.ID).Scan(&tripCount)
		require.NoError(t, err)
		require.Equal(t, int64(0), tripCount)

		var histCount int64
		err = pool.QueryRow(context.Background(),
			"SELECT COUNT(*) FROM trip_status_history WHERE trip_id = $1", tr.ID).Scan(&histCount)
		require.NoError(t, err)
		require.Equal(t, int64(0), histCount)
	})
}

func TestGetStaleTrips_Integration(t *testing.T) {
	pool, teardown := setupDB(t)
	defer teardown()

	repo := repoTrip.New(pool)
	txManager := transaction.New(pool)
	staleAfter := 60 * time.Second

	tripStale := newTrip()
	err := txManager.Do(context.Background(), func(ctx context.Context) error {
		return repo.Save(ctx, tripStale)
	})
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(),
		"UPDATE trips SET started_at = now() - interval '5 minutes' WHERE id = $1", tripStale.ID)
	require.NoError(t, err)

	tripFresh := newTrip()
	err = txManager.Do(context.Background(), func(ctx context.Context) error {
		return repo.Save(ctx, tripFresh)
	})
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(),
		"UPDATE trips SET started_at = now() - interval '5 minutes', last_position_at = now() WHERE id = $1",
		tripFresh.ID)
	require.NoError(t, err)

	tripCompleted := newTrip()
	err = txManager.Do(context.Background(), func(ctx context.Context) error {
		return repo.Save(ctx, tripCompleted)
	})
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(),
		"UPDATE trips SET started_at = now() - interval '5 minutes', status = 'completed', finished_at = now() WHERE id = $1",
		tripCompleted.ID)
	require.NoError(t, err)

	staleTrips, err := repo.GetStaleTrips(context.Background(), staleAfter)
	require.NoError(t, err)

	ids := make(map[uuid.UUID]bool)
	for _, tr := range staleTrips {
		ids[tr.ID] = true
	}

	assert.True(t, ids[tripStale.ID], "поездка без координат должна попасть")
	assert.False(t, ids[tripFresh.ID], "свежая поездка не должна попасть")
	assert.False(t, ids[tripCompleted.ID], "завершённая поездка не должна попасть")
}

func TestSavePosition_Integration(t *testing.T) {
	pool, teardown := setupDB(t)
	defer teardown()

	repo := repoTrip.New(pool)
	txManager := transaction.New(pool)

	trip := newTrip()
	err := txManager.Do(context.Background(), func(ctx context.Context) error {
		return repo.Save(ctx, trip)
	})
	require.NoError(t, err)

	t.Run("saves position and updates last_position_at", func(t *testing.T) {
		recordedAt := time.Now().UTC().Add(-10 * time.Second)
		pos := &model.TripPosition{
			TripID:      trip.ID,
			Coordinates: model.Coordinates{Latitude: 59.93, Longitude: 30.33},
			RecordedAt:  recordedAt,
		}

		err := txManager.Do(context.Background(), func(ctx context.Context) error {
			if err := repo.SavePosition(ctx, pos); err != nil {
				return err
			}
			return repo.UpdateLastPositionAt(ctx, trip.ID, recordedAt)
		})
		require.NoError(t, err)
		require.NotZero(t, pos.ID)

		var lastPositionAt *time.Time
		err = pool.QueryRow(context.Background(),
			"SELECT last_position_at FROM trips WHERE id = $1", trip.ID).Scan(&lastPositionAt)
		require.NoError(t, err)
		require.NotNil(t, lastPositionAt)
		assert.WithinDuration(t, recordedAt, *lastPositionAt, time.Second)
	})

	t.Run("older coordinate does not overwrite last_position_at", func(t *testing.T) {
		newer := time.Now().UTC().Add(-5 * time.Second)
		older := time.Now().UTC().Add(-30 * time.Second)

		err := txManager.Do(context.Background(), func(ctx context.Context) error {
			pos := &model.TripPosition{
				TripID: trip.ID, RecordedAt: newer,
				Coordinates: model.Coordinates{Latitude: 59.1, Longitude: 30.1},
			}
			if err := repo.SavePosition(ctx, pos); err != nil {
				return err
			}
			return repo.UpdateLastPositionAt(ctx, trip.ID, newer)
		})
		require.NoError(t, err)

		err = txManager.Do(context.Background(), func(ctx context.Context) error {
			pos := &model.TripPosition{
				TripID: trip.ID, RecordedAt: older,
				Coordinates: model.Coordinates{Latitude: 59.2, Longitude: 30.2},
			}
			if err := repo.SavePosition(ctx, pos); err != nil {
				return err
			}
			return repo.UpdateLastPositionAt(ctx, trip.ID, older)
		})
		require.NoError(t, err)

		var lastPositionAt *time.Time
		err = pool.QueryRow(context.Background(),
			"SELECT last_position_at FROM trips WHERE id = $1", trip.ID).Scan(&lastPositionAt)
		require.NoError(t, err)
		require.NotNil(t, lastPositionAt)
		assert.WithinDuration(t, newer, *lastPositionAt, time.Second,
			"старая координата не должна состарить поездку")
	})

	t.Run("rollback leaves no position", func(t *testing.T) {
		simulatedErr := errors.New("simulated error")
		pos := &model.TripPosition{
			TripID:      trip.ID,
			Coordinates: model.Coordinates{Latitude: 59.55, Longitude: 30.55},
			RecordedAt:  time.Now().UTC(),
		}

		err := txManager.Do(context.Background(), func(ctx context.Context) error {
			if err := repo.SavePosition(ctx, pos); err != nil {
				return err
			}
			return simulatedErr
		})
		require.ErrorIs(t, err, simulatedErr)

		var count int64
		err = pool.QueryRow(context.Background(),
			"SELECT COUNT(*) FROM trip_positions WHERE trip_id = $1 AND latitude = 59.55",
			trip.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "позиция не должна сохраниться после отката")
	})
}
