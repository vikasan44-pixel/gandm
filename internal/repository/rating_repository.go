package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

var ErrAlreadyRated = errors.New("this deal is already rated by this user")

type RatingRepository struct {
	db Querier
}

func NewRatingRepository(db Querier) *RatingRepository {
	return &RatingRepository{db: db}
}

func (r *RatingRepository) Create(ctx context.Context, rt *models.Rating) error {
	const q = `
		INSERT INTO ratings (id, deal_id, rated_user_id, rater_user_id, score, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, q, rt.ID, rt.DealID, rt.RatedUserID, rt.RaterUserID, rt.Score, rt.Comment, rt.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyRated
		}
		return err
	}
	return nil
}

// UserRatingSummary: average rounded to one decimal; Average is nil when
// there are no ratings — "no ratings yet" must never look like a real 0.
type UserRatingSummary struct {
	Average *float64 `json:"average"`
	Count   int      `json:"count"`
}

func (r *RatingRepository) SummaryForUser(ctx context.Context, userID uuid.UUID) (UserRatingSummary, error) {
	const q = `SELECT ROUND(AVG(score)::numeric, 1), COUNT(*) FROM ratings WHERE rated_user_id = $1`
	var s UserRatingSummary
	err := r.db.QueryRow(ctx, q, userID).Scan(&s.Average, &s.Count)
	return s, err
}

// SummariesForUsers bulk-fetches rating summaries for the anonymized offer
// lists (one query instead of one per offer).
func (r *RatingRepository) SummariesForUsers(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]UserRatingSummary, error) {
	out := make(map[uuid.UUID]UserRatingSummary, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT rated_user_id, ROUND(AVG(score)::numeric, 1), COUNT(*)
		FROM ratings
		WHERE rated_user_id = ANY($1)
		GROUP BY rated_user_id
	`
	rows, err := r.db.Query(ctx, q, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var s UserRatingSummary
		if err := rows.Scan(&id, &s.Average, &s.Count); err != nil {
			return nil, err
		}
		out[id] = s
	}
	return out, rows.Err()
}

// RatingComponents — сырьё многофакторного рейтинга (ТЗ §8): отзывы, срок
// на платформе, объём завершённых сделок, активность в чатах и доля чатов,
// где писали обе стороны (работа ведётся внутри платформы).
type RatingComponents struct {
	ReviewAvg      *float64 `json:"review_avg"`
	ReviewCount    int      `json:"review_count"`
	DaysOnPlatform int      `json:"days_on_platform"`
	CompletedDeals int      `json:"completed_deals"`
	ChatMessages   int      `json:"chat_messages"`
	ChatsTotal     int      `json:"chats_total"`
	ChatsActive    int      `json:"chats_active"`
}

// ComponentsForUsers bulk-fetches everything the composite rating needs in
// one query. Deal counting mirrors completedDealsBetween: matched single
// cargo via contact_reveals (both roles) + matched consolidated via
// selected offers (carrier) and selections (clients).
func (r *RatingRepository) ComponentsForUsers(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]RatingComponents, error) {
	out := make(map[uuid.UUID]RatingComponents, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT u.id,
		       rv.avg,
		       COALESCE(rv.cnt, 0),
		       GREATEST(0, EXTRACT(epoch FROM now() - u.created_at) / 86400)::int,
		       COALESCE(d.cnt, 0),
		       COALESCE(m.cnt, 0),
		       COALESCE(ch.total, 0),
		       COALESCE(ch.active, 0)
		FROM users u
		LEFT JOIN (
			SELECT rated_user_id, ROUND(AVG(score)::numeric, 1) AS avg, count(*) AS cnt
			FROM ratings WHERE rated_user_id = ANY($1) GROUP BY 1
		) rv ON rv.rated_user_id = u.id
		LEFT JOIN (
			SELECT uid, count(*) AS cnt FROM (
				SELECT rev.participant_id AS uid
				FROM contact_reveals rev JOIN cargo_requests cr ON cr.id = rev.cargo_request_id AND cr.status = 'matched'
				UNION ALL
				SELECT cr2.client_id
				FROM contact_reveals rev2 JOIN cargo_requests cr2 ON cr2.id = rev2.cargo_request_id AND cr2.status = 'matched'
				UNION ALL
				SELECT o.participant_id
				FROM offers o JOIN consolidated_requests cons ON cons.id = o.consolidated_request_id AND cons.status = 'matched'
				WHERE o.status = 'selected'
				UNION ALL
				SELECT sel.client_id
				FROM consolidated_selections sel JOIN consolidated_requests cons2 ON cons2.id = sel.consolidated_request_id AND cons2.status = 'matched'
			) t WHERE uid = ANY($1) GROUP BY uid
		) d ON d.uid = u.id
		LEFT JOIN (
			SELECT sender_id, count(*) AS cnt FROM messages WHERE sender_id = ANY($1) GROUP BY 1
		) m ON m.sender_id = u.id
		LEFT JOIN (
			SELECT cp.user_id, count(*) AS total, count(*) FILTER (WHERE ms.senders >= 2) AS active
			FROM chat_participants cp
			LEFT JOIN (SELECT chat_id, count(DISTINCT sender_id) AS senders FROM messages GROUP BY chat_id) ms
			       ON ms.chat_id = cp.chat_id
			WHERE cp.user_id = ANY($1)
			GROUP BY cp.user_id
		) ch ON ch.user_id = u.id
		WHERE u.id = ANY($1)
	`
	rows, err := r.db.Query(ctx, q, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var c RatingComponents
		if err := rows.Scan(&id, &c.ReviewAvg, &c.ReviewCount, &c.DaysOnPlatform, &c.CompletedDeals, &c.ChatMessages, &c.ChatsTotal, &c.ChatsActive); err != nil {
			return nil, err
		}
		out[id] = c
	}
	return out, rows.Err()
}

func (r *RatingRepository) ListReceived(ctx context.Context, userID uuid.UUID) ([]models.Rating, error) {
	const q = `
		SELECT id, deal_id, rated_user_id, rater_user_id, score, comment, created_at
		FROM ratings WHERE rated_user_id = $1 ORDER BY created_at DESC LIMIT 100
	`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Rating, 0)
	for rows.Next() {
		var rt models.Rating
		if err := rows.Scan(&rt.ID, &rt.DealID, &rt.RatedUserID, &rt.RaterUserID, &rt.Score, &rt.Comment, &rt.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, rt)
	}
	return items, rows.Err()
}

// completedDealsBetween finds ids of COMPLETED deals where a and b are the
// two counterparties: matched single cargo (client ↔ revealed participant)
// and matched consolidated requests (member client ↔ selected carrier).
const completedDealsBetween = `
	SELECT cr.id
	FROM cargo_requests cr
	JOIN contact_reveals rev ON rev.cargo_request_id = cr.id
	WHERE cr.status = 'matched' AND (
		(cr.client_id = $1 AND rev.participant_id = $2) OR
		(cr.client_id = $2 AND rev.participant_id = $1)
	)
	UNION
	SELECT cons.id
	FROM consolidated_requests cons
	JOIN consolidated_selections sel ON sel.consolidated_request_id = cons.id
	JOIN offers o ON o.id = sel.offer_id
	WHERE cons.status = 'matched' AND (
		(sel.client_id = $1 AND o.participant_id = $2) OR
		(sel.client_id = $2 AND o.participant_id = $1)
	)
`

// FindDealBetween returns some completed deal between the two users —
// used to backfill deal_id when the caller omits it (keeps the UNIQUE
// constraint meaningful).
func (r *RatingRepository) FindDealBetween(ctx context.Context, a, b uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, completedDealsBetween+` LIMIT 1`, a, b).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// IsDealBetween verifies that the specific deal id is a completed deal with
// exactly these two users as counterparties.
func (r *RatingRepository) IsDealBetween(ctx context.Context, dealID, a, b uuid.UUID) (bool, error) {
	q := `SELECT EXISTS (SELECT 1 FROM (` + completedDealsBetween + `) deals WHERE deals.id = $3)`
	var ok bool
	err := r.db.QueryRow(ctx, q, a, b, dealID).Scan(&ok)
	return ok, err
}
