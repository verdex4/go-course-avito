package model

import (
	"time"

	"github.com/google/uuid"
)

type TripPosition struct {
	ID          int64
	TripID      uuid.UUID
	Coordinates Coordinates
	RecordedAt  time.Time
	CreatedAt   time.Time
}
