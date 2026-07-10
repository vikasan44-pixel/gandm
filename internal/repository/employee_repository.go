package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

// EmployeeRepository — суб-аккаунты сотрудников (ТЗ §13.1). Сотрудник живёт
// в users с заполненным parent_company_id; остальной код видит его как
// обычного участника.
type EmployeeRepository struct {
	db Querier
}

func NewEmployeeRepository(db Querier) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

// Employee — представление сотрудника для компании-владельца.
type Employee struct {
	ID        uuid.UUID         `json:"id"`
	Email     string            `json:"email"`
	Phone     string            `json:"phone"`
	Status    models.UserStatus `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
}

func (r *EmployeeRepository) Create(ctx context.Context, u *models.User, parentCompanyID uuid.UUID) error {
	const q = `
		INSERT INTO users (id, email, phone, company_name, participant_type, password_hash, status, has_subscription, language, created_at, parent_company_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, q,
		u.ID, u.Email, u.Phone, u.CompanyName, u.ParticipantType, u.PasswordHash, u.Status, u.HasSubscription, u.Language, u.CreatedAt, parentCompanyID,
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

func (r *EmployeeRepository) ListByCompanyID(ctx context.Context, companyID uuid.UUID) ([]Employee, error) {
	const q = `
		SELECT id, email, phone, status, created_at
		FROM users WHERE parent_company_id = $1 ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, q, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Employee, 0)
	for rows.Next() {
		var e Employee
		if err := rows.Scan(&e.ID, &e.Email, &e.Phone, &e.Status, &e.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

// GetForCompany возвращает сотрудника, только если он принадлежит этой
// компании — чужие сотрудники читаются как not-found.
func (r *EmployeeRepository) GetForCompany(ctx context.Context, companyID, employeeID uuid.UUID) (*Employee, error) {
	const q = `
		SELECT id, email, phone, status, created_at
		FROM users WHERE id = $1 AND parent_company_id = $2
	`
	var e Employee
	err := r.db.QueryRow(ctx, q, employeeID, companyID).Scan(&e.ID, &e.Email, &e.Phone, &e.Status, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// SetStatus переключает сотрудника active/blocked (деактивация компанией).
func (r *EmployeeRepository) SetStatus(ctx context.Context, companyID, employeeID uuid.UUID, status models.UserStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET status = $3 WHERE id = $1 AND parent_company_id = $2`, employeeID, companyID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
