# Лабораторная работа 3

## Зависимости

- tripgoctl: ```curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL https://github.com/course-go-autumn-2026/course-infra/releases/latest/download/install-tripgoctl | sh```
- make + компиляторы C++ для работы тестов (в пакете уже есть make): ```sudo apt update && sudo apt install build-essential``` (linux)
- генератор заглушек (mocks) для тестов: ```go install go.uber.org/mock/mockgen@latest``` (linux)

## Запуск

```
tripgoctl cluster start
tripgoctl environment start
make migrate
make run
```

Проверка линтера
```
make lint
```

Запуск тестов
```
make test
make test-integration
```

Подключение к Jaeger и Grafana
```
tripgoctl connect
```

## Переменные окружения

tripgoctl по умолчанию создает переменные окружения сам, согласно environment.toml, поэтому файл .env не нужно редактировать. В нем есть следующие переменные:
- HTTP_ADDR
- LOG_LEVEL
- SHUTDOWN_TIMEOUT
- DATABASE_URL
- DATABASE_MAX_CONNS
- DATABASE_MIN_CONNS
- DATABASE_MAX_CONN_LIFETIME
- DATABASE_CONNECT_TIMEOUT
- DATABASE_QUERY_TIMEOUT
- OTEL_SERVICE_NAME
- OTEL_EXPORTER_OTLP_ENDPOINT
- OTEL_EXPORTER_OTLP_PROTOCOL
- OTEL_RESOURCE_ATTRIBUTES
- OTEL_TRACES_SAMPLER
- POSITION_CHECK_INTERVAL
- POSITION_STALE_AFTER
- POSITION_WORKERS
- POSITION_QUEUE_SIZE
- PUSH_HTTP_URL
- PUSH_REQUEST_TIMEOUT

## Опрос устаревших поездок

Схема работы:
```
сканер → очередь → воркеры → Push Service
```

Параметры конфигурации:
- `POSITION_CHECK_INTERVAL` - интервал опроса устаревших поездок
- `POSITION_STALE_AFTER` - период, после которого поездка считается устаревшей
- `POSITION_WORKERS` - количество параллельных воркеров для обработки запросов
- `POSITION_QUEUE_SIZE` - размер очереди запросов к Push Service
- `PUSH_HTTP_URL` - адрес сервиса Push
- `PUSH_REQUEST_TIMEOUT` - таймаут HTTP-запроса в Push Service

Поведение при исчерпании очереди:
Если очередь заполнена, новые задачи будут отбрасываться ("dropped").

## Схема слоев

```
cmd/service/main.go          // точка входа
      ↓
internal/container           // собирает все зависимости в одном месте
      ↓
internal/http/trip/*         // HTTP-обработчики
      ↓
internal/usecase/trip/*      // бизнес-логика
      ↓
internal/domain/trip         // доменные ошибки и переходы состояний
      ↓
internal/model               // структуры данных, передаваемые между слоями
      ↓
internal/repository/trip     // работа с базой данных
      ↓
internal/infra/postgres      // пул соединений, менеджер транзакций
internal/infra/telemetry     // OpenTelemetry, логгер
```