package list

import (
	api "github.com/verdex4/go-course-avito/internal/generated"
	"github.com/verdex4/go-course-avito/internal/model"
)

func toResponse(positions []*model.TripPosition) api.TripPositionsResponse {
	result := make([]api.TripPosition, 0, len(positions))
	for _, p := range positions {
		result = append(result, api.TripPosition{
			Id:     p.ID,
			TripId: p.TripID,
			Coordinates: api.Coordinates{
				Latitude:  p.Coordinates.Latitude,
				Longitude: p.Coordinates.Longitude,
			},
			RecordedAt: p.RecordedAt,
		})
	}
	return api.TripPositionsResponse{Positions: result}
}
