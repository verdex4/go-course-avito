package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/verdex4/go-course-avito/internal/config"
	api "github.com/verdex4/go-course-avito/internal/generated"
	"github.com/verdex4/go-course-avito/internal/repository"
	"github.com/verdex4/go-course-avito/internal/txmanager"
)

type Server struct {
	api.Unimplemented
	cfg       *config.Config
	pool      *pgxpool.Pool
	tripRepo  *repository.TripRepository
	txManager txmanager.TxManager
	router    *chi.Mux
	http      *http.Server
}

func NewServer(
	cfg *config.Config,
	pool *pgxpool.Pool,
	tripRepo *repository.TripRepository,
	txManager txmanager.TxManager,
) *Server {
	s := &Server{
		cfg:       cfg,
		pool:      pool,
		tripRepo:  tripRepo,
		txManager: txManager,
	}
	s.setupRouter()
	return s
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	api.HandlerFromMux(s, r)

	s.router = r
}

func (s *Server) Run() error {
	s.http = &http.Server{
		Addr:              s.cfg.HTTPAddr,
		Handler:           s.router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)

	go func() {
		slog.Info("starting HTTP server", "addr", s.cfg.HTTPAddr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		slog.Info("received shutdown signal", "signal", sig)
	}

	return s.shutdown()
}

func (s *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	slog.Info("shutting down server", "timeout", s.cfg.ShutdownTimeout)

	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	slog.Info("gracefully stopped")
	return nil
}
