CREATE TABLE IF NOT EXISTS movie_events_queue (
    event_id         String,
    user_id          String,
    movie_id         String,
    event_type       Enum8(
        'EVENT_TYPE_UNSPECIFIED' = 0,
        'VIEW_STARTED'          = 1,
        'VIEW_FINISHED'         = 2,
        'VIEW_PAUSED'           = 3,
        'VIEW_RESUMED'          = 4,
        'LIKED'                 = 5,
        'SEARCHED'              = 6
    ),
    timestamp_ms     Int64,
    device_type      Enum8(
        'DEVICE_TYPE_UNSPECIFIED' = 0,
        'MOBILE'                 = 1,
        'DESKTOP'                = 2,
        'TV'                     = 3,
        'TABLET'                 = 4
    ),
    session_id       String,
    progress_seconds Int32
) ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'kafka1:29092',
    kafka_topic_list = 'movie-events',
    kafka_group_name = 'clickhouse-consumer',
    kafka_format = 'ProtobufSingle',
    kafka_schema = 'movie_event:cinema.MovieEvent',
    kafka_num_consumers = 1,
    kafka_max_block_size = 1048576;
