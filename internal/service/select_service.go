package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

var ErrContactLimitReached = errors.New("contact reveal limit reached")

// RevealedContact is what the client gets after selecting an offer — the
// participant's identity, deliberately withheld until this moment.
type RevealedContact struct {
	CompanyName string `json:"company_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

type SelectOfferResult struct {
	Contact RevealedContact `json:"contact"`
	// ParticipantID lets the client rate the counterparty after the deal —
	// identity is already revealed at this point.
	ParticipantID uuid.UUID `json:"participant_id"`
	ChatID        uuid.UUID `json:"chat_id"`
	RevealsUsed   int       `json:"reveals_used"`
	RevealsLimit  int       `json:"reveals_limit"`
}

// SelectOffer is the retention checkpoint: the cargo owner picks an offer,
// which consumes a contact-reveal slot (lifetime limit, subscription-
// dependent), reveals the participant's contact, marks the offer selected
// and the cargo matched, and opens a chat between the two. Everything
// mutating happens in one transaction.
func (s *CargoService) SelectOffer(ctx context.Context, clientID, cargoRequestID, offerID uuid.UUID) (*SelectOfferResult, error) {
	client, err := s.requireEligibleUser(ctx, clientID)
	if err != nil {
		return nil, err
	}

	limit := s.cfg.ContactLimitFree
	if client.HasSubscription {
		limit = s.cfg.ContactLimitSubscribed
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	cargoRepo := repository.NewCargoRequestRepository(tx)
	cargo, err := cargoRepo.GetByID(ctx, cargoRequestID)
	if err != nil {
		return nil, err
	}
	if cargo.ClientID != clientID {
		return nil, ErrForbiddenNotOwner
	}
	if cargo.Status != models.CargoRequestOpen {
		return nil, ErrCargoNotOpen
	}

	offerRepo := repository.NewOfferRepository(tx)
	offer, err := offerRepo.GetByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer.CargoRequestID == nil || *offer.CargoRequestID != cargoRequestID {
		return nil, fmt.Errorf("%w: offer does not belong to this cargo request", ErrInvalidInput)
	}

	revealRepo := repository.NewContactRevealRepository(tx)
	used, err := revealRepo.CountByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if used >= limit {
		return nil, ErrContactLimitReached
	}

	now := time.Now()
	reveal := &models.ContactReveal{
		ID:             uuid.New(),
		ClientID:       clientID,
		ParticipantID:  offer.ParticipantID,
		CargoRequestID: cargoRequestID,
		IsPaid:         client.HasSubscription,
		CreatedAt:      now,
	}
	if err := revealRepo.Create(ctx, reveal); err != nil {
		return nil, err
	}

	if err := offerRepo.UpdateStatus(ctx, offerID, models.OfferSelected); err != nil {
		return nil, err
	}
	if err := cargoRepo.UpdateStatus(ctx, cargoRequestID, models.CargoRequestMatched); err != nil {
		return nil, err
	}

	chatRepo := repository.NewChatRepository(tx)
	chat := &models.Chat{ID: uuid.New(), CargoRequestID: &cargoRequestID, CreatedAt: now}
	if err := chatRepo.Create(ctx, chat); err != nil {
		return nil, err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, clientID); err != nil {
		return nil, err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, offer.ParticipantID); err != nil {
		return nil, err
	}

	userRepo := repository.NewUserRepository(tx)
	participant, err := userRepo.GetByID(ctx, offer.ParticipantID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &SelectOfferResult{
		Contact: RevealedContact{
			CompanyName: participant.CompanyName,
			Email:       participant.Email,
			Phone:       participant.Phone,
		},
		ParticipantID: participant.ID,
		ChatID:        chat.ID,
		RevealsUsed:   used + 1,
		RevealsLimit:  limit,
	}, nil
}
