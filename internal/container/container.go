package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/verdex4/go-course-avito/internal/config"
	api "github.com/verdex4/go-course-avito/internal/generated"
	httpMiddleware "github.com/verdex4/go-course-avito/internal/http/middleware"
	createHandler "github.com/verdex4/go-course-avito/internal/http/trip/create"
	finishHandler "github.com/verdex4/go-course-avito/internal/http/trip/finish"
	getHandler "github.com/verdex4/go-course-avito/internal/http/trip/get"
	"github.com/verdex4/go-course-avito/internal/infra/postgres/transaction"
	"github.com/verdex4/go-course-avito/internal/infra/telemetry"
	repoTrip "github.com/verdex4/go-course-avito/internal/repository/trip"
	createUC "github.com/verdex4/go-course-avito/internal/usecase/trip/create"
	finishUC "github.com/verdex4/go-course-avito/internal/usecase/trip/finish"
	getUC "github.com/verdex4/go-course-avito/internal/usecase/trip/get"
)

type App struct {
	cfg       *config.Config
	pool      *pgxpool.Pool
	server    *http.Server
	telemetry *telemetry.Telemetry
}

func New(cfg *config.Config) (*App, error) {
	// Инициализация Telemetry
	tel, err := telemetry.New(context.Background(), cfg.OtelEndpoint, cfg.OtelProtocol, cfg.OtelTracesSampler)
	if err != nil {
		return nil, fmt.Errorf("init telemetry: %w", err)
	}

	// Инициализация БД
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	poolConfig.MaxConns = int32(cfg.DatabaseMaxConns)
	poolConfig.MinConns = int32(cfg.DatabaseMinConns)
	poolConfig.MaxConnLifetime = cfg.DatabaseMaxConnLifetime
	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer()

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
	tel.Logger.Info("database connected successfully")

	// Репозитории и Usecases
	txManager := transaction.New(pool)
	tripRepo := repoTrip.New(pool)

	createUsecase := createUC.New(tripRepo, txManager, tel.Metrics)
	getUsecase := getUC.New(tripRepo)
	finishUsecase := finishUC.New(tripRepo, txManager, tel.Metrics)

	// HTTP Handlers
	createH := createHandler.New(createUsecase, tel.Logger)
	getH := getHandler.New(getUsecase, tel.Logger)
	finishH := finishHandler.New(finishUsecase, tel.Logger)

	// Сборка роутера
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(httpMiddleware.Logging(tel.Logger))

	// Адаптер для OpenAPI
	apiWrapper := &apiWrapper{
		pool:   pool,
		create: createH,
		get:    getH,
		finish: finishH,
	}

	api.HandlerFromMux(apiWrapper, router)

	// Оборачиваем весь роутер в otelhttp
	otelHandler := otelhttp.NewHandler(router, "trip-service",
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			// Имя спана: "METHOD /route/template"
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}
			return fmt.Sprintf("%s %s", r.Method, route)
		}),
	)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           otelHandler,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &App{
		cfg:       cfg,
		pool:      pool,
		server:    srv,
		telemetry: tel,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		a.telemetry.Logger.Info("starting HTTP server", "addr", a.cfg.HTTPAddr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		a.telemetry.Logger.Info("received shutdown signal")
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

	if err := a.telemetry.Shutdown(ctx); err != nil {
		a.telemetry.Logger.Error("telemetry shutdown failed", "error", err)
	}

	a.pool.Close()
	a.telemetry.Logger.Info("gracefully stopped")
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
	_, _ = rw.Write([]byte(`{"status":"ok"}`))
}

func (w *apiWrapper) Ready(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	rw.Header().Set("Content-Type", "application/json")
	if err := w.pool.Ping(ctx); err != nil {
		rw.WriteHeader(http.StatusServiceUnavailable)
		_, _ = rw.Write([]byte(`{"status":"unavailable"}`))
		return
	}

	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(`{"status":"ok"}`))
}
