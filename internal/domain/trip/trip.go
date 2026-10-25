package trip

import (
	"errors"
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
		return errors.New("user_id is required")
	}
	if driverID == ([16]byte{}) {
		return errors.New("driver_id is required")
	}
	if start.Latitude < -90 || start.Latitude > 90 {
		return errors.New("start latitude out of range")
	}
	if start.Longitude < -180 || start.Longitude > 180 {
		return errors.New("start longitude out of range")
	}
	if end.Latitude < -90 || end.Latitude > 90 {
		return errors.New("end latitude out of range")
	}
	if end.Longitude < -180 || end.Longitude > 180 {
		return errors.New("end longitude out of range")
	}
	if price < 0 {
		return errors.New("price must be non-negative")
	}
	return nil
}
