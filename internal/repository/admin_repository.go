package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type AdminRepository struct {
	db Querier
}

func NewAdminRepository(db Querier) *AdminRepository {
	return &AdminRepository{db: db}
}

const adminColumns = `id, email, password_hash, role, created_at`

func scanAdmin(row pgx.Row) (*models.Admin, error) {
	var a models.Admin
	err := row.Scan(&a.ID, &a.Email, &a.PasswordHash, &a.Role, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AdminRepository) GetByEmail(ctx context.Context, email string) (*models.Admin, error) {
	q := `SELECT ` + adminColumns + ` FROM admins WHERE email = $1`
	return scanAdmin(r.db.QueryRow(ctx, q, email))
}

func (r *AdminRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Admin, error) {
	q := `SELECT ` + adminColumns + ` FROM admins WHERE id = $1`
	return scanAdmin(r.db.QueryRow(ctx, q, id))
}
