CREATE TABLE IF NOT EXISTS movie_events (
    event_id         String,
    user_id          String,
    movie_id         String,
    event_type       LowCardinality(String),
    timestamp_ms     Int64,
    event_time       DateTime64(3, 'UTC') MATERIALIZED fromUnixTimestamp64Milli(timestamp_ms),
    event_date       Date MATERIALIZED toDate(event_time),
    device_type      LowCardinality(String),
    session_id       String,
    progress_seconds Int32,
    inserted_at      DateTime64(3, 'UTC') DEFAULT now64(3)
) ENGINE = ReplacingMergeTree(inserted_at)
PARTITION BY toYYYYMM(event_date)
ORDER BY (event_id)
TTL event_date + INTERVAL 1 YEAR
SETTINGS index_granularity = 8192;
