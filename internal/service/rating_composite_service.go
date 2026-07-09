package service

import (
	"context"
	"math"

	"github.com/google/uuid"

	"gandm/internal/repository"
)

// Многофакторный рейтинг (ТЗ §8): отзывы + срок работы + объём
// подтверждённых заказов + активность в чате + доля сделок, ведущихся
// внутри платформы. Веса и потолки — продуктовые константы; «% сделок с
// документами» подключится сюда же вместе с антинакруткой (§6).
const (
	weightReviews   = 0.5
	weightDeals     = 0.2
	weightTenure    = 0.1
	weightChat      = 0.1
	weightConfirmed = 0.1

	// Потолки нормализации: столько нужно для «пятёрки» по компоненту.
	capDeals        = 20.0  // завершённых сделок
	capTenureDays   = 365.0 // дней на платформе
	capChatMessages = 50.0  // сообщений в чатах
)

// RatingBreakdown — публичная расшифровка рейтинга: композит + сырые
// компоненты, чтобы участник видел, из чего складывается оценка и как её
// поднять.
type RatingBreakdown struct {
	// Composite — итоговый рейтинг 0–5; nil, пока по участнику нет вообще
	// никакого сигнала (ни отзывов, ни сделок, ни сообщений).
	Composite *float64 `json:"composite"`

	ReviewAvg      *float64 `json:"average"` // имя поля сохранено для совместимости
	ReviewCount    int      `json:"count"`
	DaysOnPlatform int      `json:"days_on_platform"`
	CompletedDeals int      `json:"completed_deals"`
	ChatMessages   int      `json:"chat_messages"`
	ChatsTotal     int      `json:"chats_total"`
	ChatsActive    int      `json:"chats_active"`
}

func compositeFromComponents(c repository.RatingComponents) *float64 {
	if c.ReviewCount == 0 && c.CompletedDeals == 0 && c.ChatMessages == 0 {
		return nil // никакого сигнала — честнее «—», чем фиктивный ноль
	}

	dealsScore := math.Min(float64(c.CompletedDeals), capDeals) / capDeals * 5
	tenureScore := math.Min(float64(c.DaysOnPlatform), capTenureDays) / capTenureDays * 5
	chatScore := math.Min(float64(c.ChatMessages), capChatMessages) / capChatMessages * 5
	confirmedScore := 0.0
	if c.ChatsTotal > 0 {
		confirmedScore = float64(c.ChatsActive) / float64(c.ChatsTotal) * 5
	}

	weighted := dealsScore*weightDeals + tenureScore*weightTenure + chatScore*weightChat + confirmedScore*weightConfirmed
	totalWeight := weightDeals + weightTenure + weightChat + weightConfirmed
	// Без отзывов их вес перераспределяется на остальные компоненты —
	// иначе новый, но активный участник был бы навсегда прижат к ~2.5.
	if c.ReviewCount > 0 && c.ReviewAvg != nil {
		weighted += *c.ReviewAvg * weightReviews
		totalWeight += weightReviews
	}

	composite := math.Round(weighted/totalWeight*10) / 10
	return &composite
}

func breakdownFromComponents(c repository.RatingComponents) RatingBreakdown {
	return RatingBreakdown{
		Composite:      compositeFromComponents(c),
		ReviewAvg:      c.ReviewAvg,
		ReviewCount:    c.ReviewCount,
		DaysOnPlatform: c.DaysOnPlatform,
		CompletedDeals: c.CompletedDeals,
		ChatMessages:   c.ChatMessages,
		ChatsTotal:     c.ChatsTotal,
		ChatsActive:    c.ChatsActive,
	}
}

// compositeSummariesForUsers — то, что уходит в анонимные предложения и
// ставки: композитный рейтинг (Average) + количество отзывов (Count).
func (s *CargoService) compositeSummariesForUsers(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]repository.UserRatingSummary, error) {
	components, err := repository.NewRatingRepository(s.db).ComponentsForUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]repository.UserRatingSummary, len(components))
	for id, c := range components {
		out[id] = repository.UserRatingSummary{
			Average: compositeFromComponents(c),
			Count:   c.ReviewCount,
		}
	}
	return out, nil
}
