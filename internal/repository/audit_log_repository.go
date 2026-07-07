package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
)

type AuditLogRepository struct {
	db Querier
}

func NewAuditLogRepository(db Querier) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

func (r *AuditLogRepository) Create(ctx context.Context, e *models.AuditLog) error {
	const q = `
		INSERT INTO audit_log (id, admin_id, action, target_user_id, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, e.ID, e.AdminID, e.Action, e.TargetUserID, e.Details, e.CreatedAt)
	return err
}

// AuditLogEntry enriches the raw audit_log row with the admin's email and a
// human-readable label for the target user — powering the dashboard's
// activity feed without the frontend needing to resolve UUIDs itself.
type AuditLogEntry struct {
	ID           uuid.UUID       `json:"id"`
	AdminID      uuid.UUID       `json:"admin_id"`
	AdminEmail   string          `json:"admin_email"`
	Action       string          `json:"action"`
	TargetUserID *uuid.UUID      `json:"target_user_id,omitempty"`
	TargetLabel  *string         `json:"target_label,omitempty"`
	Details      json.RawMessage `json:"details,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// List returns the most recent actions first — ordered by created_at, i.e.
// the actual time the admin acted, not the creation time of whatever record
// the action was on.
func (r *AuditLogRepository) List(ctx context.Context, limit, offset int) ([]AuditLogEntry, error) {
	const q = `
		SELECT al.id, al.admin_id, a.email, al.action, al.target_user_id,
		       COALESCE(u.company_name, u.email), al.details, al.created_at
		FROM audit_log al
		JOIN admins a ON a.id = al.admin_id
		LEFT JOIN users u ON u.id = al.target_user_id
		ORDER BY al.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AuditLogEntry, 0)
	for rows.Next() {
		var e AuditLogEntry
		if err := rows.Scan(
			&e.ID, &e.AdminID, &e.AdminEmail, &e.Action, &e.TargetUserID,
			&e.TargetLabel, &e.Details, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}
