package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

// batchReminderBody — автоматическое напоминание при открытии чата партии
// (ТЗ §10.1). Русский текст осознанно: это содержимое чата, не API-ошибка.
const batchReminderBody = "Ваши грузы едут вместе. Проверьте разрешительные документы — " +
	"один груз без документов задержит всю партию.\n\nГрузы в партии: %s"

// syncWarehouseBatch — жизненный цикл партии (ТЗ §10.1), вызывается при
// каждом сохранении порога отправки:
//   - набрано ≥ порога и активной партии нет → партия открывается: общий
//     чат склад + клиенты matched-сделок по направлению, напоминание про
//     документы (в чате видны только наименования грузов), уведомления;
//   - набрано < порога → активная партия считается отправленной.
func (s *CargoService) syncWarehouseBatch(ctx context.Context, warehouseID uuid.UUID, route *models.ParticipantRoute, threshold *models.DispatchThreshold) error {
	batchRepo := repository.NewWarehouseBatchRepository(s.db)

	if threshold.AccruedM3 < threshold.ThresholdM3 {
		return batchRepo.DispatchActiveForRoute(ctx, route.ID, time.Now())
	}

	hasActive, err := batchRepo.HasActiveForRoute(ctx, route.ID)
	if err != nil || hasActive {
		return err
	}

	candidates, err := batchRepo.ListBatchCandidates(ctx, warehouseID, route.ID, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
	if err != nil {
		return err
	}
	// Без сделок на платформе партия — внутреннее дело склада: чат не с кем
	// открывать (объём мог набраться из офлайн-грузов).
	if len(candidates) == 0 {
		return nil
	}

	now := time.Now()
	batch := &models.WarehouseBatch{
		ID:          uuid.New(),
		WarehouseID: warehouseID,
		RouteID:     route.ID,
		VolumeM3:    threshold.AccruedM3,
		CreatedAt:   now,
	}
	if err := batchRepo.Create(ctx, batch); err != nil {
		return err
	}

	chatRepo := repository.NewChatRepository(s.db)
	chat := &models.Chat{ID: uuid.New(), WarehouseBatchID: &batch.ID, CreatedAt: now}
	if err := chatRepo.Create(ctx, chat); err != nil {
		return err
	}
	if err := chatRepo.AddParticipant(ctx, chat.ID, warehouseID); err != nil {
		return err
	}

	cargoNames := make([]string, 0, len(candidates))
	clientSeen := make(map[uuid.UUID]bool)
	for _, c := range candidates {
		if err := batchRepo.AddMember(ctx, batch.ID, c.CargoRequestID, c.ClientID); err != nil {
			return err
		}
		if name := strings.TrimSpace(c.Description); name != "" {
			cargoNames = append(cargoNames, name)
		}
		if !clientSeen[c.ClientID] {
			clientSeen[c.ClientID] = true
			if err := chatRepo.AddParticipant(ctx, chat.ID, c.ClientID); err != nil {
				return err
			}
		}
	}

	// Напоминание — первым сообщением чата, от имени склада (владельца
	// партии): в чате видны только наименования грузов, без личных данных.
	names := strings.Join(cargoNames, ", ")
	if names == "" {
		names = "—"
	}
	msgRepo := repository.NewMessageRepository(s.db)
	if err := msgRepo.Create(ctx, &models.Message{
		ID:        uuid.New(),
		ChatID:    chat.ID,
		SenderID:  warehouseID,
		Body:      strings.Replace(batchReminderBody, "%s", names, 1),
		CreatedAt: now,
	}); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"batch_id":        batch.ID,
		"chat_id":         chat.ID,
		"direction_label": routeDirectionLabel(route),
	})
	if err != nil {
		return err
	}
	notifRepo := repository.NewNotificationRepository(s.db)
	for clientID := range clientSeen {
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    clientID,
			Type:      "batch_chat_opened",
			Payload:   payload,
			IsRead:    false,
			CreatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}
