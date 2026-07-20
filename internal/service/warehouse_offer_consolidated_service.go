package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

// ListConsolidatedForMyWarehouses returns open consolidated requests the
// owner's warehouses can collect — the consolidated bid queue.
func (s *CargoService) ListConsolidatedForMyWarehouses(ctx context.Context, ownerID uuid.UUID) ([]models.ConsolidatedRequest, error) {
	if _, err := s.requireEligibleUser(ctx, ownerID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, ownerID, ToolManageWarehouse); err != nil {
		return nil, err
	}
	return repository.NewConsolidationRepository(s.db).ListOpenMatchingOwnerWarehouses(ctx, ownerID, s.matchRadius())
}

// SubmitWarehouseOfferForConsolidated records a warehouse's price bid on a
// consolidated request.
func (s *CargoService) SubmitWarehouseOfferForConsolidated(ctx context.Context, ownerID, consolidatedID, warehouseID uuid.UUID, price float64, currency, conditions string) (*models.WarehouseOffer, error) {
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	consRepo := repository.NewConsolidationRepository(tx)
	cons, err := consRepo.GetConsolidatedByIDForUpdate(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.Status != models.CargoRequestOpen {
		return nil, ErrCargoNotOpen
	}
	offerRepo := repository.NewWarehouseOfferRepository(tx)
	if selected, err := offerRepo.HasSelectedForConsolidated(ctx, consolidatedID); err != nil {
		return nil, err
	} else if selected {
		return nil, ErrWarehouseOfferWrongState
	}
	if isMember, err := consRepo.IsConsolidatedMember(ctx, ownerID, consolidatedID); err != nil {
		return nil, err
	} else if isMember {
		return nil, fmt.Errorf("%w: cannot offer on your own consolidation", ErrInvalidInput)
	}
	if !warehouseCanCollect(warehouse, cons.Origin, cons.Destination, s.matchRadius()) {
		return nil, ErrWarehouseNotEligibleForCargo
	}

	offer := &models.WarehouseOffer{
		ID: uuid.New(), ConsolidatedRequestID: &consolidatedID, WarehouseID: warehouseID, WarehouseOwnerID: ownerID,
		Price: price, Currency: cur, Conditions: conditions, Status: models.WarehouseOfferSubmitted,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	// Check-then-act (not try-and-catch): a unique violation would abort the
	// whole transaction, so we can't fall back to UPDATE inside it.
	existing, err := offerRepo.GetForConsolidated(ctx, consolidatedID, warehouseID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		// Re-quote an existing offer (e.g. after a late join grew the volume).
		if err := offerRepo.UpdateForConsolidated(ctx, consolidatedID, warehouseID, price, cur, conditions); err != nil {
			return nil, err
		}
		offer.ID = existing.ID
	} else {
		if err := offerRepo.Create(ctx, offer); err != nil {
			return nil, err
		}
	}
	if err := s.notifyConsolidatedMembers(ctx, tx, consolidatedID, cons, "warehouse_offer_received"); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return offer, nil
}

// ListWarehouseOffersForConsolidated — a member's view of warehouse bids on the
// consolidation. Contacts appear only for a selected offer.
func (s *CargoService) ListWarehouseOffersForConsolidated(ctx context.Context, clientID, consolidatedID uuid.UUID) ([]WarehouseOfferView, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	consRepo := repository.NewConsolidationRepository(s.db)
	if isMember, err := consRepo.IsConsolidatedMember(ctx, clientID, consolidatedID); err != nil {
		return nil, err
	} else if !isMember {
		return nil, repository.ErrNotFound
	}

	offers, err := repository.NewWarehouseOfferRepository(s.db).ListByConsolidated(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	return s.warehouseOfferViews(ctx, offers)
}

// SelectWarehouseOfferForConsolidated picks a warehouse for the consolidation:
// reveals its contact, opens a shared chat with all co-owners and the
// warehouse, and rejects the other offers.
func (s *CargoService) SelectWarehouseOfferForConsolidated(ctx context.Context, clientID, consolidatedID, offerID uuid.UUID) (*WarehouseSelectResult, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	consRepo := repository.NewConsolidationRepository(tx)
	if isMember, err := consRepo.IsConsolidatedMember(ctx, clientID, consolidatedID); err != nil {
		return nil, err
	} else if !isMember {
		return nil, repository.ErrNotFound
	}
	cons, err := consRepo.GetConsolidatedByIDForUpdate(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}

	offerRepo := repository.NewWarehouseOfferRepository(tx)
	offer, err := offerRepo.GetByIDForUpdate(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer.ConsolidatedRequestID == nil || *offer.ConsolidatedRequestID != consolidatedID {
		return nil, fmt.Errorf("%w: offer does not belong to this consolidation", ErrInvalidInput)
	}
	if offer.Status != models.WarehouseOfferSubmitted {
		return nil, ErrWarehouseOfferWrongState
	}

	// Shared chat: all co-owners plus the warehouse.
	members, err := consRepo.ListMemberClients(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	chatRepo := repository.NewChatRepository(tx)
	chat := &models.Chat{ID: uuid.New(), WarehouseOfferID: &offer.ID, CreatedAt: time.Now()}
	if err := chatRepo.Create(ctx, chat); err != nil {
		return nil, err
	}
	for _, m := range members {
		if err := chatRepo.AddParticipant(ctx, chat.ID, m); err != nil {
			return nil, err
		}
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, offer.WarehouseOwnerID); err != nil {
		return nil, err
	}
	if err := offerRepo.MarkSelectedForConsolidated(ctx, consolidatedID, offerID, chat.ID); err != nil {
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
	if err := s.notifyConsolidatedMembers(ctx, tx, consolidatedID, cons, "warehouse_offer_selected"); err != nil {
		return nil, err
	}
	if err := s.notifyWarehouseConsolidation(ctx, tx, offer.WarehouseOwnerID, cons, "warehouse_offer_selected"); err != nil {
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

// notifyMatchingWarehousesOnConsolidation alerts warehouse owners whose
// warehouse can collect a newly created consolidation (best-effort).
func (s *CargoService) notifyMatchingWarehousesOnConsolidation(ctx context.Context, q repository.Querier, cons *models.ConsolidatedRequest) error {
	ownerIDs, err := repository.NewWarehouseRepository(q).ListPublishedOwnersMatching(ctx, cons.Origin.Lat, cons.Origin.Lng, cons.Destination.Lat, cons.Destination.Lng, s.matchRadius())
	if err != nil {
		return err
	}
	for _, ownerID := range ownerIDs {
		if err := s.notifyWarehouseConsolidation(ctx, q, ownerID, cons, "consolidation_available_for_warehouse"); err != nil {
			return err
		}
	}
	return nil
}

// warehouseOfferViews turns raw offers into views. Warehouses and (for
// selected offers) their owners are batch-loaded up front, so this costs two
// queries regardless of how many offers there are — not N+1.
func (s *CargoService) warehouseOfferViews(ctx context.Context, offers []models.WarehouseOffer) ([]WarehouseOfferView, error) {
	whIDs := make([]uuid.UUID, 0, len(offers))
	ownerIDs := make([]uuid.UUID, 0)
	for i := range offers {
		whIDs = append(whIDs, offers[i].WarehouseID)
		if offers[i].Status == models.WarehouseOfferSelected {
			ownerIDs = append(ownerIDs, offers[i].WarehouseOwnerID)
		}
	}
	warehouses, err := repository.NewWarehouseRepository(s.db).ListByIDs(ctx, whIDs)
	if err != nil {
		return nil, err
	}
	owners, err := repository.NewUserRepository(s.db).ListByIDs(ctx, ownerIDs)
	if err != nil {
		return nil, err
	}

	views := make([]WarehouseOfferView, 0, len(offers))
	for i := range offers {
		o := offers[i]
		wh := warehouses[o.WarehouseID]
		if wh == nil {
			return nil, repository.ErrNotFound
		}
		v := WarehouseOfferView{
			WarehouseOffer: o, WarehouseName: wh.Name, WarehouseAddress: wh.Address,
			CoveredAreaM2: wh.CoveredAreaM2, MaxWeightKg: wh.MaxWeightKg, MaxVolumeM3: wh.MaxVolumeM3,
		}
		if o.Status == models.WarehouseOfferSelected {
			owner := owners[o.WarehouseOwnerID]
			if owner == nil {
				return nil, repository.ErrNotFound
			}
			v.Contact = &WarehouseContact{WarehouseName: wh.Name, ContactName: wh.ContactName, ContactPhone: wh.ContactPhone, Email: owner.Email}
		}
		views = append(views, v)
	}
	return views, nil
}

func (s *CargoService) notifyConsolidatedMembers(ctx context.Context, q repository.Querier, consolidatedID uuid.UUID, cons *models.ConsolidatedRequest, notifType string) error {
	members, err := repository.NewConsolidationRepository(q).ListMemberClients(ctx, consolidatedID)
	if err != nil {
		return err
	}
	for _, m := range members {
		if err := s.notifyWarehouseConsolidation(ctx, q, m, cons, notifType); err != nil {
			return err
		}
	}
	return nil
}

func (s *CargoService) notifyWarehouseConsolidation(ctx context.Context, q repository.Querier, userID uuid.UUID, cons *models.ConsolidatedRequest, notifType string) error {
	payload, err := json.Marshal(map[string]any{
		"consolidated_request_id": cons.ID,
		"direction_label":         cons.Origin.Label + " → " + cons.Destination.Label,
	})
	if err != nil {
		return err
	}
	return repository.NewNotificationRepository(q).Create(ctx, &models.Notification{
		ID: uuid.New(), UserID: userID, Type: notifType, Payload: payload, IsRead: false, CreatedAt: time.Now(),
	})
}
