package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

// Гостевой (без авторизации) поиск. Гость видит анонимные карточки: направление,
// габариты, объём/вес — но НЕ контакты, НЕ владельца и НЕ описание (там может
// быть телефон). Чтобы открыть объявление целиком, гость должен войти —
// фронт показывает «войдите или зарегистрируйтесь».

// PublicCargoCard — анонимная карточка груза для гостя.
type PublicCargoCard struct {
	ID                 uuid.UUID `json:"id"`
	OriginLabel        string    `json:"origin_label"`
	OriginCountry      string    `json:"origin_country"`
	DestinationLabel   string    `json:"destination_label"`
	DestinationCountry string    `json:"destination_country"`
	VolumeM3           float64   `json:"volume_m3"`
	WeightKg           float64   `json:"weight_kg"`
	CreatedAt          time.Time `json:"created_at"`
}

// PublicPoint — подпись точки для гостя: основная + карта языков (без
// координат, чтобы не раскрывать точное место).
type PublicPoint struct {
	Label  string            `json:"label"`
	Labels map[string]string `json:"labels,omitempty"`
}

// PublicVehicleCard — анонимная карточка транспорта для гостя.
type PublicVehicleCard struct {
	ID             uuid.UUID         `json:"id"`
	BodyType       string            `json:"body_type"`
	CapacityKg     float64           `json:"capacity_kg"`
	CapacityM3     float64           `json:"capacity_m3"`
	LengthM        float64           `json:"length_m"`
	WidthM         float64           `json:"width_m"`
	HeightM        float64           `json:"height_m"`
	Axles          int               `json:"axles"`
	LocationLabel  string            `json:"location_label,omitempty"`
	LocationLabels map[string]string `json:"location_labels,omitempty"`
	Destinations   []PublicPoint     `json:"destinations"`
	CreatedAt      time.Time         `json:"created_at"`
}

// PublicSearchCargo — открытые заявки на груз по координатам from/to (обе
// опциональны). Ничего личного в карточке нет.
func (s *CargoService) PublicSearchCargo(ctx context.Context, from, to *models.GeoPoint) ([]PublicCargoCard, error) {
	repo := repository.NewCargoRequestRepository(s.db)
	rows, err := repo.SearchOpenPublicCargo(ctx, from, to, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
	if err != nil {
		return nil, err
	}
	cards := make([]PublicCargoCard, 0, len(rows))
	for _, c := range rows {
		cards = append(cards, PublicCargoCard{
			ID:                 c.ID,
			OriginLabel:        c.Origin.Label,
			OriginCountry:      c.Origin.Country,
			DestinationLabel:   c.Destination.Label,
			DestinationCountry: c.Destination.Country,
			VolumeM3:           c.VolumeM3,
			WeightKg:           c.WeightKg,
			CreatedAt:          c.CreatedAt,
		})
	}
	return cards, nil
}

// PublicSearchVehicles — транспорт по характеристикам и, опционально,
// объявленному направлению координатами.
func (s *CargoService) PublicSearchVehicles(ctx context.Context, f repository.VehicleSearchFilter) ([]PublicVehicleCard, error) {
	repo := repository.NewVehicleRepository(s.db)
	rows, err := repo.SearchPublicVehicles(ctx, f, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
	if err != nil {
		return nil, err
	}
	cards := make([]PublicVehicleCard, 0, len(rows))
	for _, v := range rows {
		card := PublicVehicleCard{
			ID:                v.ID,
			BodyType:          v.BodyType,
			CapacityKg:        v.CapacityKg,
			CapacityM3:        v.CapacityM3,
			LengthM:           v.LengthM,
			WidthM:            v.WidthM,
			HeightM:           v.HeightM,
			Axles:        v.Axles,
			Destinations: []PublicPoint{},
			CreatedAt:    v.CreatedAt,
		}
		if v.Location != nil {
			card.LocationLabel = v.Location.Label
			card.LocationLabels = v.Location.Labels
		}
		for _, d := range v.Destinations {
			card.Destinations = append(card.Destinations, PublicPoint{Label: d.Point.Label, Labels: d.Point.Labels})
		}
		cards = append(cards, card)
	}
	return cards, nil
}
