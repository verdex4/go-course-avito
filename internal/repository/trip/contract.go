package trip

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/model"
)

type TripRepository interface {
	Save(ctx context.Context, trip *model.Trip) error
	Get(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	UpdateIfActive(ctx context.Context, t *model.Trip) error
	GetByIdempotencyKey(ctx context.Context, key uuid.UUID) (*model.Trip, error)
	SavePosition(ctx context.Context, pos *model.TripPosition) error
	UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error
	GetPositions(ctx context.Context, tripID uuid.UUID) ([]*model.TripPosition, error)
	GetStaleTrips(ctx context.Context, staleAfter time.Duration) ([]*model.Trip, error)
}
