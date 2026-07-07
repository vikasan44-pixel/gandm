package models

import "github.com/google/uuid"

type UserTool struct {
	UserID uuid.UUID `db:"user_id" json:"user_id"`
	ToolID uuid.UUID `db:"tool_id" json:"tool_id"`
}

type PermissionSetTool struct {
	SetID  uuid.UUID `db:"set_id" json:"set_id"`
	ToolID uuid.UUID `db:"tool_id" json:"tool_id"`
}
