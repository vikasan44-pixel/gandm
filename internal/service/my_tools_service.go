package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

// ToolCatalog — участнические инструменты для самовыбора (регистрация и
// настройки): любой авторизованный участник видит список с ценой и
// описанием. Служебные admin-инструменты сюда не входят.
func (s *CargoService) ToolCatalog(ctx context.Context) ([]models.Tool, error) {
	return repository.NewToolRepository(s.db).ListSelfSelectable(ctx)
}

// GetMyTools — текущие инструменты участника.
func (s *CargoService) GetMyTools(ctx context.Context, userID uuid.UUID) ([]models.Tool, error) {
	if _, err := s.requireEligibleUser(ctx, userID); err != nil {
		return nil, err
	}
	return repository.NewToolRepository(s.db).ListByUserID(ctx, userID)
}

// SetMyTools заменяет набор инструментов участника выбранными. Разрешены
// ТОЛЬКО участнические (self-selectable) инструменты — админские выбрать
// себе нельзя. В период запуска (ТЗ §15) выбранные инструменты выдаются
// сразу; цена платных показывается как будущий ежемесячный платёж.
func (s *CargoService) SetMyTools(ctx context.Context, userID uuid.UUID, toolIDs []uuid.UUID) ([]models.Tool, error) {
	if _, err := s.requireEligibleUser(ctx, userID); err != nil {
		return nil, err
	}
	toolRepo := repository.NewToolRepository(s.db)
	allowed, err := toolRepo.SelfSelectableIDSet(ctx)
	if err != nil {
		return nil, err
	}
	for _, id := range toolIDs {
		if !allowed[id] {
			return nil, fmt.Errorf("%w: tool is not self-selectable", ErrForbiddenTool)
		}
	}
	if err := toolRepo.ReplaceUserTools(ctx, userID, toolIDs); err != nil {
		return nil, err
	}
	return toolRepo.ListByUserID(ctx, userID)
}
