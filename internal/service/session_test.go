package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
)

func TestIsRecentRefreshRotation(t *testing.T) {
	now := time.Now()
	revokedRecently := now.Add(-time.Second)
	revokedEarlier := now.Add(-refreshRotationOverlapGrace - time.Second)
	replacement := uuid.New()

	tests := []struct {
		name string
		row  models.RefreshToken
		want bool
	}{
		{
			name: "recent rotation",
			row:  models.RefreshToken{RevokedAt: &revokedRecently, ReplacedBy: &replacement},
			want: true,
		},
		{
			name: "old replay",
			row:  models.RefreshToken{RevokedAt: &revokedEarlier, ReplacedBy: &replacement},
			want: false,
		},
		{
			name: "logout revocation has no replacement",
			row:  models.RefreshToken{RevokedAt: &revokedRecently},
			want: false,
		},
		{
			name: "active token",
			row:  models.RefreshToken{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRecentRefreshRotation(&tt.row, now); got != tt.want {
				t.Fatalf("isRecentRefreshRotation() = %v, want %v", got, tt.want)
			}
		})
	}
}
