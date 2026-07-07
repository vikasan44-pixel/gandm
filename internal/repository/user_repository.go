package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

var (
	ErrEmailTaken = errors.New("email already registered")
	ErrNotFound   = errors.New("not found")
)

type UserRepository struct {
	db Querier
}

func NewUserRepository(db Querier) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	const q = `
		INSERT INTO users (id, email, phone, company_name, participant_type, password_hash, status, has_subscription, language, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, q,
		u.ID, u.Email, u.Phone, u.CompanyName, u.ParticipantType, u.PasswordHash, u.Status, u.HasSubscription, u.Language, u.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmailTaken
		}
		return err
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `
		SELECT id, email, phone, company_name, participant_type, password_hash, status, has_subscription, language, created_at, last_active_at
		FROM users WHERE id = $1
	`
	var u models.User
	err := r.db.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.Phone, &u.CompanyName, &u.ParticipantType, &u.PasswordHash, &u.Status, &u.HasSubscription, &u.Language, &u.CreatedAt, &u.LastActiveAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `
		SELECT id, email, phone, company_name, participant_type, password_hash, status, has_subscription, language, created_at, last_active_at
		FROM users WHERE email = $1
	`
	var u models.User
	err := r.db.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.Phone, &u.CompanyName, &u.ParticipantType, &u.PasswordHash, &u.Status, &u.HasSubscription, &u.Language, &u.CreatedAt, &u.LastActiveAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UserFilter narrows the admin user list. Nil/empty fields are not filtered on.
type UserFilter struct {
	ParticipantType *models.ParticipantType
	Status          *models.UserStatus
	Search          string // matches email, company_name or phone (case-insensitive substring)
}

func (r *UserRepository) List(ctx context.Context, f UserFilter) ([]models.User, error) {
	query := `
		SELECT id, email, phone, company_name, participant_type, password_hash, status, has_subscription, language, created_at, last_active_at
		FROM users WHERE 1=1
	`
	args := make([]any, 0, 3)
	if f.ParticipantType != nil {
		args = append(args, *f.ParticipantType)
		query += fmt.Sprintf(" AND participant_type = $%d", len(args))
	}
	if f.Status != nil {
		args = append(args, *f.Status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if strings.TrimSpace(f.Search) != "" {
		args = append(args, "%"+strings.TrimSpace(f.Search)+"%")
		n := len(args)
		query += fmt.Sprintf(" AND (email ILIKE $%d OR company_name ILIKE $%d OR phone ILIKE $%d)", n, n, n)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Phone, &u.CompanyName, &u.ParticipantType, &u.PasswordHash, &u.Status, &u.HasSubscription, &u.Language, &u.CreatedAt, &u.LastActiveAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.UserStatus) error {
	const q = `UPDATE users SET status = $2 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) UpdateSubscription(ctx context.Context, id uuid.UUID, hasSubscription bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET has_subscription = $2 WHERE id = $1`, id, hasSubscription)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) TouchLastActive(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET last_active_at = now() WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id)
	return err
}

func (r *UserRepository) CountByStatus(ctx context.Context, status models.UserStatus) (int, error) {
	const q = `SELECT count(*) FROM users WHERE status = $1`
	var n int
	err := r.db.QueryRow(ctx, q, status).Scan(&n)
	return n, err
}

func (r *UserRepository) CountCreatedSince(ctx context.Context, since time.Time) (int, error) {
	const q = `SELECT count(*) FROM users WHERE created_at >= $1`
	var n int
	err := r.db.QueryRow(ctx, q, since).Scan(&n)
	return n, err
}

func (r *UserRepository) CountActiveSince(ctx context.Context, since time.Time) (int, error) {
	const q = `SELECT count(*) FROM users WHERE last_active_at >= $1`
	var n int
	err := r.db.QueryRow(ctx, q, since).Scan(&n)
	return n, err
}
