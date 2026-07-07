package repository

import (
	"context"

	"github.com/google/uuid"

	"gandm/internal/models"
)

type NotificationRepository struct {
	db Querier
}

func NewNotificationRepository(db Querier) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(ctx context.Context, n *models.Notification) error {
	const q = `
		INSERT INTO notifications (id, user_id, type, payload, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, n.ID, n.UserID, n.Type, n.Payload, n.IsRead, n.CreatedAt)
	return err
}

// MarkAllReadByUserID flags every unread notification of the user as read.
// Idempotent: zero affected rows is not an error.
func (r *NotificationRepository) MarkAllReadByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE notifications SET is_read = true WHERE user_id = $1 AND is_read = false`, userID)
	return err
}

// ListByUserID returns the user's most recent notifications. Hard cap of
// 100 until real pagination is needed — enough for the smoke test and any
// near-term UI.
func (r *NotificationRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.Notification, error) {
	const q = `
		SELECT id, user_id, type, payload, is_read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Notification, 0)
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Payload, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	return items, rows.Err()
}
