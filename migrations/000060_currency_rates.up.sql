-- Official daily exchange rates from the National Bank of Kazakhstan, used
-- ONLY as an approximate "≈ in your currency" display hint. Deals still settle
-- in the currency the parties chose — no conversion is applied to real amounts.
CREATE TABLE currency_rates (
    code         text PRIMARY KEY,          -- ISO-4217 code, e.g. USD
    kzt_per_unit numeric NOT NULL,          -- KZT for one unit of the currency
    rate_date    date    NOT NULL,          -- NBK publication date of the rate
    updated_at   timestamptz NOT NULL DEFAULT now()
);
