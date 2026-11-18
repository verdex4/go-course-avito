package positions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/model"
)

//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type PushGateway interface {
	RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error
}

type staleTripsRepo interface {
	GetStaleTrips(ctx context.Context, staleAfter time.Duration) ([]*model.Trip, error)
}

type poolMetrics interface {
	AddStaleCheck(ctx context.Context, result string)
	AddPositionRequest(ctx context.Context, result string)
	RecordPositionRequestDuration(ctx context.Context, seconds float64, result string)
	IncWorkersActive(ctx context.Context)
	DecWorkersActive(ctx context.Context)
	RegisterQueueSizeGauge(queueLen func() int64)
}
