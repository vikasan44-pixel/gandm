package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AntifraudRepository struct {
	db Querier
}

func NewAntifraudRepository(db Querier) *AntifraudRepository {
	return &AntifraudRepository{db: db}
}

// --- избранное (ТЗ §6.2: «честно и открыто») ---

type FavoriteEntry struct {
	ParticipantID uuid.UUID `json:"participant_id"`
	CompanyName   string    `json:"company_name"`
	CreatedAt     time.Time `json:"created_at"`
}

func (r *AntifraudRepository) AddFavorite(ctx context.Context, clientID, participantID uuid.UUID) error {
	const q = `INSERT INTO favorites (client_id, participant_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, q, clientID, participantID)
	return err
}

func (r *AntifraudRepository) RemoveFavorite(ctx context.Context, clientID, participantID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM favorites WHERE client_id = $1 AND participant_id = $2`, clientID, participantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AntifraudRepository) ListFavorites(ctx context.Context, clientID uuid.UUID) ([]FavoriteEntry, error) {
	const q = `
		SELECT f.participant_id, COALESCE(NULLIF(u.company_name, ''), u.email), f.created_at
		FROM favorites f JOIN users u ON u.id = f.participant_id
		WHERE f.client_id = $1 ORDER BY f.created_at DESC
	`
	rows, err := r.db.Query(ctx, q, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]FavoriteEntry, 0)
	for rows.Next() {
		var e FavoriteEntry
		if err := rows.Scan(&e.ParticipantID, &e.CompanyName, &e.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

// --- документы сделок (ТЗ §6.2) ---

type DealDocument struct {
	ID           uuid.UUID `json:"id"`
	DealID       uuid.UUID `json:"deal_id"`
	UploaderID   uuid.UUID `json:"uploader_id"`
	FileURL      string    `json:"-"` // ключ MinIO; наружу — presigned URL
	OriginalName string    `json:"original_name"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

func (r *AntifraudRepository) CreateDealDocument(ctx context.Context, d *DealDocument) error {
	const q = `
		INSERT INTO deal_documents (id, deal_id, uploader_id, file_url, original_name, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, d.ID, d.DealID, d.UploaderID, d.FileURL, d.OriginalName, d.UploadedAt)
	return err
}

func (r *AntifraudRepository) ListDealDocuments(ctx context.Context, dealID uuid.UUID) ([]DealDocument, error) {
	const q = `
		SELECT id, deal_id, uploader_id, file_url, original_name, uploaded_at
		FROM deal_documents WHERE deal_id = $1 ORDER BY uploaded_at DESC
	`
	rows, err := r.db.Query(ctx, q, dealID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DealDocument, 0)
	for rows.Next() {
		var d DealDocument
		if err := rows.Scan(&d.ID, &d.DealID, &d.UploaderID, &d.FileURL, &d.OriginalName, &d.UploadedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

// CountDealsBetween — сколько завершённых сделок уже было у пары; >1 после
// только что закрытой сделки означает повторную работу с тем же партнёром.
func (r *AntifraudRepository) CountDealsBetween(ctx context.Context, a, b uuid.UUID) (int, error) {
	q := `SELECT count(*) FROM (` + completedDealsBetween + `) deals`
	var n int
	err := r.db.QueryRow(ctx, q, a, b).Scan(&n)
	return n, err
}

// --- подозрительные паттерны (ТЗ §6.1) ---

// SuspiciousPair: пара «клиент — исполнитель» с повторными сделками, у
// которой чаты молчат (нет двусторонней переписки) и/или нет документов.
// Система «не обвиняет — замечает аномалию»: строка попадает админу на
// проверку.
type SuspiciousPair struct {
	ClientID          uuid.UUID `json:"client_id"`
	ClientLabel       string    `json:"client_label"`
	ParticipantID     uuid.UUID `json:"participant_id"`
	ParticipantLabel  string    `json:"participant_label"`
	DealsCount        int       `json:"deals_count"`
	SilentChats       int       `json:"silent_chats"`
	DocumentedDeals   int       `json:"documented_deals"`
	IsFavorite        bool      `json:"is_favorite"`
	LastDealCreatedAt time.Time `json:"last_deal_created_at"`
}

// ListSuspiciousPairs: пары с minDeals+ завершёнными одиночными сделками.
// Консолидации в паттерне не участвуют — там всегда два клиента и общий
// конкурс, накрутка через них дороже.
func (r *AntifraudRepository) ListSuspiciousPairs(ctx context.Context, minDeals int) ([]SuspiciousPair, error) {
	const q = `
		WITH pair_deals AS (
			SELECT cr.client_id, rev.participant_id, cr.id AS deal_id, cr.created_at,
			       ch.id AS chat_id
			FROM cargo_requests cr
			JOIN contact_reveals rev ON rev.cargo_request_id = cr.id
			LEFT JOIN chats ch ON ch.cargo_request_id = cr.id
			WHERE cr.status = 'matched'
		),
		chat_senders AS (
			SELECT chat_id, count(DISTINCT sender_id) AS senders FROM messages GROUP BY chat_id
		)
		SELECT pd.client_id,
		       COALESCE(NULLIF(uc.company_name, ''), uc.email),
		       pd.participant_id,
		       COALESCE(NULLIF(up.company_name, ''), up.email),
		       count(*) AS deals,
		       count(*) FILTER (WHERE COALESCE(cs.senders, 0) < 2) AS silent_chats,
		       count(DISTINCT dd.deal_id) AS documented,
		       EXISTS (SELECT 1 FROM favorites f WHERE f.client_id = pd.client_id AND f.participant_id = pd.participant_id),
		       max(pd.created_at)
		FROM pair_deals pd
		JOIN users uc ON uc.id = pd.client_id
		JOIN users up ON up.id = pd.participant_id
		LEFT JOIN chat_senders cs ON cs.chat_id = pd.chat_id
		LEFT JOIN deal_documents dd ON dd.deal_id = pd.deal_id
		GROUP BY pd.client_id, uc.company_name, uc.email, pd.participant_id, up.company_name, up.email
		HAVING count(*) >= $1
		ORDER BY count(*) FILTER (WHERE COALESCE(cs.senders, 0) < 2) DESC, count(*) DESC
	`
	rows, err := r.db.Query(ctx, q, minDeals)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SuspiciousPair, 0)
	for rows.Next() {
		var p SuspiciousPair
		if err := rows.Scan(&p.ClientID, &p.ClientLabel, &p.ParticipantID, &p.ParticipantLabel, &p.DealsCount, &p.SilentChats, &p.DocumentedDeals, &p.IsFavorite, &p.LastDealCreatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}
