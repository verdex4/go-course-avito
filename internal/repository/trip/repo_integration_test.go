//go:build integration

package trip_test

import (
	"context"
	"os"
	"testing"

	"github.com/verdex4/go-course-avito/internal/infra/postgres/transaction"
	"github.com/verdex4/go-course-avito/internal/model"
	repoTrip "github.com/verdex4/go-course-avito/internal/repository/trip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testDBURL(t *testing.T) string {
	url := os.Getenv("DATABASE_URL")
	require.NotEmpty(t, url, "DATABASE_URL must be set for integration tests")
	return url
}

func setupDB(t *testing.T) (*pgxpool.Pool, func()) {
	pool, err := pgxpool.New(context.Background(), testDBURL(t))
	require.NoError(t, err)

	_, _ = pool.Exec(context.Background(), "TRUNCATE TABLE trip_status_history, trips CASCADE")

	return pool, func() {
		pool.Close()
	}
}

func TestTripRepository_Integration(t *testing.T) {
	pool, teardown := setupDB(t)
	defer teardown()

	repo := repoTrip.New(pool)
	txManager := transaction.New(pool)

	t.Run("save and get success", func(t *testing.T) {
		tripToSave := &model.Trip{
			ID:         uuid.New(),
			UserID:     uuid.New(),
			DriverID:   uuid.New(),
			StartPoint: model.Coordinates{Latitude: 59.9, Longitude: 30.3},
			EndPoint:   model.Coordinates{Latitude: 59.8, Longitude: 30.4},
			Price:      100,
		}

		err := txManager.Do(context.Background(), func(ctx context.Context) error {
			return repo.Save(ctx, tripToSave)
		})
		require.NoError(t, err)

		fetched, err := repo.Get(context.Background(), tripToSave.ID)
		require.NoError(t, err)
		require.Equal(t, tripToSave.ID, fetched.ID)
		require.Equal(t, model.TripStatusActive, fetched.Status)
	})

	t.Run("rollback on error: atomicity check", func(t *testing.T) {
		// Одинаковые DriverID
		driverID := uuid.New()
		trip1 := &model.Trip{ID: uuid.New(), UserID: uuid.New(), DriverID: driverID, StartPoint: model.Coordinates{Latitude: 59.9, Longitude: 30.3}, EndPoint: model.Coordinates{Latitude: 59.8, Longitude: 30.4}, Price: 100}
		trip2 := &model.Trip{ID: uuid.New(), UserID: uuid.New(), DriverID: driverID, StartPoint: model.Coordinates{Latitude: 59.9, Longitude: 30.3}, EndPoint: model.Coordinates{Latitude: 59.8, Longitude: 30.4}, Price: 100}

		// Первая сохраняется успешно
		err := txManager.Do(context.Background(), func(ctx context.Context) error {
			return repo.Save(ctx, trip1)
		})
		require.NoError(t, err)

		// Вторая падает с ошибкой
		err = txManager.Do(context.Background(), func(ctx context.Context) error {
			return repo.Save(ctx, trip2)
		})
		require.Error(t, err)

		// Должна быть одна запись
		var count int64
		err = pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM trips WHERE driver_id = $1", driverID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, int64(1), count)
	})
}
