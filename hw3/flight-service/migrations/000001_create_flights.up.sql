CREATE TABLE IF NOT EXISTS flights (
    id                   UUID PRIMARY KEY,
    flight_number        VARCHAR(10) NOT NULL,
    airline              VARCHAR(100) NOT NULL,
    origin_airport       CHAR(3) NOT NULL,
    destination_airport  CHAR(3) NOT NULL,
    departure_time       TIMESTAMP NOT NULL,
    arrival_time         TIMESTAMP NOT NULL,
    total_seats          INT NOT NULL CHECK (total_seats > 0),
    available_seats      INT NOT NULL CHECK (available_seats >= 0),
    price                NUMERIC(12, 2) NOT NULL CHECK (price > 0),
    status               VARCHAR(20) NOT NULL DEFAULT 'SCHEDULED'
                         CHECK (status IN ('SCHEDULED', 'DEPARTED', 'CANCELLED', 'COMPLETED')),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CHECK (available_seats <= total_seats),
    CHECK (arrival_time > departure_time)
);

CREATE UNIQUE INDEX idx_flights_number_date
    ON flights (flight_number, CAST(departure_time AS DATE));