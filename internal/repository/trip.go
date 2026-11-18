package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/verdex4/go-course-avito/internal/domain"
	"github.com/verdex4/go-course-avito/internal/txmanager"
)

const pgUniqueViolation = "23505"

type TripRepository struct {
	pool *pgxpool.Pool
	sb   squirrel.StatementBuilderType
}

func NewTripRepository(pool *pgxpool.Pool) *TripRepository {
	return &TripRepository{
		pool: pool,
		sb:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// Создаёт поездку и запись в журнале статусов
func (r *TripRepository) CreateTrip(ctx context.Context, trip *domain.Trip) error {
	exec := txmanager.GetExecutor(ctx, r.pool)
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
			trip.ID, trip.UserID, trip.DriverID,
			trip.StartPoint.Latitude, trip.StartPoint.Longitude,
			trip.EndPoint.Latitude, trip.EndPoint.Longitude,
			trip.Price, string(domain.TripStatusActive), now,
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
		Values(trip.ID, nil, string(domain.TripStatusActive), "trip created", now).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert history: %w", err)
	}

	if _, err := exec.Exec(ctx, insertHistory, args...); err != nil {
		return r.wrapDBError(err, "insert history")
	}

	trip.Status = domain.TripStatusActive
	trip.StartedAt = now
	trip.CreatedAt = now
	trip.UpdatedAt = now

	return nil
}

// Возвращает поездку по id или ErrTripNotFound.
func (r *TripRepository) GetTrip(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	exec := txmanager.GetExecutor(ctx, r.pool)

	query, args, err := r.sb.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude",
		"end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at",
		"created_at", "updated_at",
	).
		From("trips").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select trip: %w", err)
	}

	var trip domain.Trip
	var status string
	err = exec.QueryRow(ctx, query, args...).Scan(
		&trip.ID, &trip.UserID, &trip.DriverID,
		&trip.StartPoint.Latitude, &trip.StartPoint.Longitude,
		&trip.EndPoint.Latitude, &trip.EndPoint.Longitude,
		&trip.Price, &status, &trip.StartedAt, &trip.FinishedAt,
		&trip.CreatedAt, &trip.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTripNotFound
		}
		return nil, fmt.Errorf("select trip: %w", err)
	}
	trip.Status = domain.TripStatus(status)

	return &trip, nil
}

// Завершает активную поездку.
func (r *TripRepository) FinishTrip(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	exec := txmanager.GetExecutor(ctx, r.pool)
	now := time.Now().UTC()

	// Обновляем, только если статус active
	updateQuery, args, err := r.sb.Update("trips").
		Set("status", string(domain.TripStatusCompleted)).
		Set("finished_at", now).
		Set("updated_at", now).
		Where(squirrel.And{
			squirrel.Eq{"id": id},
			squirrel.Eq{"status": string(domain.TripStatusActive)},
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
		trip, err := r.GetTrip(ctx, id)
		if err != nil {
			return nil, err
		}
		if trip.Status == domain.TripStatusCompleted {
			return nil, domain.ErrTripCompleted
		}
		return nil, fmt.Errorf("unexpected state for trip %s", id)
	}

	// Пишем в журнал
	insertHistory, args, err := r.sb.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(id, string(domain.TripStatusActive), string(domain.TripStatusCompleted), "trip finished", now).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert history: %w", err)
	}

	if _, err := exec.Exec(ctx, insertHistory, args...); err != nil {
		return nil, fmt.Errorf("insert history: %w", err)
	}

	trip, err := r.GetTrip(ctx, id)
	if err != nil {
		return nil, err
	}
	return trip, nil
}

// Превращает ошибки PostgreSQL в доменные.
func (r *TripRepository) wrapDBError(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == pgUniqueViolation {
			// У водителя не может быть 2 активные поездки
			if pgErr.ConstraintName == "trips_driver_active_unique" {
				return domain.ErrDriverBusy
			}
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
