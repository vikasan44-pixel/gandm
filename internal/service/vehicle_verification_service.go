package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

var allowedVehicleDocumentTypes = map[models.VehicleDocumentType]bool{
	models.VehicleDocumentRegistrationCertificate: true,
	models.VehicleDocumentIdentity:                true,
	models.VehicleDocumentInsurance:               true,
	models.VehicleDocumentPhotoFront:              true,
	models.VehicleDocumentPhotoBack:               true,
	models.VehicleDocumentPhotoLeft:               true,
	models.VehicleDocumentPhotoRight:              true,
}

func isVehiclePhoto(docType models.VehicleDocumentType) bool {
	return docType == models.VehicleDocumentPhotoFront || docType == models.VehicleDocumentPhotoBack ||
		docType == models.VehicleDocumentPhotoLeft || docType == models.VehicleDocumentPhotoRight
}

func (s *CargoService) UpdateMyVehicleRegistration(ctx context.Context, userID, vehicleID uuid.UUID, country, plate, vin string, consent bool) (*models.Vehicle, error) {
	repo, _, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return nil, err
	}
	country = strings.ToUpper(strings.TrimSpace(country))
	plate = strings.ToUpper(strings.Join(strings.Fields(plate), ""))
	vin = strings.ToUpper(strings.TrimSpace(vin))
	if country == "" || plate == "" || !validVIN(vin) {
		return nil, fmt.Errorf("%w: registration country, plate and valid VIN are required", ErrInvalidInput)
	}
	if !consent {
		return nil, fmt.Errorf("%w: privacy consent is required", ErrInvalidInput)
	}
	if err := repo.UpdateRegistration(ctx, vehicleID, country, plate, vin, VehiclePrivacyConsentVersion, time.Now()); err != nil {
		return nil, err
	}
	if err := repo.RefreshVerificationStatus(ctx, vehicleID); err != nil {
		return nil, err
	}
	return repo.GetByID(ctx, vehicleID)
}

func (s *CargoService) UploadMyVehicleDocument(ctx context.Context, userID, vehicleID uuid.UUID, docType models.VehicleDocumentType, header *multipart.FileHeader) (*models.Vehicle, error) {
	if s.storage == nil {
		return nil, fmt.Errorf("vehicle document storage is not configured")
	}
	vehicleRepo, _, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return nil, err
	}
	if !allowedVehicleDocumentTypes[docType] {
		return nil, fmt.Errorf("%w: unknown vehicle document type", ErrInvalidInput)
	}
	documentRepo := repository.NewVehicleDocumentRepository(s.db)
	previous, err := documentRepo.GetByVehicleAndType(ctx, vehicleID, docType)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if header.Size > maxDocumentSize {
		return nil, ErrFileTooLarge
	}
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	sniff := make([]byte, 512)
	n, err := file.Read(sniff)
	if err != nil && err != io.EOF {
		return nil, err
	}
	contentType := http.DetectContentType(sniff[:n])
	if !allowedDocumentContentTypes[contentType] || (isVehiclePhoto(docType) && !strings.HasPrefix(contentType, "image/")) {
		return nil, ErrUnsupportedFile
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("vehicle-documents/%s/%s/%s_%s", vehicleID, docType, uuid.New(), sanitizeFilename(header.Filename))
	if err := s.storage.Upload(ctx, key, file, header.Size, contentType); err != nil {
		return nil, err
	}
	document := &models.VehicleDocument{
		ID: uuid.New(), VehicleID: vehicleID, Type: docType, FileURL: key,
		OriginalName: header.Filename, ContentType: contentType, UploadedAt: time.Now(),
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, cleanupUploadedObject(ctx, s.storage, key, err)
	}
	defer tx.Rollback(ctx)
	if err := repository.NewVehicleDocumentRepository(tx).Upsert(ctx, document); err != nil {
		return nil, cleanupUploadedObject(ctx, s.storage, key, err)
	}
	if err := repository.NewVehicleRepository(tx).RefreshVerificationStatus(ctx, vehicleID); err != nil {
		return nil, cleanupUploadedObject(ctx, s.storage, key, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, cleanupUploadedObject(ctx, s.storage, key, err)
	}
	if previous != nil && previous.FileURL != key {
		_ = deleteStoredObject(ctx, s.storage, previous.FileURL)
	}
	return vehicleRepo.GetByID(ctx, vehicleID)
}
