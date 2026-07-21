package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"gandm/internal/matching"
	"gandm/internal/models"
	"gandm/internal/money"
	"gandm/internal/payment"
	"gandm/internal/repository"
	"gandm/internal/storage"
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
	ErrOfferNotEditable  = errors.New("offer is no longer editable")
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
	// DefaultCurrency is the ISO-4217 fallback when a submitted price omits a
	// currency. Each deal still carries its own currency; this is only the
	// default. Guaranteed to be a supported code (validated at config load).
	DefaultCurrency string
}

type CargoService struct {
	db       *pgxpool.Pool
	cfg      CargoServiceConfig
	matcher  *matching.Client
	payments payment.Provider
	storage  *storage.S3Client
}

func NewCargoService(db *pgxpool.Pool, cfg CargoServiceConfig, matcher *matching.Client, payments payment.Provider, stores ...*storage.S3Client) *CargoService {
	var documentStorage *storage.S3Client
	if len(stores) > 0 {
		documentStorage = stores[0]
	}
	return &CargoService{db: db, cfg: cfg, matcher: matcher, payments: payments, storage: documentStorage}
}

// resolveCurrency validates a submitted currency code. An empty code falls
// back to the platform default; a non-empty but unsupported code is rejected.
// The return is always an upper-case supported ISO-4217 code.
func (s *CargoService) resolveCurrency(code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return s.cfg.DefaultCurrency, nil
	}
	c := money.Normalize(code)
	if c == "" {
		return "", fmt.Errorf("%w: unsupported currency %q", ErrInvalidInput, code)
	}
	return c, nil
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
	Category    models.CargoCategory
	Description string

	// Logistics detail (see models.CargoRequest).
	Packaging   models.CargoPackaging
	PlacesCount int
	Stackable   bool
	ADRRequired bool
	Items       []models.CargoRequestItem
}

func validateCargoDetails(category models.CargoCategory, description string) error {
	if !models.IsValidCargoCategory(category) {
		return fmt.Errorf("%w: invalid cargo category", ErrInvalidInput)
	}
	if category == models.CargoCategoryOther && len(strings.TrimSpace(description)) == 0 {
		return fmt.Errorf("%w: description is required for other category", ErrInvalidInput)
	}
	return nil
}

// normalizeCargoLogistics validates and normalizes the packaging fields: bulk
// cargo has no discrete places, packaged cargo carries per-place dimensions and
// a places count at least as large as the listed packages.
func normalizeCargoLogistics(in *CreateCargoRequestInput) error {
	if in.Packaging == "" {
		in.Packaging = models.CargoPackaged
	}
	if !models.IsValidCargoPackaging(in.Packaging) {
		return fmt.Errorf("%w: packaging must be \"packaged\" or \"bulk\"", ErrInvalidInput)
	}
	if in.Packaging == models.CargoBulk {
		in.PlacesCount = 0
		in.Items = nil
		return nil
	}
	for i := range in.Items {
		in.Items[i].Position = i
	}
	if in.PlacesCount < len(in.Items) {
		in.PlacesCount = len(in.Items)
	}
	if in.PlacesCount < 0 {
		in.PlacesCount = 0
	}
	return nil
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
	if err := validateCargoDetails(in.Category, in.Description); err != nil {
		return nil, err
	}
	if err := normalizeCargoLogistics(&in); err != nil {
		return nil, err
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
		Category:    in.Category,
		Description: in.Description,
		Status:      models.CargoRequestOpen,
		CreatedAt:   time.Now(),
		Packaging:   in.Packaging,
		PlacesCount: in.PlacesCount,
		Stackable:   in.Stackable,
		ADRRequired: in.ADRRequired,
		Items:       in.Items,
	}

	// Cargo row and its package rows must land together.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := repository.NewCargoRequestRepository(tx).Create(ctx, cargo); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
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
	if err := s.notifyMatchingWarehousesOnCargo(ctx, cargo); err != nil {
		log.Printf("cargo %s: notify matching warehouses: %v", cargo.ID, err)
	}

	return cargo, nil
}

func (s *CargoService) UpdateCargoRequest(ctx context.Context, clientID, cargoID uuid.UUID, in CreateCargoRequestInput) (*models.CargoRequest, error) {
	origin, err := validateGeoPoint("origin", in.Origin)
	if err != nil {
		return nil, err
	}
	destination, err := validateGeoPoint("destination", in.Destination)
	if err != nil {
		return nil, err
	}
	if in.VolumeM3 <= 0 || in.WeightKg <= 0 {
		return nil, fmt.Errorf("%w: volume_m3 and weight_kg must be positive", ErrInvalidInput)
	}
	if err := validateCargoDetails(in.Category, in.Description); err != nil {
		return nil, err
	}
	if err := normalizeCargoLogistics(&in); err != nil {
		return nil, err
	}
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	repo := repository.NewCargoRequestRepository(tx)
	cargo, err := repo.GetByIDForUpdate(ctx, cargoID)
	if err != nil {
		return nil, err
	}
	if cargo.ClientID != clientID {
		return nil, ErrForbiddenNotOwner
	}
	if cargo.Status != models.CargoRequestOpen {
		return nil, ErrCargoNotOpen
	}
	cargo.Origin = origin
	cargo.Destination = destination
	cargo.VolumeM3 = in.VolumeM3
	cargo.WeightKg = in.WeightKg
	cargo.Category = in.Category
	cargo.Description = in.Description
	cargo.Packaging = in.Packaging
	cargo.PlacesCount = in.PlacesCount
	cargo.Stackable = in.Stackable
	cargo.ADRRequired = in.ADRRequired
	cargo.Items = in.Items
	if err := repo.UpdateOpenOwned(ctx, cargo); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if err := s.suggestConsolidations(ctx); err != nil {
		log.Printf("cargo %s: refresh consolidation suggestions after edit: %v", cargo.ID, err)
	}
	return cargo, nil
}

func (s *CargoService) CancelCargoRequest(ctx context.Context, clientID, cargoID uuid.UUID) error {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cargoRepo := repository.NewCargoRequestRepository(tx)
	cargo, err := cargoRepo.GetByIDForUpdate(ctx, cargoID)
	if err != nil {
		return err
	}
	if cargo.ClientID != clientID {
		return ErrForbiddenNotOwner
	}
	if cargo.Status != models.CargoRequestOpen {
		return ErrCargoNotOpen
	}
	if err := cargoRepo.UpdateStatus(ctx, cargoID, models.CargoRequestClosed); err != nil {
		return err
	}
	if err := repository.NewOfferRepository(tx).RejectSubmittedForCargo(ctx, cargoID); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
		if userID == cargo.ClientID {
			continue
		}
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

func (s *CargoService) ListMyCargoRequests(ctx context.Context, clientID uuid.UUID, pageRequest PageRequest) (Page[models.CargoRequest], error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return Page[models.CargoRequest]{}, err
	}
	pageRequest = pageRequest.Normalize()
	cargoRepo := repository.NewCargoRequestRepository(s.db)
	items, total, err := cargoRepo.ListByClientIDPage(ctx, clientID, pageRequest.PageSize, pageRequest.Offset())
	if err != nil {
		return Page[models.CargoRequest]{}, err
	}
	return NewPage(items, total, pageRequest), nil
}

// ListAvailableCargoRequests enforces both gates from the brief: the
// receive_cargo_by_route tool AND at least one route within the per-country
// radius of both cargo endpoints (radius matching happens in SQL). A tooled
// participant with no routes correctly sees an empty list.
func (s *CargoService) ListAvailableCargoRequests(ctx context.Context, participantID uuid.UUID, from, to *models.GeoPoint, pageRequest PageRequest) (Page[models.CargoRequest], error) {
	if _, err := s.requireEligibleUser(ctx, participantID); err != nil {
		return Page[models.CargoRequest]{}, err
	}
	if err := s.requireTool(ctx, participantID, ToolReceiveCargoByRoute); err != nil {
		return Page[models.CargoRequest]{}, err
	}

	pageRequest = pageRequest.Normalize()
	cargoRepo := repository.NewCargoRequestRepository(s.db)
	items, total, err := cargoRepo.ListOpenMatchingUserRoutesPage(ctx, participantID, from, to, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm, pageRequest.PageSize, pageRequest.Offset())
	if err != nil {
		return Page[models.CargoRequest]{}, err
	}
	return NewPage(items, total, pageRequest), nil
}

type CargoCompetitionResponse struct {
	Offer          models.Offer          `json:"offer"`
	DirectionLabel string                `json:"direction_label"`
	Origin         *models.GeoPoint      `json:"origin,omitempty"`
	Destination    *models.GeoPoint      `json:"destination,omitempty"`
	Category       *models.CargoCategory `json:"category,omitempty"`
	VolumeM3       float64               `json:"volume_m3"`
	WeightKg       float64               `json:"weight_kg"`
	IsConsolidated bool                  `json:"is_consolidated"`
}

func (s *CargoService) ListMyCargoCompetitionResponses(ctx context.Context, participantID uuid.UUID) ([]CargoCompetitionResponse, error) {
	if _, err := s.requireEligibleUser(ctx, participantID); err != nil {
		return nil, err
	}
	offers, err := repository.NewOfferRepository(s.db).ListByParticipantID(ctx, participantID)
	if err != nil {
		return nil, err
	}
	cargoRepo := repository.NewCargoRequestRepository(s.db)
	consRepo := repository.NewConsolidationRepository(s.db)
	items := make([]CargoCompetitionResponse, 0, len(offers))
	for _, offer := range offers {
		row := CargoCompetitionResponse{Offer: offer}
		if offer.CargoRequestID != nil {
			cargo, err := cargoRepo.GetByID(ctx, *offer.CargoRequestID)
			if err != nil {
				return nil, err
			}
			row.DirectionLabel = cargo.Origin.Label + " → " + cargo.Destination.Label
			row.Origin = &cargo.Origin
			row.Destination = &cargo.Destination
			row.Category = &cargo.Category
			row.VolumeM3 = cargo.VolumeM3
			row.WeightKg = cargo.WeightKg
		} else if offer.ConsolidatedRequestID != nil {
			cons, err := consRepo.GetConsolidatedByID(ctx, *offer.ConsolidatedRequestID)
			if err != nil {
				return nil, err
			}
			row.DirectionLabel = cons.Origin.Label + " → " + cons.Destination.Label
			row.Origin = &cons.Origin
			row.Destination = &cons.Destination
			row.VolumeM3 = cons.TotalVolumeM3
			row.WeightKg = cons.TotalWeightKg
			row.IsConsolidated = true
		}
		items = append(items, row)
	}
	return items, nil
}

// ListMyNotifications exists primarily so notification delivery is
// observable (smoke test, future UI) — Stage 2 wrote notifications with no
// read path at all.
func (s *CargoService) ListMyNotifications(ctx context.Context, userID uuid.UUID) ([]models.Notification, error) {
	if _, err := s.requireEligibleUser(ctx, userID); err != nil {
		return nil, err
	}
	notifRepo := repository.NewNotificationRepository(s.db)
	return notifRepo.ListByUserID(ctx, userID)
}

// CountMyUnreadNotifications backs the badge poller.
func (s *CargoService) CountMyUnreadNotifications(ctx context.Context, userID uuid.UUID) (int, error) {
	if _, err := s.requireEligibleUser(ctx, userID); err != nil {
		return 0, err
	}
	notifRepo := repository.NewNotificationRepository(s.db)
	return notifRepo.CountUnreadByUserID(ctx, userID)
}

// MarkMyNotificationsRead flags all of the caller's notifications as read.
func (s *CargoService) MarkMyNotificationsRead(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.requireEligibleUser(ctx, userID); err != nil {
		return err
	}
	notifRepo := repository.NewNotificationRepository(s.db)
	return notifRepo.MarkAllReadByUserID(ctx, userID)
}

type CreateOfferInput struct {
	Price                float64
	Currency             string
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
	if cargo.ClientID == participantID {
		return nil, fmt.Errorf("%w: cannot submit an offer to your own cargo request", ErrInvalidInput)
	}

	currency, err := s.resolveCurrency(in.Currency)
	if err != nil {
		return nil, err
	}

	offer := &models.Offer{
		ID:                   uuid.New(),
		CargoRequestID:       &cargoRequestID,
		ParticipantID:        participantID,
		Price:                in.Price,
		Currency:             currency,
		Conditions:           in.Conditions,
		WarehouseFillPercent: in.WarehouseFillPercent,
		Status:               models.OfferSubmitted,
		CreatedAt:            time.Now(),
	}

	offerRepo := repository.NewOfferRepository(s.db)
	return offerRepo.CreateOrUpdateSubmitted(ctx, offer)
}

func validateOfferInput(in CreateOfferInput) error {
	if in.Price <= 0 {
		return fmt.Errorf("%w: price must be positive", ErrInvalidInput)
	}
	if in.WarehouseFillPercent != nil && (*in.WarehouseFillPercent < 0 || *in.WarehouseFillPercent > 100) {
		return fmt.Errorf("%w: warehouse_fill_percent must be between 0 and 100", ErrInvalidInput)
	}
	return nil
}

func (s *CargoService) ensureOfferTargetOpen(ctx context.Context, offer *models.Offer) error {
	if offer.CargoRequestID != nil {
		cargo, err := repository.NewCargoRequestRepository(s.db).GetByID(ctx, *offer.CargoRequestID)
		if err != nil {
			return err
		}
		if cargo.Status != models.CargoRequestOpen {
			return ErrCargoNotOpen
		}
		return nil
	}
	if offer.ConsolidatedRequestID != nil {
		cons, err := repository.NewConsolidationRepository(s.db).GetConsolidatedByID(ctx, *offer.ConsolidatedRequestID)
		if err != nil {
			return err
		}
		if cons.Status != models.CargoRequestOpen {
			return ErrCargoNotOpen
		}
		return nil
	}
	return fmt.Errorf("%w: offer has no target", ErrInvalidInput)
}

func (s *CargoService) UpdateMyOffer(ctx context.Context, participantID, offerID uuid.UUID, in CreateOfferInput) (*models.Offer, error) {
	if err := validateOfferInput(in); err != nil {
		return nil, err
	}
	if _, err := s.requireEligibleUser(ctx, participantID); err != nil {
		return nil, err
	}
	repo := repository.NewOfferRepository(s.db)
	offer, err := repo.GetByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer.ParticipantID != participantID {
		return nil, ErrForbiddenNotOwner
	}
	if offer.Status != models.OfferSubmitted {
		return nil, ErrOfferNotEditable
	}
	if err := s.ensureOfferTargetOpen(ctx, offer); err != nil {
		return nil, err
	}
	currency, err := s.resolveCurrency(in.Currency)
	if err != nil {
		return nil, err
	}
	updated, err := repo.UpdateSubmittedOwned(ctx, offerID, participantID, in.Price, currency, in.Conditions, in.WarehouseFillPercent)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrOfferNotEditable
	}
	return updated, err
}

func (s *CargoService) WithdrawMyOffer(ctx context.Context, participantID, offerID uuid.UUID) (*models.Offer, error) {
	if _, err := s.requireEligibleUser(ctx, participantID); err != nil {
		return nil, err
	}
	repo := repository.NewOfferRepository(s.db)
	offer, err := repo.GetByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer.ParticipantID != participantID {
		return nil, ErrForbiddenNotOwner
	}
	if offer.Status != models.OfferSubmitted {
		return nil, ErrOfferNotEditable
	}
	withdrawn, err := repo.WithdrawSubmittedOwned(ctx, offerID, participantID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrOfferNotEditable
	}
	return withdrawn, err
}

// AnonymizedOffer is what the client sees: enough to compare and decide,
// nothing that identifies who made the offer. OfferID is the offer's own
// uuid — needed to select it — and reveals nothing about the participant.
// Rating is the participant's real average (nil = no ratings yet, shown as
// "—", never a fake 0). LatestFill* mirror the participant's newest
// warehouse fill report as bare numbers — joined server-side so the client
// never learns whose report it is.
type AnonymizedOffer struct {
	OfferID            uuid.UUID `json:"offer_id"`
	OfferNumber        int       `json:"offer_number"`
	Rating             *float64  `json:"rating"`
	RatingCount        int       `json:"rating_count"`
	FillPercent        *float64  `json:"fill_percent,omitempty"`
	LatestFillExpected *float64  `json:"latest_fill_expected,omitempty"`
	LatestFillActual   *float64  `json:"latest_fill_actual,omitempty"`
	// Dispatch* mirror the participant's «порог отправки» on their route
	// matching this cargo direction (ТЗ §5.2: клиент видит, сколько кубов
	// набрано и сколько осталось до отправки) — bare numbers, no identity.
	DispatchThresholdM3 *float64   `json:"dispatch_threshold_m3,omitempty"`
	DispatchAccruedM3   *float64   `json:"dispatch_accrued_m3,omitempty"`
	DispatchRemainingM3 *float64   `json:"dispatch_remaining_m3,omitempty"`
	DispatchDate        *time.Time `json:"dispatch_date,omitempty"`
	DispatchStatus      string     `json:"dispatch_status,omitempty"`

	Price    float64            `json:"price"`
	Currency string             `json:"currency"`
	Status   models.OfferStatus `json:"status"`
}

func (s *CargoService) ListOffersForClient(ctx context.Context, clientID, cargoRequestID uuid.UUID) ([]AnonymizedOffer, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
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

	// Композитный рейтинг (ТЗ §8) — не только отзывы.
	ratings, err := s.compositeSummariesForUsers(ctx, participantIDs)
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
			threshold, accrued, remaining := t.ThresholdM3, t.AccruedM3, t.RemainingM3
			row.DispatchThresholdM3 = &threshold
			row.DispatchAccruedM3 = &accrued
			row.DispatchRemainingM3 = &remaining
			row.DispatchDate = t.EstimatedDispatchDate
			row.DispatchStatus = t.Status
		}
		anonymized = append(anonymized, row)
	}
	return anonymized, nil
}
