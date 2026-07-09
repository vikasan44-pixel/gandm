package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

// ToolManageWarehouseSlots gates dispatch-threshold management (ТЗ §5.2:
// «порог отправки — объявление склада», разные пороги для разных
// направлений). Направление = participant_route.
const ToolManageWarehouseSlots = "manage_warehouse_slots"

// RouteWithThreshold is the self-service view: the warehouse's route plus
// its threshold, if one is set.
type RouteWithThreshold struct {
	Route     models.ParticipantRoute   `json:"route"`
	Threshold *models.DispatchThreshold `json:"threshold,omitempty"`
}

func (s *CargoService) ListMyDispatchThresholds(ctx context.Context, userID uuid.UUID) ([]RouteWithThreshold, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageWarehouseSlots); err != nil {
		return nil, err
	}

	routeRepo := repository.NewParticipantRouteRepository(s.db)
	routes, err := routeRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	thresholds, err := repository.NewDispatchThresholdRepository(s.db).ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]RouteWithThreshold, 0, len(routes))
	for _, route := range routes {
		row := RouteWithThreshold{Route: route}
		if t, ok := thresholds[route.ID]; ok {
			threshold := t
			row.Threshold = &threshold
		}
		items = append(items, row)
	}
	return items, nil
}

// SetRouteDispatchThreshold upserts the threshold on the caller's own route.
// accrued > threshold is allowed — the warehouse may overfill before
// dispatching.
func (s *CargoService) SetRouteDispatchThreshold(ctx context.Context, userID, routeID uuid.UUID, thresholdM3, accruedM3 float64) (*models.DispatchThreshold, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageWarehouseSlots); err != nil {
		return nil, err
	}
	if thresholdM3 <= 0 {
		return nil, fmt.Errorf("%w: threshold_m3 must be positive", ErrInvalidInput)
	}
	if accruedM3 < 0 {
		return nil, fmt.Errorf("%w: accrued_m3 must be non-negative", ErrInvalidInput)
	}

	routeRepo := repository.NewParticipantRouteRepository(s.db)
	route, err := routeRepo.GetByID(ctx, routeID)
	if err != nil {
		return nil, err
	}
	if route.UserID != userID {
		return nil, repository.ErrNotFound
	}

	threshold := &models.DispatchThreshold{
		RouteID:     routeID,
		ThresholdM3: thresholdM3,
		AccruedM3:   accruedM3,
		UpdatedAt:   time.Now(),
	}
	if err := repository.NewDispatchThresholdRepository(s.db).Upsert(ctx, threshold); err != nil {
		return nil, err
	}

	// Автоматический режим конкурса водителей (ТЗ §11.4): порог достигнут —
	// система объявляет конкурс сама, если он включён настройкой.
	if err := s.maybeAutoAnnounceDriverCompetition(ctx, userID, route, threshold); err != nil {
		return nil, err
	}
	return threshold, nil
}

func (s *CargoService) DeleteRouteDispatchThreshold(ctx context.Context, userID, routeID uuid.UUID) error {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return err
	}
	if err := s.requireTool(ctx, userID, ToolManageWarehouseSlots); err != nil {
		return err
	}

	routeRepo := repository.NewParticipantRouteRepository(s.db)
	route, err := routeRepo.GetByID(ctx, routeID)
	if err != nil {
		return err
	}
	if route.UserID != userID {
		return repository.ErrNotFound
	}
	return repository.NewDispatchThresholdRepository(s.db).DeleteByRouteID(ctx, routeID)
}
