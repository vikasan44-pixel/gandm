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

	"gandm/internal/geo"
	"gandm/internal/models"
	"gandm/internal/repository"
)

var (
	ErrAlreadyResponded           = errors.New("consolidation suggestion already responded to")
	ErrConsolidationRouteMismatch = errors.New("cargo route does not match this consolidation")
	// Default capacity limits used only if the platform_settings rows are
	// missing (they're seeded by migration 000021).
	defaultMaxVolumeM3 = 90.0
	defaultMaxWeightKg = 20000.0
)

// consolidationWindow is how long members have to accept a consolidation
// suggestion before it resolves with whoever agreed (ТЗ: 3 часа).
const consolidationWindow = 3 * time.Hour

// ResolveExpiredConsolidations resolves suggestions whose response window has
// elapsed. Called periodically by a background sweep; each suggestion resolves
// in its own transaction under a row lock, safe alongside live member replies.
func (s *CargoService) ResolveExpiredConsolidations(ctx context.Context) error {
	ids, err := repository.NewConsolidationRepository(s.db).ListExpiredSuggestedIDs(ctx, time.Now())
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.resolveExpiredSuggestion(ctx, id); err != nil {
			log.Printf("resolve expired consolidation %s: %v", id, err)
		}
	}
	return nil
}

func (s *CargoService) resolveExpiredSuggestion(ctx context.Context, id uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	consRepo := repository.NewConsolidationRepository(tx)
	suggestion, err := consRepo.GetSuggestionByIDForUpdate(ctx, id)
	if err != nil {
		return err
	}
	if suggestion.Status != models.ConsolidationSuggested || time.Now().Before(suggestion.ResolvesAt) {
		return tx.Commit(ctx) // already resolved, or no longer expired
	}
	members, err := consRepo.ListSuggestionMembers(ctx, id)
	if err != nil {
		return err
	}
	if err := s.resolveSuggestionIfReady(ctx, tx, suggestion, members); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *CargoService) cargoMatchesConsolidation(cargo *models.CargoRequest, cons *models.ConsolidatedRequest) bool {
	radius := s.cfg.MatchRadiusKZKm
	if s.cfg.MatchRadiusCNKm > radius {
		radius = s.cfg.MatchRadiusCNKm
	}
	return geo.HaversineKm(cargo.Origin.Lat, cargo.Origin.Lng, cons.Origin.Lat, cons.Origin.Lng) <= radius &&
		geo.HaversineKm(cargo.Destination.Lat, cargo.Destination.Lng, cons.Destination.Lat, cons.Destination.Lng) <= radius
}

// RequestJoinConsolidation lets a client whose matching cargo missed the window
// join an existing open consolidation: the cargo joins, totals grow, the cargo
// closes, and warehouses that already offered are asked to re-quote.
func (s *CargoService) RequestJoinConsolidation(ctx context.Context, clientID, consolidatedID, cargoID uuid.UUID) (*models.ConsolidatedRequest, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	cargoRepo := repository.NewCargoRequestRepository(tx)
	cargo, err := cargoRepo.GetByIDForUpdate(ctx, cargoID)
	if err != nil {
		return nil, err
	}
	if cargo.ClientID != clientID {
		return nil, ErrForbiddenNotOwner
	}
	if cargo.Status != models.CargoRequestOpen {
		return nil, ErrCargoNotOpen
	}

	consRepo := repository.NewConsolidationRepository(tx)
	cons, err := consRepo.GetConsolidatedByIDForUpdate(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if cons.Status != models.CargoRequestOpen {
		return nil, ErrCargoNotOpen
	}
	selected, err := repository.NewWarehouseOfferRepository(tx).HasSelectedForConsolidated(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	if selected {
		return nil, ErrCargoNotOpen
	}
	if !s.cargoMatchesConsolidation(cargo, cons) {
		return nil, ErrConsolidationRouteMismatch
	}

	if err := consRepo.AddMemberToConsolidated(ctx, consolidatedID, cargoID, cargo.VolumeM3, cargo.WeightKg); err != nil {
		return nil, err
	}
	if err := cargoRepo.UpdateStatus(ctx, cargoID, models.CargoRequestClosed); err != nil {
		return nil, err
	}

	// Volume grew — ask warehouses that offered to recalculate.
	whs, err := repository.NewWarehouseOfferRepository(tx).ListWarehouseIDsByConsolidated(ctx, consolidatedID)
	if err != nil {
		return nil, err
	}
	for _, wh := range whs {
		if err := s.notifyWarehouseConsolidation(ctx, tx, wh.OwnerID, cons, "consolidation_volume_increased"); err != nil {
			return nil, err
		}
	}
	if err := s.notifyConsolidatedMembers(ctx, tx, consolidatedID, cons, "consolidation_member_joined"); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return repository.NewConsolidationRepository(s.db).GetConsolidatedByID(ctx, consolidatedID)
}

// ListMatchingConsolidationsForCargo returns open consolidations the client's
// open cargo could late-join (route within radius, not already a member).
func (s *CargoService) ListMatchingConsolidationsForCargo(ctx context.Context, clientID, cargoID uuid.UUID) ([]models.ConsolidatedRequest, error) {
	if _, err := s.requireEligibleUser(ctx, clientID); err != nil {
		return nil, err
	}
	cargo, err := repository.NewCargoRequestRepository(s.db).GetByID(ctx, cargoID)
	if err != nil {
		return nil, err
	}
	if cargo.ClientID != clientID {
		return nil, ErrForbiddenNotOwner
	}
	if cargo.Status != models.CargoRequestOpen {
		return []models.ConsolidatedRequest{}, nil
	}
	radius := s.cfg.MatchRadiusKZKm
	if s.cfg.MatchRadiusCNKm > radius {
		radius = s.cfg.MatchRadiusCNKm
	}
	return repository.NewConsolidationRepository(s.db).ListOpenMatchingCargoRoute(ctx, clientID, cargo.Origin, cargo.Destination, radius)
}

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
	// Распускаем ещё не отвеченные предложения: их грузы возвращаются в пул
	// и пересобираются с новой заявкой в одну группу побольше (ТЗ §4.2 «два
	// клиента И БОЛЕЕ» — все, кто влезает, едут вместе, а не парами).
	if err := consRepo.DeletePendingUnansweredSuggestions(ctx); err != nil {
		return err
	}
	candidates, err := consRepo.ListOpenCargoWithoutActiveSuggestion(ctx)
	if err != nil {
		return err
	}
	if len(candidates) < 2 {
		return nil
	}

	maxVolume, maxWeight := s.getCapacityLimits(ctx)
	groups, err := s.matcher.Match(ctx, candidates, matchingParams(maxVolume, maxWeight, s.cfg))
	if err != nil {
		return err
	}

	byID := make(map[uuid.UUID]models.CargoRequest, len(candidates))
	for _, c := range candidates {
		byID[c.ID] = c
	}

	for _, group := range groups {
		cargos := make([]models.CargoRequest, 0, len(group))
		known := true
		for _, id := range group {
			cargo, ok := byID[id]
			if !ok {
				known = false // matcher returned an id outside the pool we sent
				break
			}
			cargos = append(cargos, cargo)
		}
		if !known || len(cargos) < 2 {
			continue
		}

		// Два груза, однажды состоявшие в одном предложении (сохранившемся:
		// отклонённом «каждый едет сам» или уже создавшем консолидацию),
		// вместе не предлагаются снова. Отбраковываем ТОЛЬКО такие грузы
		// внутри группы, а не всю группу — остальные пусть объединяются.
		kept := make([]models.CargoRequest, 0, len(cargos))
		for _, cargo := range cargos {
			conflict := false
			for _, k := range kept {
				exists, err := consRepo.ExistsSuggestionForPair(ctx, cargo.ID, k.ID)
				if err != nil {
					return err
				}
				if exists {
					conflict = true
					break
				}
			}
			if !conflict {
				kept = append(kept, cargo)
			}
		}
		cargos = kept
		if len(cargos) < 2 {
			continue
		}

		suggestion := &models.ConsolidationSuggestion{
			ID:             uuid.New(),
			DirectionLabel: cargos[0].Origin.Label + " → " + cargos[0].Destination.Label,
			Status:         models.ConsolidationSuggested,
			CreatedAt:      time.Now(),
			ResolvesAt:     time.Now().Add(consolidationWindow),
		}
		if err := consRepo.CreateSuggestion(ctx, suggestion); err != nil {
			return err
		}
		for _, cargo := range cargos {
			if err := consRepo.AddSuggestionMember(ctx, &models.SuggestionMember{
				SuggestionID:   suggestion.ID,
				CargoRequestID: cargo.ID,
				ClientID:       cargo.ClientID,
				Response:       models.SuggestionPending,
			}); err != nil {
				return err
			}
		}

		for _, cargo := range cargos {
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
	SuggestionID   uuid.UUID                  `json:"suggestion_id"`
	Status         models.ConsolidationStatus `json:"status"`
	DirectionLabel string                     `json:"direction_label"`
	// Группа (ТЗ §4.2 «два клиента и более»): сколько заявок в предложении
	// и сколько уже согласилось. Other* — суммарный размер ЧУЖИХ грузов.
	MembersCount    int       `json:"members_count"`
	AgreedCount     int       `json:"agreed_count"`
	OtherVolumeM3   float64   `json:"other_volume_m3"`
	OtherWeightKg   float64   `json:"other_weight_kg"`
	MySideAgreed    bool      `json:"my_side_agreed"`
	OtherSideAgreed bool      `json:"other_side_agreed"`
	CreatedAt       time.Time `json:"created_at"`
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

	members, err := consRepo.ListSuggestionMembers(ctx, suggestion.ID)
	if err != nil {
		return nil, err
	}

	view := &ConsolidationView{
		SuggestionID:   suggestion.ID,
		Status:         suggestion.Status,
		DirectionLabel: suggestion.DirectionLabel,
		MembersCount:   len(members),
		CreatedAt:      suggestion.CreatedAt,
	}
	for _, m := range members {
		if m.Response == models.SuggestionAgreed {
			view.AgreedCount++
		}
		if m.CargoRequestID == cargoID {
			view.MySideAgreed = m.Response == models.SuggestionAgreed
			continue
		}
		// Суммарный размер ОСТАЛЬНЫХ грузов группы — без идентичности.
		other, err := cargoRepo.GetByID(ctx, m.CargoRequestID)
		if err != nil {
			return nil, err
		}
		view.OtherVolumeM3 += other.VolumeM3
		view.OtherWeightKg += other.WeightKg
		if m.Response == models.SuggestionAgreed {
			view.OtherSideAgreed = true
		}
	}
	return view, nil
}

// respondConsolidation validates that the caller owns the given cargo, the
// cargo is a member of the suggestion and the suggestion is still pending.
// Returns the suggestion and the caller's member row.
func (s *CargoService) respondConsolidation(ctx context.Context, q repository.Querier, clientID, cargoID, suggestionID uuid.UUID) (*models.ConsolidationSuggestion, []models.SuggestionMember, *models.SuggestionMember, error) {
	cargoRepo := repository.NewCargoRequestRepository(q)
	cargo, err := cargoRepo.GetByID(ctx, cargoID)
	if err != nil {
		return nil, nil, nil, err
	}
	if cargo.ClientID != clientID {
		return nil, nil, nil, ErrForbiddenNotOwner
	}

	consRepo := repository.NewConsolidationRepository(q)
	// Row lock on the suggestion: two members answering at the same time are
	// serialized here, so the second transaction re-reads the members below
	// AFTER the first has committed its answer. Without it both saw a stale
	// "everyone else still pending" snapshot and each parked in "wait for the
	// rest", leaving the group agreed-but-never-consolidated forever.
	suggestion, err := consRepo.GetSuggestionByIDForUpdate(ctx, suggestionID)
	if err != nil {
		return nil, nil, nil, err
	}
	if suggestion.Status == models.ConsolidationDeclined || suggestion.Status == models.ConsolidationBothAgreed {
		return nil, nil, nil, ErrAlreadyResponded
	}

	members, err := consRepo.ListSuggestionMembers(ctx, suggestionID)
	if err != nil {
		return nil, nil, nil, err
	}
	var mine *models.SuggestionMember
	for i := range members {
		if members[i].CargoRequestID == cargoID {
			mine = &members[i]
			break
		}
	}
	if mine == nil {
		return nil, nil, nil, fmt.Errorf("%w: suggestion does not involve this cargo request", ErrInvalidInput)
	}
	return suggestion, members, mine, nil
}

// resolveSuggestionIfReady закрывает групповое предложение, когда ответы
// собраны: согласившиеся (двое и больше) объединяются, отказавшиеся и
// «не набравшие пару» продолжают свои конкурсы. Предложение живёт, пока
// остаются pending-ответы И группа ещё может собраться (согласные +
// неответившие ≥ 2).
func (s *CargoService) resolveSuggestionIfReady(ctx context.Context, tx repository.Querier, suggestion *models.ConsolidationSuggestion, members []models.SuggestionMember) error {
	pending, agreed := 0, 0
	for _, m := range members {
		switch m.Response {
		case models.SuggestionPending:
			pending++
		case models.SuggestionAgreed:
			agreed++
		}
	}
	// Wait for the others only while the response window is still open. Once it
	// closes (resolves_at passed), resolve with whoever agreed — the rest may
	// late-join the created consolidation afterwards.
	if pending > 0 && agreed+pending >= 2 && time.Now().Before(suggestion.ResolvesAt) {
		return nil // ждём остальных, пока окно не истекло
	}

	consRepo := repository.NewConsolidationRepository(tx)
	suggestion.Status = models.ConsolidationDeclined
	if agreed >= 2 {
		agreedMembers := make([]models.SuggestionMember, 0, agreed)
		for _, m := range members {
			if m.Response == models.SuggestionAgreed {
				agreedMembers = append(agreedMembers, m)
			}
		}
		// createConsolidatedFromMembers may drop members that are no longer
		// open; if fewer than two survive it creates nothing and reports
		// created=false, so the suggestion is closed as declined.
		created, err := s.createConsolidatedFromMembers(ctx, tx, suggestion, agreedMembers)
		if err != nil {
			return err
		}
		if created {
			suggestion.Status = models.ConsolidationBothAgreed
		}
	}
	return consRepo.UpdateSuggestionStatus(ctx, suggestion.ID, suggestion.Status)
}

// AgreeConsolidation records the client's agreement; когда все участники
// ответили, согласившиеся (≥2) объединяются в одну консолидацию — в одной
// транзакции.
func (s *CargoService) AgreeConsolidation(ctx context.Context, clientID, cargoID, suggestionID uuid.UUID) (*models.ConsolidationSuggestion, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	suggestion, members, mine, err := s.respondConsolidation(ctx, tx, clientID, cargoID, suggestionID)
	if err != nil {
		return nil, err
	}
	if mine.Response == models.SuggestionAgreed {
		// Повторное согласие той же стороны — no-op.
		return suggestion, tx.Commit(ctx)
	}
	if mine.Response == models.SuggestionDeclined {
		return nil, ErrAlreadyResponded
	}

	consRepo := repository.NewConsolidationRepository(tx)
	if err := consRepo.SetSuggestionMemberResponse(ctx, suggestionID, cargoID, models.SuggestionAgreed); err != nil {
		return nil, err
	}
	mine.Response = models.SuggestionAgreed

	if err := s.resolveSuggestionIfReady(ctx, tx, suggestion, members); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return suggestion, nil
}

// createConsolidatedFromMembers merges the still-open agreed member requests:
// sums volume/weight, takes points from the first cargo (все в радиусе якоря по
// построению), closes the members and opens the shared competition. Returns
// created=false (without touching anything) when fewer than two members remain
// open, so the caller can close the suggestion as declined.
func (s *CargoService) createConsolidatedFromMembers(ctx context.Context, tx repository.Querier, suggestion *models.ConsolidationSuggestion, agreed []models.SuggestionMember) (bool, error) {
	cargoRepo := repository.NewCargoRequestRepository(tx)

	cargos := make([]*models.CargoRequest, 0, len(agreed))
	memberIDs := make([]uuid.UUID, 0, len(agreed))
	totalVolume, totalWeight := 0.0, 0.0
	for _, m := range agreed {
		// Row lock + status re-check inside the transaction: a member whose
		// own competition has meanwhile closed (matched carrier) is NOT open
		// anymore and must be skipped — otherwise the UpdateStatus(Closed)
		// below would silently wipe out that already-signed deal.
		cargo, err := cargoRepo.GetByIDForUpdate(ctx, m.CargoRequestID)
		if err != nil {
			return false, err
		}
		if cargo.Status != models.CargoRequestOpen {
			continue
		}
		cargos = append(cargos, cargo)
		memberIDs = append(memberIDs, cargo.ID)
		totalVolume += cargo.VolumeM3
		totalWeight += cargo.WeightKg
	}

	// The group can no longer form: not enough members are still open.
	if len(cargos) < 2 {
		return false, nil
	}

	consolidated := &models.ConsolidatedRequest{
		ID:               uuid.New(),
		Origin:           cargos[0].Origin,
		Destination:      cargos[0].Destination,
		TotalVolumeM3:    totalVolume,
		TotalWeightKg:    totalWeight,
		MemberRequestIDs: memberIDs,
		Status:           models.CargoRequestOpen,
		CreatedAt:        time.Now(),
	}

	consRepo := repository.NewConsolidationRepository(tx)
	if err := consRepo.CreateConsolidated(ctx, consolidated); err != nil {
		return false, err
	}

	notifRepo := repository.NewNotificationRepository(tx)
	for _, cargo := range cargos {
		// Member requests leave the individual competition.
		if err := cargoRepo.UpdateStatus(ctx, cargo.ID, models.CargoRequestClosed); err != nil {
			return false, err
		}
		payload, err := json.Marshal(map[string]any{
			"consolidated_request_id": consolidated.ID,
			"cargo_request_id":        cargo.ID,
			"direction_label":         suggestion.DirectionLabel,
		})
		if err != nil {
			return false, err
		}
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    cargo.ClientID,
			Type:      "consolidation_created",
			Payload:   payload,
			IsRead:    false,
			CreatedAt: time.Now(),
		}); err != nil {
			return false, err
		}
	}

	// Broadcast the new consolidation to warehouses that can collect it.
	if err := s.notifyMatchingWarehousesOnConsolidation(ctx, tx, consolidated); err != nil {
		return false, err
	}
	return true, nil
}

// DeclineConsolidation: отказавшийся выходит из группы; остальные ждут
// решения друг друга. Если после отказа группа уже не может собраться
// (согласные + неответившие < 2) — предложение закрывается, каждый
// продолжает свой конкурс.
func (s *CargoService) DeclineConsolidation(ctx context.Context, clientID, cargoID, suggestionID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	suggestion, members, mine, err := s.respondConsolidation(ctx, tx, clientID, cargoID, suggestionID)
	if err != nil {
		return err
	}
	if mine.Response == models.SuggestionDeclined {
		return tx.Commit(ctx) // повторный отказ — no-op
	}

	consRepo := repository.NewConsolidationRepository(tx)
	if err := consRepo.SetSuggestionMemberResponse(ctx, suggestionID, cargoID, models.SuggestionDeclined); err != nil {
		return err
	}
	mine.Response = models.SuggestionDeclined

	if err := s.resolveSuggestionIfReady(ctx, tx, suggestion, members); err != nil {
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
	isMember, err := consRepo.IsConsolidatedMember(ctx, participantID, consolidatedID)
	if err != nil {
		return nil, err
	}
	if isMember {
		return nil, fmt.Errorf("%w: cannot submit an offer to your own consolidated cargo request", ErrInvalidInput)
	}

	currency, err := s.resolveCurrency(in.Currency)
	if err != nil {
		return nil, err
	}

	offer := &models.Offer{
		ID:                    uuid.New(),
		ConsolidatedRequestID: &consolidatedID,
		ParticipantID:         participantID,
		Price:                 in.Price,
		Currency:              currency,
		Conditions:            in.Conditions,
		WarehouseFillPercent:  in.WarehouseFillPercent,
		Status:                models.OfferSubmitted,
		CreatedAt:             time.Now(),
	}
	offerRepo := repository.NewOfferRepository(s.db)
	return offerRepo.CreateOrUpdateSubmitted(ctx, offer)
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
