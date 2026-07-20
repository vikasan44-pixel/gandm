-- Phase 3b: a consolidation suggestion resolves after a fixed window (3 hours)
-- even if not everyone answered — whoever agreed (≥2) is merged, the rest are
-- dropped and may late-join afterwards.
ALTER TABLE consolidation_suggestions ADD COLUMN resolves_at timestamptz;

-- Preserve the original response window for historical suggestions instead
-- of granting every old pending row a fresh three hours at migration time.
UPDATE consolidation_suggestions
SET resolves_at = created_at + interval '3 hours';

ALTER TABLE consolidation_suggestions
    ALTER COLUMN resolves_at SET NOT NULL,
    ALTER COLUMN resolves_at SET DEFAULT now() + interval '3 hours';

-- Sweep index: find suggestions whose window has elapsed but are still open.
CREATE INDEX idx_consolidation_suggestions_resolve
    ON consolidation_suggestions (resolves_at)
    WHERE status = 'suggested';
