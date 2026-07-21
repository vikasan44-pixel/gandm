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

const userColumns = `id, email, phone, company_name, legal_form, participant_type, password_hash, status, has_subscription, language, created_at, last_active_at`

func NewUserRepository(db Querier) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	const q = `
		INSERT INTO users (id, email, phone, company_name, legal_form, participant_type, password_hash, status, has_subscription, language, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, q,
		u.ID, u.Email, u.Phone, u.CompanyName, u.LegalForm, u.ParticipantType, u.PasswordHash, u.Status, u.HasSubscription, u.Language, u.CreatedAt,
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
		SELECT ` + userColumns + `
		FROM users WHERE id = $1
	`
	var u models.User
	err := r.db.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.Phone, &u.CompanyName, &u.LegalForm, &u.ParticipantType, &u.PasswordHash, &u.Status, &u.HasSubscription, &u.Language, &u.CreatedAt, &u.LastActiveAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByIDForUpdate is GetByID with a row lock — serializes operations that
// consume per-user quotas (contact reveals), so parallel requests can't
// both pass the limit check. Only meaningful inside a transaction.
// ListByIDs fetches users for a set of ids in a single query, keyed by id.
// Used to avoid N+1 lookups when revealing counterparties/contacts. Ids absent
// from the table are simply missing from the map.
func (r *UserRepository) ListByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*models.User, error) {
	result := make(map[uuid.UUID]*models.User, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	const q = `
		SELECT ` + userColumns + `
		FROM users WHERE id = ANY($1)
	`
	rows, err := r.db.Query(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u models.User
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Phone, &u.CompanyName, &u.LegalForm, &u.ParticipantType, &u.PasswordHash, &u.Status, &u.HasSubscription, &u.Language, &u.CreatedAt, &u.LastActiveAt,
		); err != nil {
			return nil, err
		}
		result[u.ID] = &u
	}
	return result, rows.Err()
}

func (r *UserRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `
		SELECT ` + userColumns + `
		FROM users WHERE id = $1 FOR UPDATE
	`
	var u models.User
	err := r.db.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.Phone, &u.CompanyName, &u.LegalForm, &u.ParticipantType, &u.PasswordHash, &u.Status, &u.HasSubscription, &u.Language, &u.CreatedAt, &u.LastActiveAt,
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
		SELECT ` + userColumns + `
		FROM users WHERE email = $1
	`
	var u models.User
	err := r.db.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.Phone, &u.CompanyName, &u.LegalForm, &u.ParticipantType, &u.PasswordHash, &u.Status, &u.HasSubscription, &u.Language, &u.CreatedAt, &u.LastActiveAt,
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
	ServiceType string
	Status      *models.UserStatus
	Search      string // matches email, company_name or phone (case-insensitive substring)
}

func (r *UserRepository) List(ctx context.Context, f UserFilter) ([]models.User, error) {
	users, _, err := r.ListPage(ctx, f, 1000, 0)
	return users, err
}

func (r *UserRepository) ListPage(ctx context.Context, f UserFilter, limit, offset int) ([]models.User, int, error) {
	query := `
		SELECT ` + userColumns + `
		FROM users WHERE 1=1
	`
	args := make([]any, 0, 5)
	if f.ServiceType != "" {
		args = append(args, f.ServiceType)
		n := len(args)
		query += fmt.Sprintf(` AND CASE
			WHEN $%[1]d = 'warehouse' THEN EXISTS (SELECT 1 FROM user_tools ut JOIN tools t ON t.id=ut.tool_id WHERE ut.user_id=users.id AND t.key='manage_warehouse_slots')
			WHEN $%[1]d = 'carrier' THEN EXISTS (SELECT 1 FROM user_tools ut JOIN tools t ON t.id=ut.tool_id WHERE ut.user_id=users.id AND t.key IN ('manage_fleet','receive_cargo_by_route','submit_offer'))
			WHEN $%[1]d = 'customs_rep' THEN EXISTS (SELECT 1 FROM user_tools ut JOIN tools t ON t.id=ut.tool_id WHERE ut.user_id=users.id AND t.key='manage_customs_docs')
			WHEN $%[1]d = 'client' THEN NOT EXISTS (SELECT 1 FROM user_tools ut JOIN tools t ON t.id=ut.tool_id WHERE ut.user_id=users.id AND t.key IN ('manage_warehouse_slots','manage_fleet','receive_cargo_by_route','submit_offer','manage_customs_docs'))
			ELSE participant_type::text = $%[1]d END`, n)
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
	countQuery := strings.Replace(query, "SELECT "+userColumns, "SELECT COUNT(*)", 1)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Phone, &u.CompanyName, &u.LegalForm, &u.ParticipantType, &u.PasswordHash, &u.Status, &u.HasSubscription, &u.Language, &u.CreatedAt, &u.LastActiveAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
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

func (r *UserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, name string, legalForm models.LegalForm) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET company_name = $2, legal_form = $3 WHERE id = $1`, id, name, legalForm)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) UpdateParticipantType(ctx context.Context, id uuid.UUID, participantType models.ParticipantType) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET participant_type = $2 WHERE id = $1`, id, participantType)
	return err
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
