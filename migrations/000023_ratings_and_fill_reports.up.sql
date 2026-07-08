-- Stage 6: real ratings (replacing the hardcoded 0) and warehouse fill
-- reports.

-- deal_id deliberately has NO foreign key: a "deal" is either a
-- cargo_requests.id or a consolidated_requests.id — one column can't
-- reference two parents. The service layer validates that the pair of
-- users really are counterparties of that completed deal.
CREATE TABLE ratings (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_id       uuid,
    rated_user_id uuid NOT NULL REFERENCES users (id),
    rater_user_id uuid NOT NULL REFERENCES users (id),
    score         integer NOT NULL CHECK (score BETWEEN 1 AND 5),
    comment       text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (rated_user_id, rater_user_id, deal_id)
);

CREATE INDEX idx_ratings_rated_user_id ON ratings (rated_user_id);

CREATE TABLE warehouse_fill_reports (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expected_fill_percent numeric NOT NULL CHECK (expected_fill_percent >= 0 AND expected_fill_percent <= 100),
    actual_fill_percent   numeric NOT NULL CHECK (actual_fill_percent >= 0 AND actual_fill_percent <= 100),
    photo_url             text,
    report_date           date NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_fill_reports_user_date ON warehouse_fill_reports (user_id, report_date DESC, created_at DESC);
