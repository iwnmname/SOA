CREATE MATERIALIZED VIEW IF NOT EXISTS agg_user_first_view_mv
TO agg_user_first_view AS
SELECT
    user_id,
    minState(toDate(event_time)) AS first_view_date_state
FROM movie_events
WHERE event_type = 'VIEW_STARTED'
GROUP BY user_id;

