package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserOpRepo struct{}

func NewUserOpRepo() *UserOpRepo { return &UserOpRepo{} }

func (r *UserOpRepo) GetLastOperation(ctx context.Context, db DBTX, userID uuid.UUID, opType string) (*time.Time, error) {
	var t time.Time
	err := db.QueryRow(ctx,
		`SELECT created_at FROM user_operations
		 WHERE user_id=$1 AND operation_type=$2
		 ORDER BY created_at DESC LIMIT 1`, userID, opType,
	).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *UserOpRepo) Create(ctx context.Context, db DBTX, userID uuid.UUID, opType string) error {
	_, err := db.Exec(ctx,
		`INSERT INTO user_operations (id, user_id, operation_type) VALUES ($1, $2, $3)`,
		uuid.New(), userID, opType)
	return err
}
