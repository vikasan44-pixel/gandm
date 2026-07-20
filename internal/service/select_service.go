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
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Lock the client row first: the reveal-limit check below is
	// check-then-insert, so parallel selects (even on different cargo)
	// must serialize per client or the limit can be overshot.
	userRepo := repository.NewUserRepository(tx)
	client, err := userRepo.GetByIDForUpdate(ctx, clientID)
	if err != nil {
		return nil, err
	}
	limit := s.cfg.ContactLimitFree
	if client.HasSubscription {
		limit = s.cfg.ContactLimitSubscribed
	}

	// Row lock on the cargo request: two concurrent selects on the same
	// cargo serialize here, and the loser sees status=matched below instead
	// of closing the deal a second time.
	cargoRepo := repository.NewCargoRequestRepository(tx)
	cargo, err := cargoRepo.GetByIDForUpdate(ctx, cargoRequestID)
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
	if offer.Status != models.OfferSubmitted {
		return nil, ErrOfferNotEditable
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

	if err := offerRepo.MarkSelectedForCargo(ctx, cargoRequestID, offerID); err != nil {
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

	participant, err := userRepo.GetByID(ctx, offer.ParticipantID)
	if err != nil {
		return nil, err
	}

	// Повторная сделка с тем же партнёром — обе стороны получают просьбу
	// подтвердить её документом (ТЗ §6.2).
	NotifyRepeatDeal(ctx, tx, cargoRequestID, clientID, offer.ParticipantID)

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
