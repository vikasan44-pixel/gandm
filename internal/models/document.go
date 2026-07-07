package models

import (
	"time"

	"github.com/google/uuid"
)

type DocumentType string

const (
	DocumentIDCard             DocumentType = "id_card"
	DocumentFoundingDocs       DocumentType = "founding_docs"
	DocumentBusinessLicense    DocumentType = "business_license"
	DocumentEmploymentContract DocumentType = "employment_contract"
	DocumentVehicleDoc         DocumentType = "vehicle_doc"
)

type Document struct {
	ID           uuid.UUID    `db:"id" json:"id"`
	UserID       uuid.UUID    `db:"user_id" json:"user_id"`
	Type         DocumentType `db:"type" json:"type"`
	FileURL      string       `db:"file_url" json:"file_url"`
	OriginalName string       `db:"original_name" json:"original_name"`
	UploadedAt   time.Time    `db:"uploaded_at" json:"uploaded_at"`
}
