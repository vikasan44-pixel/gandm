package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"gandm/internal/models"
	"gandm/internal/repository"
)

var ErrForbiddenRole = errors.New("this action requires the admin role")

// RequireAdminRole re-reads the staff account and rejects non-admins —
// moderators are limited to verification and viewing users (ТЗ §19.6).
func (s *AdminService) RequireAdminRole(ctx context.Context, adminID uuid.UUID) error {
	admin, err := repository.NewAdminRepository(s.db).GetByID(ctx, adminID)
	if err != nil {
		return err
	}
	if admin.Role != models.AdminRoleAdmin {
		return ErrForbiddenRole
	}
	return nil
}

func (s *AdminService) ListModerators(ctx context.Context) ([]models.Admin, error) {
	return repository.NewAdminRepository(s.db).List(ctx)
}

// CreateModerator adds a staff account with the moderator role. Full
// admins are created only via seed/DB on purpose — the UI never mints
// accounts with unrestricted rights.
func (s *AdminService) CreateModerator(ctx context.Context, email, password string) (*models.Admin, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	admin := &models.Admin{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         models.AdminRoleModerator,
		CreatedAt:    time.Now(),
	}
	if err := repository.NewAdminRepository(s.db).Create(ctx, admin); err != nil {
		return nil, err
	}
	return admin, nil
}
