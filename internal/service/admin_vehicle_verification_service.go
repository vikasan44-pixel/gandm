package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

type VehicleDocumentView struct {
	models.VehicleDocument
	ViewURL string `json:"view_url"`
}

type VehicleVerificationDetail struct {
	Vehicle   *models.Vehicle       `json:"vehicle"`
	User      *models.User          `json:"user"`
	Documents []VehicleDocumentView `json:"documents"`
}

func (s *AdminService) VehicleVerificationQueue(ctx context.Context, status models.VehicleVerificationStatus) ([]repository.VehicleVerificationQueueItem, error) {
	return repository.NewVehicleDocumentRepository(s.db).ListVerificationQueue(ctx, status)
}

func (s *AdminService) VehicleVerificationDetail(ctx context.Context, adminID, vehicleID uuid.UUID) (*VehicleVerificationDetail, error) {
	vehicle, err := repository.NewVehicleRepository(s.db).GetByID(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	user, err := repository.NewUserRepository(s.db).GetByID(ctx, vehicle.UserID)
	if err != nil {
		return nil, err
	}
	documents, err := repository.NewVehicleDocumentRepository(s.db).ListByVehicleID(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	views := make([]VehicleDocumentView, 0, len(documents))
	for _, document := range documents {
		viewURL, err := s.storage.PresignedGetURL(ctx, document.FileURL, documentViewURLTTL)
		if err != nil {
			return nil, err
		}
		views = append(views, VehicleDocumentView{VehicleDocument: document, ViewURL: viewURL})
	}
	if err := writeAuditLog(ctx, s.db, adminID, "vehicle_documents_viewed", &vehicle.UserID, map[string]any{
		"vehicle_id": vehicleID, "document_count": len(documents),
	}); err != nil {
		return nil, err
	}
	return &VehicleVerificationDetail{Vehicle: vehicle, User: user, Documents: views}, nil
}

func (s *AdminService) ApproveVehicleVerification(ctx context.Context, adminID, vehicleID uuid.UUID) error {
	return s.reviewVehicleVerification(ctx, adminID, vehicleID, models.VehicleVerificationVerified, nil)
}

func (s *AdminService) RejectVehicleVerification(ctx context.Context, adminID, vehicleID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("%w: reason is required", ErrInvalidInput)
	}
	return s.reviewVehicleVerification(ctx, adminID, vehicleID, models.VehicleVerificationRejected, &reason)
}

func (s *AdminService) reviewVehicleVerification(ctx context.Context, adminID, vehicleID uuid.UUID, status models.VehicleVerificationStatus, reason *string) error {
	vehicle, err := repository.NewVehicleRepository(s.db).GetByID(ctx, vehicleID)
	if err != nil {
		return err
	}
	if vehicle.VerificationStatus != models.VehicleVerificationPending {
		return ErrAlreadyReviewed
	}
	if len(vehicle.UploadedDocumentTypes) != 7 {
		return fmt.Errorf("%w: full vehicle document set is required", ErrInvalidInput)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := repository.NewVehicleRepository(tx).SetVerificationResult(ctx, vehicleID, adminID, status, reason, time.Now()); err != nil {
		return err
	}
	action := "vehicle_verification_approved"
	if status == models.VehicleVerificationRejected {
		action = "vehicle_verification_rejected"
	}
	details := map[string]any{"vehicle_id": vehicleID, "plate_number": vehicle.PlateNumber}
	if reason != nil {
		details["reason"] = *reason
	}
	if err := writeAuditLog(ctx, tx, adminID, action, &vehicle.UserID, details); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
