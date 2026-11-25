package save

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/model"
	saveUC "github.com/verdex4/go-course-avito/internal/usecase/trip/position/save"
)

//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type positionUseCase interface {
	Execute(ctx context.Context, in saveUC.Input) (*model.TripPosition, error)
}

// внутренний тип для парсинга тела запроса
type saveRequest struct {
	Coordinates struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"coordinates"`
	RecordedAt time.Time `json:"recorded_at"`
}

// внутренний тип ответа
type saveResponse struct {
	ID          int64           `json:"id"`
	TripID      uuid.UUID       `json:"trip_id"`
	Coordinates coordinatesResp `json:"coordinates"`
	RecordedAt  time.Time       `json:"recorded_at"`
}

type coordinatesResp struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
