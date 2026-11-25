package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/verdex4/go-course-avito/internal/infra/postgres/transaction"
	"github.com/verdex4/go-course-avito/internal/model"
)

func (r *repo) SavePosition(ctx context.Context, pos *model.TripPosition) error {
	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sb.Insert("trip_positions").
		Columns("trip_id", "latitude", "longitude", "recorded_at").
		Values(pos.TripID, pos.Coordinates.Latitude, pos.Coordinates.Longitude, pos.RecordedAt).
		Suffix("RETURNING id, created_at").
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert position: %w", err)
	}

	if err := exec.QueryRow(ctx, query, args...).Scan(&pos.ID, &pos.CreatedAt); err != nil {
		return fmt.Errorf("insert position: %w", err)
	}

	return nil
}

func (r *repo) UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error {
	exec := transaction.GetExecutor(ctx, r.pool)

	query := `
		UPDATE trips
		SET last_position_at = GREATEST(last_position_at, $1),
		    updated_at = now()
		WHERE id = $2
	`

	if _, err := exec.Exec(ctx, query, recordedAt, tripID); err != nil {
		return fmt.Errorf("update last_position_at: %w", err)
	}

	return nil
}

func (r *repo) GetPositions(ctx context.Context, tripID uuid.UUID) ([]*model.TripPosition, error) {
	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sb.Select("id", "trip_id", "latitude", "longitude", "recorded_at", "created_at").
		From("trip_positions").
		Where(squirrel.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select positions: %w", err)
	}

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select positions: %w", err)
	}
	defer rows.Close()

	var positions []*model.TripPosition
	for rows.Next() {
		pos := &model.TripPosition{}
		if err := rows.Scan(
			&pos.ID, &pos.TripID,
			&pos.Coordinates.Latitude, &pos.Coordinates.Longitude,
			&pos.RecordedAt, &pos.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}
		positions = append(positions, pos)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err positions: %w", err)
	}

	return positions, nil
}

func (r *repo) GetStaleTrips(ctx context.Context, staleAfter time.Duration) ([]*model.Trip, error) {
	exec := transaction.GetExecutor(ctx, r.pool)

	query := `
		SELECT id, user_id, driver_id,
		       start_latitude, start_longitude,
		       end_latitude, end_longitude,
		       price, status, started_at, finished_at,
		       created_at, updated_at, last_position_at
		FROM trips
		WHERE status = 'active'
		  AND COALESCE(last_position_at, started_at) < now() - $1::interval
		ORDER BY COALESCE(last_position_at, started_at) ASC
	`

	rows, err := exec.Query(ctx, query, fmt.Sprintf("%f seconds", staleAfter.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("select stale trips: %w", err)
	}
	defer rows.Close()

	var trips []*model.Trip
	for rows.Next() {
		t := &model.Trip{}
		var status string
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.DriverID,
			&t.StartPoint.Latitude, &t.StartPoint.Longitude,
			&t.EndPoint.Latitude, &t.EndPoint.Longitude,
			&t.Price, &status, &t.StartedAt, &t.FinishedAt,
			&t.CreatedAt, &t.UpdatedAt, &t.LastPositionAt,
		); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}
		t.Status = model.TripStatus(status)
		trips = append(trips, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err stale trips: %w", err)
	}

	return trips, nil
}
