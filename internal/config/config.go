package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	LogLevel        string
	ShutdownTimeout time.Duration

	DatabaseMaxConns        int
	DatabaseMinConns        int
	DatabaseMaxConnLifetime time.Duration
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{}

	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		return nil, fmt.Errorf("HTTP_ADDR is required")
	}
	cfg.HTTPAddr = httpAddr

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	cfg.DatabaseURL = databaseURL

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		return nil, fmt.Errorf("LOG_LEVEL is required")
	}
	cfg.LogLevel = logLevel

	shutdownTimeout, err := parseDuration("SHUTDOWN_TIMEOUT", "30s")
	if err != nil {
		return nil, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
	}
	cfg.ShutdownTimeout = shutdownTimeout

	maxConns, err := parseInt("DATABASE_MAX_CONNS", 10)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_MAX_CONNS: %w", err)
	}
	cfg.DatabaseMaxConns = maxConns

	minConns, err := parseInt("DATABASE_MIN_CONNS", 2)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_MIN_CONNS: %w", err)
	}
	cfg.DatabaseMinConns = minConns

	connectTimeout, err := parseDuration("DATABASE_CONNECT_TIMEOUT", "5s")
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_CONNECT_TIMEOUT: %w", err)
	}
	cfg.DatabaseConnectTimeout = connectTimeout

	queryTimeout, err := parseDuration("DATABASE_QUERY_TIMEOUT", "30s")
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_QUERY_TIMEOUT: %w", err)
	}
	cfg.DatabaseQueryTimeout = queryTimeout

	maxConnLifetime, err := parseDuration("DATABASE_MAX_CONN_LIFETIME", "1h")
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_MAX_CONN_LIFETIME: %w", err)
	}
	cfg.DatabaseMaxConnLifetime = maxConnLifetime

	return cfg, nil
}

func parseDuration(key, defaultValue string) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		value = defaultValue
	}
	return time.ParseDuration(value)
}

func parseInt(key string, defaultValue int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(value)
}
