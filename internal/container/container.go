package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/verdex4/go-course-avito/internal/config"
	api "github.com/verdex4/go-course-avito/internal/generated"
	createHandler "github.com/verdex4/go-course-avito/internal/http/trip/create"
	finishHandler "github.com/verdex4/go-course-avito/internal/http/trip/finish"
	getHandler "github.com/verdex4/go-course-avito/internal/http/trip/get"
	"github.com/verdex4/go-course-avito/internal/infra/postgres/transaction"
	repoTrip "github.com/verdex4/go-course-avito/internal/repository/trip"
	createUC "github.com/verdex4/go-course-avito/internal/usecase/trip/create"
	finishUC "github.com/verdex4/go-course-avito/internal/usecase/trip/finish"
	getUC "github.com/verdex4/go-course-avito/internal/usecase/trip/get"
)

type App struct {
	cfg    *config.Config
	pool   *pgxpool.Pool
	server *http.Server
}

func New(cfg *config.Config) (*App, error) {
	// Инициализация БД
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	poolConfig.MaxConns = int32(cfg.DatabaseMaxConns)
	poolConfig.MinConns = int32(cfg.DatabaseMinConns)
	poolConfig.MaxConnLifetime = cfg.DatabaseMaxConnLifetime

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	slog.Info("database connected successfully")

	// Инфраструктура и репозитории
	txManager := transaction.New(pool)
	tripRepo := repoTrip.New(pool)

	// Usecases (бизнес-логика)
	createUsecase := createUC.New(tripRepo, txManager)
	getUsecase := getUC.New(tripRepo)
	finishUsecase := finishUC.New(tripRepo, txManager)

	// HTTP Handlers
	createH := createHandler.New(createUsecase)
	getH := getHandler.New(getUsecase)
	finishH := finishHandler.New(finishUsecase)

	// Сборка роутера
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(60 * time.Second))

	apiWrapper := &apiWrapper{
		pool:   pool,
		create: createH,
		get:    getH,
		finish: finishH,
	}

	api.HandlerFromMux(apiWrapper, router)

	// Создание HTTP-сервера
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &App{
		cfg:    cfg,
		pool:   pool,
		server: srv,
	}, nil
}

// Запускает сервер и ждёт сигнала отмены контекста.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		slog.Info("starting HTTP server", "addr", a.cfg.HTTPAddr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		slog.Info("received shutdown signal")
	}

	return a.shutdown(ctx)
}

func (a *App) shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, a.cfg.ShutdownTimeout)
	defer cancel()

	slog.Info("shutting down server", "timeout", a.cfg.ShutdownTimeout)

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	a.pool.Close()
	slog.Info("gracefully stopped")
	return nil
}

type apiWrapper struct {
	api.Unimplemented

	pool   *pgxpool.Pool
	create *createHandler.Handler
	get    *getHandler.Handler
	finish *finishHandler.Handler
}

func (w *apiWrapper) CreateTrip(rw http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	w.create.Handle(rw, r, params)
}

func (w *apiWrapper) GetTrip(rw http.ResponseWriter, r *http.Request, tripId api.TripId) {
	w.get.Handle(rw, r, tripId)
}

func (w *apiWrapper) FinishTrip(rw http.ResponseWriter, r *http.Request, tripId api.TripId) {
	w.finish.Handle(rw, r, tripId)
}

func (w *apiWrapper) Health(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(api.HealthResponse{Status: api.Ok})
}

func (w *apiWrapper) Ready(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	rw.Header().Set("Content-Type", "application/json")
	if err := w.pool.Ping(ctx); err != nil {
		rw.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(rw).Encode(api.HealthResponse{Status: api.Unavailable})
		return
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(api.HealthResponse{Status: api.Ok})
}
