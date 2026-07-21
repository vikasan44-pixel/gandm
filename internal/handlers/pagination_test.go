package handlers

import (
	"net/http/httptest"
	"testing"

	"gandm/internal/repository"
)

func TestPageRequestFromQuery(t *testing.T) {
	tests := []struct {
		query    string
		wantPage int
		wantSize int
		wantErr  bool
	}{
		{query: "", wantPage: 1, wantSize: 12},
		{query: "page=3&page_size=25", wantPage: 3, wantSize: 25},
		{query: "page=0", wantErr: true},
		{query: "page_size=101", wantErr: true},
		{query: "page=nope", wantErr: true},
	}
	for _, test := range tests {
		req := httptest.NewRequest("GET", "/?"+test.query, nil)
		got, err := pageRequestFromQuery(req)
		if (err != nil) != test.wantErr {
			t.Fatalf("query %q: err=%v, wantErr=%v", test.query, err, test.wantErr)
		}
		if err == nil && (got.Page != test.wantPage || got.PageSize != test.wantSize) {
			t.Fatalf("query %q: got %+v", test.query, got)
		}
	}
}

func TestCompetitionExistsMapsToConflict(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeCargoServiceError(recorder, repository.ErrOpenCompetitionExists)
	if recorder.Code != 409 {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if body := recorder.Body.String(); body == "" || !contains(body, "competition_exists") {
		t.Fatalf("response does not contain competition_exists: %s", body)
	}
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
