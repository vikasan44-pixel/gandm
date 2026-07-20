package service

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"gandm/internal/rates"
	"gandm/internal/repository"
)

// RatesService keeps the NBK exchange-rate snapshot fresh in the DB and serves
// it for the frontend's approximate "≈ in your currency" hint. It is separate
// from CargoService on purpose — this is display-only reference data and never
// touches deal amounts.
type RatesService struct {
	db     *pgxpool.Pool
	client *http.Client
}

func NewRatesService(db *pgxpool.Pool) *RatesService {
	// NBK can be slow; keep this under the 30s refresh context in main.go.
	return &RatesService{db: db, client: &http.Client{Timeout: 25 * time.Second}}
}

// RatesView is the payload the frontend consumes: KZT-based rates (KZT per one
// unit of each currency) plus the NBK publication date.
type RatesView struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// Refresh fetches today's NBK rates and upserts them. On any fetch/parse error
// it returns the error without touching stored rates, so the last good
// snapshot is preserved.
func (s *RatesService) Refresh(ctx context.Context) error {
	snap, err := rates.Fetch(ctx, s.client, time.Time{})
	if err != nil {
		return err
	}
	day, err := time.Parse("02.01.2006", snap.Date)
	if err != nil {
		day = time.Now()
	}
	return repository.NewCurrencyRateRepository(s.db).UpsertAll(ctx, snap.Rates, day)
}

// Current returns the stored rate snapshot. Rates may be empty if the first
// fetch hasn't succeeded yet — the frontend then simply shows no hint.
func (s *RatesService) Current(ctx context.Context) (*RatesView, error) {
	m, day, err := repository.NewCurrencyRateRepository(s.db).ListAll(ctx)
	if err != nil {
		return nil, err
	}
	date := ""
	if !day.IsZero() {
		date = day.Format("2006-01-02")
	}
	return &RatesView{Base: "KZT", Date: date, Rates: m}, nil
}
