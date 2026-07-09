package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type ChatRepository struct {
	db Querier
}

func NewChatRepository(db Querier) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) Create(ctx context.Context, c *models.Chat) error {
	const q = `INSERT INTO chats (id, cargo_request_id, consolidated_request_id, driver_competition_id, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(ctx, q, c.ID, c.CargoRequestID, c.ConsolidatedRequestID, c.DriverCompetitionID, c.CreatedAt)
	return err
}

// GetByDriverCompetitionID resolves the warehouse+driver chat of a closed
// competition — used by the idempotent replay of SelectDriverBid.
func (r *ChatRepository) GetByDriverCompetitionID(ctx context.Context, competitionID uuid.UUID) (*models.Chat, error) {
	const q = `SELECT id, cargo_request_id, consolidated_request_id, driver_competition_id, created_at FROM chats WHERE driver_competition_id = $1`
	var c models.Chat
	err := r.db.QueryRow(ctx, q, competitionID).Scan(&c.ID, &c.CargoRequestID, &c.ConsolidatedRequestID, &c.DriverCompetitionID, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
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

// ChatView is a chat enriched with what the list UI needs: the route labels
// (from either the cargo request or the consolidated request) and a display
// label for the other participants. Identity here is fine — a chat only
// exists after a reveal/accept.
type ChatView struct {
	ID               uuid.UUID `json:"id"`
	OriginLabel      string    `json:"origin_label"`
	DestinationLabel string    `json:"destination_label"`
	CounterpartLabel string    `json:"counterpart_label"`
	// CounterpartUserID is set only for two-party chats — it (plus DealID)
	// powers the "rate your counterparty" form in the chat window. Group
	// chats (consolidation with carrier) get nil: the rating target there
	// is picked on the consolidated panel instead.
	CounterpartUserID *uuid.UUID `json:"counterpart_user_id,omitempty"`
	// DealID is the underlying cargo_request or consolidated_request id.
	DealID    uuid.UUID `json:"deal_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *ChatRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]ChatView, error) {
	// string_agg collapses multi-party chats (consolidation chat has two
	// clients and, after the deal, the carrier) into one row per chat.
	const q = `
		SELECT c.id,
		       COALESCE(cr.origin_label, cons.origin_label, dpr.origin_label, ''),
		       COALESCE(cr.destination_label, cons.destination_label, dpr.destination_label, ''),
		       string_agg(DISTINCT COALESCE(NULLIF(u.company_name, ''), u.email), ', '),
		       CASE WHEN count(DISTINCT other.user_id) = 1 THEN (min(other.user_id::text))::uuid ELSE NULL END,
		       COALESCE(c.cargo_request_id, c.consolidated_request_id, c.driver_competition_id),
		       c.created_at
		FROM chats c
		JOIN chat_participants me ON me.chat_id = c.id AND me.user_id = $1
		JOIN chat_participants other ON other.chat_id = c.id AND other.user_id <> $1
		JOIN users u ON u.id = other.user_id
		LEFT JOIN cargo_requests cr ON cr.id = c.cargo_request_id
		LEFT JOIN consolidated_requests cons ON cons.id = c.consolidated_request_id
		LEFT JOIN driver_competitions dc ON dc.id = c.driver_competition_id
		LEFT JOIN participant_routes dpr ON dpr.id = dc.route_id
		GROUP BY c.id, cr.origin_label, cr.destination_label, cons.origin_label, cons.destination_label, dpr.origin_label, dpr.destination_label, c.cargo_request_id, c.consolidated_request_id, c.driver_competition_id, c.created_at
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
		if err := rows.Scan(&cv.ID, &cv.OriginLabel, &cv.DestinationLabel, &cv.CounterpartLabel, &cv.CounterpartUserID, &cv.DealID, &cv.CreatedAt); err != nil {
			return nil, err
		}
		chats = append(chats, cv)
	}
	return chats, rows.Err()
}
