package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

var (
	ErrKeyTaken      = errors.New("tool key already exists")
	ErrInvalidToolID = errors.New("invalid tool id")
)

type ToolRepository struct {
	db Querier
}

func NewToolRepository(db Querier) *ToolRepository {
	return &ToolRepository{db: db}
}

const toolColumns = `id, key, name, description, category, is_active, price_kzt`

func scanTool(row pgx.Row) (*models.Tool, error) {
	var t models.Tool
	err := row.Scan(&t.ID, &t.Key, &t.Name, &t.Description, &t.Category, &t.IsActive, &t.PriceKZT)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ToolRepository) Create(ctx context.Context, t *models.Tool) error {
	const q = `INSERT INTO tools (id, key, name, description, category, is_active, price_kzt) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.Exec(ctx, q, t.ID, t.Key, t.Name, t.Description, t.Category, t.IsActive, t.PriceKZT)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrKeyTaken
		}
		return err
	}
	return nil
}

// UpsertByKey creates the tool or, if the key already exists, updates its
// mutable fields. On conflict, t.ID is overwritten with the existing row's
// id so callers (e.g. the seed script) get a stable id to reference.
func (r *ToolRepository) UpsertByKey(ctx context.Context, t *models.Tool) error {
	const q = `
		INSERT INTO tools (id, key, name, description, category, is_active, price_kzt)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (key) DO UPDATE SET
			key = EXCLUDED.key
		RETURNING id
	`
	return r.db.QueryRow(ctx, q, t.ID, t.Key, t.Name, t.Description, t.Category, t.IsActive, t.PriceKZT).Scan(&t.ID)
}

func (r *ToolRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tool, error) {
	q := `SELECT ` + toolColumns + ` FROM tools WHERE id = $1`
	return scanTool(r.db.QueryRow(ctx, q, id))
}

func (r *ToolRepository) Update(ctx context.Context, t *models.Tool) error {
	const q = `UPDATE tools SET name = $2, description = $3, category = $4, is_active = $5, price_kzt = $6 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, t.ID, t.Name, t.Description, t.Category, t.IsActive, t.PriceKZT)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ToolRepository) List(ctx context.Context) ([]models.Tool, error) {
	q := `SELECT ` + toolColumns + ` FROM tools ORDER BY category, name`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tools := make([]models.Tool, 0)
	for rows.Next() {
		var t models.Tool
		if err := rows.Scan(&t.ID, &t.Key, &t.Name, &t.Description, &t.Category, &t.IsActive, &t.PriceKZT); err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

func (r *ToolRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.Tool, error) {
	const q = `
		SELECT t.id, t.key, t.name, t.description, t.category, t.is_active, t.price_kzt
		FROM tools t
		JOIN user_tools ut ON ut.tool_id = t.id
		WHERE ut.user_id = $1
		ORDER BY t.category, t.name
	`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tools := make([]models.Tool, 0)
	for rows.Next() {
		var t models.Tool
		if err := rows.Scan(&t.ID, &t.Key, &t.Name, &t.Description, &t.Category, &t.IsActive, &t.PriceKZT); err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

// ListSelfSelectable — участнические инструменты, доступные для самовыбора
// при регистрации и в настройках (всё, кроме служебных admin-инструментов).
// Только активные.
func (r *ToolRepository) ListSelfSelectable(ctx context.Context) ([]models.Tool, error) {
	q := `SELECT ` + toolColumns + ` FROM tools WHERE is_active = true AND category <> 'admin' ORDER BY price_kzt, name`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tools := make([]models.Tool, 0)
	for rows.Next() {
		var t models.Tool
		if err := rows.Scan(&t.ID, &t.Key, &t.Name, &t.Description, &t.Category, &t.IsActive, &t.PriceKZT); err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

// SelfSelectableIDSet returns the ids of self-selectable tools as a set —
// used to reject admin/inactive tools when a user picks tools for himself.
func (r *ToolRepository) SelfSelectableIDSet(ctx context.Context) (map[uuid.UUID]bool, error) {
	tools, err := r.ListSelfSelectable(ctx)
	if err != nil {
		return nil, err
	}
	set := make(map[uuid.UUID]bool, len(tools))
	for _, t := range tools {
		set[t.ID] = true
	}
	return set, nil
}

// UserHasTool is the sole access-check primitive: it looks at tool
// possession, never at participant_type. Сотрудник компании (ТЗ §13.1)
// наследует инструменты своей компании — parent_company_id подставляется
// вторым кандидатом владельца инструмента.
func (r *ToolRepository) UserHasTool(ctx context.Context, userID uuid.UUID, key string) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM user_tools ut
			JOIN tools t ON t.id = ut.tool_id
			WHERE t.key = $2 AND t.is_active = true
			  AND (ut.user_id = $1
			       OR ut.user_id = (SELECT parent_company_id FROM users WHERE id = $1))
		)
	`
	var exists bool
	err := r.db.QueryRow(ctx, q, userID, key).Scan(&exists)
	return exists, err
}

// ListActiveUserIDsWithTool returns all active users holding an active tool
// — the notification fan-out audience for events that aren't tied to a
// route (e.g. customs competitions on matched consolidations).
func (r *ToolRepository) ListActiveUserIDsWithTool(ctx context.Context, key string) ([]uuid.UUID, error) {
	const q = `
		SELECT DISTINCT ut.user_id
		FROM user_tools ut
		JOIN tools t ON t.id = ut.tool_id
		JOIN users u ON u.id = ut.user_id
		WHERE t.key = $1 AND t.is_active = true AND u.status = 'active'
	`
	rows, err := r.db.Query(ctx, q, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListUserIDsWithToolAndRoute returns active users who both hold an active
// tool AND have a route whose endpoints are each within the per-country
// radius of the given cargo endpoints — the notification fan-out audience
// for a new cargo request. Same threshold rule as
// CargoRequestRepository.ListOpenMatchingUserRoutes: cn → cnKm, else kzKm,
// GREATEST of the pair for cross-border comparisons.
func (r *ToolRepository) ListUserIDsWithToolAndRoute(ctx context.Context, key string, origin, destination models.GeoPoint, cnKm, kzKm float64) ([]uuid.UUID, error) {
	const q = `
		SELECT DISTINCT ut.user_id
		FROM user_tools ut
		JOIN tools t ON t.id = ut.tool_id
		JOIN users u ON u.id = ut.user_id
		JOIN participant_routes pr ON pr.user_id = ut.user_id
		WHERE t.key = $1 AND t.is_active = true AND u.status = 'active'
		  -- Широтная полоса (sargable) до haversine — индекс из миграции
		  -- 000030, точность даёт haversine ниже.
		  AND pr.origin_lat BETWEEN $2::float8 - GREATEST($8::float8, $9::float8) / 110.0
		                        AND $2::float8 + GREATEST($8::float8, $9::float8) / 110.0
		  AND haversine_km(pr.origin_lat, pr.origin_lng, $2, $3)
		      <= GREATEST(
		           CASE WHEN pr.origin_country = 'cn' THEN $8::float8 ELSE $9::float8 END,
		           CASE WHEN $4::text = 'cn' THEN $8::float8 ELSE $9::float8 END)
		  AND haversine_km(pr.destination_lat, pr.destination_lng, $5, $6)
		      <= GREATEST(
		           CASE WHEN pr.destination_country = 'cn' THEN $8::float8 ELSE $9::float8 END,
		           CASE WHEN $7::text = 'cn' THEN $8::float8 ELSE $9::float8 END)
	`
	rows, err := r.db.Query(ctx, q,
		key,
		origin.Lat, origin.Lng, origin.Country,
		destination.Lat, destination.Lng, destination.Country,
		cnKm, kzKm,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ReplaceUserTools atomically sets the exact set of tools assigned to a
// user, replacing whatever was assigned before (checkbox-list semantics,
// not incremental add/remove).
func (r *ToolRepository) ReplaceUserTools(ctx context.Context, userID uuid.UUID, toolIDs []uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM user_tools WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, tid := range toolIDs {
		if _, err := r.db.Exec(ctx, `INSERT INTO user_tools (user_id, tool_id) VALUES ($1, $2)`, userID, tid); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return ErrInvalidToolID
			}
			return err
		}
	}
	return nil
}
