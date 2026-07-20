package models

import (
	"time"

	"github.com/google/uuid"
)

type ParticipantType string

const (
	ParticipantClient     ParticipantType = "client"
	ParticipantWarehouse  ParticipantType = "warehouse"
	ParticipantCarrier    ParticipantType = "carrier"
	ParticipantDriver     ParticipantType = "driver"
	ParticipantBroker     ParticipantType = "broker"
	ParticipantCustomsRep ParticipantType = "customs_rep"
)

type UserStatus string
type LegalForm string

const (
	LegalFormIndividual  LegalForm = "individual"
	LegalFormLegalEntity LegalForm = "legal_entity"
)

const (
	UserStatusPending  UserStatus = "pending"
	UserStatusActive   UserStatus = "active"
	UserStatusBlocked  UserStatus = "blocked"
	UserStatusRejected UserStatus = "rejected"
)

type User struct {
	ID              uuid.UUID       `db:"id" json:"id"`
	Email           string          `db:"email" json:"email"`
	Phone           string          `db:"phone" json:"phone"`
	CompanyName     string          `db:"company_name" json:"company_name"`
	LegalForm       LegalForm       `db:"legal_form" json:"legal_form"`
	ParticipantType ParticipantType `db:"participant_type" json:"participant_type"`
	PasswordHash    string          `db:"password_hash" json:"-"`
	Status          UserStatus      `db:"status" json:"status"`
	HasSubscription bool            `db:"has_subscription" json:"has_subscription"`
	Language        string          `db:"language" json:"language"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
	LastActiveAt    *time.Time      `db:"last_active_at" json:"last_active_at,omitempty"`
}
