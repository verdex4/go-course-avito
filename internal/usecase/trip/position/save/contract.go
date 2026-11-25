package save

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/model"
)

//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type (
	tripPositionRepository interface {
		SavePosition(ctx context.Context, pos *model.TripPosition) error
		UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error
	}

	tripRepository interface {
		Get(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	positionPool interface {
		Cancel(tripID uuid.UUID)
	}

	metrics interface {
		AddPositionSaved(ctx context.Context, n int64)
	}
)

type Input struct {
	TripID      uuid.UUID
	Coordinates model.Coordinates
	RecordedAt  time.Time
}

type UseCase struct {
	tripRepo     tripRepository
	positionRepo tripPositionRepository
	tx           txManager
	pool         positionPool
	metrics      metrics
}

func New(
	tripRepo tripRepository,
	positionRepo tripPositionRepository,
	tx txManager,
	pool positionPool,
	m metrics,
) *UseCase {
	return &UseCase{
		tripRepo:     tripRepo,
		positionRepo: positionRepo,
		tx:           tx,
		pool:         pool,
		metrics:      m,
	}
}
