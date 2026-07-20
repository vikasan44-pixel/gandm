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
	ID                 uuid.UUID            `json:"id"`
	OriginLabel        string               `json:"origin_label"`
	OriginCountry      string               `json:"origin_country"`
	OriginLabels       map[string]string    `json:"origin_labels,omitempty"`
	DestinationLabel   string               `json:"destination_label"`
	DestinationCountry string               `json:"destination_country"`
	DestinationLabels  map[string]string    `json:"destination_labels,omitempty"`
	Category           models.CargoCategory `json:"category"`
	VolumeM3           float64              `json:"volume_m3"`
	WeightKg           float64              `json:"weight_kg"`
	CreatedAt          time.Time            `json:"created_at"`
}

// PublicPoint — подпись точки для гостя: основная + карта языков (без
// координат, чтобы не раскрывать точное место).
type PublicPoint struct {
	Label  string            `json:"label"`
	Labels map[string]string `json:"labels,omitempty"`
}

type PublicVehicleTrip struct {
	ID               uuid.UUID     `json:"id"`
	Origin           PublicPoint   `json:"origin"`
	Destination      PublicPoint   `json:"destination"`
	Waypoints        []PublicPoint `json:"waypoints"`
	CanPickupEnRoute bool          `json:"can_pickup_en_route"`
	PickupRadiusKm   float64       `json:"pickup_radius_km"`
	DepartureDate    time.Time     `json:"departure_date"`
	FreeWeightKg     float64       `json:"free_weight_kg"`
	FreeVolumeM3     float64       `json:"free_volume_m3"`
}

// PublicVehicleCard — анонимная карточка транспорта для гостя.
type PublicVehicleCard struct {
	ID                uuid.UUID          `json:"id"`
	BodyType          string             `json:"body_type"`
	CapacityKg        float64            `json:"capacity_kg"`
	CapacityM3        float64            `json:"capacity_m3"`
	LengthM           float64            `json:"length_m"`
	WidthM            float64            `json:"width_m"`
	HeightM           float64            `json:"height_m"`
	Axles             int                `json:"axles"`
	LocationLabel     string             `json:"location_label,omitempty"`
	LocationLabels    map[string]string  `json:"location_labels,omitempty"`
	Destinations      []PublicPoint      `json:"destinations"`
	TrustPercent      int                `json:"trust_percent"`
	DocumentsVerified bool               `json:"documents_verified"`
	HasCompletedTrips bool               `json:"has_completed_trips"`
	MaskedPlate       string             `json:"masked_plate,omitempty"`
	ActiveTrip        *PublicVehicleTrip `json:"active_trip,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
}

// PublicSearchCargo — открытые заявки на груз по координатам from/to (обе
// опциональны). Ничего личного в карточке нет.
func (s *CargoService) PublicSearchCargo(ctx context.Context, from, to *models.GeoPoint) ([]PublicCargoCard, error) {
	repo := repository.NewCargoRequestRepository(s.db)
	rows, err := repo.SearchOpenPublicCargo(ctx, from, to, nil, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
	if err != nil {
		return nil, err
	}
	cards := make([]PublicCargoCard, 0, len(rows))
	for _, c := range rows {
		cards = append(cards, PublicCargoCard{
			ID:                 c.ID,
			OriginLabel:        c.Origin.Label,
			OriginCountry:      c.Origin.Country,
			OriginLabels:       c.Origin.Labels,
			DestinationLabel:   c.Destination.Label,
			DestinationCountry: c.Destination.Country,
			DestinationLabels:  c.Destination.Labels,
			Category:           c.Category,
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
	rows, err := repo.SearchPublicVehicles(ctx, f, nil, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
	if err != nil {
		return nil, err
	}
	return publicVehicleCards(rows), nil
}

// SearchAvailableVehicles is the signed-in marketplace view. It uses the
// same anonymous cards as the guest search but never returns the caller's
// own vehicles.
func (s *CargoService) SearchAvailableVehicles(ctx context.Context, userID uuid.UUID, f repository.VehicleSearchFilter) ([]PublicVehicleCard, error) {
	if _, err := s.requireEligibleUser(ctx, userID); err != nil {
		return nil, err
	}
	repo := repository.NewVehicleRepository(s.db)
	rows, err := repo.SearchPublicVehicles(ctx, f, &userID, s.cfg.MatchRadiusCNKm, s.cfg.MatchRadiusKZKm)
	if err != nil {
		return nil, err
	}
	return publicVehicleCards(rows), nil
}

func publicVehicleCards(rows []models.Vehicle) []PublicVehicleCard {
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
			Axles:             v.Axles,
			Destinations:      []PublicPoint{},
			TrustPercent:      v.TrustPercent,
			DocumentsVerified: v.DocumentsVerified,
			HasCompletedTrips: v.HasCompletedTrips,
			MaskedPlate:       v.MaskedPlate,
			CreatedAt:         v.CreatedAt,
		}
		if v.Location != nil {
			card.LocationLabel = v.Location.Label
			card.LocationLabels = v.Location.Labels
		}
		for _, trip := range v.Trips {
			if trip.Status == models.VehicleTripCompleted {
				continue
			}
			active := &PublicVehicleTrip{
				ID:          trip.ID,
				Origin:      PublicPoint{Label: trip.Origin.Label, Labels: trip.Origin.Labels},
				Destination: PublicPoint{Label: trip.Destination.Label, Labels: trip.Destination.Labels},
				Waypoints:   []PublicPoint{}, CanPickupEnRoute: trip.CanPickupEnRoute,
				PickupRadiusKm: 50, DepartureDate: trip.DepartureDate,
				FreeWeightKg: max(0, v.CapacityKg-trip.LoadedWeightKg),
				FreeVolumeM3: max(0, v.CapacityM3-trip.LoadedVolumeM3),
			}
			for _, waypoint := range trip.Waypoints {
				active.Waypoints = append(active.Waypoints, PublicPoint{Label: waypoint.Label, Labels: waypoint.Labels})
			}
			card.ActiveTrip = active
			break
		}
		for _, d := range v.Destinations {
			card.Destinations = append(card.Destinations, PublicPoint{Label: d.Point.Label, Labels: d.Point.Labels})
		}
		cards = append(cards, card)
	}
	return cards
}
