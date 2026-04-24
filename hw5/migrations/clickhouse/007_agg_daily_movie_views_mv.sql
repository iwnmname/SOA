CREATE MATERIALIZED VIEW IF NOT EXISTS agg_daily_movie_views_mv
TO agg_daily_movie_views AS
SELECT
    toDate(event_time) AS event_date,
    movie_id,
    countIf(event_type = 'VIEW_STARTED') AS view_started,
    countIf(event_type = 'VIEW_FINISHED') AS view_finished
FROM movie_events
GROUP BY event_date, movie_id;

