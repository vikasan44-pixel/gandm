package repository

import (
	"context"

	"github.com/google/uuid"

	"gandm/internal/models"
)

type DocumentRepository struct {
	db Querier
}

func NewDocumentRepository(db Querier) *DocumentRepository {
	return &DocumentRepository{db: db}
}

func (r *DocumentRepository) Create(ctx context.Context, d *models.Document) error {
	const q = `
		INSERT INTO documents (id, user_id, type, file_url, original_name, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, d.ID, d.UserID, d.Type, d.FileURL, d.OriginalName, d.UploadedAt)
	return err
}

func (r *DocumentRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.Document, error) {
	const q = `
		SELECT id, user_id, type, file_url, original_name, uploaded_at
		FROM documents
		WHERE user_id = $1
		ORDER BY uploaded_at ASC
	`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := make([]models.Document, 0)
	for rows.Next() {
		var d models.Document
		if err := rows.Scan(&d.ID, &d.UserID, &d.Type, &d.FileURL, &d.OriginalName, &d.UploadedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

// TypeSetByUserID returns the uploaded document kinds without loading file
// metadata; verification approval uses it to enforce the minimum dossier.
func (r *DocumentRepository) TypeSetByUserID(ctx context.Context, userID uuid.UUID) (map[models.DocumentType]bool, error) {
	rows, err := r.db.Query(ctx, `SELECT DISTINCT type FROM documents WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[models.DocumentType]bool)
	for rows.Next() {
		var docType models.DocumentType
		if err := rows.Scan(&docType); err != nil {
			return nil, err
		}
		result[docType] = true
	}
	return result, rows.Err()
}
