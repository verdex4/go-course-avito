package finish_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"
	"github.com/verdex4/go-course-avito/internal/usecase/trip/finish"
)

func TestUseCase_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	m := NewMockmetrics(ctrl)
	uc := finish.New(repo, tx, m)

	ctx := context.Background()
	tripID := uuid.New()

	t.Run("success: active trip completed", func(t *testing.T) {
		activeTrip := &model.Trip{ID: tripID, Status: model.TripStatusActive}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			gomock.InOrder(
				repo.EXPECT().Get(gomock.Any(), tripID).Return(activeTrip, nil),
				repo.EXPECT().UpdateIfActive(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, tr *model.Trip) error {
					require.Equal(t, model.TripStatusCompleted, tr.Status)
					require.NotNil(t, tr.FinishedAt)
					return nil
				}),
			)
			return fn(ctx)
		})

		m.EXPECT().AddTripFinished(gomock.Any(), int64(1))

		res, err := uc.Execute(ctx, tripID)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, model.TripStatusCompleted, res.Status)
		require.NotNil(t, res.FinishedAt)
	})

	t.Run("error: trip not found", func(t *testing.T) {
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			repo.EXPECT().Get(gomock.Any(), tripID).Return(nil, trip.ErrTripNotFound)
			return fn(ctx)
		})

		res, err := uc.Execute(ctx, tripID)

		require.ErrorIs(t, err, trip.ErrTripNotFound)
		require.Nil(t, res)
	})

	t.Run("error: trip already completed", func(t *testing.T) {
		completedTrip := &model.Trip{ID: tripID, Status: model.TripStatusCompleted}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			repo.EXPECT().Get(gomock.Any(), tripID).Return(completedTrip, nil)
			return fn(ctx)
		})

		res, err := uc.Execute(ctx, tripID)

		require.ErrorIs(t, err, trip.ErrTripCompleted)
		require.Nil(t, res)
	})

	t.Run("error: internal repository error on Get", func(t *testing.T) {
		dbErr := errors.New("database connection lost")

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			repo.EXPECT().Get(gomock.Any(), tripID).Return(nil, dbErr)
			return fn(ctx)
		})

		res, err := uc.Execute(ctx, tripID)

		require.Error(t, err)
		require.Contains(t, err.Error(), "finish trip")
		require.Nil(t, res)
	})

	t.Run("error: internal repository error on Update", func(t *testing.T) {
		activeTrip := &model.Trip{ID: tripID, Status: model.TripStatusActive}
		dbErr := errors.New("database connection lost")

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			gomock.InOrder(
				repo.EXPECT().Get(gomock.Any(), tripID).Return(activeTrip, nil),
				repo.EXPECT().UpdateIfActive(gomock.Any(), gomock.Any()).Return(dbErr),
			)
			return fn(ctx)
		})

		res, err := uc.Execute(ctx, tripID)

		require.Error(t, err)
		require.Contains(t, err.Error(), "finish trip")
		require.Nil(t, res)
	})
}
