# Лабораторная работа 1

## Зависимости

- tripgoctl
- make
- goose

## Запуск

```
tripgoctl cluster start
tripgoctl environment start
make migrate
make run
```

## Переменные окружения

tripgoctl по умолчанию создает переменные окружения сам, поэтому файл .env не нужно редактировать. В нем есть следующие переменные:
- HTTP_ADDR
- LOG_LEVEL
- SHUTDOWN_TIMEOUT
- DATABASE_URL
- DATABASE_MAX_CONNS
- DATABASE_MIN_CONNS
- DATABASE_MAX_CONN_LIFETIME
- DATABASE_CONNECT_TIMEOUT
- DATABASE_QUERY_TIMEOUT

## Решения

1. Уровень изоляции транзакций: обычный Read Committed (по умолчанию в PostgreSQL).
2. Менеджер транзакций: реализован метод Do(ctx, fn). Он открывает транзакцию, кладёт её в context и вызывает переданную функцию. Если функция возвращает ошибку или происходит паника - происходит Rollback, если nil - Commit. Репозиторий не знает про pgx, он просто получает исполнителя из контекста через GetExecutor.
3. Запрет двух активных поездок: создан частичный уникальный индекс, гарантирующий корректную работу при одновременных запросах. Ошибка от БД превращается в доменную ошибку 409 Driver already has an active trip.