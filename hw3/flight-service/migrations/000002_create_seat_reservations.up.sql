CREATE TABLE IF NOT EXISTS seat_reservations (
    id          UUID PRIMARY KEY,
    flight_id   UUID NOT NULL REFERENCES flights(id),
    booking_id  UUID NOT NULL UNIQUE,
    seat_count  INT NOT NULL CHECK (seat_count > 0),
    status      VARCHAR(20) NOT NULL DEFAULT 'ACTIVE'
                CHECK (status IN ('ACTIVE', 'RELEASED', 'EXPIRED')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX idx_seat_reservations_flight_id ON seat_reservations (flight_id);
CREATE INDEX idx_seat_reservations_booking_id ON seat_reservations (booking_id);