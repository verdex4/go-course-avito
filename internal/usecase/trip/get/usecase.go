package get

import (
	"context"

	"github.com/google/uuid"
	"github.com/verdex4/go-course-avito/internal/model"
)

func (uc *UseCase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	return uc.repo.Get(ctx, id)
}
