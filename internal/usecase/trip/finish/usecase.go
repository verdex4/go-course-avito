package finish

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"
)

func (uc *UseCase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	var result *model.Trip

	err := uc.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		result, err = uc.repo.Finish(ctx, id)
		return err
	})
	if err != nil {
		if errors.Is(err, trip.ErrTripNotFound) || errors.Is(err, trip.ErrTripCompleted) {
			return nil, err
		}
		return nil, fmt.Errorf("finish trip: %w", err)
	}

	return result, nil
}
