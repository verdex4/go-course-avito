package create

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"
)

// Execute возвращает (*model.Trip, isNew bool, error)
func (uc *UseCase) Execute(ctx context.Context, in Input) (*model.Trip, bool, error) {
	if in.IdempotencyKey != nil {
		existing, err := uc.repo.GetByIdempotencyKey(ctx, *in.IdempotencyKey)
		if err == nil {
			if isBodyMatch(existing, in) {
				return existing, false, nil
			}
			return nil, false, trip.ErrIdempotencyConflict
		}
		if !errors.Is(err, trip.ErrTripNotFound) {
			return nil, false, fmt.Errorf("check idempotency: %w", err)
		}
	}

	if err := trip.Validate(in.UserID, in.DriverID, in.StartPoint, in.EndPoint, in.Price); err != nil {
		return nil, false, fmt.Errorf("validate: %w", err)
	}

	t := &model.Trip{
		ID:             uuid.New(),
		IdempotencyKey: in.IdempotencyKey,
		UserID:         in.UserID,
		DriverID:       in.DriverID,
		StartPoint:     in.StartPoint,
		EndPoint:       in.EndPoint,
		Price:          in.Price,
		Status:         model.TripStatusActive,
		StartedAt:      time.Now().UTC(),
	}

	err := uc.tx.Do(ctx, func(ctx context.Context) error {
		return uc.repo.Save(ctx, t)
	})
	if err != nil {
		if errors.Is(err, trip.ErrDriverBusy) {
			return nil, false, err
		}
		return nil, false, fmt.Errorf("save trip: %w", err)
	}

	uc.metrics.AddTripCreated(ctx, 1)

	return t, true, nil
}

func isBodyMatch(existing *model.Trip, in Input) bool {
	return existing.UserID == in.UserID &&
		existing.DriverID == in.DriverID &&
		existing.StartPoint == in.StartPoint &&
		existing.EndPoint == in.EndPoint &&
		existing.Price == in.Price
}
