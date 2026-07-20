// Package rates fetches official daily exchange rates from the National Bank
// of Kazakhstan (NBK). The rates are used only as an approximate "≈ in your
// currency" display hint — actual deals settle in the chosen currency, no
// conversion is applied to real amounts.
package rates

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const nbkURL = "https://nationalbank.kz/rss/get_rates.cfm"

// Snapshot is one day's set of rates, each normalized to KZT per single unit
// of the currency. KZT itself is always present with rate 1.
type Snapshot struct {
	Date  string             // as published by NBK, "DD.MM.YYYY"
	Rates map[string]float64 // ISO code -> KZT per 1 unit
}

type nbkFeed struct {
	XMLName xml.Name  `xml:"rates"`
	Date    string    `xml:"date"`
	Items   []nbkItem `xml:"item"`
}

type nbkItem struct {
	Title       string `xml:"title"`       // ISO code, e.g. USD
	Description string `xml:"description"` // KZT for <quant> units
	Quant       string `xml:"quant"`       // number of units the rate is quoted per
}

// Fetch pulls NBK's official rates for the given day (or today if day is zero)
// and normalizes them to KZT-per-single-unit. Returns an error on transport,
// decode, or empty responses so the caller can keep the last known rates.
func Fetch(ctx context.Context, client *http.Client, day time.Time) (*Snapshot, error) {
	if day.IsZero() {
		day = time.Now()
	}
	url := fmt.Sprintf("%s?fdate=%s", nbkURL, day.Format("02.01.2006"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nbk rates: unexpected status %d", resp.StatusCode)
	}

	var feed nbkFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("nbk rates: decode: %w", err)
	}

	snap := &Snapshot{Date: strings.TrimSpace(feed.Date), Rates: make(map[string]float64, len(feed.Items)+1)}
	for _, it := range feed.Items {
		code := strings.ToUpper(strings.TrimSpace(it.Title))
		rate, err := strconv.ParseFloat(strings.TrimSpace(it.Description), 64)
		if err != nil || rate <= 0 || code == "" {
			continue
		}
		quant, err := strconv.ParseFloat(strings.TrimSpace(it.Quant), 64)
		if err != nil || quant <= 0 {
			quant = 1
		}
		snap.Rates[code] = rate / quant
	}
	snap.Rates["KZT"] = 1
	if len(snap.Rates) <= 1 {
		return nil, fmt.Errorf("nbk rates: empty or unparseable response")
	}
	return snap, nil
}
