package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

type PermissionSetRepository struct {
	db Querier
}

func NewPermissionSetRepository(db Querier) *PermissionSetRepository {
	return &PermissionSetRepository{db: db}
}

const permissionSetColumns = `id, name, description`

func scanPermissionSet(row pgx.Row) (*models.PermissionSet, error) {
	var s models.PermissionSet
	err := row.Scan(&s.ID, &s.Name, &s.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PermissionSetRepository) Create(ctx context.Context, s *models.PermissionSet) error {
	const q = `INSERT INTO permission_sets (id, name, description) VALUES ($1, $2, $3)`
	_, err := r.db.Exec(ctx, q, s.ID, s.Name, s.Description)
	return err
}

func (r *PermissionSetRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PermissionSet, error) {
	q := `SELECT ` + permissionSetColumns + ` FROM permission_sets WHERE id = $1`
	return scanPermissionSet(r.db.QueryRow(ctx, q, id))
}

// GetByName exists for idempotent seeding — there's no unique constraint on
// name, so this is a get-or-create lookup, not a guarantee of uniqueness.
func (r *PermissionSetRepository) GetByName(ctx context.Context, name string) (*models.PermissionSet, error) {
	q := `SELECT ` + permissionSetColumns + ` FROM permission_sets WHERE name = $1 LIMIT 1`
	return scanPermissionSet(r.db.QueryRow(ctx, q, name))
}

func (r *PermissionSetRepository) Update(ctx context.Context, s *models.PermissionSet) error {
	const q = `UPDATE permission_sets SET name = $2, description = $3 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, s.ID, s.Name, s.Description)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PermissionSetRepository) List(ctx context.Context) ([]models.PermissionSet, error) {
	q := `SELECT ` + permissionSetColumns + ` FROM permission_sets ORDER BY name`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sets := make([]models.PermissionSet, 0)
	for rows.Next() {
		var s models.PermissionSet
		if err := rows.Scan(&s.ID, &s.Name, &s.Description); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}

func (r *PermissionSetRepository) GetSetToolIDs(ctx context.Context, setID uuid.UUID) ([]uuid.UUID, error) {
	const q = `SELECT tool_id FROM permission_set_tools WHERE set_id = $1`
	rows, err := r.db.Query(ctx, q, setID)
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

// ReplaceSetTools atomically sets the exact list of tools belonging to a
// permission set (the "preset" contents), replacing whatever was there.
func (r *PermissionSetRepository) ReplaceSetTools(ctx context.Context, setID uuid.UUID, toolIDs []uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM permission_set_tools WHERE set_id = $1`, setID); err != nil {
		return err
	}
	for _, tid := range toolIDs {
		if _, err := r.db.Exec(ctx, `INSERT INTO permission_set_tools (set_id, tool_id) VALUES ($1, $2)`, setID, tid); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return ErrInvalidToolID
			}
			return err
		}
	}
	return nil
}
