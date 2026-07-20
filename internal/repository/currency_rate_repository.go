package repository

import (
	"context"
	"time"
)

type CurrencyRateRepository struct {
	db Querier
}

func NewCurrencyRateRepository(db Querier) *CurrencyRateRepository {
	return &CurrencyRateRepository{db: db}
}

// UpsertAll stores the latest rate for each code. Called after a successful
// NBK fetch; on fetch failure it is simply not called, so the previous rates
// remain (graceful fallback).
func (r *CurrencyRateRepository) UpsertAll(ctx context.Context, rates map[string]float64, rateDate time.Time) error {
	const q = `
		INSERT INTO currency_rates (code, kzt_per_unit, rate_date, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (code) DO UPDATE
		SET kzt_per_unit = EXCLUDED.kzt_per_unit, rate_date = EXCLUDED.rate_date, updated_at = now()`
	for code, rate := range rates {
		if _, err := r.db.Exec(ctx, q, code, rate, rateDate); err != nil {
			return err
		}
	}
	return nil
}

// ListAll returns every stored rate (KZT per unit) plus the most recent
// publication date across them.
func (r *CurrencyRateRepository) ListAll(ctx context.Context) (map[string]float64, time.Time, error) {
	rows, err := r.db.Query(ctx, `SELECT code, kzt_per_unit, rate_date FROM currency_rates`)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()
	out := make(map[string]float64)
	var latest time.Time
	for rows.Next() {
		var code string
		var rate float64
		var date time.Time
		if err := rows.Scan(&code, &rate, &date); err != nil {
			return nil, time.Time{}, err
		}
		out[code] = rate
		if date.After(latest) {
			latest = date
		}
	}
	return out, latest, rows.Err()
}
