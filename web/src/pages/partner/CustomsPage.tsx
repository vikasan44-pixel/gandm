import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import { createCustomsOffer, getCustomsCompetitions } from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { ApiError } from "../../api/client";
import { t } from "../../i18n";
import { formatDateTime } from "../../utils/date";
import { DEFAULT_CURRENCY } from "../../utils/currency";
import type { CustomsCompetition } from "../../api/types";
import { MultilingualCargoCategory } from "../../components/common/MultilingualLabels";
import { CurrencySelect } from "../../components/common/CurrencySelect";
import { Money } from "../../components/common/Money";
import { compactDirectionLabel } from "../../utils/locationLabel";

// CustomsPage — открытые конкурсы на таможенное оформление (ТЗ §10.2).
// Представитель видит направление и наименования грузов — никаких личных
// данных клиентов. Доступ гейтится инструментом manage_customs_docs.
export function CustomsPage() {
  const competitions = useAsync(getCustomsCompetitions, []);

  return (
    <div className="page">
      <h1 className="page__title">{t("customs.title")}</h1>
      <section className="panel">
        {competitions.isLoading && <LoadingState />}
        {competitions.error && (
          <ErrorState message={competitions.error} onRetry={competitions.reload} />
        )}
        {competitions.data && competitions.data.length === 0 && (
          <EmptyState message={t("customs.empty")} />
        )}
        {competitions.data && competitions.data.length > 0 && (
          <ul className="tool-group__list">
            {competitions.data.map((c) => (
              <CompetitionRow
                key={c.consolidated_request_id}
                competition={c}
                onChanged={competitions.reload}
              />
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function CompetitionRow({
  competition,
  onChanged,
}: {
  competition: CustomsCompetition;
  onChanged: () => void;
}) {
  const [price, setPrice] = useState(competition.my_offer ? String(competition.my_offer.price) : "");
  const [currency, setCurrency] = useState<string>(competition.my_offer?.currency || DEFAULT_CURRENCY);
  const [conditions, setConditions] = useState(competition.my_offer?.conditions ?? "");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const priceNum = Number(price);
    if (!Number.isFinite(priceNum) || priceNum <= 0) {
      setError(t("customs.pricePositive"));
      return;
    }
    setIsSubmitting(true);
    try {
      await createCustomsOffer(competition.consolidated_request_id, priceNum, currency, conditions);
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <li className="tool-row" style={{ alignItems: "flex-start" }}>
      <div style={{ flex: 1 }}>
        <div className="tool-row__name">{compactDirectionLabel(competition.direction_label)}</div>
        <div className="tool-row__key">
          {t("customs.totals")}: {competition.total_volume_m3} м³ ·{" "}
          {competition.total_weight_kg.toLocaleString("ru-RU")} кг ·{" "}
          {formatDateTime(competition.created_at)}
        </div>
        <div className="tool-row__key">
          {t("customs.cargoNames")}:
          {(competition.cargo_items ?? []).map((item, index) => (
            <span key={`${item.category}-${index}`} style={{ display: "block", marginTop: 4 }}>
              <MultilingualCargoCategory category={item.category} />
              {item.description ? ` — ${t("cargo.originalDescription")}: ${item.description}` : ""}
            </span>
          ))}
          {(!competition.cargo_items || competition.cargo_items.length === 0) && (competition.cargo_names.join(", ") || "—")}
        </div>

        {competition.my_offer && competition.my_offer.status !== "withdrawn" ? (
          <p className="panel__hint">
            {t("customs.myOffer")}: <Money amount={competition.my_offer.price} currency={competition.my_offer.currency} />
            {competition.my_offer.conditions && ` — ${competition.my_offer.conditions}`}
          </p>
        ) : (
          <form className="inline-form" style={{ marginTop: 8 }} onSubmit={handleSubmit}>
            <input
              type="number"
              min={1}
              step="any"
              value={price}
              onChange={(e) => setPrice(e.target.value)}
              placeholder={t("customs.offerPrice")}
              required
            />
            <CurrencySelect value={currency} onChange={setCurrency} ariaLabel={t("common.currency")} />
            <input
              value={conditions}
              onChange={(e) => setConditions(e.target.value)}
              placeholder={t("customs.offerConditions")}
            />
            <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
              {isSubmitting ? t("common.loading") : t("customs.submitOffer")}
            </button>
          </form>
        )}
        {error && <div className="form-error">{error}</div>}
      </div>
    </li>
  );
}
