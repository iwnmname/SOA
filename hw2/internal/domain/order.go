package domain

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Status         string
	PromoCodeID    *uuid.UUID
	TotalAmount    float64
	DiscountAmount float64
	Items          []OrderItem
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OrderItem struct {
	ID           uuid.UUID
	OrderID      uuid.UUID
	ProductID    uuid.UUID
	Quantity     int
	PriceAtOrder float64
}
