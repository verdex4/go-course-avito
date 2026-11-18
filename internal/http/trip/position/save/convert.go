package save

import (
	"github.com/verdex4/go-course-avito/internal/model"
)

func toResponse(pos *model.TripPosition) saveResponse {
	return saveResponse{
		ID:     pos.ID,
		TripID: pos.TripID,
		Coordinates: coordinatesResp{
			Latitude:  pos.Coordinates.Latitude,
			Longitude: pos.Coordinates.Longitude,
		},
		RecordedAt: pos.RecordedAt,
	}
}
