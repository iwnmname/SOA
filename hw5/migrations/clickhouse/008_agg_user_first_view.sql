CREATE TABLE IF NOT EXISTS agg_user_first_view (
    user_id String,
    first_view_date_state AggregateFunction(min, Date)
) ENGINE = AggregatingMergeTree
ORDER BY user_id;

