package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/verdex4/go-course-avito/internal/config"
	"github.com/verdex4/go-course-avito/internal/repository"
	"github.com/verdex4/go-course-avito/internal/server"
	"github.com/verdex4/go-course-avito/internal/txmanager"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to parse database config", "error", err)
		os.Exit(1)
	}

	poolConfig.MaxConns = int32(cfg.DatabaseMaxConns)
	poolConfig.MinConns = int32(cfg.DatabaseMinConns)
	poolConfig.MaxConnLifetime = cfg.DatabaseMaxConnLifetime

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		slog.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("database connected successfully")

	tm := txmanager.NewTxManager(pool)
	tripRepo := repository.NewTripRepository(pool)

	srv := server.NewServer(cfg, pool, tripRepo, tm)
	if err := srv.Run(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
