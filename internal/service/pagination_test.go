package service

import "testing"

func TestPageRequestNormalizeAndOffset(t *testing.T) {
	tests := []struct {
		name       string
		input      PageRequest
		want       PageRequest
		wantOffset int
	}{
		{name: "defaults", input: PageRequest{}, want: PageRequest{Page: 1, PageSize: DefaultPageSize}, wantOffset: 0},
		{name: "requested page", input: PageRequest{Page: 3, PageSize: 20}, want: PageRequest{Page: 3, PageSize: 20}, wantOffset: 40},
		{name: "caps page size", input: PageRequest{Page: 2, PageSize: MaxPageSize + 1}, want: PageRequest{Page: 2, PageSize: MaxPageSize}, wantOffset: MaxPageSize},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.input.Normalize(); got != test.want {
				t.Fatalf("Normalize() = %+v, want %+v", got, test.want)
			}
			if got := test.input.Offset(); got != test.wantOffset {
				t.Fatalf("Offset() = %d, want %d", got, test.wantOffset)
			}
		})
	}
}

func TestNewPageUsesEmptyArray(t *testing.T) {
	page := NewPage[string](nil, 0, PageRequest{})
	if page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("Items must be a non-nil empty slice, got %#v", page.Items)
	}
}
