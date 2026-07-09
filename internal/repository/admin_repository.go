package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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

func (r *AdminRepository) List(ctx context.Context) ([]models.Admin, error) {
	q := `SELECT ` + adminColumns + ` FROM admins ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Admin, 0)
	for rows.Next() {
		var a models.Admin
		if err := rows.Scan(&a.ID, &a.Email, &a.PasswordHash, &a.Role, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *AdminRepository) Create(ctx context.Context, a *models.Admin) error {
	const q = `INSERT INTO admins (id, email, password_hash, role, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(ctx, q, a.ID, a.Email, a.PasswordHash, a.Role, a.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmailTaken
		}
		return err
	}
	return nil
}
