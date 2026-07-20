package models

import (
	"time"

	"github.com/google/uuid"
)

// Vehicle is one unit of a participant's fleet (ТЗ §11.1). BodyType is free
// text on purpose — the set of body types is business data, not schema.
type Vehicle struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Name       string    `json:"name"`
	Axles      int       `json:"axles"`
	CapacityKg float64   `json:"capacity_kg"`
	CapacityM3 float64   `json:"capacity_m3"`
	LengthM    float64   `json:"length_m"`
	WidthM     float64   `json:"width_m"`
	HeightM    float64   `json:"height_m"`
	BodyType   string    `json:"body_type"`
	// Registration data is returned only from authenticated owner endpoints.
	// Public marketplace cards are built explicitly and expose MaskedPlate only.
	RegistrationCountry      string                    `json:"registration_country"`
	PlateNumber              string                    `json:"plate_number"`
	VIN                      string                    `json:"vin"`
	VerificationStatus       VehicleVerificationStatus `json:"verification_status"`
	PrivacyConsentAt         *time.Time                `json:"privacy_consent_at,omitempty"`
	PrivacyConsentVersion    string                    `json:"privacy_consent_version,omitempty"`
	VerifiedAt               *time.Time                `json:"verified_at,omitempty"`
	VerificationRejectReason *string                   `json:"verification_reject_reason,omitempty"`
	UploadedDocumentTypes    []VehicleDocumentType     `json:"uploaded_document_types"`
	TrustPercent             int                       `json:"trust_percent"`
	DocumentsVerified        bool                      `json:"documents_verified"`
	HasCompletedTrips        bool                      `json:"has_completed_trips"`
	MaskedPlate              string                    `json:"masked_plate"`
	// Местонахождение координатами (по карте), опционально — играет роль
	// «откуда» в публичном поиске по направлению. nil, если не указано.
	Location *GeoPoint `json:"location,omitempty"`
	// Куда машина готова везти — ноль или несколько назначений (координатами).
	Destinations []VehicleDestination `json:"destinations"`
	// Trips — конкретные рейсы этой машины. Загрузка хранится на рейсе, а не
	// на машине, потому что она меняется вместе с направлением и датой.
	Trips     []VehicleTrip `json:"trips"`
	CreatedAt time.Time     `json:"created_at"`
}

// VehicleDestination — одно из назначений машины. ID нужен, чтобы удалять
// конкретное назначение; Point — координаты (как груз/маршруты).
type VehicleDestination struct {
	ID    uuid.UUID `json:"id"`
	Point GeoPoint  `json:"point"`
}

type VehicleVerificationStatus string

const (
	VehicleVerificationNotSubmitted VehicleVerificationStatus = "not_submitted"
	VehicleVerificationPending      VehicleVerificationStatus = "pending"
	VehicleVerificationVerified     VehicleVerificationStatus = "verified"
	VehicleVerificationRejected     VehicleVerificationStatus = "rejected"
)

type VehicleDocumentType string

const (
	VehicleDocumentRegistrationCertificate VehicleDocumentType = "registration_certificate"
	VehicleDocumentIdentity                VehicleDocumentType = "identity_document"
	VehicleDocumentInsurance               VehicleDocumentType = "insurance"
	VehicleDocumentPhotoFront              VehicleDocumentType = "photo_front"
	VehicleDocumentPhotoBack               VehicleDocumentType = "photo_back"
	VehicleDocumentPhotoLeft               VehicleDocumentType = "photo_left"
	VehicleDocumentPhotoRight              VehicleDocumentType = "photo_right"
)

type VehicleDocument struct {
	ID           uuid.UUID           `json:"id"`
	VehicleID    uuid.UUID           `json:"vehicle_id"`
	Type         VehicleDocumentType `json:"type"`
	FileURL      string              `json:"-"`
	OriginalName string              `json:"original_name"`
	ContentType  string              `json:"content_type"`
	UploadedAt   time.Time           `json:"uploaded_at"`
}

type VehicleTripStatus string

const (
	VehicleTripPlanned   VehicleTripStatus = "planned"
	VehicleTripLoading   VehicleTripStatus = "loading"
	VehicleTripDeparted  VehicleTripStatus = "departed"
	VehicleTripCompleted VehicleTripStatus = "completed"
)

// VehicleTrip is one dated haul. Capacity comes from its vehicle; the loaded
// values are stored here so free volume/weight can be calculated exactly for
// this trip without affecting the vehicle's other trips.
type VehicleTrip struct {
	ID               uuid.UUID         `json:"id"`
	VehicleID        uuid.UUID         `json:"vehicle_id"`
	Origin           GeoPoint          `json:"origin"`
	Destination      GeoPoint          `json:"destination"`
	Waypoints        []GeoPoint        `json:"waypoints"`
	CanPickupEnRoute bool              `json:"can_pickup_en_route"`
	DepartureDate    time.Time         `json:"departure_date"`
	LoadedWeightKg   float64           `json:"loaded_weight_kg"`
	LoadedVolumeM3   float64           `json:"loaded_volume_m3"`
	Status           VehicleTripStatus `json:"status"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// HasActiveCargo distinguishes a committed haul from a zero-load preliminary
// direction. Several preliminary directions may coexist; only one committed
// direction is allowed for a vehicle.
func (trip VehicleTrip) HasActiveCargo() bool {
	return trip.Status == VehicleTripLoading || trip.Status == VehicleTripDeparted ||
		(trip.Status == VehicleTripPlanned && (trip.LoadedWeightKg > 0 || trip.LoadedVolumeM3 > 0))
}
