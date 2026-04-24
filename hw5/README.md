# HW5

## Что реализовано

### 1-4. producer + ingestion
- Схема события описана в `proto/movie_event.proto` (Protobuf).
- Схема регистрируется в Schema Registry (`movie-events-value`, версионирование в Registry).
- Топик `movie-events` создается с 3 партициями (`scripts/create-topics.sh`).
- Kafka работает в отказоустойчивом режиме с 2 брокерами (`kafka1`, `kafka2`).
- Для `movie-events` настроены `replication.factor=2` и `min.insync.replicas=1`.
- Ключ партиционирования: `user_id` (в `producer/internal/kafka/producer.go`).
  - Обоснование: сохраняется порядок событий пользователя в рамках сессий просмотра.
- Producer поддерживает:
  - валидацию входного события;
  - публикацию с `acks=all`;
  - retry с exponential backoff;
  - логирование `event_id`, `event_type`, `timestamp_ms`.
- Режимы сервиса:
  - HTTP API (`POST /api/v1/events`, `POST /api/v1/events/batch`);
  - генератор синтетических событий (`MODE=generator|api|both`).

### 5. Агрегация с бизнес-метриками
Добавлен `Aggregation Service`:
- Читает raw события напрямую из ClickHouse (`movie_events`).
- Запускается по расписанию (`AGG_INTERVAL`) и вручную по HTTP (`POST /aggregate/run?date=YYYY-MM-DD`).
- Логирует старт и завершение каждого цикла с количеством обработанных raw-событий и длительностью.

Считаемые метрики:
- `dau` — уникальные пользователи за день.
- `avg_watch_time_seconds` — средний `progress_seconds` для `VIEW_FINISHED`.
- `top_movie_views` — рейтинг фильмов по количеству `VIEW_STARTED` (top-10).
- `conversion_rate` — доля `VIEW_FINISHED / VIEW_STARTED`.
- `retention_d1`, `retention_d7` — возвращаемость когорты первого просмотра.

Материализация в ClickHouse:
- `agg_daily_user_activity` + `agg_daily_user_activity_mv`
- `agg_daily_movie_views` + `agg_daily_movie_views_mv`
- `agg_user_first_view` + `agg_user_first_view_mv`

Выгрузка в PostgreSQL:
- Таблица `business_metrics(metric_date, metric_name, dimension, metric_value, computed_at)`.
- Идемпотентная запись через UPSERT по ключу `(metric_date, metric_name, dimension)`.
- Retry записи в PostgreSQL с exponential backoff и логированием ошибок.

### 6. Grafana dashboard
- Grafana поднимается в `docker-compose` на `http://localhost:3000`.
- Datasource `ClickHouse` провижинится автоматически (`grafana-clickhouse-datasource`).
- Дашборд `HW5 Analytics` провижинится автоматически и использует агрегаты из п.5:
  - `Retention Cohort (Day 0-7)` по когортам первого просмотра;
  - `DAU`;
  - `View Conversion (FINISHED/STARTED)`;
  - `Average Watch Time (Finished)`;
  - `Top Movies (last 7 days)`.
- Логин в Grafana: `admin` / `admin`.

### 7. ежедневный экспорт в S3
- Поднимается S3-совместимое хранилище MinIO (`minio`, `minio-init`) в `docker-compose`.
- Агрегатор экспортирует метрики из PostgreSQL (`business_metrics`) в JSON-файл с полями:
  - `metric_date`, `metric_name`, `dimension`, `value`, `computed_at`.
- Путь объекта детерминированный (идемпотентная перезапись):
  - `s3://movie-analytics/daily/YYYY-MM-DD/aggregates.json`
- Автоэкспорт по расписанию (`EXPORT_INTERVAL`, по умолчанию `24h`) и ручной запуск:
  - `POST /export/run?date=YYYY-MM-DD`
- При ошибках PostgreSQL/S3:
  - ошибка логируется;
  - выполняются retry с exponential backoff;
  - следующая попытка произойдет на следующем тике расписания.

## Быстрый старт

```bash
make up
```

Проверка здоровья:

```bash
make check-health
make check-kafka
```

Отправить тестовое событие:

```bash
make test-event
```

Проверить ClickHouse:

```bash
make check-ch
make check-ch-sample
```

Запустить агрегацию за дату и посмотреть PostgreSQL:

```bash
make run-agg
make run-export
make check-pg
make check-pg-sample
make check-s3
make check-grafana
```

Открыть Grafana:

```bash
open http://localhost:3000
```

## Полезные логи

```bash
make logs
make logs-ch
make logs-aggregator
make logs-grafana
make logs-all
```

