package models

import (
	"time"

	"github.com/google/uuid"
)

type CargoRequestStatus string
type CargoCategory string

const (
	CargoRequestOpen    CargoRequestStatus = "open"
	CargoRequestMatched CargoRequestStatus = "matched"
	CargoRequestClosed  CargoRequestStatus = "closed"
)

const (
	CargoCategoryChemicals         CargoCategory = "chemicals"
	CargoCategoryEquipment         CargoCategory = "equipment"
	CargoCategoryBuildingMaterials CargoCategory = "building_materials"
	CargoCategoryHomeAppliances    CargoCategory = "home_appliances"
	CargoCategoryFurniture         CargoCategory = "furniture"
	CargoCategoryFood              CargoCategory = "food"
	CargoCategoryTextiles          CargoCategory = "textiles"
	CargoCategoryAutoParts         CargoCategory = "auto_parts"
	CargoCategoryMetals            CargoCategory = "metals"
	CargoCategoryTimber            CargoCategory = "timber"
	CargoCategoryMedicalGoods      CargoCategory = "medical_goods"
	CargoCategoryAgriculturalGoods CargoCategory = "agricultural_goods"
	CargoCategoryPlastics          CargoCategory = "plastics"
	CargoCategoryDangerousGoods    CargoCategory = "dangerous_goods"
	CargoCategoryOther             CargoCategory = "other"
)

func IsValidCargoCategory(category CargoCategory) bool {
	switch category {
	case CargoCategoryChemicals, CargoCategoryEquipment, CargoCategoryBuildingMaterials,
		CargoCategoryHomeAppliances, CargoCategoryFurniture, CargoCategoryFood,
		CargoCategoryTextiles, CargoCategoryAutoParts, CargoCategoryMetals,
		CargoCategoryTimber, CargoCategoryMedicalGoods, CargoCategoryAgriculturalGoods,
		CargoCategoryPlastics, CargoCategoryDangerousGoods, CargoCategoryOther:
		return true
	default:
		return false
	}
}

// CargoPackaging distinguishes discrete packages from loose/bulk cargo.
type CargoPackaging string

const (
	// CargoPackaged — тарно-штучный: discrete places with dimensions.
	CargoPackaged CargoPackaging = "packaged"
	// CargoBulk — россыпью: loose/bulk, no discrete places.
	CargoBulk CargoPackaging = "bulk"
)

func IsValidCargoPackaging(p CargoPackaging) bool {
	return p == CargoPackaged || p == CargoBulk
}

// CargoRequestItem is one package ("место") with its dimensions. Present only
// for packaged cargo.
type CargoRequestItem struct {
	ID       uuid.UUID `json:"id"`
	Position int       `json:"position"`
	LengthM  float64   `json:"length_m"`
	WidthM   float64   `json:"width_m"`
	HeightM  float64   `json:"height_m"`
}

type CargoRequest struct {
	ID          uuid.UUID          `json:"id"`
	ClientID    uuid.UUID          `json:"client_id"`
	Origin      GeoPoint           `json:"origin"`
	Destination GeoPoint           `json:"destination"`
	VolumeM3    float64            `json:"volume_m3"`
	WeightKg    float64            `json:"weight_kg"`
	Category    CargoCategory      `json:"category"`
	Description string             `json:"description"`
	Status      CargoRequestStatus `json:"status"`
	CreatedAt   time.Time          `json:"created_at"`

	// Logistics detail: packaging kind, package count + per-place dimensions,
	// stackability, and whether ADR (dangerous-goods) transport is required.
	Packaging   CargoPackaging     `json:"packaging"`
	PlacesCount int                `json:"places_count"`
	Stackable   bool               `json:"stackable"`
	ADRRequired bool               `json:"adr_required"`
	Items       []CargoRequestItem `json:"items"`
}

// CargoItemSummary is safe to expose in consolidation/customs previews:
// the category is translated by the UI, while description stays the
// author's original text. It contains no client identity.
type CargoItemSummary struct {
	Category    CargoCategory `json:"category"`
	Description string        `json:"description"`
}
