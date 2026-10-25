package create

import (
	"context"

	"github.com/verdex4/go-course-avito/internal/model"
	"github.com/verdex4/go-course-avito/internal/usecase/trip/create"
)

//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type tripUseCase interface {
	Execute(ctx context.Context, in create.Input) (*model.Trip, error)
}
