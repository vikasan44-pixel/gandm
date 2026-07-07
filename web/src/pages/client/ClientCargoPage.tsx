import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAsync } from "../../hooks/useAsync";
import {
  agreeConsolidation,
  createCargo,
  declineConsolidation,
  getCargoOffers,
  getConsolidatedOffers,
  getConsolidation,
  getMyCargo,
  getMyConsolidated,
  selectOffer,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { CargoStatusPill, OfferStatusPill } from "../../components/common/StatusPill";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { ApiError } from "../../api/client";
import { formatDateTime } from "../../utils/date";
import { t } from "../../i18n";
import type {
  AnonymizedOffer,
  CargoRequest,
  ConsolidatedRequest,
  GeoPoint,
  SelectOfferResult,
} from "../../api/types";

export function ClientCargoPage() {
  const cargo = useAsync(getMyCargo, []);
  const consolidated = useAsync(getMyConsolidated, []);
  const [selected, setSelected] = useState<CargoRequest | null>(null);
  const [selectedCons, setSelectedCons] = useState<ConsolidatedRequest | null>(null);
  const [isCreating, setIsCreating] = useState(false);

  function reloadAll() {
    cargo.reload();
    consolidated.reload();
  }

  return (
    <div className="page page--split">
      <div className="page__list">
        <div className="panel__header">
          <h1 className="page__title">{t("nav.myCargo")}</h1>
          <button
            className="btn btn--primary btn--sm"
            onClick={() => setIsCreating((v) => !v)}
          >
            {isCreating ? t("common.cancel") : t("cargo.newRequest")}
          </button>
        </div>

        {isCreating && (
          <NewCargoForm
            onCreated={() => {
              setIsCreating(false);
              cargo.reload();
            }}
          />
        )}

        {consolidated.data && consolidated.data.length > 0 && (
          <section>
            <h2 className="panel__title">{t("consolidation.consolidatedTitle")}</h2>
            <ul className="queue-list">
              {consolidated.data.map((item) => (
                <li
                  key={item.id}
                  className={
                    "queue-list__item" +
                    (selectedCons?.id === item.id ? " queue-list__item--active" : "")
                  }
                  onClick={() => {
                    setSelectedCons(item);
                    setSelected(null);
                  }}
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
                  <span className="pill pill--yellow">{t("consolidation.consolidatedMark")}</span>
                </li>
              ))}
            </ul>
          </section>
        )}

        {cargo.isLoading && <LoadingState />}
        {cargo.error && <ErrorState message={cargo.error} onRetry={cargo.reload} />}
        {cargo.data && cargo.data.length === 0 && (
          <EmptyState message={t("cargo.listEmpty")} />
        )}
        {cargo.data && cargo.data.length > 0 && (
          <ul className="queue-list">
            {cargo.data.map((item) => (
              <li
                key={item.id}
                className={
                  "queue-list__item" +
                  (selected?.id === item.id ? " queue-list__item--active" : "")
                }
                onClick={() => {
                  setSelected(item);
                  setSelectedCons(null);
                }}
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
        {selectedCons ? (
          <ConsolidatedOffersPanel key={selectedCons.id} consolidated={selectedCons} />
        ) : selected ? (
          <OffersPanel key={selected.id} cargo={selected} onSelected={reloadAll} />
        ) : (
          <EmptyState message={t("cargo.selectHint")} />
        )}
      </div>
    </div>
  );
}

function NewCargoForm({ onCreated }: { onCreated: () => void }) {
  const [origin, setOrigin] = useState<GeoPoint | null>(null);
  const [destination, setDestination] = useState<GeoPoint | null>(null);
  const [volume, setVolume] = useState("");
  const [weight, setWeight] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);

    if (!origin || !destination || !origin.label.trim() || !destination.label.trim()) {
      setError(t("geo.pointsRequired"));
      return;
    }
    const volumeNum = Number(volume);
    const weightNum = Number(weight);
    if (!Number.isFinite(volumeNum) || volumeNum <= 0 || !Number.isFinite(weightNum) || weightNum <= 0) {
      setError(t("cargo.numbersPositive"));
      return;
    }

    setIsSubmitting(true);
    try {
      await createCargo({
        origin,
        destination,
        volume_m3: volumeNum,
        weight_kg: weightNum,
        description: description.trim(),
      });
      onCreated();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <form className="inline-form inline-form--stacked" onSubmit={handleSubmit}>
      <GeoPointField title={t("geo.originPoint")} value={origin} onChange={setOrigin} />
      <GeoPointField
        title={t("geo.destinationPoint")}
        value={destination}
        onChange={setDestination}
      />
      <input
        type="number"
        min="0"
        step="0.1"
        placeholder={t("cargo.volume")}
        value={volume}
        onChange={(e) => setVolume(e.target.value)}
      />
      <input
        type="number"
        min="0"
        step="1"
        placeholder={t("cargo.weight")}
        value={weight}
        onChange={(e) => setWeight(e.target.value)}
      />
      <textarea
        placeholder={t("cargo.description")}
        value={description}
        onChange={(e) => setDescription(e.target.value)}
      />
      {error && <div className="form-error">{error}</div>}
      <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
        {isSubmitting ? t("common.loading") : t("cargo.submit")}
      </button>
    </form>
  );
}

// OffersPanel renders offers exactly as the API anonymizes them: number,
// rating, fill percent, price, status. No participant identity exists in
// the response before select, and none may be added here. The contact card
// below appears only from the select response — after the reveal.
function OffersPanel({
  cargo,
  onSelected,
}: {
  cargo: CargoRequest;
  onSelected: () => void;
}) {
  const offers = useAsync(() => getCargoOffers(cargo.id), [cargo.id]);
  const [reveal, setReveal] = useState<SelectOfferResult | null>(null);
  const [selectError, setSelectError] = useState<string | null>(null);
  const [isSelecting, setIsSelecting] = useState(false);
  const canSelect = cargo.status === "open" && !reveal;

  async function handleSelect(offerId: string) {
    if (!window.confirm(t("select.confirm"))) return;
    setSelectError(null);
    setIsSelecting(true);
    try {
      const result = await selectOffer(cargo.id, offerId);
      setReveal(result);
      offers.reload();
      onSelected();
    } catch (err) {
      setSelectError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSelecting(false);
    }
  }

  return (
    <div className="detail-panel">
      <ConsolidationBlock cargo={cargo} onChanged={onSelected} />

      <h2 className="detail-panel__title">{t("cargo.offersTitle")}</h2>
      {offers.isLoading && <LoadingState />}
      {offers.error && <ErrorState message={offers.error} onRetry={offers.reload} />}
      {offers.data && offers.data.length === 0 && (
        <EmptyState message={t("cargo.offersEmpty")} />
      )}
      {offers.data && offers.data.length > 0 && (
        <div className="table-scroll">
          <table className="table table--compact">
            <thead>
              <tr>
                <th>{t("cargo.offerNumber")}</th>
                <th>{t("cargo.rating")}</th>
                <th>{t("cargo.fillPercent")}</th>
                <th>{t("cargo.price")}</th>
                <th>{t("users.columnStatus")}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {offers.data.map((offer) => (
                <tr key={offer.offer_id}>
                  <td>№{offer.offer_number}</td>
                  <td>{offer.rating}</td>
                  <td>{offer.fill_percent != null ? `${offer.fill_percent}%` : "—"}</td>
                  <td>
                    {offer.price.toLocaleString("ru-RU")} {offer.currency}
                  </td>
                  <td>
                    <OfferStatusPill status={offer.status} />
                  </td>
                  <td>
                    {canSelect && offer.status === "submitted" && (
                      <button
                        className="btn btn--primary btn--sm"
                        disabled={isSelecting}
                        onClick={() => handleSelect(offer.offer_id)}
                      >
                        {t("select.button")}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selectError && <div className="form-error">{selectError}</div>}

      {reveal && (
        <div className="contact-card">
          <h3 className="detail-panel__subtitle">{t("select.contactTitle")}</h3>
          <dl className="detail-panel__fields">
            <div>
              <dt>{t("select.company")}</dt>
              <dd>{reveal.contact.company_name || "—"}</dd>
            </div>
            <div>
              <dt>{t("login.email")}</dt>
              <dd>{reveal.contact.email}</dd>
            </div>
            <div>
              <dt>{t("users.phone")}</dt>
              <dd>{reveal.contact.phone || "—"}</dd>
            </div>
            <div>
              <dt>{t("select.revealsUsed")}</dt>
              <dd>
                {reveal.reveals_used} / {reveal.reveals_limit}
              </dd>
            </div>
          </dl>
          <Link className="panel__link" to="/client/chats">
            {t("select.goToChat")}
          </Link>
        </div>
      )}
    </div>
  );
}

// ConsolidationBlock shows the "similar cargo nearby" hint with
// agree/decline. Only size and direction of the other cargo are shown —
// the other client's identity is never present in the API response.
function ConsolidationBlock({
  cargo,
  onChanged,
}: {
  cargo: CargoRequest;
  onChanged: () => void;
}) {
  const consolidation = useAsync(() => getConsolidation(cargo.id), [cargo.id]);
  const [error, setError] = useState<string | null>(null);
  const [isBusy, setIsBusy] = useState(false);

  if (consolidation.isLoading || consolidation.error || !consolidation.data) {
    return null;
  }
  const view = consolidation.data;

  async function respond(action: "agree" | "decline") {
    const confirmText =
      action === "agree" ? t("consolidation.agreeConfirm") : t("consolidation.declineConfirm");
    if (!window.confirm(confirmText)) return;
    setError(null);
    setIsBusy(true);
    try {
      if (action === "agree") {
        await agreeConsolidation(cargo.id, view.suggestion_id);
      } else {
        await declineConsolidation(cargo.id, view.suggestion_id);
      }
      consolidation.reload();
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  return (
    <div className="consolidation-hint">
      <p className="consolidation-hint__text">{t("consolidation.hint")}</p>
      <p className="consolidation-hint__meta">
        {t("consolidation.otherCargo")}: {view.other_volume_m3} м³ · {view.other_weight_kg} кг ·{" "}
        {view.direction_label}
      </p>
      {view.my_side_agreed ? (
        <p className="consolidation-hint__meta">{t("consolidation.waitingOther")}</p>
      ) : (
        <div className="detail-panel__actions">
          <button
            className="btn btn--primary btn--sm"
            disabled={isBusy}
            onClick={() => respond("agree")}
          >
            {t("consolidation.agree")}
          </button>
          <button
            className="btn btn--secondary btn--sm"
            disabled={isBusy}
            onClick={() => respond("decline")}
          >
            {t("consolidation.decline")}
          </button>
        </div>
      )}
      {error && <div className="form-error">{error}</div>}
    </div>
  );
}

function AnonymizedOffersTable({ offers }: { offers: AnonymizedOffer[] }) {
  return (
    <div className="table-scroll">
      <table className="table table--compact">
        <thead>
          <tr>
            <th>{t("cargo.offerNumber")}</th>
            <th>{t("cargo.rating")}</th>
            <th>{t("cargo.fillPercent")}</th>
            <th>{t("cargo.price")}</th>
            <th>{t("users.columnStatus")}</th>
          </tr>
        </thead>
        <tbody>
          {offers.map((offer) => (
            <tr key={offer.offer_id}>
              <td>№{offer.offer_number}</td>
              <td>{offer.rating}</td>
              <td>{offer.fill_percent != null ? `${offer.fill_percent}%` : "—"}</td>
              <td>
                {offer.price.toLocaleString("ru-RU")} {offer.currency}
              </td>
              <td>
                <OfferStatusPill status={offer.status} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ConsolidatedOffersPanel: the shared competition view. Offers are
// anonymized exactly like single-cargo ones; selecting a winner on a
// consolidated request is a later stage (both clients must pick the same
// participant), so there is no select button here yet.
function ConsolidatedOffersPanel({ consolidated }: { consolidated: ConsolidatedRequest }) {
  const offers = useAsync(() => getConsolidatedOffers(consolidated.id), [consolidated.id]);

  return (
    <div className="detail-panel">
      <h2 className="detail-panel__title">
        {consolidated.origin.label} → {consolidated.destination.label}
      </h2>
      <p className="panel__hint">
        {t("consolidation.consolidatedMark")} · {t("consolidation.total")}:{" "}
        {consolidated.total_volume_m3} м³ · {consolidated.total_weight_kg} кг
      </p>

      <h3 className="detail-panel__subtitle">{t("cargo.offersTitle")}</h3>
      {offers.isLoading && <LoadingState />}
      {offers.error && <ErrorState message={offers.error} onRetry={offers.reload} />}
      {offers.data && offers.data.length === 0 && (
        <EmptyState message={t("cargo.offersEmpty")} />
      )}
      {offers.data && offers.data.length > 0 && <AnonymizedOffersTable offers={offers.data} />}
    </div>
  );
}
