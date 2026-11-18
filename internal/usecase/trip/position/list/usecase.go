package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/model"
)

func (uc *UseCase) Execute(ctx context.Context, tripID uuid.UUID) ([]*model.TripPosition, error) {
	if _, err := uc.tripRepo.Get(ctx, tripID); err != nil {
		return nil, err
	}

	return uc.positionRepo.GetPositions(ctx, tripID)
}
