include .env
export $(shell test -f .env && sed 's/=.*//' .env)

.PHONY: generate migrate migrate-down run test lint test test-integration

# Генерация кода: OpenAPI и моки для тестов
generate:
	@echo "Generating OpenAPI code and tests mocks..."
	mkdir -p internal/generated
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml
	go generate ./...

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
	go run ./cmd/service

# Запуск linter
lint:
	@echo "Running linter..."
	go tool golangci-lint run ./...

# Unit-тесты с race detector и выводом % покрытия
# CGO_ENABLED=1 - включение race detector
test:
	@echo "Running tests..."
	CGO_ENABLED=1 go test -race -cover ./internal/domain/... ./internal/usecase/... ./internal/http/...

# Запуск интеграционных тестов, требующих подключение к БД через tripgoctl
# CGO_ENABLED=1 - включение race detector
test-integration:
	@echo "Running integration tests..."
	CGO_ENABLED=1 go test -race -tags=integration -cover ./internal/repository/...