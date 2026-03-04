package domain

import (
	"time"

	"github.com/google/uuid"
)

type PromoCode struct {
	ID             uuid.UUID
	Code           string
	DiscountType   string
	DiscountValue  float64
	MinOrderAmount float64
	MaxUses        int
	CurrentUses    int
	ValidFrom      time.Time
	ValidUntil     time.Time
	Active         bool
}
