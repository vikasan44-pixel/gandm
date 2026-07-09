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
	ErrCompetitionClosed = errors.New("this driver competition is already closed")
	ErrNoVehicles        = errors.New("at least one vehicle is required to bid")
)

// DriverCompetitionView — конкурс глазами склада: направление + анонимные
// ставки (водитель раскрывается только после выбора).
type DriverCompetitionView struct {
	Competition    models.DriverCompetition `json:"competition"`
	DirectionLabel string                   `json:"direction_label"`
	Bids           []AnonymizedDriverBid    `json:"bids"`
}

// AnonymizedDriverBid — та же identity-free политика, что и у офферов.
type AnonymizedDriverBid struct {
	BidID       uuid.UUID              `json:"bid_id"`
	BidNumber   int                    `json:"bid_number"`
	Rating      *float64               `json:"rating"`
	RatingCount int                    `json:"rating_count"`
	Price       float64                `json:"price"`
	Currency    string                 `json:"currency"`
	Comment     string                 `json:"comment"`
	Status      models.DriverBidStatus `json:"status"`
}

// OpenDriverCompetition — конкурс глазами водителя: направление, объём,
// дата — «без названия склада» (ТЗ §11.4).
type OpenDriverCompetition struct {
	CompetitionID  uuid.UUID `json:"competition_id"`
	DirectionLabel string    `json:"direction_label"`
	VolumeM3       float64   `json:"volume_m3"`
	DispatchDate   string    `json:"dispatch_date"`
	CreatedAt      time.Time `json:"created_at"`
	// MyBid — ставка водителя на этот конкурс, если уже подана.
	MyBid *models.DriverCompetitionBid `json:"my_bid,omitempty"`
}

func routeDirectionLabel(route *models.ParticipantRoute) string {
	return route.Origin.Label + " → " + route.Destination.Label
}

// CreateDriverCompetition — склад объявляет конкурс по своему направлению
// (ручной режим, ТЗ §11.4). Водители с manage_fleet и маршрутом,
// совпадающим по радиусу, получают уведомление без названия склада.
func (s *CargoService) CreateDriverCompetition(ctx context.Context, warehouseID, routeID uuid.UUID, volumeM3 float64, dispatchDate string) (*models.DriverCompetition, error) {
	if volumeM3 <= 0 {
		return nil, fmt.Errorf("%w: volume_m3 must be positive", ErrInvalidInput)
	}
	if err := s.requireActiveUser(ctx, warehouseID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, warehouseID, ToolManageWarehouseSlots); err != nil {
		return nil, err
	}

	routeRepo := repository.NewParticipantRouteRepository(s.db)
	route, err := routeRepo.GetByID(ctx, routeID)
	if err != nil {
		return nil, err
	}
	if route.UserID != warehouseID {
		return nil, repository.ErrNotFound
	}

	competition := &models.DriverCompetition{
		ID:           uuid.New(),
		WarehouseID:  warehouseID,
		RouteID:      routeID,
		VolumeM3:     volumeM3,
		DispatchDate: strings.TrimSpace(dispatchDate),
		Status:       models.DriverCompetitionOpen,
		CreatedAt:    time.Now(),
	}
	compRepo := repository.NewDriverCompetitionRepository(s.db)
	if err := compRepo.Create(ctx, competition); err != nil {
		return nil, err
	}

	if err := s.notifyDriversAboutCompetition(ctx, competition, route); err != nil {
		return nil, err
	}
	return competition, nil
}

func (s *CargoService) notifyDriversAboutCompetition(ctx context.Context, competition *models.DriverCompetition, route *models.ParticipantRoute) error {
	toolRepo := repository.NewToolRepository(s.db)
	driverIDs, err := toolRepo.ListUserIDsWithToolAndRoute(ctx, ToolManageFleet, route.Origin, route.Destination, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"competition_id":  competition.ID,
		"direction_label": routeDirectionLabel(route),
		"volume_m3":       competition.VolumeM3,
		"dispatch_date":   competition.DispatchDate,
	})
	if err != nil {
		return err
	}

	notifRepo := repository.NewNotificationRepository(s.db)
	now := time.Now()
	for _, driverID := range driverIDs {
		// Склад не зовёт сам себя возить свой же груз.
		if driverID == competition.WarehouseID {
			continue
		}
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    driverID,
			Type:      "driver_competition_open",
			Payload:   payload,
			IsRead:    false,
			CreatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

// maybeAutoAnnounceDriverCompetition — автоматический режим (ТЗ §11.4,
// «опционально в настройках»): при достижении порога отправки система сама
// объявляет конкурс, если по направлению ещё нет открытого. Включается
// настройкой driver_competition_auto = "true".
func (s *CargoService) maybeAutoAnnounceDriverCompetition(ctx context.Context, warehouseID uuid.UUID, route *models.ParticipantRoute, threshold *models.DispatchThreshold) error {
	if threshold.AccruedM3 < threshold.ThresholdM3 {
		return nil
	}
	settingsRepo := repository.NewSettingsRepository(s.db)
	auto, err := settingsRepo.Get(ctx, repository.SettingDriverCompetitionAuto)
	if err != nil || strings.TrimSpace(auto) != "true" {
		return nil
	}

	compRepo := repository.NewDriverCompetitionRepository(s.db)
	hasOpen, err := compRepo.HasOpenForRoute(ctx, route.ID)
	if err != nil || hasOpen {
		return err
	}

	competition := &models.DriverCompetition{
		ID:           uuid.New(),
		WarehouseID:  warehouseID,
		RouteID:      route.ID,
		VolumeM3:     threshold.AccruedM3,
		DispatchDate: "",
		Status:       models.DriverCompetitionOpen,
		CreatedAt:    time.Now(),
	}
	if err := compRepo.Create(ctx, competition); err != nil {
		return err
	}
	return s.notifyDriversAboutCompetition(ctx, competition, route)
}

// ListMyDriverCompetitions — склад видит свои конкурсы с анонимными
// ставками (номер, рейтинг, цена — «по цене и рейтингу», ТЗ §11.4).
func (s *CargoService) ListMyDriverCompetitions(ctx context.Context, warehouseID uuid.UUID) ([]DriverCompetitionView, error) {
	if err := s.requireActiveUser(ctx, warehouseID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, warehouseID, ToolManageWarehouseSlots); err != nil {
		return nil, err
	}

	compRepo := repository.NewDriverCompetitionRepository(s.db)
	competitions, err := compRepo.ListByWarehouseID(ctx, warehouseID)
	if err != nil {
		return nil, err
	}
	routeRepo := repository.NewParticipantRouteRepository(s.db)
	ratingRepo := repository.NewRatingRepository(s.db)

	views := make([]DriverCompetitionView, 0, len(competitions))
	for i := range competitions {
		comp := competitions[i]
		view := DriverCompetitionView{Competition: comp}
		if route, err := routeRepo.GetByID(ctx, comp.RouteID); err == nil {
			view.DirectionLabel = routeDirectionLabel(route)
		}

		bids, err := compRepo.ListBidsByCompetitionID(ctx, comp.ID)
		if err != nil {
			return nil, err
		}
		driverIDs := make([]uuid.UUID, 0, len(bids))
		seen := make(map[uuid.UUID]bool, len(bids))
		for _, b := range bids {
			if !seen[b.DriverID] {
				seen[b.DriverID] = true
				driverIDs = append(driverIDs, b.DriverID)
			}
		}
		ratings, err := ratingRepo.SummariesForUsers(ctx, driverIDs)
		if err != nil {
			return nil, err
		}
		view.Bids = make([]AnonymizedDriverBid, 0, len(bids))
		for j, b := range bids {
			row := AnonymizedDriverBid{
				BidID:     b.ID,
				BidNumber: j + 1,
				Price:     b.Price,
				Currency:  b.Currency,
				Comment:   b.Comment,
				Status:    b.Status,
			}
			if summary, ok := ratings[b.DriverID]; ok {
				row.Rating = summary.Average
				row.RatingCount = summary.Count
			}
			view.Bids = append(view.Bids, row)
		}
		views = append(views, view)
	}
	return views, nil
}

// ListOpenDriverCompetitions — лента открытых конкурсов для водителя.
func (s *CargoService) ListOpenDriverCompetitions(ctx context.Context, driverID uuid.UUID) ([]OpenDriverCompetition, error) {
	if err := s.requireActiveUser(ctx, driverID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, driverID, ToolManageFleet); err != nil {
		return nil, err
	}

	compRepo := repository.NewDriverCompetitionRepository(s.db)
	competitions, err := compRepo.ListOpen(ctx)
	if err != nil {
		return nil, err
	}
	myBids, err := compRepo.ListBidsByDriverID(ctx, driverID)
	if err != nil {
		return nil, err
	}
	routeRepo := repository.NewParticipantRouteRepository(s.db)

	items := make([]OpenDriverCompetition, 0, len(competitions))
	for i := range competitions {
		comp := competitions[i]
		// Свои конкурсы склад в водительской ленте не видит.
		if comp.WarehouseID == driverID {
			continue
		}
		row := OpenDriverCompetition{
			CompetitionID: comp.ID,
			VolumeM3:      comp.VolumeM3,
			DispatchDate:  comp.DispatchDate,
			CreatedAt:     comp.CreatedAt,
		}
		if route, err := routeRepo.GetByID(ctx, comp.RouteID); err == nil {
			row.DirectionLabel = routeDirectionLabel(route)
		}
		if bid, ok := myBids[comp.ID]; ok {
			b := bid
			row.MyBid = &b
		}
		items = append(items, row)
	}
	return items, nil
}

// CreateDriverBid — водитель ставит цену. Нужна хотя бы одна машина в
// автопарке: предложение без транспорта бессмысленно.
func (s *CargoService) CreateDriverBid(ctx context.Context, driverID, competitionID uuid.UUID, price float64, comment string) (*models.DriverCompetitionBid, error) {
	if price <= 0 {
		return nil, fmt.Errorf("%w: price must be positive", ErrInvalidInput)
	}
	if err := s.requireActiveUser(ctx, driverID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, driverID, ToolManageFleet); err != nil {
		return nil, err
	}
	vehicleCount, err := repository.NewVehicleRepository(s.db).CountByUserID(ctx, driverID)
	if err != nil {
		return nil, err
	}
	if vehicleCount == 0 {
		return nil, ErrNoVehicles
	}

	compRepo := repository.NewDriverCompetitionRepository(s.db)
	competition, err := compRepo.GetByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}
	if competition.Status != models.DriverCompetitionOpen {
		return nil, ErrCompetitionClosed
	}
	if competition.WarehouseID == driverID {
		return nil, fmt.Errorf("%w: cannot bid on your own competition", ErrInvalidInput)
	}

	bid := &models.DriverCompetitionBid{
		ID:            uuid.New(),
		CompetitionID: competitionID,
		DriverID:      driverID,
		Price:         price,
		Currency:      "KZT",
		Comment:       strings.TrimSpace(comment),
		Status:        models.DriverBidSubmitted,
		CreatedAt:     time.Now(),
	}
	if err := compRepo.CreateBid(ctx, bid); err != nil {
		return nil, err
	}
	return bid, nil
}

type DriverSelectResult struct {
	Contact  RevealedContact `json:"contact"`
	DriverID uuid.UUID       `json:"driver_id"`
	ChatID   uuid.UUID       `json:"chat_id"`
}

// SelectDriverBid закрывает конкурс: контакт водителя раскрывается складу,
// открывается чат склад+водитель (ТЗ §11.3), обоим уходят уведомления.
// Строчная блокировка конкурса сериализует параллельные выборы; повторный
// выбор уже победившей ставки идемпотентно возвращает тот же результат.
func (s *CargoService) SelectDriverBid(ctx context.Context, warehouseID, competitionID, bidID uuid.UUID) (*DriverSelectResult, error) {
	if err := s.requireActiveUser(ctx, warehouseID); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	compRepo := repository.NewDriverCompetitionRepository(tx)
	competition, err := compRepo.GetByIDForUpdate(ctx, competitionID)
	if err != nil {
		return nil, err
	}
	if competition.WarehouseID != warehouseID {
		return nil, repository.ErrNotFound
	}

	bid, err := compRepo.GetBidByID(ctx, bidID)
	if err != nil {
		return nil, err
	}
	if bid.CompetitionID != competitionID {
		return nil, fmt.Errorf("%w: bid does not belong to this competition", ErrInvalidInput)
	}

	userRepo := repository.NewUserRepository(tx)
	chatRepo := repository.NewChatRepository(tx)

	// Идемпотентный повтор: эта ставка уже выбрана — вернуть контакт и чат
	// без повторных побочных эффектов.
	if competition.Status == models.DriverCompetitionClosed {
		if bid.Status != models.DriverBidSelected {
			return nil, ErrCompetitionClosed
		}
		driver, err := userRepo.GetByID(ctx, bid.DriverID)
		if err != nil {
			return nil, err
		}
		chat, err := chatRepo.GetByDriverCompetitionID(ctx, competitionID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &DriverSelectResult{
			Contact:  RevealedContact{CompanyName: driver.CompanyName, Email: driver.Email, Phone: driver.Phone},
			DriverID: driver.ID,
			ChatID:   chat.ID,
		}, nil
	}

	if err := compRepo.MarkBidSelected(ctx, competitionID, bidID); err != nil {
		return nil, err
	}
	if err := compRepo.Close(ctx, competitionID); err != nil {
		return nil, err
	}

	now := time.Now()
	chat := &models.Chat{ID: uuid.New(), DriverCompetitionID: &competitionID, CreatedAt: now}
	if err := chatRepo.Create(ctx, chat); err != nil {
		return nil, err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, warehouseID); err != nil {
		return nil, err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, bid.DriverID); err != nil {
		return nil, err
	}

	driver, err := userRepo.GetByID(ctx, bid.DriverID)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(map[string]any{
		"competition_id": competitionID,
		"bid_id":         bidID,
		"chat_id":        chat.ID,
	})
	if err != nil {
		return nil, err
	}
	notifRepo := repository.NewNotificationRepository(tx)
	for _, uid := range []uuid.UUID{warehouseID, bid.DriverID} {
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    uid,
			Type:      "driver_selected",
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
	return &DriverSelectResult{
		Contact:  RevealedContact{CompanyName: driver.CompanyName, Email: driver.Email, Phone: driver.Phone},
		DriverID: driver.ID,
		ChatID:   chat.ID,
	}, nil
}
