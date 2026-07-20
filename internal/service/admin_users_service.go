package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/models"
	"gandm/internal/repository"
)

type UserListFilter struct {
	ParticipantType string
	Status          string
	Search          string
}

type UserDetail struct {
	User         *models.User                 `json:"user"`
	Tools        []models.Tool                `json:"tools"`
	Verification *models.VerificationRequest  `json:"verification,omitempty"`
	Rating       repository.UserRatingSummary `json:"rating"`
	// Иерархия «компания → сотрудники» (ТЗ §13.1): у сотрудника заполнен
	// ParentCompany, у компании — список Employees. Без этого админ видел
	// сотрудников как безликих участников в общем списке.
	ParentCompany *CompanyRef           `json:"parent_company,omitempty"`
	Employees     []repository.Employee `json:"employees,omitempty"`
}

type CompanyRef struct {
	ID          uuid.UUID `json:"id"`
	CompanyName string    `json:"company_name"`
	Email       string    `json:"email"`
}

func (s *AdminService) ListUsers(ctx context.Context, f UserListFilter) ([]models.User, error) {
	filter := repository.UserFilter{Search: strings.TrimSpace(f.Search)}

	if f.ParticipantType != "" {
		filter.ServiceType = f.ParticipantType
	}
	if f.Status != "" {
		st := models.UserStatus(f.Status)
		if !allowedUserStatuses[st] {
			return nil, fmt.Errorf("%w: unknown status", ErrInvalidInput)
		}
		filter.Status = &st
	}

	userRepo := repository.NewUserRepository(s.db)
	return userRepo.List(ctx, filter)
}

func (s *AdminService) GetUser(ctx context.Context, userID uuid.UUID) (*UserDetail, error) {
	userRepo := repository.NewUserRepository(s.db)
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	toolRepo := repository.NewToolRepository(s.db)
	tools, err := toolRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	verRepo := repository.NewVerificationRepository(s.db)
	verification, err := verRepo.GetLatestByUserID(ctx, userID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repository.ErrNotFound) {
		verification = nil
	}

	ratingRepo := repository.NewRatingRepository(s.db)
	rating, err := ratingRepo.SummaryForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	detail := &UserDetail{User: user, Tools: tools, Verification: verification, Rating: rating}

	// Иерархия сотрудников: сотрудник → ссылка на компанию, компания →
	// список своих сотрудников.
	var parentID *uuid.UUID
	if err := s.db.QueryRow(ctx, `SELECT parent_company_id FROM users WHERE id = $1`, userID).Scan(&parentID); err != nil {
		return nil, err
	}
	if parentID != nil {
		if company, err := userRepo.GetByID(ctx, *parentID); err == nil {
			detail.ParentCompany = &CompanyRef{ID: company.ID, CompanyName: company.CompanyName, Email: company.Email}
		}
	} else {
		employees, err := repository.NewEmployeeRepository(s.db).ListByCompanyID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if len(employees) > 0 {
			detail.Employees = employees
		}
	}

	return detail, nil
}

// SetUserTools replaces a user's whole tool assignment (checkbox-list
// semantics, not incremental add/remove) — the only path business logic
// elsewhere should check to decide what a user can do.
func (s *AdminService) SetUserTools(ctx context.Context, adminID, userID uuid.UUID, toolIDs []uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	userRepo := repository.NewUserRepository(tx)
	if _, err := userRepo.GetByID(ctx, userID); err != nil {
		return err
	}

	toolRepo := repository.NewToolRepository(tx)
	if err := toolRepo.ReplaceUserTools(ctx, userID, toolIDs); err != nil {
		return err
	}
	tools, err := toolRepo.ListByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if err := userRepo.UpdateParticipantType(ctx, userID, legacyParticipantType(tools)); err != nil {
		return err
	}

	if err := writeAuditLog(ctx, tx, adminID, "user_tools_updated", &userID, map[string]any{"tool_ids": toolIDs}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ApplyPermissionSet replaces a user's tools with exactly the set's tools —
// applying a preset, not merging with whatever they had before.
func (s *AdminService) ApplyPermissionSet(ctx context.Context, adminID, userID, setID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	userRepo := repository.NewUserRepository(tx)
	if _, err := userRepo.GetByID(ctx, userID); err != nil {
		return err
	}

	setRepo := repository.NewPermissionSetRepository(tx)
	if _, err := setRepo.GetByID(ctx, setID); err != nil {
		return err
	}
	toolIDs, err := setRepo.GetSetToolIDs(ctx, setID)
	if err != nil {
		return err
	}

	toolRepo := repository.NewToolRepository(tx)
	if err := toolRepo.ReplaceUserTools(ctx, userID, toolIDs); err != nil {
		return err
	}
	tools, err := toolRepo.ListByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if err := userRepo.UpdateParticipantType(ctx, userID, legacyParticipantType(tools)); err != nil {
		return err
	}

	if err := writeAuditLog(ctx, tx, adminID, "permission_set_applied", &userID, map[string]any{"set_id": setID}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// SetUserSubscription flips the manual subscription flag — the payments
// stand-in until real billing exists (raises the contact-reveal limit).
func (s *AdminService) SetUserSubscription(ctx context.Context, adminID, userID uuid.UUID, hasSubscription bool) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	userRepo := repository.NewUserRepository(tx)
	if err := userRepo.UpdateSubscription(ctx, userID, hasSubscription); err != nil {
		return err
	}

	details := map[string]any{"has_subscription": hasSubscription}
	if err := writeAuditLog(ctx, tx, adminID, "user_subscription_updated", &userID, details); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// MarkConsolidatedPayment lets an admin manually record a one-time
// consolidation payment for a client — the testing stand-in alongside the
// sandbox provider.
func (s *AdminService) MarkConsolidatedPayment(ctx context.Context, adminID, consolidatedID, clientID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	consRepo := repository.NewConsolidationRepository(tx)
	if _, err := consRepo.GetConsolidatedByID(ctx, consolidatedID); err != nil {
		return err
	}

	if err := consRepo.CreatePayment(ctx, &models.ConsolidatedPayment{
		ID:                    uuid.New(),
		ConsolidatedRequestID: consolidatedID,
		ClientID:              clientID,
		Provider:              "manual",
		ProviderRef:           "admin-" + adminID.String(),
		CreatedAt:             time.Now(),
	}); err != nil {
		return err
	}

	details := map[string]any{"consolidated_request_id": consolidatedID}
	if err := writeAuditLog(ctx, tx, adminID, "consolidated_payment_marked", &clientID, details); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *AdminService) BlockUser(ctx context.Context, adminID, userID uuid.UUID) error {
	return s.setUserStatus(ctx, adminID, userID, models.UserStatusBlocked, "user_blocked")
}

func (s *AdminService) UnblockUser(ctx context.Context, adminID, userID uuid.UUID) error {
	return s.setUserStatus(ctx, adminID, userID, models.UserStatusActive, "user_unblocked")
}

func (s *AdminService) setUserStatus(ctx context.Context, adminID, userID uuid.UUID, status models.UserStatus, action string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	userRepo := repository.NewUserRepository(tx)
	if err := userRepo.UpdateStatus(ctx, userID, status); err != nil {
		return err
	}

	// Blocking must end existing sessions: revoke every refresh token so the
	// account can't be silently renewed. Live access tokens still work until
	// they expire (≤ JWT_ACCESS_TTL), which is the accepted short window.
	if status == models.UserStatusBlocked {
		rtRepo := repository.NewRefreshTokenRepository(tx)
		if err := rtRepo.RevokeAllForSubject(ctx, auth.SubjectUser, userID, time.Now()); err != nil {
			return err
		}
	}

	if err := writeAuditLog(ctx, tx, adminID, action, &userID, nil); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
