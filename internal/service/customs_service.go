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

// ToolManageCustomsDocs gates the customs-representative side of the
// competition (ТЗ §10.2): reps holding the tool are notified about matched
// consolidations and may bid for the clearance work.
const ToolManageCustomsDocs = "manage_customs_docs"

var (
	ErrConsolidationNotMatched = errors.New("consolidation is not matched yet")
	ErrCustomsAlreadySelected  = errors.New("a customs representative is already selected for this consolidation")
)

// CustomsCompetition is what a customs rep sees about an open competition:
// direction, totals and cargo names («упаковочный лист») — no personal data
// of the clients (ТЗ §10.2).
type CustomsCompetition struct {
	ConsolidatedRequestID uuid.UUID `json:"consolidated_request_id"`
	DirectionLabel        string    `json:"direction_label"`
	TotalVolumeM3         float64   `json:"total_volume_m3"`
	TotalWeightKg         float64   `json:"total_weight_kg"`
	CargoNames            []string  `json:"cargo_names"`
	CreatedAt             time.Time `json:"created_at"`
	// MyOffer is the rep's own bid on this competition, if any.
	MyOffer *models.ConsolidatedCustomsOffer `json:"my_offer,omitempty"`
}

// AnonymizedCustomsOffer mirrors AnonymizedOffer for the customs
// competition: clients compare number, rating and price — never identity.
type AnonymizedCustomsOffer struct {
	OfferID     uuid.UUID                 `json:"offer_id"`
	OfferNumber int                       `json:"offer_number"`
	Rating      *float64                  `json:"rating"`
	RatingCount int                       `json:"rating_count"`
	Price       float64                   `json:"price"`
	Currency    string                    `json:"currency"`
	Conditions  string                    `json:"conditions"`
	Status      models.CustomsOfferStatus `json:"status"`
}

// notifyCustomsReps fans out the «объявлен конкурс на оформление» event to
// every active holder of manage_customs_docs. Called inside the
// deal-closing transaction of SelectConsolidatedOffer.
func (s *CargoService) notifyCustomsReps(ctx context.Context, q repository.Querier, cons *models.ConsolidatedRequest) error {
	toolRepo := repository.NewToolRepository(q)
	repIDs, err := toolRepo.ListActiveUserIDsWithTool(ctx, ToolManageCustomsDocs)
	if err != nil {
		return err
	}
	if len(repIDs) == 0 {
		return nil
	}

	consRepo := repository.NewConsolidationRepository(q)
	cargoNames, err := consRepo.ListMemberDescriptions(ctx, cons.ID)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"consolidated_request_id": cons.ID,
		"direction_label":         cons.Origin.Label + " → " + cons.Destination.Label,
		"total_volume_m3":         cons.TotalVolumeM3,
		"total_weight_kg":         cons.TotalWeightKg,
		"cargo_names":             cargoNames,
	})
	if err != nil {
		return err
	}

	notifRepo := repository.NewNotificationRepository(q)
	now := time.Now()
	for _, repID := range repIDs {
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    repID,
			Type:      "consolidated_customs_available",
			Payload:   payload,
			IsRead:    false,
			CreatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ListCustomsCompetitions: open competitions for a tooled rep — matched
// consolidations without a selected clearance offer, plus the rep's own bid
// on each.
func (s *CargoService) ListCustomsCompetitions(ctx context.Context, repID uuid.UUID) ([]CustomsCompetition, error) {
	if err := s.requireActiveUser(ctx, repID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, repID, ToolManageCustomsDocs); err != nil {
		return nil, err
	}

	consRepo := repository.NewConsolidationRepository(s.db)
	consolidations, err := consRepo.ListMatchedWithoutSelectedCustoms(ctx)
	if err != nil {
		return nil, err
	}
	myOffers, err := repository.NewCustomsOfferRepository(s.db).ListByRepID(ctx, repID)
	if err != nil {
		return nil, err
	}

	items := make([]CustomsCompetition, 0, len(consolidations))
	for i := range consolidations {
		cons := &consolidations[i]
		cargoNames, err := consRepo.ListMemberDescriptions(ctx, cons.ID)
		if err != nil {
			return nil, err
		}
		row := CustomsCompetition{
			ConsolidatedRequestID: cons.ID,
			DirectionLabel:        cons.Origin.Label + " → " + cons.Destination.Label,
			TotalVolumeM3:         cons.TotalVolumeM3,
			TotalWeightKg:         cons.TotalWeightKg,
			CargoNames:            cargoNames,
			CreatedAt:             cons.CreatedAt,
		}
		if offer, ok := myOffers[cons.ID]; ok {
			o := offer
			row.MyOffer = &o
		}
		items = append(items, row)
	}
	return items, nil
}

// CreateCustomsOffer: a tooled rep bids on a matched consolidation.
func (s *CargoService) CreateCustomsOffer(ctx context.Context, repID, consolidatedID uuid.UUID, price float64, conditions string) (*models.ConsolidatedCustomsOffer, error) {
	if price <= 0 {
		return nil, fmt.Errorf("%w: price must be positive", ErrInvalidInput)
	}
	if err := s.requireActiveUser(ctx, repID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, repID, ToolManageCustomsDocs); err != nil {
		return nil, err
	}

	consRepo := repository.NewConsolidationRepository(s.db)
	cons, err := consRepo.GetConsolidatedByID(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.Status != models.CargoRequestMatched {
		return nil, ErrConsolidationNotMatched
	}

	customsRepo := repository.NewCustomsOfferRepository(s.db)
	selected, err := customsRepo.HasSelected(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if selected {
		return nil, ErrCustomsAlreadySelected
	}

	offer := &models.ConsolidatedCustomsOffer{
		ID:                    uuid.New(),
		ConsolidatedRequestID: consolidatedID,
		CustomsRepID:          repID,
		Price:                 price,
		Currency:              "KZT",
		Conditions:            conditions,
		Status:                models.CustomsOfferSubmitted,
		CreatedAt:             time.Now(),
	}
	if err := customsRepo.Create(ctx, offer); err != nil {
		return nil, err
	}
	return offer, nil
}

// ListCustomsOffersForClient: consolidation members compare the clearance
// bids anonymously, same policy as carrier offers.
func (s *CargoService) ListCustomsOffersForClient(ctx context.Context, clientID, consolidatedID uuid.UUID) ([]AnonymizedCustomsOffer, error) {
	if _, err := s.getConsolidatedForMember(ctx, s.db, clientID, consolidatedID); err != nil {
		return nil, err
	}

	offers, err := repository.NewCustomsOfferRepository(s.db).ListByConsolidatedID(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}

	repIDs := make([]uuid.UUID, 0, len(offers))
	seen := make(map[uuid.UUID]bool, len(offers))
	for _, o := range offers {
		if !seen[o.CustomsRepID] {
			seen[o.CustomsRepID] = true
			repIDs = append(repIDs, o.CustomsRepID)
		}
	}
	ratings, err := repository.NewRatingRepository(s.db).SummariesForUsers(ctx, repIDs)
	if err != nil {
		return nil, err
	}

	items := make([]AnonymizedCustomsOffer, 0, len(offers))
	for i, o := range offers {
		row := AnonymizedCustomsOffer{
			OfferID:     o.ID,
			OfferNumber: i + 1,
			Price:       o.Price,
			Currency:    o.Currency,
			Conditions:  o.Conditions,
			Status:      o.Status,
		}
		if summary, ok := ratings[o.CustomsRepID]; ok {
			row.Rating = summary.Average
			row.RatingCount = summary.Count
		}
		items = append(items, row)
	}
	return items, nil
}

type CustomsSelectResult struct {
	Contact      RevealedContact `json:"contact"`
	CustomsRepID uuid.UUID       `json:"customs_rep_id"`
}

// SelectCustomsOffer closes the customs competition: either member client
// picks the winner (the joint discussion happens in the shared chat, ТЗ
// §10.2), the rep's contact is revealed and the rep joins the shared chat.
// Row lock on the consolidation serializes concurrent selects; a repeated
// select of the already-chosen offer returns the same result without
// duplicate side effects.
func (s *CargoService) SelectCustomsOffer(ctx context.Context, clientID, consolidatedID, customsOfferID uuid.UUID) (*CustomsSelectResult, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	consRepo := repository.NewConsolidationRepository(tx)
	isMember, err := consRepo.IsConsolidatedMember(ctx, clientID, consolidatedID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, repository.ErrNotFound
	}
	cons, err := consRepo.GetConsolidatedByIDForUpdate(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.Status != models.CargoRequestMatched {
		return nil, ErrConsolidationNotMatched
	}

	customsRepo := repository.NewCustomsOfferRepository(tx)
	offer, err := customsRepo.GetByID(ctx, customsOfferID)
	if err != nil {
		return nil, err
	}
	if offer.ConsolidatedRequestID != consolidatedID {
		return nil, fmt.Errorf("%w: offer does not belong to this consolidation", ErrInvalidInput)
	}

	userRepo := repository.NewUserRepository(tx)

	// Idempotent replay: this offer already won — return the revealed
	// contact without repeating side effects.
	if offer.Status == models.CustomsOfferSelected {
		rep, err := userRepo.GetByID(ctx, offer.CustomsRepID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &CustomsSelectResult{
			Contact:      RevealedContact{CompanyName: rep.CompanyName, Email: rep.Email, Phone: rep.Phone},
			CustomsRepID: rep.ID,
		}, nil
	}

	selected, err := customsRepo.HasSelected(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if selected {
		return nil, ErrCustomsAlreadySelected
	}

	if err := customsRepo.MarkSelected(ctx, consolidatedID, customsOfferID); err != nil {
		return nil, err
	}

	// The rep joins the shared consolidation chat — documents and the
	// clearance discussion happen there (ТЗ §10.3).
	if cons.ChatID != nil {
		chatRepo := repository.NewChatRepository(tx)
		if err := chatRepo.AddParticipant(ctx, *cons.ChatID, offer.CustomsRepID); err != nil {
			return nil, err
		}
	}

	rep, err := userRepo.GetByID(ctx, offer.CustomsRepID)
	if err != nil {
		return nil, err
	}

	clients, err := consRepo.ListMemberClients(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"consolidated_request_id": consolidatedID,
		"customs_offer_id":        customsOfferID,
	})
	if err != nil {
		return nil, err
	}
	notifRepo := repository.NewNotificationRepository(tx)
	now := time.Now()
	for _, uid := range append(clients, offer.CustomsRepID) {
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    uid,
			Type:      "customs_rep_selected",
			Payload:   payload,
			IsRead:    false,
			CreatedAt: now,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &CustomsSelectResult{
		Contact:      RevealedContact{CompanyName: rep.CompanyName, Email: rep.Email, Phone: rep.Phone},
		CustomsRepID: rep.ID,
	}, nil
}
