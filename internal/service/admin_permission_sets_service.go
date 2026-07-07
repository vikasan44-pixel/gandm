package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

type PermissionSetView struct {
	models.PermissionSet
	ToolIDs []uuid.UUID `json:"tool_ids"`
}

type CreatePermissionSetInput struct {
	Name        string
	Description string
	ToolIDs     []uuid.UUID
}

type PermissionSetPatch struct {
	Name        *string
	Description *string
	ToolIDs     *[]uuid.UUID
}

func (s *AdminService) ListPermissionSets(ctx context.Context) ([]PermissionSetView, error) {
	setRepo := repository.NewPermissionSetRepository(s.db)
	sets, err := setRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]PermissionSetView, 0, len(sets))
	for _, set := range sets {
		toolIDs, err := setRepo.GetSetToolIDs(ctx, set.ID)
		if err != nil {
			return nil, err
		}
		views = append(views, PermissionSetView{PermissionSet: set, ToolIDs: toolIDs})
	}
	return views, nil
}

func (s *AdminService) CreatePermissionSet(ctx context.Context, adminID uuid.UUID, in CreatePermissionSetInput) (*PermissionSetView, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	set := &models.PermissionSet{ID: uuid.New(), Name: in.Name, Description: in.Description}
	setRepo := repository.NewPermissionSetRepository(tx)
	if err := setRepo.Create(ctx, set); err != nil {
		return nil, err
	}
	if err := setRepo.ReplaceSetTools(ctx, set.ID, in.ToolIDs); err != nil {
		return nil, err
	}

	if err := writeAuditLog(ctx, tx, adminID, "permission_set_created", nil, map[string]any{"set_id": set.ID}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &PermissionSetView{PermissionSet: *set, ToolIDs: in.ToolIDs}, nil
}

func (s *AdminService) UpdatePermissionSet(ctx context.Context, adminID, setID uuid.UUID, patch PermissionSetPatch) (*PermissionSetView, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	setRepo := repository.NewPermissionSetRepository(tx)
	set, err := setRepo.GetByID(ctx, setID)
	if err != nil {
		return nil, err
	}

	if patch.Name != nil {
		set.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.Description != nil {
		set.Description = *patch.Description
	}
	if set.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if err := setRepo.Update(ctx, set); err != nil {
		return nil, err
	}

	toolIDs, err := setRepo.GetSetToolIDs(ctx, setID)
	if err != nil {
		return nil, err
	}
	if patch.ToolIDs != nil {
		if err := setRepo.ReplaceSetTools(ctx, setID, *patch.ToolIDs); err != nil {
			return nil, err
		}
		toolIDs = *patch.ToolIDs
	}

	if err := writeAuditLog(ctx, tx, adminID, "permission_set_updated", nil, map[string]any{"set_id": set.ID}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &PermissionSetView{PermissionSet: *set, ToolIDs: toolIDs}, nil
}
