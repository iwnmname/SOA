package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"hw3/flight-service/internal/model"
)

type FlightRepository struct {
	db *sql.DB
}

func NewFlightRepository(db *sql.DB) *FlightRepository {
	return &FlightRepository{db: db}
}

func (r *FlightRepository) GetByID(ctx context.Context, id string) (*model.Flight, error) {
	query := `
		SELECT id, flight_number, airline, origin_airport, destination_airport,
		       departure_time, arrival_time, total_seats, available_seats,
		       price, status, created_at, updated_at
		FROM flights
		WHERE id = $1`

	f := &model.Flight{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&f.ID, &f.FlightNumber, &f.Airline, &f.OriginAirport, &f.DestinationAirport,
		&f.DepartureTime, &f.ArrivalTime, &f.TotalSeats, &f.AvailableSeats,
		&f.Price, &f.Status, &f.CreatedAt, &f.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (r *FlightRepository) Search(ctx context.Context, origin, destination string, date *time.Time) ([]*model.Flight, error) {
	var rows *sql.Rows
	var err error

	if date != nil {
		query := `
			SELECT id, flight_number, airline, origin_airport, destination_airport,
			       departure_time, arrival_time, total_seats, available_seats,
			       price, status, created_at, updated_at
			FROM flights
			WHERE origin_airport = $1
			  AND destination_airport = $2
			  AND CAST(departure_time AS DATE) = $3
			  AND status = 'SCHEDULED'
			ORDER BY departure_time`

		rows, err = r.db.QueryContext(ctx, query, origin, destination, date.Format("2006-01-02"))
	} else {
		query := `
			SELECT id, flight_number, airline, origin_airport, destination_airport,
			       departure_time, arrival_time, total_seats, available_seats,
			       price, status, created_at, updated_at
			FROM flights
			WHERE origin_airport = $1
			  AND destination_airport = $2
			  AND status = 'SCHEDULED'
			ORDER BY departure_time`

		rows, err = r.db.QueryContext(ctx, query, origin, destination)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flights []*model.Flight
	for rows.Next() {
		f := &model.Flight{}
		if err := rows.Scan(
			&f.ID, &f.FlightNumber, &f.Airline, &f.OriginAirport, &f.DestinationAirport,
			&f.DepartureTime, &f.ArrivalTime, &f.TotalSeats, &f.AvailableSeats,
			&f.Price, &f.Status, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, err
		}
		flights = append(flights, f)
	}
	return flights, rows.Err()
}

func (r *FlightRepository) ReserveSeats(ctx context.Context, flightID, bookingID string, seatCount int32) (*model.SeatReservation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var availableSeats int32
	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT available_seats, status FROM flights WHERE id = $1 FOR UPDATE`,
		flightID,
	).Scan(&availableSeats, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFlightNotFound
	}
	if err != nil {
		return nil, err
	}

	if status != "SCHEDULED" {
		return nil, ErrFlightNotScheduled
	}

	if availableSeats < seatCount {
		return nil, ErrNotEnoughSeats
	}

	var exists bool
	err = tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM seat_reservations WHERE booking_id = $1)`,
		bookingID,
	).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrReservationExists
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE flights SET available_seats = available_seats - $1, updated_at = NOW() WHERE id = $2`,
		seatCount, flightID,
	)
	if err != nil {
		return nil, err
	}

	reservation := &model.SeatReservation{}
	err = tx.QueryRowContext(ctx,
		`INSERT INTO seat_reservations (id, flight_id, booking_id, seat_count, status)
		 VALUES (gen_random_uuid(), $1, $2, $3, 'ACTIVE')
		 RETURNING id, flight_id, booking_id, seat_count, status, created_at, updated_at`,
		flightID, bookingID, seatCount,
	).Scan(
		&reservation.ID, &reservation.FlightID, &reservation.BookingID,
		&reservation.SeatCount, &reservation.Status,
		&reservation.CreatedAt, &reservation.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return reservation, nil
}

func (r *FlightRepository) ReleaseReservation(ctx context.Context, bookingID string) (*model.SeatReservation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var reservation model.SeatReservation
	err = tx.QueryRowContext(ctx,
		`SELECT id, flight_id, booking_id, seat_count, status, created_at, updated_at
		 FROM seat_reservations
		 WHERE booking_id = $1
		 FOR UPDATE`,
		bookingID,
	).Scan(
		&reservation.ID, &reservation.FlightID, &reservation.BookingID,
		&reservation.SeatCount, &reservation.Status,
		&reservation.CreatedAt, &reservation.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrReservationNotFound
	}
	if err != nil {
		return nil, err
	}

	if reservation.Status != "ACTIVE" {
		return nil, ErrReservationNotActive
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE flights SET available_seats = available_seats + $1, updated_at = NOW() WHERE id = $2`,
		reservation.SeatCount, reservation.FlightID,
	)
	if err != nil {
		return nil, err
	}

	err = tx.QueryRowContext(ctx,
		`UPDATE seat_reservations SET status = 'RELEASED', updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, flight_id, booking_id, seat_count, status, created_at, updated_at`,
		reservation.ID,
	).Scan(
		&reservation.ID, &reservation.FlightID, &reservation.BookingID,
		&reservation.SeatCount, &reservation.Status,
		&reservation.CreatedAt, &reservation.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &reservation, nil
}
