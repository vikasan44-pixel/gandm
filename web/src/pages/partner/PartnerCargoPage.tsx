import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAsync } from "../../hooks/useAsync";
import {
  createConsolidatedOffer,
  createOffer,
  getAvailableCargo,
  getAvailableConsolidated,
  getRoutes,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { CargoStatusPill } from "../../components/common/StatusPill";
import { ApiError } from "../../api/client";
import { formatDateTime } from "../../utils/date";
import { t } from "../../i18n";
import type { CreateOfferInput } from "../../api/participant";
import type { CargoRequest, ConsolidatedRequest } from "../../api/types";

// A participant can offer on a single cargo request or on a consolidated
// one — same form, different endpoint.
type Selection =
  | { kind: "single"; cargo: CargoRequest }
  | { kind: "consolidated"; cons: ConsolidatedRequest };

export function PartnerCargoPage() {
  const cargo = useAsync(getAvailableCargo, []);
  const consolidated = useAsync(getAvailableConsolidated, []);
  // Routes are fetched only to tell "нет направлений" apart from "по вашим
  // направлениям пока нет грузов" — both come back as an empty cargo list.
  const routes = useAsync(getRoutes, []);
  const [selection, setSelection] = useState<Selection | null>(null);

  const noRoutes = routes.data !== null && routes.data.length === 0;

  return (
    <div className="page page--split">
      <div className="page__list">
        <h1 className="page__title">{t("nav.availableCargo")}</h1>

        {consolidated.data && consolidated.data.length > 0 && (
          <section>
            <h2 className="panel__title">{t("consolidation.consolidatedTitle")}</h2>
            <ul className="queue-list">
              {consolidated.data.map((item) => (
                <li
                  key={item.id}
                  className={
                    "queue-list__item" +
                    (selection?.kind === "consolidated" && selection.cons.id === item.id
                      ? " queue-list__item--active"
                      : "")
                  }
                  onClick={() => setSelection({ kind: "consolidated", cons: item })}
                >
                  <div className="queue-list__main">
                    <div className="queue-list__name">
                      {item.origin.label} → {item.destination.label}
                    </div>
                    <div className="queue-list__meta">
                      {t("consolidation.total")}: {item.total_volume_m3} м³ ·{" "}
                      {item.total_weight_kg} кг
                    </div>
                  </div>
                  <span className="pill pill--yellow">
                    {t("consolidation.consolidatedMark")}
                  </span>
                </li>
              ))}
            </ul>
          </section>
        )}

        {cargo.isLoading && <LoadingState />}
        {cargo.error && <ErrorState message={cargo.error} onRetry={cargo.reload} />}

        {cargo.data && cargo.data.length === 0 && (
          <div className="state state--empty">
            <p>{noRoutes ? t("partner.noRoutesHint") : t("partner.availableEmpty")}</p>
            {noRoutes && (
              <Link className="panel__link" to="/partner/routes">
                {t("partner.goToRoutes")}
              </Link>
            )}
          </div>
        )}

        {cargo.data && cargo.data.length > 0 && (
          <ul className="queue-list">
            {cargo.data.map((item) => (
              <li
                key={item.id}
                className={
                  "queue-list__item" +
                  (selection?.kind === "single" && selection.cargo.id === item.id
                    ? " queue-list__item--active"
                    : "")
                }
                onClick={() => setSelection({ kind: "single", cargo: item })}
              >
                <div className="queue-list__main">
                  <div className="queue-list__name">
                    {item.origin.label} → {item.destination.label}
                  </div>
                  <div className="queue-list__meta">
                    {item.volume_m3} м³ · {item.weight_kg} кг · {formatDateTime(item.created_at)}
                  </div>
                </div>
                <CargoStatusPill status={item.status} />
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="page__detail">
        {selection === null ? (
          <EmptyState message={t("partner.selectCargoHint")} />
        ) : selection.kind === "single" ? (
          // key remounts the panel per target so form state never leaks
          // from one request to another.
          <OfferFormPanel
            key={selection.cargo.id}
            title={`${selection.cargo.origin.label} → ${selection.cargo.destination.label}`}
            volumeM3={selection.cargo.volume_m3}
            weightKg={selection.cargo.weight_kg}
            description={selection.cargo.description}
            submitOffer={(input) => createOffer(selection.cargo.id, input)}
          />
        ) : (
          <OfferFormPanel
            key={selection.cons.id}
            title={`${selection.cons.origin.label} → ${selection.cons.destination.label}`}
            badge={t("consolidation.consolidatedMark")}
            volumeM3={selection.cons.total_volume_m3}
            weightKg={selection.cons.total_weight_kg}
            submitOffer={(input) => createConsolidatedOffer(selection.cons.id, input)}
          />
        )}
      </div>
    </div>
  );
}

function OfferFormPanel({
  title,
  badge,
  volumeM3,
  weightKg,
  description,
  submitOffer,
}: {
  title: string;
  badge?: string;
  volumeM3: number;
  weightKg: number;
  description?: string;
  submitOffer: (input: CreateOfferInput) => Promise<unknown>;
}) {
  const [price, setPrice] = useState("");
  const [conditions, setConditions] = useState("");
  const [fillPercent, setFillPercent] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSent, setIsSent] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);

    const priceNum = Number(price);
    if (!Number.isFinite(priceNum) || priceNum <= 0) {
      setError(t("partner.offerPricePositive"));
      return;
    }
    let fillValue: number | null = null;
    if (fillPercent.trim() !== "") {
      const parsed = Number(fillPercent);
      if (!Number.isFinite(parsed) || parsed < 0 || parsed > 100) {
        setError(t("partner.offerFillRange"));
        return;
      }
      fillValue = parsed;
    }

    setIsSubmitting(true);
    try {
      await submitOffer({
        price: priceNum,
        conditions: conditions.trim(),
        warehouse_fill_percent: fillValue,
      });
      setIsSent(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="detail-panel">
      <h2 className="detail-panel__title">{title}</h2>
      {badge && <span className="pill pill--yellow">{badge}</span>}
      <dl className="detail-panel__fields">
        <div>
          <dt>{t("cargo.volume")}</dt>
          <dd>{volumeM3}</dd>
        </div>
        <div>
          <dt>{t("cargo.weight")}</dt>
          <dd>{weightKg}</dd>
        </div>
        {description && (
          <div>
            <dt>{t("cargo.description")}</dt>
            <dd>{description}</dd>
          </div>
        )}
      </dl>

      <h3 className="detail-panel__subtitle">{t("partner.makeOffer")}</h3>
      {isSent ? (
        <div className="detail-panel__resolved">
          <p>{t("partner.offerSent")}</p>
        </div>
      ) : (
        <form className="inline-form inline-form--stacked" onSubmit={handleSubmit}>
          <input
            type="number"
            min="0"
            step="1"
            placeholder={t("partner.offerPrice")}
            value={price}
            onChange={(e) => setPrice(e.target.value)}
          />
          <textarea
            placeholder={t("partner.offerConditions")}
            value={conditions}
            onChange={(e) => setConditions(e.target.value)}
          />
          <input
            type="number"
            min="0"
            max="100"
            step="1"
            placeholder={t("partner.offerFill")}
            value={fillPercent}
            onChange={(e) => setFillPercent(e.target.value)}
          />
          {error && <div className="form-error">{error}</div>}
          <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("common.loading") : t("partner.offerSubmit")}
          </button>
        </form>
      )}
    </div>
  );
}
