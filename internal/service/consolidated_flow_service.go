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

// InviteConsolidated: участник с подпиской открывает платный флоу — ВСЕ
// остальные клиенты группы получают приглашение (ТЗ §4.2 «два клиента и
// более»). Инициатор считается принявшим сразу.
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

	consRepo := repository.NewConsolidationRepository(s.db)
	if err := consRepo.SetInvite(ctx, consolidatedID, clientID); err != nil {
		return err
	}
	if err := consRepo.AddAcceptance(ctx, consolidatedID, clientID); err != nil {
		return err
	}

	clients, err := consRepo.ListMemberClients(ctx, consolidatedID)
	if err != nil {
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
	for _, memberID := range clients {
		if memberID == clientID {
			continue
		}
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    memberID,
			Type:      "consolidation_invite",
			Payload:   payload,
			IsRead:    false,
			CreatedAt: time.Now(),
		}); err != nil {
			return err
		}
	}
	return nil
}

// PayConsolidated: one-time sandbox charge that unlocks THIS consolidation
// for an invited member (a subscription would unlock all of them).
func (s *CargoService) PayConsolidated(ctx context.Context, clientID, consolidatedID uuid.UUID) (*models.ConsolidatedPayment, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	cons, err := s.getConsolidatedForMember(ctx, s.db, clientID, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.InviteStatus == models.InviteNone {
		return nil, ErrInviteWrongState
	}
	if cons.InitiatorClientID != nil && *cons.InitiatorClientID == clientID {
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

// AcceptConsolidated: приглашённый участник входит в группу — бесплатно по
// подписке, иначе после разовой оплаты. Первый принявший вместе с
// инициатором открывает общий чат; следующие подключаются к нему. Контакты
// раскрываются между всеми принявшими (это платная ценность).
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

	consRepo := repository.NewConsolidationRepository(tx)
	isMember, err := consRepo.IsConsolidatedMember(ctx, clientID, consolidatedID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, repository.ErrNotFound
	}
	// Row lock: параллельные принятия сериализуются — чат создаётся один.
	cons, err := consRepo.GetConsolidatedByIDForUpdate(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.InviteStatus == models.InviteNone {
		return nil, ErrInviteWrongState
	}
	if cons.InitiatorClientID != nil && *cons.InitiatorClientID == clientID {
		return nil, ErrNotInvitedClient
	}
	accepted, err := consRepo.HasAcceptance(ctx, consolidatedID, clientID)
	if err != nil {
		return nil, err
	}
	if accepted {
		return nil, ErrInviteWrongState
	}

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
	if err := consRepo.AddAcceptance(ctx, consolidatedID, clientID); err != nil {
		return nil, err
	}

	chatRepo := repository.NewChatRepository(tx)
	var chat *models.Chat
	if cons.ChatID == nil {
		chat = &models.Chat{ID: uuid.New(), ConsolidatedRequestID: &consolidatedID, CreatedAt: now}
		if err := chatRepo.Create(ctx, chat); err != nil {
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
	} else {
		chat = &models.Chat{ID: *cons.ChatID, ConsolidatedRequestID: &consolidatedID, CreatedAt: now}
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, clientID); err != nil {
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

// ClientContact is what accepted group members see about EACH OTHER —
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
	// Группа: сколько клиентов всего и сколько приняло объединение.
	MembersCount  int `json:"members_count"`
	AcceptedCount int `json:"accepted_count"`
	// Counterpart — первый из принявших (совместимость с парным UI);
	// Counterparts — все принявшие, кроме зрителя.
	Counterpart  *ClientContact  `json:"counterpart,omitempty"`
	Counterparts []ClientContact `json:"counterparts,omitempty"`
	// MyOfferID/OtherHasChosen describe the joint selection without
	// revealing anything about the carrier before the majority converges.
	MyOfferID      *uuid.UUID `json:"my_offer_id,omitempty"`
	OtherHasChosen bool       `json:"other_has_chosen"`
	// SelectionState: none | waiting_other | mismatch | matched.
	SelectionState string `json:"selection_state"`
	// CarrierContact/CarrierID appear only when большинство принявших
	// сошлось на одном оффере — CarrierID enables the post-deal rating.
	CarrierContact *RevealedContact `json:"carrier_contact,omitempty"`
	CarrierID      *uuid.UUID       `json:"carrier_id,omitempty"`
}

// groupSelectionState — выбор перевозчика группой: сделка закрывается,
// когда БОЛЬШИНСТВО принявших участников сошлось на одном оффере (для
// пары это в точности «оба выбрали одно»). Возвращает состояние и оффер
// большинства, если он есть.
func groupSelectionState(selections []models.ConsolidatedSelection, acceptedIDs []uuid.UUID) (string, *uuid.UUID) {
	acceptedSet := make(map[uuid.UUID]bool, len(acceptedIDs))
	for _, id := range acceptedIDs {
		acceptedSet[id] = true
	}

	votes := make(map[uuid.UUID]int)
	voters := 0
	for _, sel := range selections {
		if !acceptedSet[sel.ClientID] {
			continue // выбор не принявшего участника не считается
		}
		votes[sel.OfferID]++
		voters++
	}
	if voters == 0 {
		return "none", nil
	}

	for offerID, count := range votes {
		if count*2 > len(acceptedIDs) {
			winner := offerID
			return "matched", &winner
		}
	}
	if voters == len(acceptedIDs) {
		return "mismatch", nil
	}
	return "waiting_other", nil
}

func (s *CargoService) GetConsolidatedStatus(ctx context.Context, clientID, consolidatedID uuid.UUID) (*ConsolidatedStatusView, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	cons, err := s.getConsolidatedForMember(ctx, s.db, clientID, consolidatedID)
	if err != nil {
		return nil, err
	}

	consRepo := repository.NewConsolidationRepository(s.db)
	userRepo := repository.NewUserRepository(s.db)

	memberClients, err := consRepo.ListMemberClients(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	acceptedIDs, err := consRepo.ListAcceptances(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}

	view := &ConsolidatedStatusView{
		Consolidated:   cons,
		AmInitiator:    cons.InitiatorClientID != nil && *cons.InitiatorClientID == clientID,
		AmInvited:      cons.InviteStatus != models.InviteNone && (cons.InitiatorClientID == nil || *cons.InitiatorClientID != clientID),
		MembersCount:   len(memberClients),
		AcceptedCount:  len(acceptedIDs),
		SelectionState: "none",
	}

	paid, err := consRepo.HasPayment(ctx, consolidatedID, clientID)
	if err != nil {
		return nil, err
	}
	view.PaymentDone = paid

	// Контакты раскрываются между принявшими участниками.
	callerAccepted := false
	for _, id := range acceptedIDs {
		if id == clientID {
			callerAccepted = true
			break
		}
	}
	if callerAccepted {
		for _, id := range acceptedIDs {
			if id == clientID {
				continue
			}
			other, err := userRepo.GetByID(ctx, id)
			if err != nil {
				return nil, err
			}
			view.Counterparts = append(view.Counterparts, ClientContact{
				CompanyName: other.CompanyName, Email: other.Email, Phone: other.Phone,
			})
		}
		if len(view.Counterparts) > 0 {
			view.Counterpart = &view.Counterparts[0]
		}
	}

	selections, err := consRepo.ListSelections(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	for i := range selections {
		if selections[i].ClientID == clientID {
			view.MyOfferID = &selections[i].OfferID
		} else {
			view.OtherHasChosen = true
		}
	}
	state, majorityOffer := groupSelectionState(selections, acceptedIDs)
	view.SelectionState = state

	// The carrier is revealed ONLY once the majority converged.
	if state == "matched" && majorityOffer != nil {
		offerRepo := repository.NewOfferRepository(s.db)
		offer, err := offerRepo.GetByID(ctx, *majorityOffer)
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

// SelectConsolidatedOffer records the member's choice. Когда большинство
// принявших сошлось на одном оффере, the deal closes: offer selected,
// consolidated matched, carrier revealed to the group and added to the
// shared chat.
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
	// Row lock on the consolidation serializes the group: параллельные
	// выборы видят состояние друг друга, побочные эффекты закрытия сделки
	// выполняются ровно один раз.
	cons, err := consRepo.GetConsolidatedByIDForUpdate(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.InviteStatus != models.InviteAccepted {
		return nil, ErrConsolidationNotAccepted
	}
	if cons.Status == models.CargoRequestMatched {
		// The deal is already closed by a concurrent call — don't repeat
		// side effects, just reveal the matched carrier.
		result, err := s.consolidatedMatchedResult(ctx, tx, consolidatedID)
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

	// Выбирать могут только принявшие объединение участники.
	callerAccepted, err := consRepo.HasAcceptance(ctx, consolidatedID, clientID)
	if err != nil {
		return nil, err
	}
	if !callerAccepted {
		return nil, ErrConsolidationNotAccepted
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
	acceptedIDs, err := consRepo.ListAcceptances(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	state, majorityOffer := groupSelectionState(selections, acceptedIDs)
	result := &ConsolidatedSelectResult{SelectionState: state}

	if state == "matched" && majorityOffer != nil {
		winner, err := offerRepo.GetByID(ctx, *majorityOffer)
		if err != nil {
			return nil, err
		}
		if err := offerRepo.UpdateStatus(ctx, winner.ID, models.OfferSelected); err != nil {
			return nil, err
		}
		if err := consRepo.UpdateConsolidatedStatus(ctx, consolidatedID, models.CargoRequestMatched); err != nil {
			return nil, err
		}

		// The carrier joins the shared chat — no separate chat needed.
		if cons.ChatID != nil {
			chatRepo := repository.NewChatRepository(tx)
			if err := chatRepo.AddParticipant(ctx, *cons.ChatID, winner.ParticipantID); err != nil {
				return nil, err
			}
		}

		userRepo := repository.NewUserRepository(tx)
		carrier, err := userRepo.GetByID(ctx, winner.ParticipantID)
		if err != nil {
			return nil, err
		}
		result.CarrierContact = &RevealedContact{CompanyName: carrier.CompanyName, Email: carrier.Email, Phone: carrier.Phone}
		result.CarrierID = &carrier.ID

		// Notify the group and the carrier about the closed deal.
		clients, err := consRepo.ListMemberClients(ctx, consolidatedID)
		if err != nil {
			return nil, err
		}
		notifRepo := repository.NewNotificationRepository(tx)
		payload, err := json.Marshal(map[string]any{
			"consolidated_request_id": consolidatedID,
			"offer_id":                winner.ID,
		})
		if err != nil {
			return nil, err
		}
		for _, uid := range append(clients, winner.ParticipantID) {
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
func (s *CargoService) consolidatedMatchedResult(ctx context.Context, q repository.Querier, consolidatedID uuid.UUID) (*ConsolidatedSelectResult, error) {
	consRepo := repository.NewConsolidationRepository(q)
	selections, err := consRepo.ListSelections(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	acceptedIDs, err := consRepo.ListAcceptances(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	state, majorityOffer := groupSelectionState(selections, acceptedIDs)
	result := &ConsolidatedSelectResult{SelectionState: state}
	if state != "matched" || majorityOffer == nil {
		return result, nil
	}

	offer, err := repository.NewOfferRepository(q).GetByID(ctx, *majorityOffer)
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
