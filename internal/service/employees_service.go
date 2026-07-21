package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"gandm/internal/auth"
	"gandm/internal/models"
	"gandm/internal/repository"
)

// ToolManageEmployees — право компании создавать суб-аккаунты сотрудников
// (ТЗ §13.1, вариант А). Выдаётся админом проверенным компаниям.
const ToolManageEmployees = "manage_employees"

var ErrEmployeeOfEmployee = errors.New("employees cannot create their own employees")

type CreateEmployeeInput struct {
	Email    string
	Phone    string
	Password string
}

// isEmployee: у сотрудника заполнен parent_company_id.
func (s *CargoService) isEmployee(ctx context.Context, userID uuid.UUID) (bool, error) {
	var parent *uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT parent_company_id FROM users WHERE id = $1`, userID).Scan(&parent)
	if err != nil {
		return false, err
	}
	return parent != nil, nil
}

// CreateEmployee: компания создаёт сотрудника. Сотрудник сразу active —
// компания уже прошла верификацию (ТЗ §13.1: «после проверки компания…
// может создавать логины и пароли для своих сотрудников»). Тип участника и
// название компании наследуются; инструменты наследуются динамически через
// UserHasTool.
func (s *CargoService) CreateEmployee(ctx context.Context, companyID uuid.UUID, in CreateEmployeeInput) (*repository.Employee, error) {
	company, err := s.requireEligibleUser(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if company.Status != models.UserStatusActive {
		return nil, ErrAccountNotEligible
	}
	if err := s.requireTool(ctx, companyID, ToolManageEmployees); err != nil {
		return nil, err
	}
	// Сотрудник не может плодить собственных сотрудников — иерархия
	// одноуровневая: компания → сотрудники.
	isEmp, err := s.isEmployee(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if isEmp {
		return nil, ErrEmployeeOfEmployee
	}

	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = strings.TrimSpace(in.Phone)
	if in.Email == "" || !strings.Contains(in.Email, "@") {
		return nil, fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if len(in.Password) < 8 {
		return nil, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	employee := &models.User{
		ID:              uuid.New(),
		Email:           in.Email,
		Phone:           in.Phone,
		CompanyName:     company.CompanyName,
		ParticipantType: company.ParticipantType,
		PasswordHash:    string(hash),
		Status:          models.UserStatusActive,
		HasSubscription: false,
		Language:        company.Language,
		CreatedAt:       now,
	}
	empRepo := repository.NewEmployeeRepository(s.db)
	if err := empRepo.Create(ctx, employee, companyID); err != nil {
		return nil, err
	}
	return &repository.Employee{
		ID:        employee.ID,
		Email:     employee.Email,
		Phone:     employee.Phone,
		Status:    employee.Status,
		CreatedAt: employee.CreatedAt,
	}, nil
}

func (s *CargoService) ListMyEmployees(ctx context.Context, companyID uuid.UUID) ([]repository.Employee, error) {
	if err := s.requireActiveUser(ctx, companyID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, companyID, ToolManageEmployees); err != nil {
		return nil, err
	}
	return repository.NewEmployeeRepository(s.db).ListByCompanyID(ctx, companyID)
}

// SetEmployeeBlocked: компания деактивирует/возвращает своего сотрудника.
// Чужой сотрудник читается как not-found.
func (s *CargoService) SetEmployeeBlocked(ctx context.Context, companyID, employeeID uuid.UUID, blocked bool) (*repository.Employee, error) {
	if err := s.requireActiveUser(ctx, companyID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, companyID, ToolManageEmployees); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	empRepo := repository.NewEmployeeRepository(tx)
	status := models.UserStatusActive
	if blocked {
		status = models.UserStatusBlocked
	}
	if err := empRepo.SetStatus(ctx, companyID, employeeID, status); err != nil {
		return nil, err
	}
	if blocked {
		if err := repository.NewRefreshTokenRepository(tx).RevokeAllForSubject(ctx, auth.SubjectUser, employeeID, time.Now()); err != nil {
			return nil, err
		}
		if err := repository.NewActiveSessionRepository(tx).DeleteForSubject(ctx, auth.SubjectUser, employeeID); err != nil {
			return nil, err
		}
	}
	employee, err := empRepo.GetForCompany(ctx, companyID, employeeID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return employee, nil
}
