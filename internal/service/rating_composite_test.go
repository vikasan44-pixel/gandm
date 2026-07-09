package service

import (
	"testing"

	"gandm/internal/repository"
)

func f(v float64) *float64 { return &v }

func TestCompositeFromComponents(t *testing.T) {
	tests := []struct {
		name string
		in   repository.RatingComponents
		want *float64 // nil = нет сигнала
	}{
		{
			name: "no signal at all",
			in:   repository.RatingComponents{DaysOnPlatform: 100},
			want: nil,
		},
		{
			name: "perfect participant",
			in: repository.RatingComponents{
				ReviewAvg: f(5), ReviewCount: 10,
				DaysOnPlatform: 365, CompletedDeals: 20,
				ChatMessages: 50, ChatsTotal: 10, ChatsActive: 10,
			},
			want: f(5),
		},
		{
			name: "reviews only, everything else zero",
			in:   repository.RatingComponents{ReviewAvg: f(5), ReviewCount: 1},
			// 5*0.5 / 1.0 total weight = 2.5
			want: f(2.5),
		},
		{
			name: "active newcomer without reviews is not pinned to zero",
			in: repository.RatingComponents{
				CompletedDeals: 10, DaysOnPlatform: 36,
				ChatMessages: 25, ChatsTotal: 4, ChatsActive: 4,
			},
			// deals 2.5*0.2 + tenure ~0.49*0.1 + chat 2.5*0.1 + confirmed 5*0.1
			// = 1.299 / 0.5 = 2.6
			want: f(2.6),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compositeFromComponents(tt.in)
			switch {
			case tt.want == nil && got != nil:
				t.Errorf("composite = %v, want nil", *got)
			case tt.want != nil && got == nil:
				t.Errorf("composite = nil, want %v", *tt.want)
			case tt.want != nil && got != nil && *got != *tt.want:
				t.Errorf("composite = %v, want %v", *got, *tt.want)
			}
		})
	}
}

func TestCompositeNeverExceedsFive(t *testing.T) {
	in := repository.RatingComponents{
		ReviewAvg: f(5), ReviewCount: 100,
		DaysOnPlatform: 10000, CompletedDeals: 1000,
		ChatMessages: 99999, ChatsTotal: 500, ChatsActive: 500,
	}
	got := compositeFromComponents(in)
	if got == nil || *got > 5 {
		t.Errorf("composite = %v, want <= 5", got)
	}
}
