package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

var (
	ErrProposalNotParty     = errors.New("not a party to this proposal")
	ErrProposalWrongState   = errors.New("proposal is not in a state that allows this action")
	ErrProposalToOwnVehicle = errors.New("cannot send a proposal to your own vehicle")
)

// SendTransportProposalInput carries the cargo the client offers to a vehicle.
// Either CargoRequestID (reuse one of the client's posted requests) or the
// inline fields are used.
type SendTransportProposalInput struct {
	CargoRequestID *uuid.UUID
	Origin         models.GeoPoint
	Destination    models.GeoPoint
	CargoName      string
	VolumeM3       float64
	WeightKg       float64
	PickupDate     string
	Currency       string
	Items          []models.TransportProposalItem
}

// TransportProposalView is a proposal plus what the viewer is allowed to see:
// their role in the negotiation and, once agreed, the counterparty's contact.
type TransportProposalView struct {
	models.TransportProposal
	ViewerRole    string           `json:"viewer_role"` // "client" | "carrier"
	Counterpart   *RevealedContact `json:"counterpart,omitempty"`
	CounterpartID *uuid.UUID       `json:"counterpart_id,omitempty"`
}

// SendTransportProposal starts a negotiation: the client sends cargo details to
// a specific vehicle's carrier, who will name a price.
func (s *CargoService) SendTransportProposal(ctx context.Context, clientID, vehicleID uuid.UUID, in SendTransportProposalInput) (*models.TransportProposal, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}

	vehicle, err := repository.NewVehicleRepository(s.db).GetByID(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	if vehicle.UserID == clientID {
		return nil, ErrProposalToOwnVehicle
	}
	if err := s.requireActiveUser(ctx, vehicle.UserID); err != nil {
		// Do not expose suspended/rejected carriers through a direct vehicle id.
		return nil, repository.ErrNotFound
	}

	// The client picks the currency the whole negotiation is conducted in; the
	// carrier's quote and every counter-move stay in it (no FX conversion).
	currency, err := s.resolveCurrency(in.Currency)
	if err != nil {
		return nil, err
	}

	p := &models.TransportProposal{
		ID:         uuid.New(),
		ClientID:   clientID,
		VehicleID:  vehicleID,
		CarrierID:  vehicle.UserID,
		CargoName:  strings.TrimSpace(in.CargoName),
		PickupDate: strings.TrimSpace(in.PickupDate),
		Status:     models.ProposalSent,
		Currency:   currency,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if in.CargoRequestID != nil {
		// Reuse one of the client's own posted requests: copy its route and load.
		cargo, err := repository.NewCargoRequestRepository(s.db).GetByID(ctx, *in.CargoRequestID)
		if err != nil {
			return nil, err
		}
		if cargo.ClientID != clientID {
			return nil, ErrForbiddenNotOwner
		}
		if cargo.Status != models.CargoRequestOpen {
			return nil, ErrCargoNotOpen
		}
		p.CargoRequestID = in.CargoRequestID
		p.Origin = cargo.Origin
		p.Destination = cargo.Destination
		p.VolumeM3 = cargo.VolumeM3
		p.WeightKg = cargo.WeightKg
		if p.CargoName == "" {
			p.CargoName = string(cargo.Category)
		}
		for _, cargoItem := range cargo.Items {
			p.Items = append(p.Items, models.TransportProposalItem{
				ID: uuid.New(), Position: cargoItem.Position,
				LengthM: cargoItem.LengthM, WidthM: cargoItem.WidthM, HeightM: cargoItem.HeightM,
			})
		}
	} else {
		origin, err := validateGeoPoint("origin", in.Origin)
		if err != nil {
			return nil, err
		}
		destination, err := validateGeoPoint("destination", in.Destination)
		if err != nil {
			return nil, err
		}
		if in.VolumeM3 <= 0 || in.WeightKg <= 0 {
			return nil, fmt.Errorf("%w: volume and weight must be positive", ErrInvalidInput)
		}
		p.Origin = origin
		p.Destination = destination
		p.VolumeM3 = in.VolumeM3
		p.WeightKg = in.WeightKg
	}

	if in.CargoRequestID == nil {
		if len(in.Items) > 500 {
			return nil, fmt.Errorf("%w: no more than 500 cargo items are allowed", ErrInvalidInput)
		}
		for i := range in.Items {
			it := in.Items[i]
			if it.LengthM <= 0 || it.WidthM <= 0 || it.HeightM <= 0 {
				return nil, fmt.Errorf("%w: item dimensions must be positive", ErrInvalidInput)
			}
			it.ID = uuid.New()
			it.Position = i
			p.Items = append(p.Items, it)
		}
	}
	p.PlacesCount = len(p.Items)
	if p.VolumeM3 > vehicle.CapacityM3 || p.WeightKg > vehicle.CapacityKg {
		return nil, fmt.Errorf("%w: cargo exceeds vehicle capacity", ErrInvalidInput)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := repository.NewTransportProposalRepository(tx).Create(ctx, p); err != nil {
		return nil, err
	}
	if err := s.notifyProposal(ctx, tx, p.CarrierID, p, "transport_proposal_received"); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *CargoService) notifyProposal(ctx context.Context, q repository.Querier, userID uuid.UUID, p *models.TransportProposal, notifType string) error {
	payload, err := json.Marshal(map[string]any{
		"transport_proposal_id": p.ID,
		"direction_label":       p.Origin.Label + " → " + p.Destination.Label,
		"status":                p.Status,
	})
	if err != nil {
		return err
	}
	return repository.NewNotificationRepository(q).Create(ctx, &models.Notification{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      notifType,
		Payload:   payload,
		IsRead:    false,
		CreatedAt: time.Now(),
	})
}

// ListMyTransportProposals — proposals the client sent (client's view).
func (s *CargoService) ListMyTransportProposals(ctx context.Context, clientID uuid.UUID) ([]TransportProposalView, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	list, err := repository.NewTransportProposalRepository(s.db).ListByClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return s.proposalViews(ctx, list, models.PricePartyClient)
}

// ListIncomingTransportProposals — proposals a carrier received (carrier's view).
func (s *CargoService) ListIncomingTransportProposals(ctx context.Context, carrierID uuid.UUID) ([]TransportProposalView, error) {
	if _, err := s.requireEligibleUser(ctx, carrierID); err != nil {
		return nil, err
	}
	list, err := repository.NewTransportProposalRepository(s.db).ListByCarrier(ctx, carrierID)
	if err != nil {
		return nil, err
	}
	return s.proposalViews(ctx, list, models.PricePartyCarrier)
}

// proposalViews attaches the viewer's role and, for agreed deals, the
// counterparty's contact (revealed only once both sides agreed).
func (s *CargoService) proposalViews(ctx context.Context, list []models.TransportProposal, viewerRole string) ([]TransportProposalView, error) {
	userRepo := repository.NewUserRepository(s.db)
	views := make([]TransportProposalView, 0, len(list))
	for i := range list {
		p := list[i]
		v := TransportProposalView{TransportProposal: p, ViewerRole: viewerRole}
		if p.Status == models.ProposalAgreed {
			counterpartID := p.CarrierID
			if viewerRole == models.PricePartyCarrier {
				counterpartID = p.ClientID
			}
			other, err := userRepo.GetByID(ctx, counterpartID)
			if err != nil {
				return nil, err
			}
			v.Counterpart = &RevealedContact{CompanyName: other.CompanyName, Email: other.Email, Phone: other.Phone}
			v.CounterpartID = &counterpartID
		}
		views = append(views, v)
	}
	return views, nil
}

// carrierPriceMove is the shared body for the carrier's price moves (quote and
// final), differing only in the allowed source state and the resulting state.
func (s *CargoService) carrierPriceMove(ctx context.Context, carrierID, proposalID uuid.UUID, price float64, from, to models.TransportProposalStatus, notifyType string) (*models.TransportProposal, error) {
	if price <= 0 {
		return nil, fmt.Errorf("%w: price must be positive", ErrInvalidInput)
	}
	return s.proposalMove(ctx, proposalID, func(p *models.TransportProposal, rtRepo *repository.TransportProposalRepository, q repository.Querier) (uuid.UUID, error) {
		if p.CarrierID != carrierID {
			return uuid.Nil, ErrProposalNotParty
		}
		if p.Status != from {
			return uuid.Nil, ErrProposalWrongState
		}
		if err := rtRepo.UpdateNegotiation(ctx, p.ID, to, price, models.PricePartyCarrier); err != nil {
			return uuid.Nil, err
		}
		p.Status, p.CurrentPrice, p.LastPriceBy = to, &price, models.PricePartyCarrier
		return p.ClientID, nil
	}, notifyType)
}

// QuoteTransportProposal — carrier names the first price (sent → carrier_quoted).
func (s *CargoService) QuoteTransportProposal(ctx context.Context, carrierID, proposalID uuid.UUID, price float64) (*models.TransportProposal, error) {
	return s.carrierPriceMove(ctx, carrierID, proposalID, price, models.ProposalSent, models.ProposalCarrierQuoted, "transport_proposal_quoted")
}

// FinalTransportProposal — carrier's final price (client_countered → carrier_final).
func (s *CargoService) FinalTransportProposal(ctx context.Context, carrierID, proposalID uuid.UUID, price float64) (*models.TransportProposal, error) {
	return s.carrierPriceMove(ctx, carrierID, proposalID, price, models.ProposalClientCountered, models.ProposalCarrierFinal, "transport_proposal_final")
}

// CounterTransportProposal — client proposes their own price (carrier_quoted → client_countered).
func (s *CargoService) CounterTransportProposal(ctx context.Context, clientID, proposalID uuid.UUID, price float64) (*models.TransportProposal, error) {
	if price <= 0 {
		return nil, fmt.Errorf("%w: price must be positive", ErrInvalidInput)
	}
	return s.proposalMove(ctx, proposalID, func(p *models.TransportProposal, rtRepo *repository.TransportProposalRepository, q repository.Querier) (uuid.UUID, error) {
		if p.ClientID != clientID {
			return uuid.Nil, ErrProposalNotParty
		}
		if p.Status != models.ProposalCarrierQuoted {
			return uuid.Nil, ErrProposalWrongState
		}
		if err := rtRepo.UpdateNegotiation(ctx, p.ID, models.ProposalClientCountered, price, models.PricePartyClient); err != nil {
			return uuid.Nil, err
		}
		p.Status, p.CurrentPrice, p.LastPriceBy = models.ProposalClientCountered, &price, models.PricePartyClient
		return p.CarrierID, nil
	}, "transport_proposal_countered")
}

// RejectTransportProposal — either party ends the negotiation without a deal.
func (s *CargoService) RejectTransportProposal(ctx context.Context, actorID, proposalID uuid.UUID) (*models.TransportProposal, error) {
	return s.proposalMove(ctx, proposalID, func(p *models.TransportProposal, rtRepo *repository.TransportProposalRepository, q repository.Querier) (uuid.UUID, error) {
		if p.ClientID != actorID && p.CarrierID != actorID {
			return uuid.Nil, ErrProposalNotParty
		}
		if p.Status == models.ProposalAgreed || p.Status == models.ProposalRejected {
			return uuid.Nil, ErrProposalWrongState
		}
		if err := rtRepo.SetStatus(ctx, p.ID, models.ProposalRejected); err != nil {
			return uuid.Nil, err
		}
		p.Status = models.ProposalRejected
		// Notify the other party.
		if actorID == p.ClientID {
			return p.CarrierID, nil
		}
		return p.ClientID, nil
	}, "transport_proposal_rejected")
}

// AcceptTransportProposal accepts the current price. Whoever's turn it is
// accepts: the client accepts a carrier_quoted/carrier_final price, the carrier
// accepts a client_countered price. On agreement contacts are revealed and a
// chat opens between the two.
func (s *CargoService) AcceptTransportProposal(ctx context.Context, actorID, proposalID uuid.UUID) (*TransportProposalView, error) {
	if _, err := s.requireEligibleUser(ctx, actorID); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rtRepo := repository.NewTransportProposalRepository(tx)
	p, err := rtRepo.GetByIDForUpdate(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if p.ClientID != actorID && p.CarrierID != actorID {
		return nil, ErrProposalNotParty
	}

	// Validate it's the actor's turn to accept the standing price.
	switch p.Status {
	case models.ProposalCarrierQuoted, models.ProposalCarrierFinal:
		if actorID != p.ClientID {
			return nil, ErrProposalWrongState
		}
	case models.ProposalClientCountered:
		if actorID != p.CarrierID {
			return nil, ErrProposalWrongState
		}
	default:
		return nil, ErrProposalWrongState
	}

	chatRepo := repository.NewChatRepository(tx)
	chat := &models.Chat{ID: uuid.New(), TransportProposalID: &p.ID, CreatedAt: time.Now()}
	if err := chatRepo.Create(ctx, chat); err != nil {
		return nil, err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, p.ClientID); err != nil {
		return nil, err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, p.CarrierID); err != nil {
		return nil, err
	}
	if err := rtRepo.SetAgreed(ctx, p.ID, chat.ID); err != nil {
		return nil, err
	}
	p.Status = models.ProposalAgreed
	p.ChatID = &chat.ID

	// Notify the counterparty that the deal is on.
	other := p.CarrierID
	if actorID == p.CarrierID {
		other = p.ClientID
	}
	if err := s.notifyProposal(ctx, tx, other, p, "transport_proposal_agreed"); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	viewerRole := models.PricePartyClient
	if actorID == p.CarrierID {
		viewerRole = models.PricePartyCarrier
	}
	views, err := s.proposalViews(ctx, []models.TransportProposal{*p}, viewerRole)
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// proposalMove wraps the lock-read-mutate-notify pattern shared by the
// non-accept negotiation moves. The mutate callback returns the user id to
// notify.
func (s *CargoService) proposalMove(
	ctx context.Context,
	proposalID uuid.UUID,
	mutate func(p *models.TransportProposal, rtRepo *repository.TransportProposalRepository, q repository.Querier) (uuid.UUID, error),
	notifyType string,
) (*models.TransportProposal, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rtRepo := repository.NewTransportProposalRepository(tx)
	p, err := rtRepo.GetByIDForUpdate(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	notifyUser, err := mutate(p, rtRepo, tx)
	if err != nil {
		return nil, err
	}
	if err := s.notifyProposal(ctx, tx, notifyUser, p, notifyType); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p, nil
}
