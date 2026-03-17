package repository

import "errors"

var (
	ErrFlightNotFound       = errors.New("flight not found")
	ErrFlightNotScheduled   = errors.New("flight is not in SCHEDULED status")
	ErrNotEnoughSeats       = errors.New("not enough available seats")
	ErrReservationExists    = errors.New("reservation for this booking already exists")
	ErrReservationNotFound  = errors.New("reservation not found")
	ErrReservationNotActive = errors.New("reservation is not in ACTIVE status")
)
