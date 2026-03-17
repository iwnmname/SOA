CREATE TABLE IF NOT EXISTS bookings (
    id              UUID PRIMARY KEY,
    user_id         VARCHAR(255) NOT NULL,
    flight_id       UUID NOT NULL,
    passenger_name  VARCHAR(255) NOT NULL,
    passenger_email VARCHAR(255) NOT NULL,
    seat_count      INT NOT NULL CHECK (seat_count > 0),
    total_price     NUMERIC(12, 2) NOT NULL CHECK (total_price >= 0),
    status          VARCHAR(20) NOT NULL DEFAULT 'CONFIRMED'
                    CHECK (status IN ('CONFIRMED', 'CANCELLED')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bookings_user_id ON bookings (user_id);
CREATE INDEX idx_bookings_flight_id ON bookings (flight_id);