package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"hw2/internal/domain"
)

type UserRepo struct{}

func NewUserRepo() *UserRepo { return &UserRepo{} }

func (r *UserRepo) Create(ctx context.Context, db DBTX, u *domain.User) error {
	u.ID = uuid.New()
	return db.QueryRow(ctx,
		`INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, $3, $4)
		 RETURNING created_at`,
		u.ID, u.Email, u.PasswordHash, u.Role,
	).Scan(&u.CreatedAt)
}

func (r *UserRepo) GetByEmail(ctx context.Context, db DBTX, email string) (*domain.User, error) {
	u := &domain.User{}
	err := db.QueryRow(ctx,
		`SELECT id, email, password_hash, role, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) GetByID(ctx context.Context, db DBTX, id uuid.UUID) (*domain.User, error) {
	u := &domain.User{}
	err := db.QueryRow(ctx,
		`SELECT id, email, password_hash, role, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) SaveRefreshToken(ctx context.Context, db DBTX, rt *domain.RefreshToken) error {
	rt.ID = uuid.New()
	_, err := db.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token, expires_at) VALUES ($1, $2, $3, $4)`,
		rt.ID, rt.UserID, rt.Token, rt.ExpiresAt)
	return err
}

func (r *UserRepo) GetRefreshToken(ctx context.Context, db DBTX, token string) (*domain.RefreshToken, error) {
	rt := &domain.RefreshToken{}
	err := db.QueryRow(ctx,
		`SELECT id, user_id, token, expires_at, created_at FROM refresh_tokens WHERE token = $1`, token,
	).Scan(&rt.ID, &rt.UserID, &rt.Token, &rt.ExpiresAt, &rt.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return rt, err
}

func (r *UserRepo) DeleteRefreshToken(ctx context.Context, db DBTX, token string) error {
	_, err := db.Exec(ctx, `DELETE FROM refresh_tokens WHERE token = $1`, token)
	return err
}

func (r *UserRepo) DeleteExpiredTokens(ctx context.Context, db DBTX, userID uuid.UUID) error {
	_, err := db.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE user_id = $1 AND expires_at < $2`, userID, time.Now())
	return err
}
