package trip

import (
	"context"

	"github.com/google/uuid"
	"github.com/verdex4/go-course-avito/internal/model"
)

type TripRepository interface {
	Save(ctx context.Context, trip *model.Trip) error
	Get(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	Finish(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}
