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
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/verdex4/go-course-avito/internal/config"
	gatewayPush "github.com/verdex4/go-course-avito/internal/gateway/push"
	api "github.com/verdex4/go-course-avito/internal/generated"
	httpMiddleware "github.com/verdex4/go-course-avito/internal/http/middleware"
	createHandler "github.com/verdex4/go-course-avito/internal/http/trip/create"
	finishHandler "github.com/verdex4/go-course-avito/internal/http/trip/finish"
	getHandler "github.com/verdex4/go-course-avito/internal/http/trip/get"
	listPositionHandler "github.com/verdex4/go-course-avito/internal/http/trip/position/list"
	savePositionHandler "github.com/verdex4/go-course-avito/internal/http/trip/position/save"
	"github.com/verdex4/go-course-avito/internal/infra/postgres/transaction"
	"github.com/verdex4/go-course-avito/internal/infra/telemetry"
	repoTrip "github.com/verdex4/go-course-avito/internal/repository/trip"
	createUC "github.com/verdex4/go-course-avito/internal/usecase/trip/create"
	finishUC "github.com/verdex4/go-course-avito/internal/usecase/trip/finish"
	getUC "github.com/verdex4/go-course-avito/internal/usecase/trip/get"
	listPositionUC "github.com/verdex4/go-course-avito/internal/usecase/trip/position/list"
	savePositionUC "github.com/verdex4/go-course-avito/internal/usecase/trip/position/save"
	"github.com/verdex4/go-course-avito/internal/worker/positions"
)

type App struct {
	cfg           *config.Config
	pool          *pgxpool.Pool
	server        *http.Server
	telemetry     *telemetry.Telemetry
	positionsPool *positions.Pool
}

func New(cfg *config.Config) (*App, error) {
	// Инициализация Telemetry
	tel, err := telemetry.New(context.Background(), cfg.OtelEndpoint, cfg.OtelTracesSampler)
	if err != nil {
		return nil, fmt.Errorf("init telemetry: %w", err)
	}

	// Инициализация БД
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	poolConfig.MaxConns = cfg.DatabaseMaxConns
	poolConfig.MinConns = cfg.DatabaseMinConns
	poolConfig.MaxConnLifetime = cfg.DatabaseMaxConnLifetime
	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseConnectTimeout)
	defer cancel()

	dbPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	if err := dbPool.Ping(ctx); err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	tel.Logger.Info("database connected successfully")

	// Репозитории и usecases
	txManager := transaction.New(dbPool)
	tripRepo := repoTrip.New(dbPool)

	createUsecase := createUC.New(tripRepo, txManager, tel.Metrics)
	getUsecase := getUC.New(tripRepo)
	finishUsecase := finishUC.New(tripRepo, txManager, tel.Metrics)

	// Gateway Push Service
	pushGateway := gatewayPush.New(cfg.PushHTTPURL, cfg.PushRequestTimeout)

	// Worker Pool для координат
	poolCfg := positions.Config{
		CheckInterval:  cfg.PositionCheckInterval,
		StaleAfter:     cfg.PositionStaleAfter,
		Workers:        cfg.PositionWorkers,
		QueueSize:      cfg.PositionQueueSize,
		RequestTimeout: cfg.PushRequestTimeout,
	}
	positionsMetrics := positions.NewMetricsAdapter(tel.Metrics, tel.Meter)
	positionsPool := positions.NewPool(tripRepo, pushGateway, poolCfg, positionsMetrics, tel.Logger)

	// Usecases для координат
	savePositionUsecase := savePositionUC.New(tripRepo, tripRepo, txManager, positionsPool, tel.Metrics)
	listPositionUsecase := listPositionUC.New(tripRepo, tripRepo)

	// HTTP Handlers
	createH := createHandler.New(createUsecase, tel.Logger)
	getH := getHandler.New(getUsecase, tel.Logger)
	finishH := finishHandler.New(finishUsecase, tel.Logger)
	savePositionH := savePositionHandler.New(savePositionUsecase, tel.Logger)
	listPositionH := listPositionHandler.New(listPositionUsecase, tel.Logger)

	// Сборка роутера
	router := chi.NewRouter()
	router.Use(httpMiddleware.Recover(tel.Logger))
	router.Use(httpMiddleware.SetHTTPRoute)
	router.Use(httpMiddleware.TrackActiveRequests(tel.Metrics.ActiveRequests))
	router.Use(httpMiddleware.Logging(tel.Logger))

	apiWrapper := &apiWrapper{
		pool:         dbPool,
		create:       createH,
		get:          getH,
		finish:       finishH,
		savePosition: savePositionH,
		listPosition: listPositionH,
	}

	api.HandlerFromMux(apiWrapper, router)

	otelHandler := otelhttp.NewHandler(router, "trip-service",
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
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
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	return &App{
		cfg:           cfg,
		pool:          dbPool,
		server:        srv,
		telemetry:     tel,
		positionsPool: positionsPool,
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

	// Запускаем пул воркеров в отдельной горутине с собственным контекстом
	poolCtx, poolCancel := context.WithCancel(context.Background())
	poolDone := make(chan struct{})
	go func() {
		defer close(poolDone)
		a.positionsPool.Start(poolCtx)
	}()

	select {
	case err := <-errCh:
		poolCancel()
		<-poolDone
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		a.telemetry.Logger.Info("received shutdown signal")
	}

	return a.shutdown(poolCancel, poolDone)
}

func (a *App) shutdown(poolCancel context.CancelFunc, poolDone <-chan struct{}) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()

	slog.Info("shutting down server", "timeout", a.cfg.ShutdownTimeout)

	// 1. Останавливаем HTTP-сервер — прекращаем принимать новые запросы
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	// 2. Останавливаем сканер; воркеры дорабатывают очередь
	poolCancel()
	select {
	case <-poolDone:
	case <-shutdownCtx.Done():
		slog.Warn("positions pool did not stop in time")
	}

	// 3. Закрываем телеметрию
	if err := a.telemetry.Shutdown(shutdownCtx); err != nil {
		a.telemetry.Logger.Error("telemetry shutdown failed", "error", err)
	}

	// 4. Закрываем пул БД
	a.pool.Close()
	a.telemetry.Logger.Info("gracefully stopped")
	return nil
}

type apiWrapper struct {
	api.Unimplemented
	pool         *pgxpool.Pool
	create       *createHandler.Handler
	get          *getHandler.Handler
	finish       *finishHandler.Handler
	savePosition *savePositionHandler.Handler
	listPosition *listPositionHandler.Handler
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

func (w *apiWrapper) CreateTripPosition(rw http.ResponseWriter, r *http.Request, tripId api.TripId) {
	w.savePosition.Handle(rw, r, tripId)
}

func (w *apiWrapper) ListTripPositions(rw http.ResponseWriter, r *http.Request, tripId api.TripId) {
	w.listPosition.Handle(rw, r, tripId)
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
