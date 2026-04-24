CREATE MATERIALIZED VIEW IF NOT EXISTS movie_events_mv
TO movie_events AS
SELECT
    event_id,
    user_id,
    movie_id,
    CAST(event_type, 'String') AS event_type,
    timestamp_ms,
    CAST(device_type, 'String') AS device_type,
    session_id,
    progress_seconds
FROM movie_events_queue;
