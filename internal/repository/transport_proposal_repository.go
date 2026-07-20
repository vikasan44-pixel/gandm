package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type TransportProposalRepository struct {
	db Querier
}

func NewTransportProposalRepository(db Querier) *TransportProposalRepository {
	return &TransportProposalRepository{db: db}
}

const transportProposalColumns = `id, client_id, vehicle_id, carrier_id, cargo_request_id,
	origin_lat, origin_lng, origin_label, origin_source, origin_country, origin_labels,
	destination_lat, destination_lng, destination_label, destination_source, destination_country, destination_labels,
	cargo_name, volume_m3, weight_kg, places_count, pickup_date,
	status, current_price, last_price_by, currency, chat_id, created_at, updated_at`

func scanTransportProposal(row pgx.Row, p *models.TransportProposal) error {
	var originLabels, destinationLabels []byte
	var lastPriceBy *string
	err := row.Scan(
		&p.ID, &p.ClientID, &p.VehicleID, &p.CarrierID, &p.CargoRequestID,
		&p.Origin.Lat, &p.Origin.Lng, &p.Origin.Label, &p.Origin.Source, &p.Origin.Country, &originLabels,
		&p.Destination.Lat, &p.Destination.Lng, &p.Destination.Label, &p.Destination.Source, &p.Destination.Country, &destinationLabels,
		&p.CargoName, &p.VolumeM3, &p.WeightKg, &p.PlacesCount, &p.PickupDate,
		&p.Status, &p.CurrentPrice, &lastPriceBy, &p.Currency, &p.ChatID, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return err
	}
	p.Origin.Labels = scanLabels(originLabels)
	p.Destination.Labels = scanLabels(destinationLabels)
	if lastPriceBy != nil {
		p.LastPriceBy = *lastPriceBy
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Create inserts the proposal and its item rows. The caller owns the
// transaction so both land together.
func (r *TransportProposalRepository) Create(ctx context.Context, p *models.TransportProposal) error {
	const q = `
		INSERT INTO transport_proposals (
			id, client_id, vehicle_id, carrier_id, cargo_request_id,
			origin_lat, origin_lng, origin_label, origin_source, origin_country, origin_labels,
			destination_lat, destination_lng, destination_label, destination_source, destination_country, destination_labels,
			cargo_name, volume_m3, weight_kg, places_count, pickup_date,
			status, current_price, last_price_by, currency, chat_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
	`
	_, err := r.db.Exec(ctx, q,
		p.ID, p.ClientID, p.VehicleID, p.CarrierID, p.CargoRequestID,
		p.Origin.Lat, p.Origin.Lng, p.Origin.Label, p.Origin.Source, p.Origin.Country, marshalLabels(p.Origin.Labels),
		p.Destination.Lat, p.Destination.Lng, p.Destination.Label, p.Destination.Source, p.Destination.Country, marshalLabels(p.Destination.Labels),
		p.CargoName, p.VolumeM3, p.WeightKg, p.PlacesCount, p.PickupDate,
		p.Status, p.CurrentPrice, nilIfEmpty(p.LastPriceBy), p.Currency, p.ChatID, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return err
	}
	for _, item := range p.Items {
		const iq = `INSERT INTO transport_proposal_items (id, proposal_id, position, length_m, width_m, height_m) VALUES ($1, $2, $3, $4, $5, $6)`
		if _, err := r.db.Exec(ctx, iq, uuid.New(), p.ID, item.Position, item.LengthM, item.WidthM, item.HeightM); err != nil {
			return err
		}
	}
	return nil
}

func (r *TransportProposalRepository) getRow(ctx context.Context, q string, id uuid.UUID) (*models.TransportProposal, error) {
	var p models.TransportProposal
	if err := scanTransportProposal(r.db.QueryRow(ctx, q, id), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	items, err := r.loadItems(ctx, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	p.Items = items[id]
	return &p, nil
}

func (r *TransportProposalRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TransportProposal, error) {
	return r.getRow(ctx, `SELECT `+transportProposalColumns+` FROM transport_proposals WHERE id = $1`, id)
}

// GetByIDForUpdate row-locks the proposal so negotiation moves (quote, counter,
// accept) serialize. Only meaningful inside a transaction.
func (r *TransportProposalRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.TransportProposal, error) {
	return r.getRow(ctx, `SELECT `+transportProposalColumns+` FROM transport_proposals WHERE id = $1 FOR UPDATE`, id)
}

func (r *TransportProposalRepository) loadItems(ctx context.Context, proposalIDs []uuid.UUID) (map[uuid.UUID][]models.TransportProposalItem, error) {
	out := make(map[uuid.UUID][]models.TransportProposalItem, len(proposalIDs))
	if len(proposalIDs) == 0 {
		return out, nil
	}
	const q = `SELECT id, proposal_id, position, length_m, width_m, height_m FROM transport_proposal_items WHERE proposal_id = ANY($1) ORDER BY position ASC`
	rows, err := r.db.Query(ctx, q, proposalIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it models.TransportProposalItem
		var pid uuid.UUID
		if err := rows.Scan(&it.ID, &pid, &it.Position, &it.LengthM, &it.WidthM, &it.HeightM); err != nil {
			return nil, err
		}
		out[pid] = append(out[pid], it)
	}
	return out, rows.Err()
}

func (r *TransportProposalRepository) list(ctx context.Context, q string, arg uuid.UUID) ([]models.TransportProposal, error) {
	rows, err := r.db.Query(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.TransportProposal, 0)
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var p models.TransportProposal
		if err := scanTransportProposal(rows, &p); err != nil {
			return nil, err
		}
		items = append(items, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	byID, err := r.loadItems(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Items = byID[items[i].ID]
	}
	return items, nil
}

// ListByClient returns the proposals the client sent (newest first).
func (r *TransportProposalRepository) ListByClient(ctx context.Context, clientID uuid.UUID) ([]models.TransportProposal, error) {
	return r.list(ctx, `SELECT `+transportProposalColumns+` FROM transport_proposals WHERE client_id = $1 ORDER BY created_at DESC`, clientID)
}

// ListByCarrier returns the proposals a carrier received (newest first).
func (r *TransportProposalRepository) ListByCarrier(ctx context.Context, carrierID uuid.UUID) ([]models.TransportProposal, error) {
	return r.list(ctx, `SELECT `+transportProposalColumns+` FROM transport_proposals WHERE carrier_id = $1 ORDER BY created_at DESC`, carrierID)
}

// UpdateNegotiation advances the price ping-pong: new status, latest price and
// who set it.
func (r *TransportProposalRepository) UpdateNegotiation(ctx context.Context, id uuid.UUID, status models.TransportProposalStatus, price float64, by string) error {
	const q = `UPDATE transport_proposals SET status = $2, current_price = $3, last_price_by = $4, updated_at = now() WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, status, price, nilIfEmpty(by))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStatus flips the status without touching the price (accept / reject).
func (r *TransportProposalRepository) SetStatus(ctx context.Context, id uuid.UUID, status models.TransportProposalStatus) error {
	const q = `UPDATE transport_proposals SET status = $2, updated_at = now() WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAgreed closes the deal: status=agreed and the opened chat attached.
func (r *TransportProposalRepository) SetAgreed(ctx context.Context, id, chatID uuid.UUID) error {
	const q = `UPDATE transport_proposals SET status = 'agreed', chat_id = $2, updated_at = now() WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, chatID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
