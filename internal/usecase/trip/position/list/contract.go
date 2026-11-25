package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/model"
)

//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type (
	tripRepository interface {
		Get(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	}

	tripPositionRepository interface {
		GetPositions(ctx context.Context, tripID uuid.UUID) ([]*model.TripPosition, error)
	}
)

type UseCase struct {
	tripRepo     tripRepository
	positionRepo tripPositionRepository
}

func New(tripRepo tripRepository, positionRepo tripPositionRepository) *UseCase {
	return &UseCase{tripRepo: tripRepo, positionRepo: positionRepo}
}
