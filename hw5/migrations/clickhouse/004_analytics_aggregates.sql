CREATE TABLE IF NOT EXISTS agg_daily_user_activity (
    event_date Date,
    user_id String,
    view_started UInt64,
    view_finished UInt64,
    finished_progress_sum Int64
) ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (event_date, user_id);

