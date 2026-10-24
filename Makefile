include .env
export $(shell test -f .env && sed 's/=.*//' .env)

.PHONY: generate migrate migrate-down run test

# Генерация кода по OpenAPI
generate:
	@echo "Generating OpenAPI code..."
	mkdir -p internal/generated
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml

# Накатывание миграций PostgreSQL через goose
migrate:
	@echo "Applying database migrations..."
	go tool goose -dir migrations postgres "$(DATABASE_URL)" up

# Откат миграций PostgreSQL
migrate-down:
	@echo "Rolling back database migrations..."
	go tool goose -dir migrations postgres "$(DATABASE_URL)" down-to 0

# Запуск сервиса
run:
	@echo "Running service..."
	go run ./cmd/trip-service

# Запуск тестов
test:
	@echo "Running tests..."
	go test -race ./...