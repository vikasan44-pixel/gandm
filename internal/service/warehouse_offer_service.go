package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gandm/internal/geo"
	"gandm/internal/models"
	"gandm/internal/repository"
)

var (
	ErrWarehouseNotEligibleForCargo = errors.New("this warehouse cannot collect this cargo")
	ErrWarehouseOfferWrongState     = errors.New("warehouse offer is not in the required state")
	ErrWarehouseNotPublished        = errors.New("warehouse must be published with pickup enabled to offer")
)

// WarehouseContact is revealed to the cargo owner only after they pick this
// warehouse's offer.
type WarehouseContact struct {
	WarehouseName string `json:"warehouse_name"`
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	Email         string `json:"email"`
}

// WarehouseOfferView is what the cargo owner sees per offer: warehouse name and
// capacity + price, but NO contact until they select this offer.
type WarehouseOfferView struct {
	models.WarehouseOffer
	WarehouseName    string            `json:"warehouse_name"`
	WarehouseAddress models.GeoPoint   `json:"warehouse_address"`
	CoveredAreaM2    float64           `json:"covered_area_m2"`
	MaxWeightKg      float64           `json:"max_weight_kg"`
	MaxVolumeM3      float64           `json:"max_volume_m3"`
	Contact          *WarehouseContact `json:"contact,omitempty"`
}

type WarehouseSelectResult struct {
	Contact WarehouseContact `json:"contact"`
	ChatID  uuid.UUID        `json:"chat_id"`
}

// matchRadius is the destination-side tolerance for the dispatch-direction
// check (the wider per-country matching radius).
func (s *CargoService) matchRadius() float64 {
	if s.cfg.MatchRadiusCNKm > s.cfg.MatchRadiusKZKm {
		return s.cfg.MatchRadiusCNKm
	}
	return s.cfg.MatchRadiusKZKm
}

// warehouseCanCollect reports whether a warehouse can handle a cargo:
//   - pickup: the cargo origin is within pickup_radius_km of the warehouse
//     address OR any of its declared pickup cities (the factories it serves);
//   - dispatch: if the warehouse declares dispatch routes, the cargo
//     destination must be near one of them; a warehouse with no routes is not
//     direction-restricted.
func warehouseCanCollect(w *models.Warehouse, origin, destination models.GeoPoint, dispatchRadiusKm float64) bool {
	pickup := geo.HaversineKm(origin.Lat, origin.Lng, w.Address.Lat, w.Address.Lng) <= w.PickupRadiusKm
	for _, pc := range w.PickupCities {
		if geo.HaversineKm(origin.Lat, origin.Lng, pc.Lat, pc.Lng) <= w.PickupRadiusKm {
			pickup = true
			break
		}
	}
	if !pickup {
		return false
	}
	if len(w.DispatchRoutes) == 0 {
		return true
	}
	for _, dr := range w.DispatchRoutes {
		if geo.HaversineKm(destination.Lat, destination.Lng, dr.Destination.Lat, dr.Destination.Lng) <= dispatchRadiusKm {
			return true
		}
	}
	return false
}

// ListCargoForMyWarehouses returns open cargo the owner's warehouses can
// collect (origin within a warehouse's pickup radius) — the bid queue.
func (s *CargoService) ListCargoForMyWarehouses(ctx context.Context, ownerID uuid.UUID) ([]models.CargoRequest, error) {
	if _, err := s.requireEligibleUser(ctx, ownerID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, ownerID, ToolManageWarehouse); err != nil {
		return nil, err
	}
	return repository.NewCargoRequestRepository(s.db).ListOpenMatchingOwnerWarehouses(ctx, ownerID, s.matchRadius())
}

// SubmitWarehouseOffer records a warehouse's price bid on a cargo request.
func (s *CargoService) SubmitWarehouseOffer(ctx context.Context, ownerID, cargoID, warehouseID uuid.UUID, price float64, currency, conditions string) (*models.WarehouseOffer, error) {
	if _, err := s.requireEligibleUser(ctx, ownerID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, ownerID, ToolManageWarehouse); err != nil {
		return nil, err
	}
	if price <= 0 {
		return nil, fmt.Errorf("%w: price must be positive", ErrInvalidInput)
	}
	cur, err := s.resolveCurrency(currency)
	if err != nil {
		return nil, err
	}

	warehouse, err := repository.NewWarehouseRepository(s.db).GetOwned(ctx, warehouseID, ownerID)
	if err != nil {
		return nil, err
	}
	if warehouse.Status != models.WarehousePublished || !warehouse.PickupEnabled {
		return nil, ErrWarehouseNotPublished
	}

	cargo, err := repository.NewCargoRequestRepository(s.db).GetByID(ctx, cargoID)
	if err != nil {
		return nil, err
	}
	if cargo.ClientID == ownerID {
		return nil, fmt.Errorf("%w: cannot offer on your own cargo", ErrInvalidInput)
	}
	if cargo.Status != models.CargoRequestOpen {
		return nil, ErrCargoNotOpen
	}
	if !warehouseCanCollect(warehouse, cargo.Origin, cargo.Destination, s.matchRadius()) {
		return nil, ErrWarehouseNotEligibleForCargo
	}

	offer := &models.WarehouseOffer{
		ID: uuid.New(), CargoRequestID: &cargoID, WarehouseID: warehouseID, WarehouseOwnerID: ownerID,
		Price: price, Currency: cur, Conditions: conditions, Status: models.WarehouseOfferSubmitted,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := repository.NewWarehouseOfferRepository(tx).Create(ctx, offer); err != nil {
		return nil, err
	}
	if err := s.notifyWarehouseOffer(ctx, tx, cargo.ClientID, cargo, "warehouse_offer_received"); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return offer, nil
}

// ListWarehouseOffersForCargo — the cargo owner's view of warehouse bids on
// their cargo. Contacts appear only for an offer they already selected.
func (s *CargoService) ListWarehouseOffersForCargo(ctx context.Context, clientID, cargoID uuid.UUID) ([]WarehouseOfferView, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	cargo, err := repository.NewCargoRequestRepository(s.db).GetByID(ctx, cargoID)
	if err != nil {
		return nil, err
	}
	if cargo.ClientID != clientID {
		return nil, ErrForbiddenNotOwner
	}

	offers, err := repository.NewWarehouseOfferRepository(s.db).ListByCargo(ctx, cargoID)
	if err != nil {
		return nil, err
	}
	// Shared batched builder (2 queries total), not an N+1 loop per offer.
	return s.warehouseOfferViews(ctx, offers)
}

// SelectWarehouseOffer picks one warehouse offer: reveals its contact, opens a
// shared chat, and rejects the other offers on the cargo.
func (s *CargoService) SelectWarehouseOffer(ctx context.Context, clientID, cargoID, offerID uuid.UUID) (*WarehouseSelectResult, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	cargoRepo := repository.NewCargoRequestRepository(tx)
	cargo, err := cargoRepo.GetByIDForUpdate(ctx, cargoID)
	if err != nil {
		return nil, err
	}
	if cargo.ClientID != clientID {
		return nil, ErrForbiddenNotOwner
	}

	offerRepo := repository.NewWarehouseOfferRepository(tx)
	offer, err := offerRepo.GetByIDForUpdate(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer.CargoRequestID == nil || *offer.CargoRequestID != cargoID {
		return nil, fmt.Errorf("%w: offer does not belong to this cargo", ErrInvalidInput)
	}
	if offer.Status != models.WarehouseOfferSubmitted {
		return nil, ErrWarehouseOfferWrongState
	}

	chatRepo := repository.NewChatRepository(tx)
	chat := &models.Chat{ID: uuid.New(), WarehouseOfferID: &offer.ID, CreatedAt: time.Now()}
	if err := chatRepo.Create(ctx, chat); err != nil {
		return nil, err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, clientID); err != nil {
		return nil, err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, offer.WarehouseOwnerID); err != nil {
		return nil, err
	}
	if err := offerRepo.MarkSelected(ctx, cargoID, offerID, chat.ID); err != nil {
		return nil, err
	}

	wh, err := repository.NewWarehouseRepository(tx).GetByID(ctx, offer.WarehouseID)
	if err != nil {
		return nil, err
	}
	owner, err := repository.NewUserRepository(tx).GetByID(ctx, offer.WarehouseOwnerID)
	if err != nil {
		return nil, err
	}
	if err := s.notifyWarehouseOffer(ctx, tx, offer.WarehouseOwnerID, cargo, "warehouse_offer_selected"); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &WarehouseSelectResult{
		Contact: WarehouseContact{WarehouseName: wh.Name, ContactName: wh.ContactName, ContactPhone: wh.ContactPhone, Email: owner.Email},
		ChatID:  chat.ID,
	}, nil
}

// notifyMatchingWarehousesOnCargo alerts warehouse owners whose warehouse can
// collect a newly posted cargo (best-effort, called after cargo creation).
func (s *CargoService) notifyMatchingWarehousesOnCargo(ctx context.Context, cargo *models.CargoRequest) error {
	ownerIDs, err := repository.NewWarehouseRepository(s.db).ListPublishedOwnersMatching(ctx, cargo.Origin.Lat, cargo.Origin.Lng, cargo.Destination.Lat, cargo.Destination.Lng, s.matchRadius())
	if err != nil {
		return err
	}
	for _, ownerID := range ownerIDs {
		if ownerID == cargo.ClientID {
			continue
		}
		if err := s.notifyWarehouseOffer(ctx, s.db, ownerID, cargo, "cargo_available_for_warehouse"); err != nil {
			return err
		}
	}
	return nil
}

func (s *CargoService) notifyWarehouseOffer(ctx context.Context, q repository.Querier, userID uuid.UUID, cargo *models.CargoRequest, notifType string) error {
	payload, err := json.Marshal(map[string]any{
		"cargo_request_id": cargo.ID,
		"direction_label":  cargo.Origin.Label + " → " + cargo.Destination.Label,
	})
	if err != nil {
		return err
	}
	return repository.NewNotificationRepository(q).Create(ctx, &models.Notification{
		ID: uuid.New(), UserID: userID, Type: notifType, Payload: payload, IsRead: false, CreatedAt: time.Now(),
	})
}
