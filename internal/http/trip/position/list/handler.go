package list

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	api "github.com/verdex4/go-course-avito/internal/generated"
	"github.com/verdex4/go-course-avito/internal/http/domainerror"
)

type Handler struct {
	uc     positionUseCase
	logger *slog.Logger
}

func New(uc positionUseCase, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{uc: uc, logger: logger}
}

// GET /api/v1/trips/{tripId}/positions
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) {
	ctx := r.Context()

	pos, err := h.uc.Execute(ctx, tripID)
	if err != nil {
		status, body := domainerror.Problem(err)

		if status >= http.StatusInternalServerError {
			h.logger.ErrorContext(ctx, "get positions failed",
				slog.String("trip_id", tripID.String()),
				slog.Int("status", status),
				slog.Any("error", err),
			)
		} else {
			h.logger.WarnContext(ctx, "get positions rejected",
				slog.String("trip_id", tripID.String()),
				slog.Int("status", status),
				slog.Any("error", err),
			)
		}

		h.writeProblem(ctx, w, status, body)
		return
	}

	resp := toResponse(pos)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.ErrorContext(ctx, "encode positions response failed",
			slog.String("trip_id", tripID.String()),
			slog.Any("error", err),
		)
	}
}

func (h *Handler) writeProblem(ctx context.Context, w http.ResponseWriter, status int, body api.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.logger.ErrorContext(ctx, "encode problem response failed",
			slog.Int("status", status),
			slog.Any("error", err),
		)
	}
}
