package finish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"
	"github.com/verdex4/go-course-avito/internal/usecase/trip/finish"
	"go.uber.org/mock/gomock"
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
	now := time.Now().UTC()

	t.Run("success", func(t *testing.T) {
		m.EXPECT().AddTripFinished(gomock.Any(), int64(1))
		expectedTrip := &model.Trip{ID: tripID, Status: model.TripStatusCompleted, FinishedAt: &now}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			repo.EXPECT().Finish(gomock.Any(), tripID).Return(expectedTrip, nil)
			return fn(ctx)
		})

		res, err := uc.Execute(ctx, tripID)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, model.TripStatusCompleted, res.Status)
	})

	t.Run("error: trip already completed", func(t *testing.T) {
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			repo.EXPECT().Finish(gomock.Any(), tripID).Return(nil, trip.ErrTripCompleted)
			return fn(ctx)
		})

		res, err := uc.Execute(ctx, tripID)

		require.ErrorIs(t, err, trip.ErrTripCompleted)
		require.Nil(t, res)
	})

	t.Run("error: internal repository error", func(t *testing.T) {
		dbErr := errors.New("database connection lost")
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			repo.EXPECT().Finish(gomock.Any(), tripID).Return(nil, dbErr)
			return fn(ctx)
		})

		res, err := uc.Execute(ctx, tripID)

		require.Error(t, err)
		require.Contains(t, err.Error(), "finish trip")
		require.Nil(t, res)
	})
}
