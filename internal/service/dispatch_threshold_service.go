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

const (
	DispatchPlanCollecting = "collecting"
	DispatchPlanReady      = "ready"
	DispatchPlanPaused     = "paused"
	DispatchPlanDispatched = "dispatched"
)

type DispatchThresholdInput struct {
	WarehouseID           *uuid.UUID
	ThresholdM3           float64
	ManualAccruedM3       float64
	EstimatedDispatchDate *time.Time
	Status                string
}

// RouteWithThreshold is the self-service view: the warehouse's route plus
// its threshold, if one is set.
type RouteWithThreshold struct {
	WarehouseID   uuid.UUID                 `json:"warehouse_id"`
	Route         models.ParticipantRoute   `json:"route"`
	ActiveCargoM3 float64                   `json:"active_cargo_m3"`
	Threshold     *models.DispatchThreshold `json:"threshold,omitempty"`
}

func (s *CargoService) ListMyDispatchThresholds(ctx context.Context, userID uuid.UUID) ([]RouteWithThreshold, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageWarehouseSlots); err != nil {
		return nil, err
	}

	warehouses, err := repository.NewWarehouseRepository(s.db).ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	thresholds, err := repository.NewDispatchThresholdRepository(s.db).ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	activeCargo, err := repository.NewCargoRequestRepository(s.db).ActiveVolumeByOwnedRoute(
		ctx, userID, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm,
	)
	if err != nil {
		return nil, err
	}

	items := make([]RouteWithThreshold, 0)
	for _, warehouse := range warehouses {
		for _, warehouseRoute := range warehouse.DispatchRoutes {
			route := models.ParticipantRoute{
				ID: warehouseRoute.ID, UserID: userID, Origin: warehouseRoute.Origin,
				Destination: warehouseRoute.Destination, CreatedAt: warehouse.CreatedAt,
			}
			platformAccruedM3 := activeCargo[route.ID]
			row := RouteWithThreshold{WarehouseID: warehouse.ID, Route: route, ActiveCargoM3: platformAccruedM3}
			if t, ok := thresholds[route.ID]; ok && t.WarehouseID != nil && *t.WarehouseID == warehouse.ID {
				threshold := t
				threshold.PlatformAccruedM3 = platformAccruedM3
				threshold.AccruedM3 = threshold.ManualAccruedM3 + platformAccruedM3
				threshold.RemainingM3 = max(0, threshold.ThresholdM3-threshold.AccruedM3)
				if threshold.Status == DispatchPlanCollecting || threshold.Status == DispatchPlanReady {
					if threshold.AccruedM3 >= threshold.ThresholdM3 {
						threshold.Status = DispatchPlanReady
					} else {
						threshold.Status = DispatchPlanCollecting
					}
				}
				row.Threshold = &threshold
			}
			items = append(items, row)
		}
	}
	return items, nil
}

// SetRouteDispatchThreshold upserts the threshold on the caller's own route.
// accrued > threshold is allowed — the warehouse may overfill before
// dispatching.
func (s *CargoService) SetRouteDispatchThreshold(ctx context.Context, userID, routeID uuid.UUID, input DispatchThresholdInput) (*models.DispatchThreshold, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageWarehouseSlots); err != nil {
		return nil, err
	}
	if input.ThresholdM3 <= 0 {
		return nil, fmt.Errorf("%w: threshold_m3 must be positive", ErrInvalidInput)
	}
	if input.ManualAccruedM3 < 0 {
		return nil, fmt.Errorf("%w: manual_accrued_m3 must be non-negative", ErrInvalidInput)
	}

	routeRepo := repository.NewParticipantRouteRepository(s.db)
	route, err := routeRepo.GetByID(ctx, routeID)
	if err != nil {
		return nil, err
	}
	if route.UserID != userID {
		return nil, repository.ErrNotFound
	}
	if input.WarehouseID == nil {
		return nil, fmt.Errorf("%w: warehouse_id is required", ErrInvalidInput)
	}
	warehouse, err := repository.NewWarehouseRepository(s.db).GetOwned(ctx, *input.WarehouseID, userID)
	if err != nil {
		return nil, err
	}
	routeBelongsToWarehouse := false
	for _, warehouseRoute := range warehouse.DispatchRoutes {
		if warehouseRoute.ID == routeID {
			routeBelongsToWarehouse = true
			break
		}
	}
	if !routeBelongsToWarehouse {
		return nil, fmt.Errorf("%w: route is not configured for this warehouse", ErrInvalidInput)
	}
	activeCargo, err := repository.NewCargoRequestRepository(s.db).ActiveVolumeByOwnedRoute(
		ctx, userID, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm,
	)
	if err != nil {
		return nil, err
	}
	platformAccruedM3 := activeCargo[routeID]
	totalAccruedM3 := input.ManualAccruedM3 + platformAccruedM3

	status := input.Status
	if status == "" {
		status = DispatchPlanCollecting
	}
	if status != DispatchPlanCollecting && status != DispatchPlanReady &&
		status != DispatchPlanPaused && status != DispatchPlanDispatched {
		return nil, fmt.Errorf("%w: invalid dispatch plan status", ErrInvalidInput)
	}
	// Готовность определяется цифрами, а не ручным выбором. Пауза и отметка
	// об отправке остаются осознанными действиями склада.
	if status == DispatchPlanCollecting || status == DispatchPlanReady {
		if totalAccruedM3 >= input.ThresholdM3 {
			status = DispatchPlanReady
		} else {
			status = DispatchPlanCollecting
		}
	}
	if status == DispatchPlanDispatched {
		// «Отправлено» завершает текущую партию и начинает следующий цикл с
		// нуля. Площадь склада при этом не затрагивается.
		input.ManualAccruedM3 = 0
		totalAccruedM3 = 0
	}

	threshold := &models.DispatchThreshold{
		RouteID:               routeID,
		WarehouseID:           input.WarehouseID,
		ThresholdM3:           input.ThresholdM3,
		AccruedM3:             totalAccruedM3,
		PlatformAccruedM3:     platformAccruedM3,
		ManualAccruedM3:       input.ManualAccruedM3,
		RemainingM3:           max(0, input.ThresholdM3-totalAccruedM3),
		EstimatedDispatchDate: input.EstimatedDispatchDate,
		Status:                status,
		UpdatedAt:             time.Now(),
	}
	if err := repository.NewDispatchThresholdRepository(s.db).Upsert(ctx, threshold); err != nil {
		return nil, err
	}

	// Автоматический режим конкурса водителей (ТЗ §11.4): порог достигнут —
	// система объявляет конкурс сама, если он включён настройкой.
	if status == DispatchPlanCollecting || status == DispatchPlanReady {
		if err := s.maybeAutoAnnounceDriverCompetition(ctx, userID, route, threshold); err != nil {
			return nil, err
		}
	}

	// Партия склада (ТЗ §10.1): порог достигнут → общий чат клиентов
	// партии; порог снова не добран → активная партия отправлена.
	if status != DispatchPlanPaused {
		if err := s.syncWarehouseBatch(ctx, userID, route, threshold); err != nil {
			return nil, err
		}
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
