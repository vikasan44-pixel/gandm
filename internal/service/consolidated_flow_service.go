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

var (
	ErrSubscriptionRequired     = errors.New("subscription required to initiate consolidation")
	ErrPaymentRequired          = errors.New("payment or subscription required to accept consolidation")
	ErrInviteWrongState         = errors.New("consolidation invite is not in the required state")
	ErrConsolidationNotAccepted = errors.New("consolidation must be accepted before selecting a carrier")
	ErrNotInvitedClient         = errors.New("only the invited client can perform this action")
)

// getConsolidatedForMember loads the consolidated request and enforces
// membership — outsiders get not-found, never confirmation of existence.
func (s *CargoService) getConsolidatedForMember(ctx context.Context, q repository.Querier, clientID, consolidatedID uuid.UUID) (*models.ConsolidatedRequest, error) {
	consRepo := repository.NewConsolidationRepository(q)
	isMember, err := consRepo.IsConsolidatedMember(ctx, clientID, consolidatedID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, repository.ErrNotFound
	}
	return consRepo.GetConsolidatedByID(ctx, consolidatedID)
}

// otherMemberClient resolves the second client of the consolidation.
func (s *CargoService) otherMemberClient(ctx context.Context, q repository.Querier, consolidatedID, clientID uuid.UUID) (uuid.UUID, error) {
	consRepo := repository.NewConsolidationRepository(q)
	clients, err := consRepo.ListMemberClients(ctx, consolidatedID)
	if err != nil {
		return uuid.Nil, err
	}
	for _, id := range clients {
		if id != clientID {
			return id, nil
		}
	}
	return uuid.Nil, fmt.Errorf("consolidation %s has no second client", consolidatedID)
}

// InviteConsolidated: the subscribed initiator opens the paid flow and the
// other client gets a "с вами хотят объединиться" notification.
func (s *CargoService) InviteConsolidated(ctx context.Context, clientID, consolidatedID uuid.UUID) error {
	client, err := s.requireEligibleUser(ctx, clientID)
	if err != nil {
		return err
	}
	if !client.HasSubscription {
		return ErrSubscriptionRequired
	}

	cons, err := s.getConsolidatedForMember(ctx, s.db, clientID, consolidatedID)
	if err != nil {
		return err
	}
	if cons.InviteStatus != models.InviteNone {
		return ErrInviteWrongState
	}

	invitedID, err := s.otherMemberClient(ctx, s.db, consolidatedID, clientID)
	if err != nil {
		return err
	}

	consRepo := repository.NewConsolidationRepository(s.db)
	if err := consRepo.SetInvite(ctx, consolidatedID, clientID, invitedID); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"consolidated_request_id": consolidatedID,
		"direction_label":         cons.Origin.Label + " → " + cons.Destination.Label,
	})
	if err != nil {
		return err
	}
	notifRepo := repository.NewNotificationRepository(s.db)
	return notifRepo.Create(ctx, &models.Notification{
		ID:        uuid.New(),
		UserID:    invitedID,
		Type:      "consolidation_invite",
		Payload:   payload,
		IsRead:    false,
		CreatedAt: time.Now(),
	})
}

// PayConsolidated: one-time sandbox charge that unlocks THIS consolidation
// for the invited client (a subscription would unlock all of them).
func (s *CargoService) PayConsolidated(ctx context.Context, clientID, consolidatedID uuid.UUID) (*models.ConsolidatedPayment, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	cons, err := s.getConsolidatedForMember(ctx, s.db, clientID, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.InviteStatus != models.InviteInvited {
		return nil, ErrInviteWrongState
	}
	if cons.InvitedClientID == nil || *cons.InvitedClientID != clientID {
		return nil, ErrNotInvitedClient
	}

	ref, err := s.payments.Charge(ctx, clientID, "consolidation:"+consolidatedID.String())
	if err != nil {
		return nil, err
	}

	paymentRow := &models.ConsolidatedPayment{
		ID:                    uuid.New(),
		ConsolidatedRequestID: consolidatedID,
		ClientID:              clientID,
		Provider:              s.payments.Name(),
		ProviderRef:           ref,
		CreatedAt:             time.Now(),
	}
	consRepo := repository.NewConsolidationRepository(s.db)
	if err := consRepo.CreatePayment(ctx, paymentRow); err != nil {
		return nil, err
	}
	return paymentRow, nil
}

// AcceptConsolidated: the invited client joins — free with a subscription,
// otherwise only after the one-time payment. Creates the shared client chat
// and mutually reveals the two clients (that's the paid value).
func (s *CargoService) AcceptConsolidated(ctx context.Context, clientID, consolidatedID uuid.UUID) (*models.Chat, error) {
	client, err := s.requireEligibleUser(ctx, clientID)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	cons, err := s.getConsolidatedForMember(ctx, tx, clientID, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.InviteStatus != models.InviteInvited {
		return nil, ErrInviteWrongState
	}
	if cons.InvitedClientID == nil || *cons.InvitedClientID != clientID {
		return nil, ErrNotInvitedClient
	}

	consRepo := repository.NewConsolidationRepository(tx)
	if !client.HasSubscription {
		paid, err := consRepo.HasPayment(ctx, consolidatedID, clientID)
		if err != nil {
			return nil, err
		}
		if !paid {
			return nil, ErrPaymentRequired
		}
	}

	now := time.Now()
	chat := &models.Chat{ID: uuid.New(), ConsolidatedRequestID: &consolidatedID, CreatedAt: now}
	chatRepo := repository.NewChatRepository(tx)
	if err := chatRepo.Create(ctx, chat); err != nil {
		return nil, err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, clientID); err != nil {
		return nil, err
	}
	if cons.InitiatorClientID != nil {
		if err := chatRepo.AddParticipant(ctx, chat.ID, *cons.InitiatorClientID); err != nil {
			return nil, err
		}
	}

	if err := consRepo.SetAccepted(ctx, consolidatedID, chat.ID); err != nil {
		return nil, err
	}

	if cons.InitiatorClientID != nil {
		payload, err := json.Marshal(map[string]any{
			"consolidated_request_id": consolidatedID,
			"chat_id":                 chat.ID,
		})
		if err != nil {
			return nil, err
		}
		notifRepo := repository.NewNotificationRepository(tx)
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    *cons.InitiatorClientID,
			Type:      "consolidation_accepted",
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
	return chat, nil
}

// ClientContact is what the two clients see about EACH OTHER after accept —
// mutual visibility is the paid value of consolidation.
type ClientContact struct {
	CompanyName string `json:"company_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

type ConsolidatedStatusView struct {
	Consolidated *models.ConsolidatedRequest `json:"consolidated"`
	AmInitiator  bool                        `json:"am_initiator"`
	AmInvited    bool                        `json:"am_invited"`
	PaymentDone  bool                        `json:"payment_done"`
	// Counterpart is nil until the invite is accepted.
	Counterpart *ClientContact `json:"counterpart,omitempty"`
	// MyOfferID/OtherHasChosen describe the joint selection without
	// revealing anything about the carrier before choices match.
	MyOfferID      *uuid.UUID `json:"my_offer_id,omitempty"`
	OtherHasChosen bool       `json:"other_has_chosen"`
	// SelectionState: none | waiting_other | mismatch | matched.
	SelectionState string `json:"selection_state"`
	// CarrierContact/CarrierID appear only when both clients picked the
	// same offer — CarrierID enables the post-deal rating form.
	CarrierContact *RevealedContact `json:"carrier_contact,omitempty"`
	CarrierID      *uuid.UUID       `json:"carrier_id,omitempty"`
}

func (s *CargoService) selectionState(mine *uuid.UUID, other *uuid.UUID) string {
	switch {
	case mine == nil && other == nil:
		return "none"
	case mine != nil && other == nil, mine == nil && other != nil:
		return "waiting_other"
	case *mine == *other:
		return "matched"
	default:
		return "mismatch"
	}
}

func (s *CargoService) GetConsolidatedStatus(ctx context.Context, clientID, consolidatedID uuid.UUID) (*ConsolidatedStatusView, error) {
	cons, err := s.getConsolidatedForMember(ctx, s.db, clientID, consolidatedID)
	if err != nil {
		return nil, err
	}

	consRepo := repository.NewConsolidationRepository(s.db)
	userRepo := repository.NewUserRepository(s.db)

	view := &ConsolidatedStatusView{
		Consolidated:   cons,
		AmInitiator:    cons.InitiatorClientID != nil && *cons.InitiatorClientID == clientID,
		AmInvited:      cons.InvitedClientID != nil && *cons.InvitedClientID == clientID,
		SelectionState: "none",
	}

	paid, err := consRepo.HasPayment(ctx, consolidatedID, clientID)
	if err != nil {
		return nil, err
	}
	view.PaymentDone = paid

	if cons.InviteStatus == models.InviteAccepted {
		otherID, err := s.otherMemberClient(ctx, s.db, consolidatedID, clientID)
		if err != nil {
			return nil, err
		}
		other, err := userRepo.GetByID(ctx, otherID)
		if err != nil {
			return nil, err
		}
		view.Counterpart = &ClientContact{CompanyName: other.CompanyName, Email: other.Email, Phone: other.Phone}
	}

	selections, err := consRepo.ListSelections(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	var mine, other *uuid.UUID
	for i := range selections {
		if selections[i].ClientID == clientID {
			mine = &selections[i].OfferID
		} else {
			other = &selections[i].OfferID
		}
	}
	view.MyOfferID = mine
	view.OtherHasChosen = other != nil
	view.SelectionState = s.selectionState(mine, other)

	// The carrier is revealed ONLY once both clients picked the same offer.
	if view.SelectionState == "matched" && mine != nil {
		offerRepo := repository.NewOfferRepository(s.db)
		offer, err := offerRepo.GetByID(ctx, *mine)
		if err != nil {
			return nil, err
		}
		carrier, err := userRepo.GetByID(ctx, offer.ParticipantID)
		if err != nil {
			return nil, err
		}
		view.CarrierContact = &RevealedContact{CompanyName: carrier.CompanyName, Email: carrier.Email, Phone: carrier.Phone}
		view.CarrierID = &carrier.ID
	}

	return view, nil
}

type ConsolidatedSelectResult struct {
	SelectionState string           `json:"selection_state"`
	CarrierContact *RevealedContact `json:"carrier_contact,omitempty"`
	CarrierID      *uuid.UUID       `json:"carrier_id,omitempty"`
}

// SelectConsolidatedOffer records the client's choice. When both clients
// converge on the same offer, the deal closes: offer selected, consolidated
// matched, carrier revealed to both and added to the shared chat.
func (s *CargoService) SelectConsolidatedOffer(ctx context.Context, clientID, consolidatedID, offerID uuid.UUID) (*ConsolidatedSelectResult, error) {
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
	// Row lock on the consolidation serializes the two clients: when both
	// select "simultaneously", the second transaction waits here and then
	// sees the status the first one committed, so the deal-closing side
	// effects below run exactly once.
	cons, err := consRepo.GetConsolidatedByIDForUpdate(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.InviteStatus != models.InviteAccepted {
		return nil, ErrConsolidationNotAccepted
	}
	if cons.Status == models.CargoRequestMatched {
		// The other client's concurrent call already closed the deal —
		// don't repeat side effects or notifications, just reveal the
		// matched carrier.
		result, err := s.consolidatedMatchedResult(ctx, tx, clientID, consolidatedID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return result, nil
	}
	if cons.Status != models.CargoRequestOpen {
		return nil, ErrCargoNotOpen
	}

	offerRepo := repository.NewOfferRepository(tx)
	offer, err := offerRepo.GetByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer.ConsolidatedRequestID == nil || *offer.ConsolidatedRequestID != consolidatedID {
		return nil, fmt.Errorf("%w: offer does not belong to this consolidated request", ErrInvalidInput)
	}

	if err := consRepo.UpsertSelection(ctx, &models.ConsolidatedSelection{
		ConsolidatedRequestID: consolidatedID,
		ClientID:              clientID,
		OfferID:               offerID,
		CreatedAt:             time.Now(),
	}); err != nil {
		return nil, err
	}

	selections, err := consRepo.ListSelections(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	var mine, other *uuid.UUID
	for i := range selections {
		if selections[i].ClientID == clientID {
			mine = &selections[i].OfferID
		} else {
			other = &selections[i].OfferID
		}
	}
	state := s.selectionState(mine, other)
	result := &ConsolidatedSelectResult{SelectionState: state}

	if state == "matched" {
		if err := offerRepo.UpdateStatus(ctx, offerID, models.OfferSelected); err != nil {
			return nil, err
		}
		if err := consRepo.UpdateConsolidatedStatus(ctx, consolidatedID, models.CargoRequestMatched); err != nil {
			return nil, err
		}

		// The carrier joins the shared chat — no separate chat needed.
		if cons.ChatID != nil {
			chatRepo := repository.NewChatRepository(tx)
			if err := chatRepo.AddParticipant(ctx, *cons.ChatID, offer.ParticipantID); err != nil {
				return nil, err
			}
		}

		userRepo := repository.NewUserRepository(tx)
		carrier, err := userRepo.GetByID(ctx, offer.ParticipantID)
		if err != nil {
			return nil, err
		}
		result.CarrierContact = &RevealedContact{CompanyName: carrier.CompanyName, Email: carrier.Email, Phone: carrier.Phone}
		result.CarrierID = &carrier.ID

		// Notify both clients and the carrier about the closed deal.
		clients, err := consRepo.ListMemberClients(ctx, consolidatedID)
		if err != nil {
			return nil, err
		}
		notifRepo := repository.NewNotificationRepository(tx)
		payload, err := json.Marshal(map[string]any{
			"consolidated_request_id": consolidatedID,
			"offer_id":                offerID,
		})
		if err != nil {
			return nil, err
		}
		for _, uid := range append(clients, offer.ParticipantID) {
			if err := notifRepo.Create(ctx, &models.Notification{
				ID:        uuid.New(),
				UserID:    uid,
				Type:      "consolidated_matched",
				Payload:   payload,
				IsRead:    false,
				CreatedAt: time.Now(),
			}); err != nil {
				return nil, err
			}
		}

		// The batch is heading out — open the customs-clearance competition
		// for tooled representatives (ТЗ §10.2).
		if err := s.notifyCustomsReps(ctx, tx, cons); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// consolidatedMatchedResult rebuilds the "matched" response for a selection
// call that arrives after the deal is already closed — read-only, so the
// race loser gets the revealed carrier without duplicate side effects.
func (s *CargoService) consolidatedMatchedResult(ctx context.Context, q repository.Querier, clientID, consolidatedID uuid.UUID) (*ConsolidatedSelectResult, error) {
	consRepo := repository.NewConsolidationRepository(q)
	selections, err := consRepo.ListSelections(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	var mine, other *uuid.UUID
	for i := range selections {
		if selections[i].ClientID == clientID {
			mine = &selections[i].OfferID
		} else {
			other = &selections[i].OfferID
		}
	}
	result := &ConsolidatedSelectResult{SelectionState: s.selectionState(mine, other)}
	if result.SelectionState != "matched" {
		return result, nil
	}

	offer, err := repository.NewOfferRepository(q).GetByID(ctx, *mine)
	if err != nil {
		return nil, err
	}
	carrier, err := repository.NewUserRepository(q).GetByID(ctx, offer.ParticipantID)
	if err != nil {
		return nil, err
	}
	result.CarrierContact = &RevealedContact{CompanyName: carrier.CompanyName, Email: carrier.Email, Phone: carrier.Phone}
	result.CarrierID = &carrier.ID
	return result, nil
}
