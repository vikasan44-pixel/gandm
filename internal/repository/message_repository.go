package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type MessageRepository struct {
	db Querier
}

func NewMessageRepository(db Querier) *MessageRepository {
	return &MessageRepository{db: db}
}

const messageColumns = `id, chat_id, sender_id, body, attachment_url, created_at`

func (r *MessageRepository) Create(ctx context.Context, m *models.Message) error {
	const q = `
		INSERT INTO messages (id, chat_id, sender_id, body, attachment_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, m.ID, m.ChatID, m.SenderID, m.Body, m.AttachmentURL, m.CreatedAt)
	return err
}

func (r *MessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	q := `SELECT ` + messageColumns + ` FROM messages WHERE id = $1`
	var m models.Message
	err := r.db.QueryRow(ctx, q, id).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Body, &m.AttachmentURL, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListByChatID returns messages oldest-first. A non-nil after narrows the
// result to messages created strictly later — the polling cursor. Hard cap
// of 200 per request until real pagination is needed.
func (r *MessageRepository) ListByChatID(ctx context.Context, chatID uuid.UUID, after *time.Time) ([]models.Message, error) {
	q := `SELECT ` + messageColumns + ` FROM messages WHERE chat_id = $1`
	args := []any{chatID}
	if after != nil {
		args = append(args, *after)
		q += ` AND created_at > $2`
	}
	q += ` ORDER BY created_at ASC LIMIT 200`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Message, 0)
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Body, &m.AttachmentURL, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}
