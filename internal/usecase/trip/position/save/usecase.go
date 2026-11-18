package save

import (
	"context"
	"fmt"
	"time"

	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/model"
)

func (uc *UseCase) Execute(ctx context.Context, in Input) (*model.TripPosition, error) {
	t, err := uc.tripRepo.Get(ctx, in.TripID)
	if err != nil {
		return nil, err
	}
	if t.Status != model.TripStatusActive {
		return nil, trip.ErrTripCompleted
	}

	if in.Coordinates.Latitude < -90 || in.Coordinates.Latitude > 90 {
		return nil, &trip.ValidationError{Field: "coordinates.latitude", Message: "latitude must be between -90 and 90"}
	}
	if in.Coordinates.Longitude < -180 || in.Coordinates.Longitude > 180 {
		return nil, &trip.ValidationError{Field: "coordinates.longitude", Message: "longitude must be between -180 and 180"}
	}

	if in.RecordedAt.After(time.Now().Add(time.Minute)) {
		return nil, &trip.ValidationError{Field: "recorded_at", Message: "recorded_at must not be more than 1 minute in the future"}
	}

	pos := &model.TripPosition{
		TripID:      in.TripID,
		Coordinates: in.Coordinates,
		RecordedAt:  in.RecordedAt,
	}

	err = uc.tx.Do(ctx, func(ctx context.Context) error {
		if err := uc.positionRepo.SavePosition(ctx, pos); err != nil {
			return fmt.Errorf("save position: %w", err)
		}
		if err := uc.positionRepo.UpdateLastPositionAt(ctx, in.TripID, in.RecordedAt); err != nil {
			return fmt.Errorf("update last_position_at: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	uc.metrics.AddPositionSaved(ctx, 1)

	uc.pool.Cancel(in.TripID)

	return pos, nil
}
