package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"gandm/internal/auth"
	"gandm/internal/models"
	"gandm/internal/repository"
	"gandm/internal/storage"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAlreadyReviewed    = errors.New("verification request already reviewed")
)

const documentViewURLTTL = 15 * time.Minute

type DashboardStats struct {
	WaitingVerification int `json:"waiting_verification"`
	NewToday            int `json:"new_today"`
	ActiveUsers         int `json:"active_users"`
	Visits              int `json:"visits"`
}

type DocumentView struct {
	models.Document
	ViewURL string `json:"view_url"`
}

type VerificationDetail struct {
	Verification *models.VerificationRequest `json:"verification"`
	User         *models.User                `json:"user"`
	Documents    []DocumentView               `json:"documents"`
}

type AdminService struct {
	db      *pgxpool.Pool
	tokens  *auth.Manager
	storage *storage.S3Client
}

func NewAdminService(db *pgxpool.Pool, tokens *auth.Manager, storage *storage.S3Client) *AdminService {
	return &AdminService{db: db, tokens: tokens, storage: storage}
}

func (s *AdminService) Login(ctx context.Context, email, password string) (*models.Admin, auth.TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	adminRepo := repository.NewAdminRepository(s.db)
	admin, err := adminRepo.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, auth.TokenPair{}, ErrInvalidCredentials
	}
	if err != nil {
		return nil, auth.TokenPair{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, auth.TokenPair{}, ErrInvalidCredentials
	}

	tokens, err := s.tokens.IssueTokenPair(admin.ID, auth.SubjectAdmin)
	if err != nil {
		return nil, auth.TokenPair{}, err
	}

	return admin, tokens, nil
}

// DashboardStats aggregates the four dashboard cards. "new_today" and
// "visits" use the server's local calendar day as the boundary for "today".
// "visits" counts participants whose last_active_at falls today — the only
// activity signal the current schema carries; there's no separate page-view
// tracking table.
func (s *AdminService) DashboardStats(ctx context.Context) (DashboardStats, error) {
	userRepo := repository.NewUserRepository(s.db)
	verRepo := repository.NewVerificationRepository(s.db)

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	waiting, err := verRepo.CountByStatus(ctx, models.VerificationPending)
	if err != nil {
		return DashboardStats{}, err
	}
	newToday, err := userRepo.CountCreatedSince(ctx, startOfDay)
	if err != nil {
		return DashboardStats{}, err
	}
	active, err := userRepo.CountByStatus(ctx, models.UserStatusActive)
	if err != nil {
		return DashboardStats{}, err
	}
	visits, err := userRepo.CountActiveSince(ctx, startOfDay)
	if err != nil {
		return DashboardStats{}, err
	}

	return DashboardStats{
		WaitingVerification: waiting,
		NewToday:            newToday,
		ActiveUsers:         active,
		Visits:              visits,
	}, nil
}

func (s *AdminService) VerificationQueue(ctx context.Context, status models.VerificationStatus) ([]repository.QueueItem, error) {
	verRepo := repository.NewVerificationRepository(s.db)
	return verRepo.ListQueue(ctx, status)
}

func (s *AdminService) VerificationDetail(ctx context.Context, verificationID uuid.UUID) (*VerificationDetail, error) {
	verRepo := repository.NewVerificationRepository(s.db)
	verification, err := verRepo.GetByID(ctx, verificationID)
	if err != nil {
		return nil, err
	}

	userRepo := repository.NewUserRepository(s.db)
	user, err := userRepo.GetByID(ctx, verification.UserID)
	if err != nil {
		return nil, err
	}

	docRepo := repository.NewDocumentRepository(s.db)
	docs, err := docRepo.ListByUserID(ctx, verification.UserID)
	if err != nil {
		return nil, err
	}

	views := make([]DocumentView, 0, len(docs))
	for _, d := range docs {
		viewURL, err := s.storage.PresignedGetURL(ctx, d.FileURL, documentViewURLTTL)
		if err != nil {
			return nil, err
		}
		views = append(views, DocumentView{Document: d, ViewURL: viewURL})
	}

	return &VerificationDetail{Verification: verification, User: user, Documents: views}, nil
}

func (s *AdminService) ApproveVerification(ctx context.Context, adminID, verificationID uuid.UUID) error {
	return s.reviewVerification(ctx, adminID, verificationID, models.VerificationApproved, models.UserStatusActive, nil, "verification_approved")
}

func (s *AdminService) RejectVerification(ctx context.Context, adminID, verificationID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("%w: reason is required", ErrInvalidInput)
	}
	return s.reviewVerification(ctx, adminID, verificationID, models.VerificationRejected, models.UserStatusRejected, &reason, "verification_rejected")
}

func (s *AdminService) reviewVerification(ctx context.Context, adminID, verificationID uuid.UUID, newVerStatus models.VerificationStatus, newUserStatus models.UserStatus, reason *string, auditAction string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	verRepo := repository.NewVerificationRepository(tx)
	verification, err := verRepo.GetByID(ctx, verificationID)
	if err != nil {
		return err
	}
	if verification.Status != models.VerificationPending {
		return ErrAlreadyReviewed
	}

	now := time.Now()
	if err := verRepo.UpdateStatus(ctx, verificationID, newVerStatus, reason, adminID, now); err != nil {
		return err
	}

	userRepo := repository.NewUserRepository(tx)
	if err := userRepo.UpdateStatus(ctx, verification.UserID, newUserStatus); err != nil {
		return err
	}

	details := map[string]any{"verification_id": verificationID}
	if reason != nil {
		details["reason"] = *reason
	}
	if err := writeAuditLog(ctx, tx, adminID, auditAction, &verification.UserID, details); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// writeAuditLog is the single place every admin mutation goes through to
// satisfy the "every admin action is logged" rule. It takes a Querier (not a
// *pgxpool.Pool) so callers can write the log entry inside the same
// transaction as the mutation it's recording.
func writeAuditLog(ctx context.Context, q repository.Querier, adminID uuid.UUID, action string, targetUserID *uuid.UUID, details map[string]any) error {
	var detailsJSON []byte
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return err
		}
		detailsJSON = b
	}
	auditRepo := repository.NewAuditLogRepository(q)
	return auditRepo.Create(ctx, &models.AuditLog{
		ID:           uuid.New(),
		AdminID:      adminID,
		Action:       action,
		TargetUserID: targetUserID,
		Details:      detailsJSON,
		CreatedAt:    time.Now(),
	})
}
