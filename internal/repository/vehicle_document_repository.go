package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type VehicleVerificationQueueItem struct {
	VehicleID   uuid.UUID                        `json:"vehicle_id"`
	UserID      uuid.UUID                        `json:"user_id"`
	CompanyName string                           `json:"company_name"`
	Email       string                           `json:"email"`
	PlateNumber string                           `json:"plate_number"`
	VIN         string                           `json:"vin"`
	Status      models.VehicleVerificationStatus `json:"status"`
	CreatedAt   time.Time                        `json:"created_at"`
}

type VehicleDocumentRepository struct{ db Querier }

func NewVehicleDocumentRepository(db Querier) *VehicleDocumentRepository {
	return &VehicleDocumentRepository{db: db}
}

func (r *VehicleDocumentRepository) Upsert(ctx context.Context, document *models.VehicleDocument) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO vehicle_documents (id, vehicle_id, type, file_url, original_name, content_type, uploaded_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (vehicle_id, type) DO UPDATE
		SET id=EXCLUDED.id, file_url=EXCLUDED.file_url, original_name=EXCLUDED.original_name,
		    content_type=EXCLUDED.content_type, uploaded_at=EXCLUDED.uploaded_at`,
		document.ID, document.VehicleID, document.Type, document.FileURL,
		document.OriginalName, document.ContentType, document.UploadedAt)
	return err
}

func scanVehicleDocument(row pgx.Row) (*models.VehicleDocument, error) {
	var document models.VehicleDocument
	err := row.Scan(&document.ID, &document.VehicleID, &document.Type, &document.FileURL,
		&document.OriginalName, &document.ContentType, &document.UploadedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &document, nil
}

func (r *VehicleDocumentRepository) ListByVehicleID(ctx context.Context, vehicleID uuid.UUID) ([]models.VehicleDocument, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, vehicle_id, type, file_url, original_name, content_type, uploaded_at
		FROM vehicle_documents WHERE vehicle_id=$1 ORDER BY type`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	documents := make([]models.VehicleDocument, 0)
	for rows.Next() {
		document, err := scanVehicleDocument(rows)
		if err != nil {
			return nil, err
		}
		documents = append(documents, *document)
	}
	return documents, rows.Err()
}

func (r *VehicleDocumentRepository) ListVerificationQueue(ctx context.Context, status models.VehicleVerificationStatus) ([]VehicleVerificationQueueItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT vehicle.id, vehicle.user_id, participant.company_name, participant.email,
		       vehicle.plate_number, vehicle.vin, vehicle.verification_status, vehicle.created_at
		FROM vehicles vehicle
		JOIN users participant ON participant.id=vehicle.user_id
		WHERE vehicle.verification_status=$1
		ORDER BY vehicle.created_at ASC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]VehicleVerificationQueueItem, 0)
	for rows.Next() {
		var item VehicleVerificationQueueItem
		if err := rows.Scan(&item.VehicleID, &item.UserID, &item.CompanyName, &item.Email,
			&item.PlateNumber, &item.VIN, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
