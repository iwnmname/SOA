package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"hw2/internal/domain"
	"hw2/internal/repository"
)

var (
	ErrNotFound               = errors.New("product not found")
	ErrInvalidStateTransition = errors.New("invalid state transition")
)

type ProductService struct {
	db   *pgxpool.Pool
	repo *repository.ProductRepo
}

func NewProductService(db *pgxpool.Pool, repo *repository.ProductRepo) *ProductService {
	return &ProductService{db: db, repo: repo}
}

func (s *ProductService) Create(ctx context.Context, p *domain.Product) error {
	return s.repo.Create(ctx, s.db, p)
}

func (s *ProductService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	p, err := s.repo.GetByID(ctx, s.db, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	return p, err
}

func (s *ProductService) Update(ctx context.Context, id uuid.UUID, upd *domain.Product) (*domain.Product, error) {
	existing, err := s.repo.GetByID(ctx, s.db, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if existing.Status == "ARCHIVED" {
		return nil, ErrInvalidStateTransition
	}
	upd.ID = id
	upd.CreatedAt = existing.CreatedAt
	upd.SellerID = existing.SellerID
	if err := s.repo.Update(ctx, s.db, upd); err != nil {
		return nil, err
	}
	return upd, nil
}

func (s *ProductService) Delete(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	existing, err := s.repo.GetByID(ctx, s.db, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if existing.Status == "ARCHIVED" {
		return nil, ErrInvalidStateTransition
	}
	existing.Status = "ARCHIVED"
	if err := s.repo.Update(ctx, s.db, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *ProductService) List(ctx context.Context, f repository.ProductFilter) ([]*domain.Product, int64, error) {
	return s.repo.List(ctx, s.db, f)
}
