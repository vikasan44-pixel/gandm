package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"gandm/internal/matching"
	"gandm/internal/models"
	"gandm/internal/payment"
	"gandm/internal/repository"
)

// Tool keys gating the two participant-facing cargo actions. Submitting a
// cargo request itself is not tool-gated — any eligible (pending/active)
// user can do that for free, per the brief.
const (
	ToolReceiveCargoByRoute = "receive_cargo_by_route"
	ToolSubmitOffer         = "submit_offer"
)

var (
	ErrForbiddenTool     = errors.New("missing required tool")
	ErrForbiddenNotOwner = errors.New("not the owner of this resource")
	ErrCargoNotOpen      = errors.New("cargo request is not open")
)

// CargoServiceConfig carries the env-tunable knobs so the constructor
// doesn't grow a positional parameter per stage.
type CargoServiceConfig struct {
	// Per-country matching radii (MATCH_RADIUS_CN_KM / MATCH_RADIUS_KZ_KM):
	// the threshold for comparing two points is picked by each point's
	// country — cn gets the wider radius, kz and everything else (including
	// unknown) the narrower one.
	MatchRadiusCNKm float64
	MatchRadiusKZKm float64
	// Lifetime contact-reveal limits per client, without/with subscription.
	ContactLimitFree       int
	ContactLimitSubscribed int
}

type CargoService struct {
	db       *pgxpool.Pool
	cfg      CargoServiceConfig
	matcher  *matching.Client
	payments payment.Provider
}

func NewCargoService(db *pgxpool.Pool, cfg CargoServiceConfig, matcher *matching.Client, payments payment.Provider) *CargoService {
	return &CargoService{db: db, cfg: cfg, matcher: matcher, payments: payments}
}

func matchingParams(maxVolume, maxWeight float64, cfg CargoServiceConfig) matching.MatchParams {
	return matching.MatchParams{
		MaxVolumeM3: maxVolume,
		MaxWeightKg: maxWeight,
		CNRadiusKm:  cfg.MatchRadiusCNKm,
		KZRadiusKm:  cfg.MatchRadiusKZKm,
	}
}

func (s *CargoService) requireEligibleUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	userRepo := repository.NewUserRepository(s.db)
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !isEligibleStatus(user.Status) {
		return nil, ErrAccountNotEligible
	}
	return user, nil
}

func (s *CargoService) requireTool(ctx context.Context, userID uuid.UUID, toolKey string) error {
	toolRepo := repository.NewToolRepository(s.db)
	has, err := toolRepo.UserHasTool(ctx, userID, toolKey)
	if err != nil {
		return err
	}
	if !has {
		return ErrForbiddenTool
	}
	return nil
}

type CreateCargoRequestInput struct {
	Origin      models.GeoPoint
	Destination models.GeoPoint
	VolumeM3    float64
	WeightKg    float64
	Description string
}

// CreateCargoRequest is the free client action — no tool required, only an
// eligible (pending/active) account. participant_type is deliberately not
// checked (Block A principle): whoever calls this, if eligible, can submit.
func (s *CargoService) CreateCargoRequest(ctx context.Context, clientID uuid.UUID, in CreateCargoRequestInput) (*models.CargoRequest, error) {
	origin, err := validateGeoPoint("origin", in.Origin)
	if err != nil {
		return nil, err
	}
	destination, err := validateGeoPoint("destination", in.Destination)
	if err != nil {
		return nil, err
	}
	if in.VolumeM3 <= 0 {
		return nil, fmt.Errorf("%w: volume_m3 must be positive", ErrInvalidInput)
	}
	if in.WeightKg <= 0 {
		return nil, fmt.Errorf("%w: weight_kg must be positive", ErrInvalidInput)
	}

	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}

	cargo := &models.CargoRequest{
		ID:          uuid.New(),
		ClientID:    clientID,
		Origin:      origin,
		Destination: destination,
		VolumeM3:    in.VolumeM3,
		WeightKg:    in.WeightKg,
		Description: in.Description,
		Status:      models.CargoRequestOpen,
		CreatedAt:   time.Now(),
	}

	cargoRepo := repository.NewCargoRequestRepository(s.db)
	if err := cargoRepo.Create(ctx, cargo); err != nil {
		return nil, err
	}

	// Notifications and consolidation matching are secondary side-effects
	// of a successful submission — failures here (e.g. the Python matcher
	// being down) must not fail the cargo request itself.
	if err := s.notifyMatchingParticipants(ctx, cargo); err != nil {
		log.Printf("cargo %s: notify matching participants: %v", cargo.ID, err)
	}
	if err := s.suggestConsolidations(ctx); err != nil {
		log.Printf("cargo %s: suggest consolidations: %v", cargo.ID, err)
	}

	return cargo, nil
}

// notifyMatchingParticipants fans out a new-cargo notification to every
// active participant who BOTH holds the receive_cargo_by_route tool AND has
// a route whose endpoints each fall within the per-country radius (cn/kz)
// of the request's endpoints (haversine). No routes → no notifications, by
// design.
func (s *CargoService) notifyMatchingParticipants(ctx context.Context, cargo *models.CargoRequest) error {
	toolRepo := repository.NewToolRepository(s.db)
	userIDs, err := toolRepo.ListUserIDsWithToolAndRoute(ctx, ToolReceiveCargoByRoute, cargo.Origin, cargo.Destination, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
	if err != nil {
		return err
	}
	if len(userIDs) == 0 {
		return nil
	}

	payload, err := json.Marshal(map[string]any{
		"cargo_request_id":  cargo.ID,
		"origin_label":      cargo.Origin.Label,
		"destination_label": cargo.Destination.Label,
	})
	if err != nil {
		return err
	}

	notifRepo := repository.NewNotificationRepository(s.db)
	for _, userID := range userIDs {
		n := &models.Notification{
			ID:        uuid.New(),
			UserID:    userID,
			Type:      "cargo_request_available",
			Payload:   payload,
			IsRead:    false,
			CreatedAt: time.Now(),
		}
		if err := notifRepo.Create(ctx, n); err != nil {
			return err
		}
	}
	return nil
}

func (s *CargoService) ListMyCargoRequests(ctx context.Context, clientID uuid.UUID) ([]models.CargoRequest, error) {
	cargoRepo := repository.NewCargoRequestRepository(s.db)
	return cargoRepo.ListByClientID(ctx, clientID)
}

// ListAvailableCargoRequests enforces both gates from the brief: the
// receive_cargo_by_route tool AND at least one route within the per-country
// radius of both cargo endpoints (radius matching happens in SQL). A tooled
// participant with no routes correctly sees an empty list.
func (s *CargoService) ListAvailableCargoRequests(ctx context.Context, participantID uuid.UUID) ([]models.CargoRequest, error) {
	if _, err := s.requireEligibleUser(ctx, participantID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, participantID, ToolReceiveCargoByRoute); err != nil {
		return nil, err
	}

	cargoRepo := repository.NewCargoRequestRepository(s.db)
	return cargoRepo.ListOpenMatchingUserRoutes(ctx, participantID, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
}

// ListMyNotifications exists primarily so notification delivery is
// observable (smoke test, future UI) — Stage 2 wrote notifications with no
// read path at all.
func (s *CargoService) ListMyNotifications(ctx context.Context, userID uuid.UUID) ([]models.Notification, error) {
	notifRepo := repository.NewNotificationRepository(s.db)
	return notifRepo.ListByUserID(ctx, userID)
}

// CountMyUnreadNotifications backs the badge poller.
func (s *CargoService) CountMyUnreadNotifications(ctx context.Context, userID uuid.UUID) (int, error) {
	notifRepo := repository.NewNotificationRepository(s.db)
	return notifRepo.CountUnreadByUserID(ctx, userID)
}

// MarkMyNotificationsRead flags all of the caller's notifications as read.
// Own-data operation — no status/tool gate, same as reading them.
func (s *CargoService) MarkMyNotificationsRead(ctx context.Context, userID uuid.UUID) error {
	notifRepo := repository.NewNotificationRepository(s.db)
	return notifRepo.MarkAllReadByUserID(ctx, userID)
}

type CreateOfferInput struct {
	Price                float64
	Conditions           string
	WarehouseFillPercent *float64
}

func (s *CargoService) CreateOffer(ctx context.Context, participantID, cargoRequestID uuid.UUID, in CreateOfferInput) (*models.Offer, error) {
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

	cargoRepo := repository.NewCargoRequestRepository(s.db)
	cargo, err := cargoRepo.GetByID(ctx, cargoRequestID)
	if err != nil {
		return nil, err
	}
	if cargo.Status != models.CargoRequestOpen {
		return nil, ErrCargoNotOpen
	}

	offer := &models.Offer{
		ID:                   uuid.New(),
		CargoRequestID:       &cargoRequestID,
		ParticipantID:        participantID,
		Price:                in.Price,
		Currency:             "KZT",
		Conditions:           in.Conditions,
		WarehouseFillPercent: in.WarehouseFillPercent,
		Status:               models.OfferSubmitted,
		CreatedAt:            time.Now(),
	}

	offerRepo := repository.NewOfferRepository(s.db)
	if err := offerRepo.Create(ctx, offer); err != nil {
		return nil, err
	}
	return offer, nil
}

// AnonymizedOffer is what the client sees: enough to compare and decide,
// nothing that identifies who made the offer. OfferID is the offer's own
// uuid — needed to select it — and reveals nothing about the participant.
// Rating is the participant's real average (nil = no ratings yet, shown as
// "—", never a fake 0). LatestFill* mirror the participant's newest
// warehouse fill report as bare numbers — joined server-side so the client
// never learns whose report it is.
type AnonymizedOffer struct {
	OfferID            uuid.UUID          `json:"offer_id"`
	OfferNumber        int                `json:"offer_number"`
	Rating             *float64           `json:"rating"`
	RatingCount        int                `json:"rating_count"`
	FillPercent        *float64           `json:"fill_percent,omitempty"`
	LatestFillExpected *float64           `json:"latest_fill_expected,omitempty"`
	LatestFillActual   *float64           `json:"latest_fill_actual,omitempty"`
	// Dispatch* mirror the participant's «порог отправки» on their route
	// matching this cargo direction (ТЗ §5.2: клиент видит, сколько кубов
	// набрано и сколько осталось до отправки) — bare numbers, no identity.
	DispatchThresholdM3 *float64 `json:"dispatch_threshold_m3,omitempty"`
	DispatchAccruedM3   *float64 `json:"dispatch_accrued_m3,omitempty"`

	Price    float64            `json:"price"`
	Currency string             `json:"currency"`
	Status   models.OfferStatus `json:"status"`
}

func (s *CargoService) ListOffersForClient(ctx context.Context, clientID, cargoRequestID uuid.UUID) ([]AnonymizedOffer, error) {
	cargoRepo := repository.NewCargoRequestRepository(s.db)
	cargo, err := cargoRepo.GetByID(ctx, cargoRequestID)
	if err != nil {
		return nil, err
	}
	if cargo.ClientID != clientID {
		return nil, ErrForbiddenNotOwner
	}

	offerRepo := repository.NewOfferRepository(s.db)
	offers, err := offerRepo.ListByCargoRequestID(ctx, cargoRequestID)
	if err != nil {
		return nil, err
	}
	return s.anonymizeOffers(ctx, offers, cargo.Origin, cargo.Destination)
}

// anonymizeOffers strips participant identity from offers for the
// client-facing views (single and consolidated competitions alike),
// enriching each row with the participant's real rating average and latest
// fill report — numbers only, no identity.
func (s *CargoService) anonymizeOffers(ctx context.Context, offers []models.Offer, origin, destination models.GeoPoint) ([]AnonymizedOffer, error) {
	participantIDs := make([]uuid.UUID, 0, len(offers))
	seen := make(map[uuid.UUID]bool, len(offers))
	for _, o := range offers {
		if !seen[o.ParticipantID] {
			seen[o.ParticipantID] = true
			participantIDs = append(participantIDs, o.ParticipantID)
		}
	}

	ratingRepo := repository.NewRatingRepository(s.db)
	ratings, err := ratingRepo.SummariesForUsers(ctx, participantIDs)
	if err != nil {
		return nil, err
	}
	fillRepo := repository.NewFillReportRepository(s.db)
	fills, err := fillRepo.LatestForUsers(ctx, participantIDs)
	if err != nil {
		return nil, err
	}
	thresholdRepo := repository.NewDispatchThresholdRepository(s.db)
	thresholds, err := thresholdRepo.ForUsersMatchingRoute(ctx, participantIDs, origin, destination, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
	if err != nil {
		return nil, err
	}

	anonymized := make([]AnonymizedOffer, 0, len(offers))
	for i, o := range offers {
		row := AnonymizedOffer{
			OfferID:     o.ID,
			OfferNumber: i + 1,
			FillPercent: o.WarehouseFillPercent,
			Price:       o.Price,
			Currency:    o.Currency,
			Status:      o.Status,
		}
		if summary, ok := ratings[o.ParticipantID]; ok {
			row.Rating = summary.Average
			row.RatingCount = summary.Count
		}
		if report, ok := fills[o.ParticipantID]; ok {
			expected, actual := report.ExpectedFillPercent, report.ActualFillPercent
			row.LatestFillExpected = &expected
			row.LatestFillActual = &actual
		}
		if t, ok := thresholds[o.ParticipantID]; ok {
			threshold, accrued := t.ThresholdM3, t.AccruedM3
			row.DispatchThresholdM3 = &threshold
			row.DispatchAccruedM3 = &accrued
		}
		anonymized = append(anonymized, row)
	}
	return anonymized, nil
}
