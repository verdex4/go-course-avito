package create

import (
	"context"

	"github.com/google/uuid"
	"github.com/verdex4/go-course-avito/internal/model"
)

//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type (
	tripRepository interface {
		Save(ctx context.Context, trip *model.Trip) error
	}
	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
	metrics interface {
		AddTripCreated(ctx context.Context, n int64)
	}
)

type Input struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint model.Coordinates
	EndPoint   model.Coordinates
	Price      int64
}

type UseCase struct {
	repo    tripRepository
	tx      txManager
	metrics metrics
}

func New(repo tripRepository, tx txManager, m metrics) *UseCase {
	return &UseCase{repo: repo, tx: tx, metrics: m}
}
