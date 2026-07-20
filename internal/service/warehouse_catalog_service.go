package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/geo"
	"gandm/internal/models"
	"gandm/internal/repository"
)

const ToolManageWarehouse = "manage_warehouse_slots"

func validateWarehouse(in models.Warehouse) (models.Warehouse, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, fmt.Errorf("%w: warehouse name is required", ErrInvalidInput)
	}
	address, err := validateGeoPoint("address", in.Address)
	if err != nil {
		return in, err
	}
	in.Address = address
	if in.CoveredAreaM2 < 0 || in.OpenAreaM2 < 0 || in.AvailableCoveredAreaM2 < 0 || in.AvailableOpenAreaM2 < 0 {
		return in, fmt.Errorf("%w: areas must not be negative", ErrInvalidInput)
	}
	if in.AvailableCoveredAreaM2 > in.CoveredAreaM2 || in.AvailableOpenAreaM2 > in.OpenAreaM2 {
		return in, fmt.Errorf("%w: available area cannot exceed total area", ErrInvalidInput)
	}
	if in.CoveredAreaM2+in.OpenAreaM2 <= 0 {
		return in, fmt.Errorf("%w: warehouse area must be positive", ErrInvalidInput)
	}
	if in.Status != models.WarehouseDraft && in.Status != models.WarehousePublished && in.Status != models.WarehousePaused {
		in.Status = models.WarehouseDraft
	}
	in.Services = uniqueStrings(in.Services)
	for i, point := range in.PickupCities {
		valid, err := validateGeoPoint(fmt.Sprintf("pickup_cities[%d]", i), point)
		if err != nil {
			return in, err
		}
		in.PickupCities[i] = valid
	}
	// Города забора и направления отправки — независимые списки. Город, из
	// которого склад может забрать груз, не означает, что склад формирует
	// партию по направлению «склад → этот город».
	seenRoutes := map[string]bool{}
	validRoutes := make([]models.WarehouseDispatchRoute, 0, len(in.DispatchRoutes))
	for i, route := range in.DispatchRoutes {
		origin, err := validateGeoPoint(fmt.Sprintf("dispatch_routes[%d].origin", i), route.Origin)
		if err != nil {
			return in, err
		}
		destination, err := validateGeoPoint(fmt.Sprintf("dispatch_routes[%d].destination", i), route.Destination)
		if err != nil {
			return in, err
		}
		key := fmt.Sprintf("%.7f:%.7f:%.7f:%.7f", origin.Lat, origin.Lng, destination.Lat, destination.Lng)
		if seenRoutes[key] {
			continue
		}
		seenRoutes[key] = true
		route.Origin = origin
		route.Destination = destination
		validRoutes = append(validRoutes, route)
	}
	in.DispatchRoutes = validRoutes
	if !in.ConsolidationEnabled {
		in.ConsolidationMinVolumeM3 = 0
		in.ConsolidationFrequency = ""
	}
	if !in.PickupEnabled {
		in.PickupCities = []models.GeoPoint{}
		in.PickupRadiusKm = 0
		in.OwnTransport = false
		in.PickupMaxWeightKg = 0
		in.PickupMaxVolumeM3 = 0
		in.PickupPriceMode = ""
	}
	return in, nil
}

func syncWarehouseDispatchRoutes(ctx context.Context, q repository.Querier, userID uuid.UUID, routes []models.WarehouseDispatchRoute) ([]models.WarehouseDispatchRoute, error) {
	repo := repository.NewParticipantRouteRepository(q)
	result := make([]models.WarehouseDispatchRoute, 0, len(routes))
	for _, item := range routes {
		existing, err := repo.GetByUserAndPoints(ctx, userID, item.Origin, item.Destination)
		if err == nil {
			item.ID = existing.ID
			result = append(result, item)
			continue
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
		if item.ID == uuid.Nil {
			item.ID = uuid.New()
		}
		route := &models.ParticipantRoute{
			ID: item.ID, UserID: userID, Origin: item.Origin,
			Destination: item.Destination, CreatedAt: time.Now(),
		}
		if err := repo.CreateForWarehouse(ctx, route); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

// PublicWarehouseCard is a warehouse in search results WITHOUT contacts —
// contact_name/phone are subscription-gated (the UI shows a subscription
// upsell instead of the contact for now).
type PublicWarehouseCard struct {
	ID                     uuid.UUID                       `json:"id"`
	Name                   string                          `json:"name"`
	Address                models.GeoPoint                 `json:"address"`
	Description            string                          `json:"description"`
	WorkHours              string                          `json:"work_hours"`
	CoveredAreaM2          float64                         `json:"covered_area_m2"`
	OpenAreaM2             float64                         `json:"open_area_m2"`
	AvailableCoveredAreaM2 float64                         `json:"available_covered_area_m2"`
	AvailableOpenAreaM2    float64                         `json:"available_open_area_m2"`
	MaxWeightKg            float64                         `json:"max_weight_kg"`
	MaxVolumeM3            float64                         `json:"max_volume_m3"`
	Services               []string                        `json:"services"`
	ConsolidationEnabled   bool                            `json:"consolidation_enabled"`
	PickupEnabled          bool                            `json:"pickup_enabled"`
	PickupRadiusKm         float64                         `json:"pickup_radius_km"`
	OwnTransport           bool                            `json:"own_transport"`
	DispatchRoutes         []models.WarehouseDispatchRoute `json:"dispatch_routes"`
}

func publicWarehouseCard(w models.Warehouse) PublicWarehouseCard {
	return PublicWarehouseCard{
		ID: w.ID, Name: w.Name, Address: w.Address, Description: w.Description, WorkHours: w.WorkHours,
		CoveredAreaM2: w.CoveredAreaM2, OpenAreaM2: w.OpenAreaM2,
		AvailableCoveredAreaM2: w.AvailableCoveredAreaM2, AvailableOpenAreaM2: w.AvailableOpenAreaM2,
		MaxWeightKg: w.MaxWeightKg, MaxVolumeM3: w.MaxVolumeM3, Services: w.Services,
		ConsolidationEnabled: w.ConsolidationEnabled, PickupEnabled: w.PickupEnabled,
		PickupRadiusKm: w.PickupRadiusKm, OwnTransport: w.OwnTransport, DispatchRoutes: w.DispatchRoutes,
	}
}

// SearchWarehouses returns published warehouses within radiusKm of the point,
// nearest first, without contacts. Any eligible user can search; no tool is
// required (this is the searcher side, not the warehouse-owner side).
func (s *CargoService) SearchWarehouses(ctx context.Context, userID uuid.UUID, lat, lng, radiusKm float64) ([]PublicWarehouseCard, error) {
	if _, err := s.requireEligibleUser(ctx, userID); err != nil {
		return nil, err
	}
	if !geo.ValidLatLng(lat, lng) {
		return nil, fmt.Errorf("%w: coordinates out of WGS-84 range", ErrInvalidInput)
	}
	if radiusKm <= 0 || radiusKm > 3000 {
		return nil, fmt.Errorf("%w: radius_km must be between 0 and 3000", ErrInvalidInput)
	}
	list, err := repository.NewWarehouseRepository(s.db).SearchPublishedNear(ctx, lat, lng, radiusKm)
	if err != nil {
		return nil, err
	}
	cards := make([]PublicWarehouseCard, 0, len(list))
	for _, w := range list {
		cards = append(cards, publicWarehouseCard(w))
	}
	return cards, nil
}

func (s *CargoService) ListMyWarehouses(ctx context.Context, userID uuid.UUID) ([]models.Warehouse, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageWarehouse); err != nil {
		return nil, err
	}
	return repository.NewWarehouseRepository(s.db).ListByUserID(ctx, userID)
}

func (s *CargoService) CreateMyWarehouse(ctx context.Context, userID uuid.UUID, input models.Warehouse) (*models.Warehouse, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageWarehouse); err != nil {
		return nil, err
	}
	input.UserID = userID
	valid, err := validateWarehouse(input)
	if err != nil {
		return nil, err
	}
	valid.ID = uuid.New()
	valid.CreatedAt = time.Now()
	valid.UpdatedAt = valid.CreatedAt
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	valid.DispatchRoutes, err = syncWarehouseDispatchRoutes(ctx, tx, userID, valid.DispatchRoutes)
	if err != nil {
		return nil, err
	}
	if err := repository.NewWarehouseRepository(tx).Create(ctx, &valid); err != nil {
		return nil, err
	}
	routeIDs := make([]uuid.UUID, 0, len(valid.DispatchRoutes))
	for _, route := range valid.DispatchRoutes {
		routeIDs = append(routeIDs, route.ID)
	}
	if err := repository.NewParticipantRouteRepository(tx).ReplaceWarehouseLinks(ctx, valid.ID, routeIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &valid, nil
}

func (s *CargoService) UpdateMyWarehouse(ctx context.Context, userID, warehouseID uuid.UUID, input models.Warehouse) (*models.Warehouse, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageWarehouse); err != nil {
		return nil, err
	}
	existing, err := repository.NewWarehouseRepository(s.db).GetOwned(ctx, warehouseID, userID)
	if err != nil {
		return nil, err
	}
	input.ID = warehouseID
	input.UserID = userID
	valid, err := validateWarehouse(input)
	if err != nil {
		return nil, err
	}
	valid.UpdatedAt = time.Now()
	valid.CreatedAt = existing.CreatedAt
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	valid.DispatchRoutes, err = syncWarehouseDispatchRoutes(ctx, tx, userID, valid.DispatchRoutes)
	if err != nil {
		return nil, err
	}
	if err := repository.NewWarehouseRepository(tx).Update(ctx, &valid); err != nil {
		return nil, err
	}
	routeIDs := make([]uuid.UUID, 0, len(valid.DispatchRoutes))
	for _, route := range valid.DispatchRoutes {
		routeIDs = append(routeIDs, route.ID)
	}
	if err := repository.NewParticipantRouteRepository(tx).ReplaceWarehouseLinks(ctx, valid.ID, routeIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &valid, nil
}

func (s *CargoService) DeleteMyWarehouse(ctx context.Context, userID, warehouseID uuid.UUID) error {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return err
	}
	if err := s.requireTool(ctx, userID, ToolManageWarehouse); err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := repository.NewWarehouseRepository(tx).Delete(ctx, warehouseID, userID); err != nil {
		return err
	}
	if err := repository.NewParticipantRouteRepository(tx).ReplaceWarehouseLinks(ctx, warehouseID, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
