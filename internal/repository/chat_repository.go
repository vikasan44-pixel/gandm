package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
)

type ChatRepository struct {
	db Querier
}

func NewChatRepository(db Querier) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) Create(ctx context.Context, c *models.Chat) error {
	const q = `INSERT INTO chats (id, cargo_request_id, created_at) VALUES ($1, $2, $3)`
	_, err := r.db.Exec(ctx, q, c.ID, c.CargoRequestID, c.CreatedAt)
	return err
}

func (r *ChatRepository) AddParticipant(ctx context.Context, chatID, userID uuid.UUID) error {
	const q = `INSERT INTO chat_participants (chat_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, q, chatID, userID)
	return err
}

// IsParticipant is the single access-check primitive for chats: only rows
// in chat_participants grant read/write access to a chat.
func (r *ChatRepository) IsParticipant(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM chat_participants WHERE chat_id = $1 AND user_id = $2)`
	var exists bool
	err := r.db.QueryRow(ctx, q, chatID, userID).Scan(&exists)
	return exists, err
}

// ChatView is a chat enriched with what the list UI needs: the cargo route
// labels and a display label for the other participant. Identity here is
// fine — a chat only exists after the client revealed the contact.
type ChatView struct {
	ID               uuid.UUID `json:"id"`
	CargoRequestID   uuid.UUID `json:"cargo_request_id"`
	OriginLabel      string    `json:"origin_label"`
	DestinationLabel string    `json:"destination_label"`
	CounterpartLabel string    `json:"counterpart_label"`
	CreatedAt        time.Time `json:"created_at"`
}

func (r *ChatRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]ChatView, error) {
	const q = `
		SELECT c.id, c.cargo_request_id, cr.origin_label, cr.destination_label,
		       COALESCE(NULLIF(u.company_name, ''), u.email), c.created_at
		FROM chats c
		JOIN chat_participants me ON me.chat_id = c.id AND me.user_id = $1
		JOIN chat_participants other ON other.chat_id = c.id AND other.user_id <> $1
		JOIN users u ON u.id = other.user_id
		JOIN cargo_requests cr ON cr.id = c.cargo_request_id
		ORDER BY c.created_at DESC
	`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make([]ChatView, 0)
	for rows.Next() {
		var cv ChatView
		if err := rows.Scan(&cv.ID, &cv.CargoRequestID, &cv.OriginLabel, &cv.DestinationLabel, &cv.CounterpartLabel, &cv.CreatedAt); err != nil {
			return nil, err
		}
		chats = append(chats, cv)
	}
	return chats, rows.Err()
}
