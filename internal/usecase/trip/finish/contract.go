package finish

import (
	"context"

	"github.com/google/uuid"
	"github.com/verdex4/go-course-avito/internal/model"
)

//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type (
	tripRepository interface {
		Finish(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	}
	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
)

type UseCase struct {
	repo tripRepository
	tx   txManager
}

func New(repo tripRepository, tx txManager) *UseCase {
	return &UseCase{repo: repo, tx: tx}
}
