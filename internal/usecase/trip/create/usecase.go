package create

import (
	"context"
	"errors"
	"fmt"

	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"

	"github.com/google/uuid"
)

func (uc *UseCase) Execute(ctx context.Context, in Input) (*model.Trip, error) {
	if err := trip.Validate(in.UserID, in.DriverID, in.StartPoint, in.EndPoint, in.Price); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	t := &model.Trip{
		ID:         uuid.New(),
		UserID:     in.UserID,
		DriverID:   in.DriverID,
		StartPoint: in.StartPoint,
		EndPoint:   in.EndPoint,
		Price:      in.Price,
	}

	err := uc.tx.Do(ctx, func(ctx context.Context) error {
		return uc.repo.Save(ctx, t)
	})
	if err != nil {
		if errors.Is(err, trip.ErrDriverBusy) {
			return nil, err
		}
		return nil, fmt.Errorf("save trip: %w", err)
	}

	return t, nil
}
