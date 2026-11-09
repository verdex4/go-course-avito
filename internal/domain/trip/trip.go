package trip

import (
	"time"

	"github.com/verdex4/go-course-avito/internal/model"
)

func Finish(t *model.Trip, at time.Time) error {
	if t.Status != model.TripStatusActive {
		return ErrTripCompleted
	}
	t.Status = model.TripStatusCompleted
	t.FinishedAt = &at
	return nil
}

func Validate(userID, driverID [16]byte, start, end model.Coordinates, price int64) error {
	if userID == ([16]byte{}) {
		return &ValidationError{Field: "user_id", Message: "user_id is required"}
	}
	if driverID == ([16]byte{}) {
		return &ValidationError{Field: "driver_id", Message: "driver_id is required"}
	}
	if start.Latitude < -90 || start.Latitude > 90 {
		return &ValidationError{Field: "start_point.latitude", Message: "latitude must be between -90 and 90"}
	}
	if start.Longitude < -180 || start.Longitude > 180 {
		return &ValidationError{Field: "start_point.longitude", Message: "longitude must be between -180 and 180"}
	}
	if end.Latitude < -90 || end.Latitude > 90 {
		return &ValidationError{Field: "end_point.latitude", Message: "latitude must be between -90 and 90"}
	}
	if end.Longitude < -180 || end.Longitude > 180 {
		return &ValidationError{Field: "end_point.longitude", Message: "longitude must be between -180 and 180"}
	}
	if price < 0 {
		return &ValidationError{Field: "price", Message: "price must be non-negative"}
	}
	return nil
}
