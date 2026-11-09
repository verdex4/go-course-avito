package finish

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"
)

func (uc *UseCase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	var result *model.Trip

	err := uc.tx.Do(ctx, func(ctx context.Context) error {
		tr, err := uc.repo.Get(ctx, id)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		if err := trip.Finish(tr, now); err != nil {
			return err
		}

		if err := uc.repo.UpdateIfActive(ctx, tr); err != nil {
			return err
		}

		result = tr
		return nil
	})
	if err != nil {
		if errors.Is(err, trip.ErrTripNotFound) || errors.Is(err, trip.ErrTripCompleted) {
			return nil, err
		}
		return nil, fmt.Errorf("finish trip: %w", err)
	}

	uc.metrics.AddTripFinished(ctx, 1)

	return result, nil
}
