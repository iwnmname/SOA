CREATE MATERIALIZED VIEW IF NOT EXISTS agg_daily_user_activity_mv
TO agg_daily_user_activity AS
SELECT
    toDate(event_time) AS event_date,
    user_id,
    countIf(event_type = 'VIEW_STARTED') AS view_started,
    countIf(event_type = 'VIEW_FINISHED') AS view_finished,
    sumIf(toInt64(progress_seconds), event_type = 'VIEW_FINISHED') AS finished_progress_sum
FROM movie_events
GROUP BY event_date, user_id;

