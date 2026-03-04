package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"hw2/internal/domain"
)

type OrderRepo struct{}

func NewOrderRepo() *OrderRepo { return &OrderRepo{} }

func (r *OrderRepo) Create(ctx context.Context, db DBTX, o *domain.Order) error {
	o.ID = uuid.New()
	return db.QueryRow(ctx,
		`INSERT INTO orders (id, user_id, status, promo_code_id, total_amount, discount_amount)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING created_at, updated_at`,
		o.ID, o.UserID, o.Status, o.PromoCodeID, o.TotalAmount, o.DiscountAmount,
	).Scan(&o.CreatedAt, &o.UpdatedAt)
}

func (r *OrderRepo) GetByID(ctx context.Context, db DBTX, id uuid.UUID) (*domain.Order, error) {
	o := &domain.Order{}
	err := db.QueryRow(ctx,
		`SELECT id, user_id, status, promo_code_id, total_amount, discount_amount, created_at, updated_at
		 FROM orders WHERE id = $1`, id,
	).Scan(&o.ID, &o.UserID, &o.Status, &o.PromoCodeID, &o.TotalAmount, &o.DiscountAmount, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	items, err := r.GetItems(ctx, db, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

func (r *OrderRepo) Update(ctx context.Context, db DBTX, o *domain.Order) error {
	_, err := db.Exec(ctx,
		`UPDATE orders SET status=$2, promo_code_id=$3, total_amount=$4, discount_amount=$5
		 WHERE id=$1`,
		o.ID, o.Status, o.PromoCodeID, o.TotalAmount, o.DiscountAmount)
	return err
}

func (r *OrderRepo) HasActiveOrder(ctx context.Context, db DBTX, userID uuid.UUID) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM orders WHERE user_id=$1 AND status IN ('CREATED','PAYMENT_PENDING'))`,
		userID).Scan(&exists)
	return exists, err
}

func (r *OrderRepo) CreateItem(ctx context.Context, db DBTX, item *domain.OrderItem) error {
	item.ID = uuid.New()
	_, err := db.Exec(ctx,
		`INSERT INTO order_items (id, order_id, product_id, quantity, price_at_order)
		 VALUES ($1, $2, $3, $4, $5)`,
		item.ID, item.OrderID, item.ProductID, item.Quantity, item.PriceAtOrder)
	return err
}

func (r *OrderRepo) GetItems(ctx context.Context, db DBTX, orderID uuid.UUID) ([]domain.OrderItem, error) {
	rows, err := db.Query(ctx,
		`SELECT id, order_id, product_id, quantity, price_at_order
		 FROM order_items WHERE order_id=$1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Quantity, &item.PriceAtOrder); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *OrderRepo) DeleteItems(ctx context.Context, db DBTX, orderID uuid.UUID) error {
	_, err := db.Exec(ctx, `DELETE FROM order_items WHERE order_id=$1`, orderID)
	return err
}
