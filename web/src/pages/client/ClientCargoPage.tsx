import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAsync } from "../../hooks/useAsync";
import {
  acceptConsolidated,
  addFavorite,
  agreeConsolidation,
  createCargo,
  declineConsolidation,
  getCargoOffers,
  getConsolidatedOffers,
  getConsolidatedStatus,
  getConsolidation,
  getCustomsOffers,
  getMyCargo,
  getMyConsolidated,
  inviteConsolidated,
  payConsolidated,
  selectConsolidatedOffer,
  selectCustomsOffer,
  selectOffer,
} from "../../api/participant";
import { useAuth } from "../../auth/AuthContext";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { CargoStatusPill, OfferStatusPill } from "../../components/common/StatusPill";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { RatingForm } from "../../components/rating/RatingForm";
import { ApiError } from "../../api/client";
import { formatDateTime } from "../../utils/date";
import { t } from "../../i18n";
import type {
  CargoRequest,
  ConsolidatedRequest,
  CustomsSelectResult,
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
                  <td>{offer.rating !== null ? `★ ${offer.rating}` : "—"}</td>
                  <td>
                    {offer.fill_percent != null ? `${offer.fill_percent}%` : "—"}
                    {offer.latest_fill_actual != null && (
                      <div className="tool-row__key">
                        {t("fill.latestLabel")}: {offer.latest_fill_expected ?? "—"}% /{" "}
                        {offer.latest_fill_actual}%
                      </div>
                    )}
                    {offer.dispatch_threshold_m3 != null && (
                      <div className="tool-row__key">
                        {t("dispatch.offerLabel")} {offer.dispatch_accrued_m3 ?? 0} из{" "}
                        {offer.dispatch_threshold_m3} м³
                      </div>
                    )}
                  </td>
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
          <FavoriteButton participantId={reveal.participant_id} />
          <RatingForm ratedUserId={reveal.participant_id} dealId={cargo.id} />
        </div>
      )}
    </div>
  );
}

// «В избранное» (ТЗ §6.2) — доступно только после раскрытия контакта, то
// есть для реального контрагента по сделке; бэкенд это же и проверяет.
function FavoriteButton({ participantId }: { participantId: string }) {
  const [isDone, setIsDone] = useState(false);
  const [isBusy, setIsBusy] = useState(false);

  async function handleAdd() {
    setIsBusy(true);
    try {
      await addFavorite(participantId);
      setIsDone(true);
    } catch {
      // уже в избранном или сеть — не критично для карточки контакта
      setIsDone(true);
    } finally {
      setIsBusy(false);
    }
  }

  return isDone ? (
    <p className="panel__hint">★ {t("favorites.added")}</p>
  ) : (
    <button className="btn btn--secondary btn--sm" disabled={isBusy} onClick={() => void handleAdd()}>
      ★ {t("favorites.add")}
    </button>
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
      <p className="consolidation-hint__text">
        {view.members_count > 2 ? t("consolidation.groupHint") : t("consolidation.hint")}
      </p>
      {view.members_count > 2 && (
        <p className="consolidation-hint__meta">
          {t("consolidation.groupMeta")}: {view.members_count} · {t("consolidation.agreedMeta")}:{" "}
          {view.agreed_count}
        </p>
      )}
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

// ConsolidatedOffersPanel: the full paid flow. Invite (subscribed
// initiator) → pay/accept (invited client) → mutual client visibility +
// shared chat → joint carrier selection. The carrier stays anonymous until
// BOTH clients picked the same offer.
function ConsolidatedOffersPanel({ consolidated }: { consolidated: ConsolidatedRequest }) {
  const { user } = useAuth();
  const status = useAsync(() => getConsolidatedStatus(consolidated.id), [consolidated.id]);
  const offers = useAsync(() => getConsolidatedOffers(consolidated.id), [consolidated.id]);
  const [error, setError] = useState<string | null>(null);
  const [isBusy, setIsBusy] = useState(false);

  async function run(action: () => Promise<unknown>) {
    setError(null);
    setIsBusy(true);
    try {
      await action();
      status.reload();
      offers.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  function handleSelect(offerId: string) {
    if (!window.confirm(t("consolidation.selectConfirm"))) return;
    void run(() => selectConsolidatedOffer(consolidated.id, offerId));
  }

  const view = status.data;
  const accepted = view?.consolidated.invite_status === "accepted";
  const canChoose = accepted && view?.consolidated.status === "open";

  return (
    <div className="detail-panel">
      <h2 className="detail-panel__title">
        {consolidated.origin.label} → {consolidated.destination.label}
      </h2>
      <p className="panel__hint">
        {t("consolidation.consolidatedMark")} · {t("consolidation.total")}:{" "}
        {consolidated.total_volume_m3} м³ · {consolidated.total_weight_kg} кг
      </p>

      {status.isLoading && <LoadingState />}
      {status.error && <ErrorState message={status.error} onRetry={status.reload} />}

      {view && view.consolidated.invite_status === "none" && (
        <div className="consolidation-hint">
          {user?.has_subscription ? (
            <button
              className="btn btn--primary btn--sm"
              disabled={isBusy}
              onClick={() => void run(() => inviteConsolidated(consolidated.id))}
            >
              {t("consolidation.inviteButton")}
            </button>
          ) : (
            <p className="consolidation-hint__meta">{t("consolidation.inviteNeedsSub")}</p>
          )}
        </div>
      )}

      {view && view.consolidated.invite_status === "invited" && view.am_initiator && (
        <div className="consolidation-hint">
          <p className="consolidation-hint__meta">{t("consolidation.inviteSent")}</p>
        </div>
      )}

      {view && view.consolidated.invite_status === "invited" && view.am_invited && (
        <div className="consolidation-hint">
          <p className="consolidation-hint__text">{t("consolidation.inviteReceived")}</p>
          {user?.has_subscription || view.payment_done ? (
            <>
              {view.payment_done && (
                <p className="consolidation-hint__meta">{t("consolidation.payDone")}</p>
              )}
              <button
                className="btn btn--primary btn--sm"
                disabled={isBusy}
                onClick={() => void run(() => acceptConsolidated(consolidated.id))}
              >
                {t("consolidation.acceptButton")}
              </button>
            </>
          ) : (
            <>
              <p className="consolidation-hint__meta">{t("consolidation.payOrSubscribe")}</p>
              <button
                className="btn btn--primary btn--sm"
                disabled={isBusy}
                onClick={() => void run(() => payConsolidated(consolidated.id))}
              >
                {t("consolidation.payButton")}
              </button>
            </>
          )}
        </div>
      )}

      {view && accepted && (view.counterparts?.length ?? 0) > 0 && (
        <div className="contact-card">
          <h3 className="detail-panel__subtitle">
            {t("consolidation.counterpartTitle")}
            {view.members_count > 2 && (
              <span className="pill pill--neutral" style={{ marginLeft: 8 }}>
                {view.accepted_count} / {view.members_count} {t("consolidation.membersLabel")}
              </span>
            )}
          </h3>
          {(view.counterparts ?? []).map((cp, i) => (
            <dl className="detail-panel__fields" key={i}>
              <div>
                <dt>{t("select.company")}</dt>
                <dd>{cp.company_name || "—"}</dd>
              </div>
              <div>
                <dt>{t("login.email")}</dt>
                <dd>{cp.email}</dd>
              </div>
              <div>
                <dt>{t("users.phone")}</dt>
                <dd>{cp.phone || "—"}</dd>
              </div>
            </dl>
          ))}
          <Link className="panel__link" to="/client/chats">
            {t("consolidation.goToSharedChat")}
          </Link>
        </div>
      )}

      {view && accepted && view.selection_state === "waiting_other" && (
        <p className="panel__hint">{t("consolidation.waitingOtherChoice")}</p>
      )}
      {view && accepted && view.selection_state === "mismatch" && (
        <div className="form-error">{t("consolidation.mismatchChoice")}</div>
      )}
      {view && view.selection_state === "matched" && view.carrier_contact && (
        <div className="contact-card">
          <h3 className="detail-panel__subtitle">{t("consolidation.carrierTitle")}</h3>
          <p className="panel__hint">{t("consolidation.matchedChoice")}</p>
          <dl className="detail-panel__fields">
            <div>
              <dt>{t("select.company")}</dt>
              <dd>{view.carrier_contact.company_name || "—"}</dd>
            </div>
            <div>
              <dt>{t("login.email")}</dt>
              <dd>{view.carrier_contact.email}</dd>
            </div>
            <div>
              <dt>{t("users.phone")}</dt>
              <dd>{view.carrier_contact.phone || "—"}</dd>
            </div>
          </dl>
          {view.carrier_id && (
            <RatingForm ratedUserId={view.carrier_id} dealId={consolidated.id} />
          )}
        </div>
      )}

      {view && view.selection_state === "matched" && (
        <CustomsSection consolidatedId={consolidated.id} />
      )}

      {error && <div className="form-error">{error}</div>}

      <h3 className="detail-panel__subtitle">{t("cargo.offersTitle")}</h3>
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
                  <td>{offer.rating !== null ? `★ ${offer.rating}` : "—"}</td>
                  <td>
                    {offer.fill_percent != null ? `${offer.fill_percent}%` : "—"}
                    {offer.latest_fill_actual != null && (
                      <div className="tool-row__key">
                        {t("fill.latestLabel")}: {offer.latest_fill_expected ?? "—"}% /{" "}
                        {offer.latest_fill_actual}%
                      </div>
                    )}
                    {offer.dispatch_threshold_m3 != null && (
                      <div className="tool-row__key">
                        {t("dispatch.offerLabel")} {offer.dispatch_accrued_m3 ?? 0} из{" "}
                        {offer.dispatch_threshold_m3} м³
                      </div>
                    )}
                  </td>
                  <td>
                    {offer.price.toLocaleString("ru-RU")} {offer.currency}
                  </td>
                  <td>
                    <OfferStatusPill status={offer.status} />
                  </td>
                  <td>
                    {canChoose && (
                      <button
                        className="btn btn--primary btn--sm"
                        disabled={isBusy || view?.my_offer_id === offer.offer_id}
                        onClick={() => handleSelect(offer.offer_id)}
                      >
                        {view?.my_offer_id
                          ? t("consolidation.changeChoice")
                          : t("consolidation.selectCarrier")}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// CustomsSection — конкурс таможенных представителей на закрытой партии
// (ТЗ §10.2/10.3): анонимные предложения, выбор любым из клиентов
// (обсуждение — в общем чате), после выбора контакт раскрыт и представитель
// подключён к общему чату.
function CustomsSection({ consolidatedId }: { consolidatedId: string }) {
  const offers = useAsync(() => getCustomsOffers(consolidatedId), [consolidatedId]);
  const [result, setResult] = useState<CustomsSelectResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isBusy, setIsBusy] = useState(false);

  const selectedOffer = offers.data?.find((o) => o.status === "selected") ?? null;

  async function handleSelect(offerId: string) {
    if (!window.confirm(t("customs.selectConfirm"))) return;
    setError(null);
    setIsBusy(true);
    try {
      const res = await selectCustomsOffer(consolidatedId, offerId);
      setResult(res);
      offers.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  // Контакт уже выбранного представителя доступен через повторный
  // идемпотентный select — подгружаем его, когда выбор сделан ранее.
  async function revealSelected(offerId: string) {
    setError(null);
    setIsBusy(true);
    try {
      const res = await selectCustomsOffer(consolidatedId, offerId);
      setResult(res);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  return (
    <div className="consolidation-hint">
      <h3 className="detail-panel__subtitle">{t("customs.clientSectionTitle")}</h3>

      {offers.isLoading && <LoadingState />}
      {offers.error && <ErrorState message={offers.error} onRetry={offers.reload} />}
      {offers.data && offers.data.length === 0 && (
        <p className="panel__hint">{t("customs.clientEmpty")}</p>
      )}

      {offers.data && offers.data.length > 0 && !selectedOffer && (
        <>
          <p className="panel__hint">{t("customs.clientHint")}</p>
          <div className="table-scroll">
            <table className="table table--compact">
              <thead>
                <tr>
                  <th>{t("cargo.offerNumber")}</th>
                  <th>{t("cargo.rating")}</th>
                  <th>{t("cargo.price")}</th>
                  <th>{t("customs.offerConditions")}</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {offers.data.map((offer) => (
                  <tr key={offer.offer_id}>
                    <td>№{offer.offer_number}</td>
                    <td>{offer.rating !== null ? `★ ${offer.rating}` : "—"}</td>
                    <td>
                      {offer.price.toLocaleString("ru-RU")} {offer.currency}
                    </td>
                    <td>{offer.conditions || "—"}</td>
                    <td>
                      <button
                        className="btn btn--primary btn--sm"
                        disabled={isBusy}
                        onClick={() => void handleSelect(offer.offer_id)}
                      >
                        {t("customs.select")}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {selectedOffer && !result && (
        <button
          className="btn btn--secondary btn--sm"
          disabled={isBusy}
          onClick={() => void revealSelected(selectedOffer.offer_id)}
        >
          {t("customs.selectedTitle")} →
        </button>
      )}

      {result && (
        <div className="contact-card">
          <h3 className="detail-panel__subtitle">{t("customs.selectedTitle")}</h3>
          <p className="panel__hint">{t("customs.selectedNote")}</p>
          <dl className="detail-panel__fields">
            <div>
              <dt>{t("select.company")}</dt>
              <dd>{result.contact.company_name || "—"}</dd>
            </div>
            <div>
              <dt>{t("login.email")}</dt>
              <dd>{result.contact.email}</dd>
            </div>
            <div>
              <dt>{t("users.phone")}</dt>
              <dd>{result.contact.phone || "—"}</dd>
            </div>
          </dl>
          <RatingForm ratedUserId={result.customs_rep_id} dealId={consolidatedId} />
        </div>
      )}

      {error && <div className="form-error">{error}</div>}
    </div>
  );
}
