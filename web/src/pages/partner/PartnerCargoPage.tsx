import { useEffect, useState, type FormEvent, type MouseEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useAsync } from "../../hooks/useAsync";
import {
  createConsolidatedOffer,
  createOffer,
  getAvailableCargo,
  getAvailableConsolidated,
  getMyCargoCompetitionResponses,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { CargoStatusPill } from "../../components/common/StatusPill";
import { MultilingualCargoCategory, MultilingualRoute } from "../../components/common/MultilingualLabels";
import { CurrencySelect } from "../../components/common/CurrencySelect";
import { Pagination, SEARCH_PAGE_SIZE } from "../../components/common/Pagination";
import { ApiError } from "../../api/client";
import { formatDateTime } from "../../utils/date";
import { DEFAULT_CURRENCY } from "../../utils/currency";
import { t } from "../../i18n";
import type { CreateOfferInput } from "../../api/participant";
import type { CargoCompetitionResponse } from "../../api/participant";
import type { CargoRequest, ConsolidatedRequest, GeoPoint } from "../../api/types";

// A participant can offer on a single cargo request or on a consolidated
// one — same form, different endpoint.
type Selection =
  | { kind: "single"; cargo: CargoRequest }
  | { kind: "consolidated"; cons: ConsolidatedRequest };

export function PartnerCargoPage() {
  const cargo = useAsync(getAvailableCargo, []);
  const consolidated = useAsync(getAvailableConsolidated, []);
  const myOffers = useAsync(getMyCargoCompetitionResponses, []);
  const [selection, setSelection] = useState<Selection | null>(null);
  const [from, setFrom] = useState<GeoPoint | null>(null);
  const [to, setTo] = useState<GeoPoint | null>(null);
  const [filteredCargo, setFilteredCargo] = useState<CargoRequest[] | null>(null);
  const [isFiltering, setIsFiltering] = useState(false);
  const [filterError, setFilterError] = useState<string | null>(null);
  const [isMapVisible, setIsMapVisible] = useState(true);
  const [page, setPage] = useState(1);

  const visibleCargo = filteredCargo ?? cargo.data;
  const pageStart = (page - 1) * SEARCH_PAGE_SIZE;
  const pagedCargo = (visibleCargo ?? []).slice(pageStart, pageStart + SEARCH_PAGE_SIZE);

  async function handleDirectionSearch() {
    setFilterError(null);
    setIsFiltering(true);
    setPage(1);
    try {
      setFilteredCargo(await getAvailableCargo(from, to));
      if (from && to) setIsMapVisible(false);
    } catch (err) {
      setFilterError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsFiltering(false);
    }
  }

  useEffect(() => {
    if (selection === null) return;

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setSelection(null);
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [selection]);

  return (
    <div className="page page--cargo-marketplace">
      <div className="page__list">
        <h1 className="page__title">{t("nav.availableCargo")}</h1>

        <section className="panel cargo-search">
          <div className="cargo-search__header">
            <div>
              <h2 className="panel__title">{t("partner.directionSearch")}</h2>
              {!isMapVisible && from && to && (
                <p className="cargo-search__route"><MultilingualRoute origin={from} destination={to} /></p>
              )}
            </div>
            {!isMapVisible && (
              <button className="btn btn--ghost btn--sm" type="button" onClick={() => setIsMapVisible(true)}>
                {t("partner.showMap")}
              </button>
            )}
          </div>
          {isMapVisible && (
            <div className="field-row">
              <GeoPointField title={t("landing.search.from")} value={from} onChange={setFrom} />
              <GeoPointField title={t("landing.search.to")} value={to} onChange={setTo} />
            </div>
          )}
          <button className="btn btn--primary" type="button" disabled={isFiltering} onClick={() => void handleDirectionSearch()}>
            {isFiltering ? t("common.loading") : t("partner.findCargo")}
          </button>
          {filterError && <div className="form-error">{filterError}</div>}
        </section>

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
                      <MultilingualRoute origin={item.origin} destination={item.destination} />
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

        {visibleCargo && visibleCargo.length === 0 && (
          <div className="state state--empty">
            <p>{t("partner.availableEmpty")}</p>
          </div>
        )}

        {visibleCargo && visibleCargo.length > 0 && (
          <section className="search-results-section">
            <div className="landing-search__count">
              {t("landing.search.resultsCargo")}: {visibleCargo.length}
            </div>
            <ul className="queue-list">
              {pagedCargo.map((item) => (
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
                      <MultilingualRoute origin={item.origin} destination={item.destination} />
                    </div>
                    <div className="queue-list__meta">
                      {item.volume_m3} м³ · {item.weight_kg} кг · {formatDateTime(item.created_at)}
                    </div>
                  </div>
                  <CargoStatusPill status={item.status} />
                </li>
              ))}
            </ul>
            <Pagination page={page} totalItems={visibleCargo.length} onPageChange={setPage} />
          </section>
        )}
      </div>

      {selection !== null &&
        createPortal(
          <div
            className="cargo-modal"
            role="presentation"
            onMouseDown={(event: MouseEvent<HTMLDivElement>) => {
              if (event.target === event.currentTarget) setSelection(null);
            }}
          >
            <section
              className="cargo-modal__dialog"
              role="dialog"
              aria-modal="true"
              aria-labelledby="cargo-modal-title"
            >
              <button
                className="cargo-modal__close"
                type="button"
                aria-label={t("common.close")}
                onClick={() => setSelection(null)}
                autoFocus
              >
                ×
              </button>
              {selection.kind === "single" ? (
          // key remounts the panel per target so form state never leaks
          // from one request to another.
          <OfferFormPanel
            key={selection.cargo.id}
            title={<MultilingualRoute origin={selection.cargo.origin} destination={selection.cargo.destination} />}
            volumeM3={selection.cargo.volume_m3}
            weightKg={selection.cargo.weight_kg}
            description={selection.cargo.description}
            category={selection.cargo.category}
            cargo={selection.cargo}
            existingOffer={myOffers.data?.find((item) =>
              item.offer.cargo_request_id === selection.cargo.id && item.offer.status === "submitted"
            )?.offer}
            submitOffer={(input) => createOffer(selection.cargo.id, input)}
            onSubmitted={myOffers.reload}
          />
        ) : (
          <OfferFormPanel
            key={selection.cons.id}
            title={<MultilingualRoute origin={selection.cons.origin} destination={selection.cons.destination} />}
            badge={t("consolidation.consolidatedMark")}
            volumeM3={selection.cons.total_volume_m3}
            weightKg={selection.cons.total_weight_kg}
            existingOffer={myOffers.data?.find((item) =>
              item.offer.consolidated_request_id === selection.cons.id && item.offer.status === "submitted"
            )?.offer}
            submitOffer={(input) => createConsolidatedOffer(selection.cons.id, input)}
            onSubmitted={myOffers.reload}
          />
              )}
            </section>
          </div>,
          document.body,
        )}
    </div>
  );
}

function OfferFormPanel({
  title,
  badge,
  volumeM3,
  weightKg,
  description,
  category,
  cargo,
  existingOffer,
  submitOffer,
  onSubmitted,
}: {
  title: ReactNode;
  badge?: string;
  volumeM3: number;
  weightKg: number;
  description?: string;
  category?: CargoRequest["category"];
  cargo?: CargoRequest;
  existingOffer?: CargoCompetitionResponse["offer"];
  submitOffer: (input: CreateOfferInput) => Promise<unknown>;
  onSubmitted: () => void;
}) {
  const [price, setPrice] = useState(existingOffer ? String(existingOffer.price) : "");
  const [currency, setCurrency] = useState<string>(existingOffer?.currency || DEFAULT_CURRENCY);
  const [conditions, setConditions] = useState(existingOffer?.conditions ?? "");
  const [fillPercent, setFillPercent] = useState(existingOffer?.warehouse_fill_percent == null ? "" : String(existingOffer.warehouse_fill_percent));
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
        currency,
        conditions: conditions.trim(),
        warehouse_fill_percent: fillValue,
      });
      setIsSent(true);
      onSubmitted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="detail-panel">
      <h2 className="detail-panel__title" id="cargo-modal-title">
        {title}
      </h2>
      {badge && <span className="pill pill--yellow">{badge}</span>}
      <dl className="detail-panel__fields">
        {category && (
          <div>
            <dt>{t("cargo.category")}</dt>
            <dd><MultilingualCargoCategory category={category} /></dd>
          </div>
        )}
        <div>
          <dt>{t("cargo.volume")}</dt>
          <dd>{volumeM3}</dd>
        </div>
        <div>
          <dt>{t("cargo.weight")}</dt>
          <dd>{weightKg}</dd>
        </div>
        {cargo && (
          <>
            <div>
              <dt>{t("cargoLogistics.typeLabel")}</dt>
              <dd>
                {cargo.packaging === "bulk" ? t("cargoLogistics.bulkShort") : t("cargoLogistics.packagedShort")}
                {cargo.packaging === "packaged" && cargo.places_count > 0 ? ` · ${t("cargoLogistics.placesShort")}: ${cargo.places_count}` : ""}
              </dd>
            </div>
            {(cargo.items?.length ?? 0) > 0 && (
              <div>
                <dt>{t("cargoLogistics.dimsLabel")}</dt>
                <dd>{cargo.items?.map((it, i) => `${i + 1}) ${it.length_m}×${it.width_m}×${it.height_m}${t("fleet.unitM")}`).join("; ")}</dd>
              </div>
            )}
            <div>
              <dt>{t("cargoLogistics.stacking")}</dt>
              <dd>{cargo.stackable ? t("cargoLogistics.canStack") : t("cargoLogistics.noStack")}</dd>
            </div>
            <div>
              <dt>{t("cargoLogistics.adrLabel")}</dt>
              <dd>{cargo.adr_required ? t("cargoLogistics.adrYes") : t("cargoLogistics.adrNo")}</dd>
            </div>
          </>
        )}
        {description && (
          <div>
            <dt>{t("cargo.originalDescription")}</dt>
            <dd>{description}</dd>
          </div>
        )}
      </dl>

      <h3 className="detail-panel__subtitle">{existingOffer ? t("myCompetitions.myOffer") : t("partner.makeOffer")}</h3>
      {isSent ? (
        <div className="detail-panel__resolved">
          <p>{t("partner.offerSent")}</p>
        </div>
      ) : (
        <form className="inline-form inline-form--stacked" onSubmit={handleSubmit}>
          <div style={{ display: "flex", gap: 6 }}>
            <input
              type="number"
              min="0"
              step="1"
              placeholder={t("partner.offerPrice")}
              value={price}
              onChange={(e) => setPrice(e.target.value)}
              style={{ flex: 1 }}
            />
            <CurrencySelect value={currency} onChange={setCurrency} ariaLabel={t("common.currency")} />
          </div>
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
            {isSubmitting ? t("common.loading") : existingOffer ? t("common.save") : t("partner.offerSubmit")}
          </button>
        </form>
      )}
    </div>
  );
}
