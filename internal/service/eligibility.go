package service

import "gandm/internal/models"

// isEligibleStatus mirrors the rule already established for document
// upload: pending and active accounts can act on the platform, blocked and
// rejected accounts cannot — regardless of what tools they hold. Status is
// an account-integrity gate, separate from (and checked in addition to)
// tool-based authorization.
func isEligibleStatus(status models.UserStatus) bool {
	return status == models.UserStatusPending || status == models.UserStatusActive
}
