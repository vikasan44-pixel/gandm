package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

var (
	ErrAlreadyResponded = errors.New("consolidation suggestion already responded to")
	// Default capacity limits used only if the platform_settings rows are
	// missing (they're seeded by migration 000021).
	defaultMaxVolumeM3 = 90.0
	defaultMaxWeightKg = 20000.0
)

// getCapacityLimits reads the consolidation capacity limits from the DB on
// every call — admins edit them at runtime, no restart required.
func (s *CargoService) getCapacityLimits(ctx context.Context) (maxVolume, maxWeight float64) {
	maxVolume, maxWeight = defaultMaxVolumeM3, defaultMaxWeightKg
	settingsRepo := repository.NewSettingsRepository(s.db)
	if v, err := settingsRepo.Get(ctx, repository.SettingMaxVolumeM3); err == nil {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil && parsed > 0 {
			maxVolume = parsed
		}
	}
	if v, err := settingsRepo.Get(ctx, repository.SettingMaxWeightKg); err == nil {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil && parsed > 0 {
			maxWeight = parsed
		}
	}
	return maxVolume, maxWeight
}

// suggestConsolidations runs after a new cargo request is created:
// assembles the candidate pool, asks the Python matching service for pairs,
// and records suggestions + notifications for both clients. Best-effort —
// failures are logged by the caller, never failing the cargo submission.
func (s *CargoService) suggestConsolidations(ctx context.Context) error {
	if s.matcher == nil {
		return nil
	}

	consRepo := repository.NewConsolidationRepository(s.db)
	candidates, err := consRepo.ListOpenCargoWithoutActiveSuggestion(ctx)
	if err != nil {
		return err
	}
	if len(candidates) < 2 {
		return nil
	}

	maxVolume, maxWeight := s.getCapacityLimits(ctx)
	pairs, err := s.matcher.Match(ctx, candidates, matchingParams(maxVolume, maxWeight, s.cfg))
	if err != nil {
		return err
	}

	byID := make(map[uuid.UUID]models.CargoRequest, len(candidates))
	for _, c := range candidates {
		byID[c.ID] = c
	}

	for _, pair := range pairs {
		cargoA, okA := byID[pair.A]
		cargoB, okB := byID[pair.B]
		if !okA || !okB {
			continue // matcher returned an id outside the pool we sent
		}

		// A pair that was already suggested once (including declined —
		// "each goes alone" must stick) is never re-suggested.
		alreadySuggested, err := consRepo.ExistsSuggestionForPair(ctx, cargoA.ID, cargoB.ID)
		if err != nil {
			return err
		}
		if alreadySuggested {
			continue
		}

		suggestion := &models.ConsolidationSuggestion{
			ID:             uuid.New(),
			CargoRequestA:  cargoA.ID,
			CargoRequestB:  cargoB.ID,
			DirectionLabel: cargoA.Origin.Label + " → " + cargoA.Destination.Label,
			Status:         models.ConsolidationSuggested,
			CreatedAt:      time.Now(),
		}
		if err := consRepo.CreateSuggestion(ctx, suggestion); err != nil {
			return err
		}

		for _, cargo := range []models.CargoRequest{cargoA, cargoB} {
			if err := s.notifyConsolidation(ctx, cargo.ClientID, "consolidation_suggested", suggestion, cargo.ID); err != nil {
				log.Printf("consolidation %s: notify client %s: %v", suggestion.ID, cargo.ClientID, err)
			}
		}
	}
	return nil
}

func (s *CargoService) notifyConsolidation(ctx context.Context, clientID uuid.UUID, notifType string, suggestion *models.ConsolidationSuggestion, cargoID uuid.UUID) error {
	payload, err := json.Marshal(map[string]any{
		"suggestion_id":    suggestion.ID,
		"cargo_request_id": cargoID,
		"direction_label":  suggestion.DirectionLabel,
	})
	if err != nil {
		return err
	}
	notifRepo := repository.NewNotificationRepository(s.db)
	return notifRepo.Create(ctx, &models.Notification{
		ID:        uuid.New(),
		UserID:    clientID,
		Type:      notifType,
		Payload:   payload,
		IsRead:    false,
		CreatedAt: time.Now(),
	})
}

// ConsolidationView is the client-facing suggestion: status, direction and
// the OTHER cargo's size — deliberately nothing that identifies the other
// client. MySideAgreed lets the UI distinguish "you already agreed, waiting
// for the other client" from "your response is needed" without exposing
// which side (a/b) the caller is.
type ConsolidationView struct {
	SuggestionID    uuid.UUID                  `json:"suggestion_id"`
	Status          models.ConsolidationStatus `json:"status"`
	DirectionLabel  string                     `json:"direction_label"`
	OtherVolumeM3   float64                    `json:"other_volume_m3"`
	OtherWeightKg   float64                    `json:"other_weight_kg"`
	MySideAgreed    bool                       `json:"my_side_agreed"`
	OtherSideAgreed bool                       `json:"other_side_agreed"`
	CreatedAt       time.Time                  `json:"created_at"`
}

// GetActiveConsolidation returns the pending suggestion for the client's
// cargo request, or nil if there is none.
func (s *CargoService) GetActiveConsolidation(ctx context.Context, clientID, cargoID uuid.UUID) (*ConsolidationView, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	cargoRepo := repository.NewCargoRequestRepository(s.db)
	cargo, err := cargoRepo.GetByID(ctx, cargoID)
	if err != nil {
		return nil, err
	}
	if cargo.ClientID != clientID {
		return nil, ErrForbiddenNotOwner
	}

	consRepo := repository.NewConsolidationRepository(s.db)
	suggestion, err := consRepo.GetActiveSuggestionByCargoID(ctx, cargoID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	isSideA := suggestion.CargoRequestA == cargoID
	otherID := suggestion.CargoRequestB
	if !isSideA {
		otherID = suggestion.CargoRequestA
	}
	other, err := cargoRepo.GetByID(ctx, otherID)
	if err != nil {
		return nil, err
	}

	return &ConsolidationView{
		SuggestionID:    suggestion.ID,
		Status:          suggestion.Status,
		DirectionLabel:  suggestion.DirectionLabel,
		OtherVolumeM3:   other.VolumeM3,
		OtherWeightKg:   other.WeightKg,
		MySideAgreed:    (isSideA && suggestion.Status == models.ConsolidationAAgreed) || (!isSideA && suggestion.Status == models.ConsolidationBAgreed),
		OtherSideAgreed: (isSideA && suggestion.Status == models.ConsolidationBAgreed) || (!isSideA && suggestion.Status == models.ConsolidationAAgreed),
		CreatedAt:       suggestion.CreatedAt,
	}, nil
}

// respondConsolidation validates that the caller owns the given cargo and
// that the cargo belongs to the suggestion, then returns the suggestion and
// whether the caller is side A.
func (s *CargoService) respondConsolidation(ctx context.Context, q repository.Querier, clientID, cargoID, suggestionID uuid.UUID) (*models.ConsolidationSuggestion, bool, error) {
	cargoRepo := repository.NewCargoRequestRepository(q)
	cargo, err := cargoRepo.GetByID(ctx, cargoID)
	if err != nil {
		return nil, false, err
	}
	if cargo.ClientID != clientID {
		return nil, false, ErrForbiddenNotOwner
	}

	consRepo := repository.NewConsolidationRepository(q)
	suggestion, err := consRepo.GetSuggestionByID(ctx, suggestionID)
	if err != nil {
		return nil, false, err
	}
	if suggestion.CargoRequestA != cargoID && suggestion.CargoRequestB != cargoID {
		return nil, false, fmt.Errorf("%w: suggestion does not involve this cargo request", ErrInvalidInput)
	}
	if suggestion.Status == models.ConsolidationDeclined || suggestion.Status == models.ConsolidationBothAgreed {
		return nil, false, ErrAlreadyResponded
	}
	return suggestion, suggestion.CargoRequestA == cargoID, nil
}

// AgreeConsolidation records the client's agreement; when both sides have
// agreed it creates the consolidated request, closes the member requests
// and notifies both clients — all in one transaction.
func (s *CargoService) AgreeConsolidation(ctx context.Context, clientID, cargoID, suggestionID uuid.UUID) (*models.ConsolidationSuggestion, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	suggestion, isSideA, err := s.respondConsolidation(ctx, tx, clientID, cargoID, suggestionID)
	if err != nil {
		return nil, err
	}

	var newStatus models.ConsolidationStatus
	switch {
	case suggestion.Status == models.ConsolidationSuggested && isSideA:
		newStatus = models.ConsolidationAAgreed
	case suggestion.Status == models.ConsolidationSuggested && !isSideA:
		newStatus = models.ConsolidationBAgreed
	case suggestion.Status == models.ConsolidationAAgreed && !isSideA,
		suggestion.Status == models.ConsolidationBAgreed && isSideA:
		newStatus = models.ConsolidationBothAgreed
	default:
		// The same side agreeing twice is a no-op.
		return suggestion, tx.Commit(ctx)
	}

	consRepo := repository.NewConsolidationRepository(tx)
	if err := consRepo.UpdateSuggestionStatus(ctx, suggestionID, newStatus); err != nil {
		return nil, err
	}
	suggestion.Status = newStatus

	if newStatus == models.ConsolidationBothAgreed {
		if err := s.createConsolidatedFromSuggestion(ctx, tx, suggestion); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return suggestion, nil
}

// createConsolidatedFromSuggestion merges the two member requests: sums
// volume/weight, takes points from cargo A (both are within the matching
// radius by construction), closes the members and opens the shared
// competition.
func (s *CargoService) createConsolidatedFromSuggestion(ctx context.Context, tx repository.Querier, suggestion *models.ConsolidationSuggestion) error {
	cargoRepo := repository.NewCargoRequestRepository(tx)
	cargoA, err := cargoRepo.GetByID(ctx, suggestion.CargoRequestA)
	if err != nil {
		return err
	}
	cargoB, err := cargoRepo.GetByID(ctx, suggestion.CargoRequestB)
	if err != nil {
		return err
	}

	consolidated := &models.ConsolidatedRequest{
		ID:               uuid.New(),
		Origin:           cargoA.Origin,
		Destination:      cargoA.Destination,
		TotalVolumeM3:    cargoA.VolumeM3 + cargoB.VolumeM3,
		TotalWeightKg:    cargoA.WeightKg + cargoB.WeightKg,
		MemberRequestIDs: []uuid.UUID{cargoA.ID, cargoB.ID},
		Status:           models.CargoRequestOpen,
		CreatedAt:        time.Now(),
	}

	consRepo := repository.NewConsolidationRepository(tx)
	if err := consRepo.CreateConsolidated(ctx, consolidated); err != nil {
		return err
	}

	// Member requests leave the individual competition.
	if err := cargoRepo.UpdateStatus(ctx, cargoA.ID, models.CargoRequestClosed); err != nil {
		return err
	}
	if err := cargoRepo.UpdateStatus(ctx, cargoB.ID, models.CargoRequestClosed); err != nil {
		return err
	}

	for _, cargo := range []*models.CargoRequest{cargoA, cargoB} {
		payload, err := json.Marshal(map[string]any{
			"consolidated_request_id": consolidated.ID,
			"cargo_request_id":        cargo.ID,
			"direction_label":         suggestion.DirectionLabel,
		})
		if err != nil {
			return err
		}
		notifRepo := repository.NewNotificationRepository(tx)
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    cargo.ClientID,
			Type:      "consolidation_created",
			Payload:   payload,
			IsRead:    false,
			CreatedAt: time.Now(),
		}); err != nil {
			return err
		}
	}
	return nil
}

// DeclineConsolidation: one "no" ends the suggestion for both sides — each
// cargo request simply stays in its own competition.
func (s *CargoService) DeclineConsolidation(ctx context.Context, clientID, cargoID, suggestionID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, _, err := s.respondConsolidation(ctx, tx, clientID, cargoID, suggestionID); err != nil {
		return err
	}

	consRepo := repository.NewConsolidationRepository(tx)
	if err := consRepo.UpdateSuggestionStatus(ctx, suggestionID, models.ConsolidationDeclined); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *CargoService) ListMyConsolidated(ctx context.Context, clientID uuid.UUID) ([]models.ConsolidatedRequest, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	consRepo := repository.NewConsolidationRepository(s.db)
	return consRepo.ListConsolidatedForClient(ctx, clientID)
}

// ListAvailableConsolidated mirrors ListAvailableCargoRequests for the
// shared competitions: tool + per-country radius, both required.
func (s *CargoService) ListAvailableConsolidated(ctx context.Context, participantID uuid.UUID) ([]models.ConsolidatedRequest, error) {
	if _, err := s.requireEligibleUser(ctx, participantID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, participantID, ToolReceiveCargoByRoute); err != nil {
		return nil, err
	}
	consRepo := repository.NewConsolidationRepository(s.db)
	return consRepo.ListOpenConsolidatedMatchingUserRoutes(ctx, participantID, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
}

func (s *CargoService) CreateConsolidatedOffer(ctx context.Context, participantID, consolidatedID uuid.UUID, in CreateOfferInput) (*models.Offer, error) {
	if in.Price <= 0 {
		return nil, fmt.Errorf("%w: price must be positive", ErrInvalidInput)
	}
	if in.WarehouseFillPercent != nil && (*in.WarehouseFillPercent < 0 || *in.WarehouseFillPercent > 100) {
		return nil, fmt.Errorf("%w: warehouse_fill_percent must be between 0 and 100", ErrInvalidInput)
	}
	if _, err := s.requireEligibleUser(ctx, participantID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, participantID, ToolSubmitOffer); err != nil {
		return nil, err
	}

	consRepo := repository.NewConsolidationRepository(s.db)
	consolidated, err := consRepo.GetConsolidatedByID(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if consolidated.Status != models.CargoRequestOpen {
		return nil, ErrCargoNotOpen
	}

	offer := &models.Offer{
		ID:                    uuid.New(),
		ConsolidatedRequestID: &consolidatedID,
		ParticipantID:         participantID,
		Price:                 in.Price,
		Currency:              "KZT",
		Conditions:            in.Conditions,
		WarehouseFillPercent:  in.WarehouseFillPercent,
		Status:                models.OfferSubmitted,
		CreatedAt:             time.Now(),
	}
	offerRepo := repository.NewOfferRepository(s.db)
	if err := offerRepo.Create(ctx, offer); err != nil {
		return nil, err
	}
	return offer, nil
}

// ListConsolidatedOffersForClient: any member client sees the shared
// competition, anonymized exactly like single-cargo offers.
func (s *CargoService) ListConsolidatedOffersForClient(ctx context.Context, clientID, consolidatedID uuid.UUID) ([]AnonymizedOffer, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	consRepo := repository.NewConsolidationRepository(s.db)
	isMember, err := consRepo.IsConsolidatedMember(ctx, clientID, consolidatedID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, repository.ErrNotFound
	}
	cons, err := consRepo.GetConsolidatedByID(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}

	offerRepo := repository.NewOfferRepository(s.db)
	offers, err := offerRepo.ListByConsolidatedRequestID(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	return s.anonymizeOffers(ctx, offers, cons.Origin, cons.Destination)
}
