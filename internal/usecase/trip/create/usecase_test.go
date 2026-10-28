package create_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"
	"github.com/verdex4/go-course-avito/internal/usecase/trip/create"
	"go.uber.org/mock/gomock"
)

func TestUseCase_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	m := NewMockmetrics(ctrl)
	uc := create.New(repo, tx, m)

	ctx := context.Background()
	validInput := create.Input{
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 59.9, Longitude: 30.3},
		EndPoint:   model.Coordinates{Latitude: 59.8, Longitude: 30.4},
		Price:      100,
	}

	t.Run("success", func(t *testing.T) {
		m.EXPECT().AddTripCreated(gomock.Any(), int64(1))
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			return fn(ctx)
		})

		res, err := uc.Execute(ctx, validInput)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, model.TripStatusActive, res.Status)
	})

	t.Run("error: driver busy", func(t *testing.T) {
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(trip.ErrDriverBusy)
			return fn(ctx)
		})

		res, err := uc.Execute(ctx, validInput)

		require.ErrorIs(t, err, trip.ErrDriverBusy)
		require.Nil(t, res)
	})
}
