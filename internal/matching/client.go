// Package matching is the HTTP client for the Python matching service.
// The contract is documented in matching/README.md; the service is
// stateless — limits and radii travel in every request.
package matching

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

type pointDTO struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Country string  `json:"country"`
}

type cargoDTO struct {
	ID          string   `json:"id"`
	ClientID    string   `json:"client_id"`
	Origin      pointDTO `json:"origin"`
	Destination pointDTO `json:"destination"`
	VolumeM3    float64  `json:"volume_m3"`
	WeightKg    float64  `json:"weight_kg"`
}

type limitsDTO struct {
	MaxVolumeM3 float64 `json:"max_volume_m3"`
	MaxWeightKg float64 `json:"max_weight_kg"`
}

type radiiDTO struct {
	CNKm float64 `json:"cn_km"`
	KZKm float64 `json:"kz_km"`
}

type matchRequest struct {
	Requests []cargoDTO `json:"requests"`
	Limits   limitsDTO  `json:"limits"`
	Radii    radiiDTO   `json:"radii"`
}

type pairDTO struct {
	A string `json:"a"`
	B string `json:"b"`
}

type matchResponse struct {
	Pairs []pairDTO `json:"pairs"`
}

type Pair struct {
	A uuid.UUID
	B uuid.UUID
}

type MatchParams struct {
	MaxVolumeM3 float64
	MaxWeightKg float64
	CNRadiusKm  float64
	KZRadiusKm  float64
}

func toPointDTO(p models.GeoPoint) pointDTO {
	return pointDTO{Lat: p.Lat, Lng: p.Lng, Country: p.Country}
}

// Match sends the candidate pool to the Python service and returns the
// suggested pairs. Any transport or decode failure is returned as-is — the
// caller decides whether matching is critical (it isn't: cargo submission
// must not fail because the matcher is down).
func (c *Client) Match(ctx context.Context, candidates []models.CargoRequest, params MatchParams) ([]Pair, error) {
	reqBody := matchRequest{
		Requests: make([]cargoDTO, 0, len(candidates)),
		Limits:   limitsDTO{MaxVolumeM3: params.MaxVolumeM3, MaxWeightKg: params.MaxWeightKg},
		Radii:    radiiDTO{CNKm: params.CNRadiusKm, KZKm: params.KZRadiusKm},
	}
	for _, cargo := range candidates {
		reqBody.Requests = append(reqBody.Requests, cargoDTO{
			ID:          cargo.ID.String(),
			ClientID:    cargo.ClientID.String(),
			Origin:      toPointDTO(cargo.Origin),
			Destination: toPointDTO(cargo.Destination),
			VolumeM3:    cargo.VolumeM3,
			WeightKg:    cargo.WeightKg,
		})
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/match", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("matching service returned HTTP %d", resp.StatusCode)
	}

	var decoded matchResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}

	pairs := make([]Pair, 0, len(decoded.Pairs))
	for _, p := range decoded.Pairs {
		a, err := uuid.Parse(p.A)
		if err != nil {
			return nil, fmt.Errorf("matching service returned invalid id %q", p.A)
		}
		b, err := uuid.Parse(p.B)
		if err != nil {
			return nil, fmt.Errorf("matching service returned invalid id %q", p.B)
		}
		pairs = append(pairs, Pair{A: a, B: b})
	}
	return pairs, nil
}
