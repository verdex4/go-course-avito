package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/verdex4/go-course-avito/internal/domain/trip"
	"github.com/verdex4/go-course-avito/internal/infra/postgres/transaction"
	"github.com/verdex4/go-course-avito/internal/model"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const pgUniqueViolation = "23505"

func (r *repo) Save(ctx context.Context, t *model.Trip) error {
	exec := transaction.GetExecutor(ctx, r.pool)
	now := time.Now().UTC()

	insertTrip, args, err := r.sb.Insert("trips").
		Columns(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude",
			"end_latitude", "end_longitude",
			"price", "status", "started_at",
			"created_at", "updated_at",
		).
		Values(
			t.ID, t.UserID, t.DriverID,
			t.StartPoint.Latitude, t.StartPoint.Longitude,
			t.EndPoint.Latitude, t.EndPoint.Longitude,
			t.Price, string(model.TripStatusActive), now,
			now, now,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip: %w", err)
	}

	if _, err := exec.Exec(ctx, insertTrip, args...); err != nil {
		return r.wrapDBError(err, "insert trip")
	}

	insertHistory, args, err := r.sb.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(t.ID, nil, string(model.TripStatusActive), "trip created", now).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert history: %w", err)
	}

	if _, err := exec.Exec(ctx, insertHistory, args...); err != nil {
		return r.wrapDBError(err, "insert history")
	}

	t.Status = model.TripStatusActive
	t.StartedAt = now
	t.CreatedAt = now
	t.UpdatedAt = now

	return nil
}

func (r *repo) Finish(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	exec := transaction.GetExecutor(ctx, r.pool)
	now := time.Now().UTC()

	updateQuery, args, err := r.sb.Update("trips").
		Set("status", string(model.TripStatusCompleted)).
		Set("finished_at", now).
		Set("updated_at", now).
		Where(squirrel.And{
			squirrel.Eq{"id": id},
			squirrel.Eq{"status": string(model.TripStatusActive)},
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update trip: %w", err)
	}

	tag, err := exec.Exec(ctx, updateQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("update trip: %w", err)
	}

	if tag.RowsAffected() == 0 {
		existing, err := r.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if existing.Status == model.TripStatusCompleted {
			return nil, trip.ErrTripCompleted
		}
		return nil, fmt.Errorf("unexpected state for trip %s", id)
	}

	insertHistory, args, err := r.sb.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(id, string(model.TripStatusActive), string(model.TripStatusCompleted), "trip finished", now).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert history: %w", err)
	}

	if _, err := exec.Exec(ctx, insertHistory, args...); err != nil {
		return nil, fmt.Errorf("insert history: %w", err)
	}

	return r.Get(ctx, id)
}

// wrapDBError превращает ошибки PostgreSQL в доменные.
func (r *repo) wrapDBError(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "trips_driver_active_unique" {
			return trip.ErrDriverBusy
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
