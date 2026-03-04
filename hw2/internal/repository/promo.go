package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"hw2/internal/domain"
)

type PromoRepo struct{}

func NewPromoRepo() *PromoRepo { return &PromoRepo{} }

func (r *PromoRepo) Create(ctx context.Context, db DBTX, p *domain.PromoCode) error {
	p.ID = uuid.New()
	_, err := db.Exec(ctx,
		`INSERT INTO promo_codes (id, code, discount_type, discount_value, min_order_amount, max_uses, current_uses, valid_from, valid_until, active)
		 VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, $9)`,
		p.ID, p.Code, p.DiscountType, p.DiscountValue, p.MinOrderAmount, p.MaxUses, p.ValidFrom, p.ValidUntil, p.Active)
	return err
}

func (r *PromoRepo) GetByCode(ctx context.Context, db DBTX, code string) (*domain.PromoCode, error) {
	p := &domain.PromoCode{}
	err := db.QueryRow(ctx,
		`SELECT id, code, discount_type, discount_value, min_order_amount, max_uses, current_uses, valid_from, valid_until, active
		 FROM promo_codes WHERE code=$1 FOR UPDATE`, code,
	).Scan(&p.ID, &p.Code, &p.DiscountType, &p.DiscountValue, &p.MinOrderAmount,
		&p.MaxUses, &p.CurrentUses, &p.ValidFrom, &p.ValidUntil, &p.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *PromoRepo) GetByID(ctx context.Context, db DBTX, id uuid.UUID) (*domain.PromoCode, error) {
	p := &domain.PromoCode{}
	err := db.QueryRow(ctx,
		`SELECT id, code, discount_type, discount_value, min_order_amount, max_uses, current_uses, valid_from, valid_until, active
		 FROM promo_codes WHERE id=$1 FOR UPDATE`, id,
	).Scan(&p.ID, &p.Code, &p.DiscountType, &p.DiscountValue, &p.MinOrderAmount,
		&p.MaxUses, &p.CurrentUses, &p.ValidFrom, &p.ValidUntil, &p.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *PromoRepo) IncrementUses(ctx context.Context, db DBTX, id uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE promo_codes SET current_uses = current_uses + 1 WHERE id=$1`, id)
	return err
}

func (r *PromoRepo) DecrementUses(ctx context.Context, db DBTX, id uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE promo_codes SET current_uses = current_uses - 1 WHERE id=$1`, id)
	return err
}
