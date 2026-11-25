package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/model"
)

//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type positionUseCase interface {
	Execute(ctx context.Context, tripID uuid.UUID) ([]*model.TripPosition, error)
}
