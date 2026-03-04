package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"hw2/internal/domain"
)

type ProductRepo struct{}

func NewProductRepo() *ProductRepo {
	return &ProductRepo{}
}

func (r *ProductRepo) Create(ctx context.Context, db DBTX, p *domain.Product) error {
	p.ID = uuid.New()
	return db.QueryRow(ctx,
		`INSERT INTO products (id, name, description, price, stock, category, status, seller_id)
		 VALUES ($1, $2, $3, $4, $5, $6, 'ACTIVE', $7)
		 RETURNING created_at, updated_at`,
		p.ID, p.Name, p.Description, p.Price, p.Stock, p.Category, p.SellerID,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
}

func (r *ProductRepo) GetByID(ctx context.Context, db DBTX, id uuid.UUID) (*domain.Product, error) {
	p := &domain.Product{}
	err := db.QueryRow(ctx,
		`SELECT id, name, description, price, stock, category, status, seller_id, created_at, updated_at
		 FROM products WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock,
		&p.Category, &p.Status, &p.SellerID, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *ProductRepo) GetByIDForUpdate(ctx context.Context, db DBTX, id uuid.UUID) (*domain.Product, error) {
	p := &domain.Product{}
	err := db.QueryRow(ctx,
		`SELECT id, name, description, price, stock, category, status, seller_id, created_at, updated_at
		 FROM products WHERE id = $1 FOR UPDATE`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock,
		&p.Category, &p.Status, &p.SellerID, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *ProductRepo) Update(ctx context.Context, db DBTX, p *domain.Product) error {
	err := db.QueryRow(ctx,
		`UPDATE products SET name=$2, description=$3, price=$4, stock=$5, category=$6, status=$7, seller_id=$8
		 WHERE id=$1 RETURNING updated_at`,
		p.ID, p.Name, p.Description, p.Price, p.Stock, p.Category, p.Status, p.SellerID,
	).Scan(&p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (r *ProductRepo) UpdateStock(ctx context.Context, db DBTX, id uuid.UUID, delta int) error {
	_, err := db.Exec(ctx,
		`UPDATE products SET stock = stock + $2 WHERE id = $1`, id, delta)
	return err
}

func (r *ProductRepo) List(ctx context.Context, db DBTX, f ProductFilter) ([]*domain.Product, int64, error) {
	var conditions []string
	var args []any
	idx := 1

	if f.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.Category != nil {
		conditions = append(conditions, fmt.Sprintf("category = $%d", idx))
		args = append(args, *f.Category)
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	err := db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM products %s", where), args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, name, description, price, stock, category, status, seller_id, created_at, updated_at
		 FROM products %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, f.Size, f.Page*f.Size)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		p := &domain.Product{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock,
			&p.Category, &p.Status, &p.SellerID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	return products, total, nil
}

type ProductFilter struct {
	Status   *string
	Category *string
	Page     int
	Size     int
}
