package get_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"
	"github.com/verdex4/go-course-avito/internal/usecase/trip/get"
	"go.uber.org/mock/gomock"
)

func TestUseCase_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := NewMocktripRepository(ctrl)
	uc := get.New(repo)

	ctx := context.Background()
	tripID := uuid.New()
	now := time.Now().UTC()

	t.Run("success", func(t *testing.T) {
		expectedTrip := &model.Trip{ID: tripID, Status: model.TripStatusActive, StartedAt: now}
		repo.EXPECT().Get(gomock.Any(), tripID).Return(expectedTrip, nil)

		res, err := uc.Execute(ctx, tripID)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, tripID, res.ID)
	})

	t.Run("error: trip not found", func(t *testing.T) {
		repo.EXPECT().Get(gomock.Any(), tripID).Return(nil, trip.ErrTripNotFound)

		res, err := uc.Execute(ctx, tripID)

		require.ErrorIs(t, err, trip.ErrTripNotFound)
		require.Nil(t, res)
	})
}
