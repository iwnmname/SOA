package repository

import (
	"context"
	"database/sql"
	"errors"

	"hw3/booking-service/internal/model"
)

type BookingRepository struct {
	db *sql.DB
}

func NewBookingRepository(db *sql.DB) *BookingRepository {
	return &BookingRepository{db: db}
}

func (r *BookingRepository) Create(ctx context.Context, b *model.Booking) error {
	query := `
		INSERT INTO bookings (id, user_id, flight_id, passenger_name, passenger_email, seat_count, total_price, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`

	return r.db.QueryRowContext(ctx, query,
		b.ID, b.UserID, b.FlightID, b.PassengerName, b.PassengerEmail,
		b.SeatCount, b.TotalPrice, b.Status,
	).Scan(&b.CreatedAt, &b.UpdatedAt)
}

func (r *BookingRepository) GetByID(ctx context.Context, id string) (*model.Booking, error) {
	query := `
		SELECT id, user_id, flight_id, passenger_name, passenger_email,
		       seat_count, total_price, status, created_at, updated_at
		FROM bookings
		WHERE id = $1`

	b := &model.Booking{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&b.ID, &b.UserID, &b.FlightID, &b.PassengerName, &b.PassengerEmail,
		&b.SeatCount, &b.TotalPrice, &b.Status, &b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *BookingRepository) GetByUserID(ctx context.Context, userID string) ([]*model.Booking, error) {
	query := `
		SELECT id, user_id, flight_id, passenger_name, passenger_email,
		       seat_count, total_price, status, created_at, updated_at
		FROM bookings
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*model.Booking
	for rows.Next() {
		b := &model.Booking{}
		if err := rows.Scan(
			&b.ID, &b.UserID, &b.FlightID, &b.PassengerName, &b.PassengerEmail,
			&b.SeatCount, &b.TotalPrice, &b.Status, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

func (r *BookingRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `UPDATE bookings SET status = $1, updated_at = NOW() WHERE id = $2`

	res, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
