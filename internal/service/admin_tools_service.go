package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

type CreateToolInput struct {
	Key         string
	Name        string
	Description string
	Category    string
	PriceKZT    float64
}

type ToolPatch struct {
	Name        *string
	Description *string
	Category    *string
	IsActive    *bool
	PriceKZT    *float64
}

func (s *AdminService) ListTools(ctx context.Context) ([]models.Tool, error) {
	toolRepo := repository.NewToolRepository(s.db)
	return toolRepo.List(ctx)
}

func (s *AdminService) CreateTool(ctx context.Context, adminID uuid.UUID, in CreateToolInput) (*models.Tool, error) {
	in.Key = strings.TrimSpace(in.Key)
	in.Name = strings.TrimSpace(in.Name)
	if in.Key == "" {
		return nil, fmt.Errorf("%w: key is required", ErrInvalidInput)
	}
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if in.PriceKZT < 0 {
		return nil, fmt.Errorf("%w: price_kzt must be non-negative", ErrInvalidInput)
	}
	tool := &models.Tool{
		ID:          uuid.New(),
		Key:         in.Key,
		Name:        in.Name,
		Description: in.Description,
		Category:    in.Category,
		IsActive:    true,
		PriceKZT:    in.PriceKZT,
	}

	toolRepo := repository.NewToolRepository(tx)
	if err := toolRepo.Create(ctx, tool); err != nil {
		return nil, err
	}

	if err := writeAuditLog(ctx, tx, adminID, "tool_created", nil, map[string]any{"tool_id": tool.ID, "key": tool.Key}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return tool, nil
}

func (s *AdminService) UpdateTool(ctx context.Context, adminID, toolID uuid.UUID, patch ToolPatch) (*models.Tool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	toolRepo := repository.NewToolRepository(tx)
	tool, err := toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return nil, err
	}

	if patch.Name != nil {
		tool.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.Description != nil {
		tool.Description = *patch.Description
	}
	if patch.Category != nil {
		tool.Category = *patch.Category
	}
	if patch.IsActive != nil {
		tool.IsActive = *patch.IsActive
	}
	if patch.PriceKZT != nil {
		if *patch.PriceKZT < 0 {
			return nil, fmt.Errorf("%w: price_kzt must be non-negative", ErrInvalidInput)
		}
		tool.PriceKZT = *patch.PriceKZT
	}
	if tool.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}

	if err := toolRepo.Update(ctx, tool); err != nil {
		return nil, err
	}

	if err := writeAuditLog(ctx, tx, adminID, "tool_updated", nil, map[string]any{"tool_id": tool.ID}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return tool, nil
}
