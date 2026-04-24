CREATE TABLE IF NOT EXISTS agg_daily_movie_views (
    event_date Date,
    movie_id String,
    view_started UInt64,
    view_finished UInt64
) ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (event_date, movie_id);

