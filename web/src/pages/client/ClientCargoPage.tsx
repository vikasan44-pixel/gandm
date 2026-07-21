import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useAsync } from "../../hooks/useAsync";
import {
  acceptConsolidated,
  addFavorite,
  agreeConsolidation,
  cancelCargo,
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
  selectWarehouseOffer,
  getWarehouseOffersForCargo,
  getWarehouseOffersForConsolidated,
  selectWarehouseOfferForConsolidated,
  getMatchingConsolidations,
  joinConsolidation,
  updateCargo,
} from "../../api/participant";
import type { WarehouseOfferView } from "../../api/types";
import { useAuth } from "../../auth/AuthContext";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { Pagination, SEARCH_PAGE_SIZE } from "../../components/common/Pagination";
import { DetailModal } from "../../components/common/DetailModal";
import { useConfirm } from "../../components/common/ConfirmDialog";
import { MultilingualCargoCategory, MultilingualRoute } from "../../components/common/MultilingualLabels";
import { CargoStatusPill, OfferStatusPill } from "../../components/common/StatusPill";
import { Money } from "../../components/common/Money";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { RatingForm } from "../../components/rating/RatingForm";
import { ApiError } from "../../api/client";
import { formatDateTime } from "../../utils/date";
import { t } from "../../i18n";
import { CARGO_CATEGORIES, cargoCategoryLabel } from "../../utils/cargoCategories";
import { pickLabel } from "../../utils/geoLabel";
import { compactDirectionLabel } from "../../utils/locationLabel";
import type {
  AnonymizedOffer,
  CargoRequest,
  ConsolidatedRequest,
  CustomsSelectResult,
  GeoPoint,
  SelectOfferResult,
} from "../../api/types";

export function ClientCargoPage() {
	const [page, setPage] = useState(1);
	const cargo = useAsync(() => getMyCargo(page), [page]);
  const consolidated = useAsync(getMyConsolidated, []);
  const [selected, setSelected] = useState<CargoRequest | null>(null);
  const [selectedCons, setSelectedCons] = useState<ConsolidatedRequest | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [editingCargo, setEditingCargo] = useState<CargoRequest | null>(null);

  function reloadAll() {
    cargo.reload();
    consolidated.reload();
  }

  return (
    <div className="page">
      <div className="page__list">
        <div className="panel__header">
          <h1 className="page__title">{t("nav.myCargo")}</h1>
          <button
            className="btn btn--primary btn--sm"
            onClick={() => setIsCreating(true)}
          >
            {t("cargo.newRequest")}
          </button>
        </div>

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
                      <MultilingualRoute origin={item.origin} destination={item.destination} />
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
		{cargo.data && cargo.data.items.length === 0 && (
          <EmptyState message={t("cargo.listEmpty")} />
        )}
		{cargo.data && cargo.data.items.length > 0 && (
          <ul className="queue-list">
			{cargo.data.items.map((item) => (
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
		)}
		{cargo.data && <Pagination page={page} pageSize={SEARCH_PAGE_SIZE} totalItems={cargo.data.total} onPageChange={setPage} />}
      </div>

      {isCreating && (
        <DetailModal onClose={() => setIsCreating(false)} wide>
          <NewCargoForm onCreated={() => { setIsCreating(false); cargo.reload(); }} />
        </DetailModal>
      )}

      {editingCargo && (
        <DetailModal onClose={() => setEditingCargo(null)} wide>
          <NewCargoForm
            initial={editingCargo}
            onCreated={() => { setEditingCargo(null); reloadAll(); }}
          />
        </DetailModal>
      )}

      {(selectedCons || selected) && (
        <DetailModal onClose={() => { setSelected(null); setSelectedCons(null); }} wide>
          {selectedCons ? (
            <ConsolidatedOffersPanel key={selectedCons.id} consolidated={selectedCons} />
          ) : selected ? (
            <>
              <OffersPanel
                key={selected.id}
                cargo={selected}
                onSelected={reloadAll}
                onEdit={() => {
                  setEditingCargo(selected);
                  setSelected(null);
                }}
                onCancelled={() => {
                  setSelected(null);
                  reloadAll();
                }}
              />
              <WarehouseOffersPanel key={`wh-${selected.id}`} kind="cargo" id={selected.id} />
              <LateJoinPanel key={`join-${selected.id}`} cargo={selected} onJoined={reloadAll} />
            </>
          ) : null}
        </DetailModal>
      )}
    </div>
  );
}

function NewCargoForm({ onCreated, initial }: { onCreated: () => void; initial?: CargoRequest }) {
  const [origin, setOrigin] = useState<GeoPoint | null>(initial?.origin ?? null);
  const [destination, setDestination] = useState<GeoPoint | null>(initial?.destination ?? null);
  const [volume, setVolume] = useState(initial ? String(initial.volume_m3) : "");
  const [weight, setWeight] = useState(initial ? String(initial.weight_kg) : "");
  const [category, setCategory] = useState<CargoRequest["category"] | "">(initial?.category ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [packaging, setPackaging] = useState<"packaged" | "bulk">(initial?.packaging ?? "packaged");
  const [stackable, setStackable] = useState<boolean>(initial?.stackable ?? true);
  const [adrRequired, setAdrRequired] = useState<boolean>(initial?.adr_required ?? false);
  const [places, setPlaces] = useState<{ length_m: string; width_m: string; height_m: string }[]>(
    initial?.items?.length
      ? initial.items.map((it) => ({ length_m: String(it.length_m), width_m: String(it.width_m), height_m: String(it.height_m) }))
      : [{ length_m: "", width_m: "", height_m: "" }]
  );
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  function setPlace(i: number, patch: Partial<{ length_m: string; width_m: string; height_m: string }>) {
    setPlaces((prev) => prev.map((p, idx) => (idx === i ? { ...p, ...patch } : p)));
  }

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
    if (!category) {
      setError(t("cargo.categoryRequired"));
      return;
    }
    if (category === "other" && !description.trim()) {
      setError(t("cargo.otherNameRequired"));
      return;
    }

    setIsSubmitting(true);
    try {
      const items =
        packaging === "packaged"
          ? places.map((p) => ({
              length_m: Number(p.length_m) || 0,
              width_m: Number(p.width_m) || 0,
              height_m: Number(p.height_m) || 0,
            }))
          : [];
      const input = {
        origin,
        destination,
        volume_m3: volumeNum,
        weight_kg: weightNum,
        category,
        description: description.trim(),
        packaging,
        places_count: packaging === "packaged" ? items.length : 0,
        stackable,
        adr_required: adrRequired,
        items,
      };
      if (initial) await updateCargo(initial.id, input);
      else await createCargo(input);
      onCreated();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <form className="inline-form inline-form--stacked" onSubmit={handleSubmit}>
      <h2 className="detail-panel__title">{initial ? t("cargo.editRequest") : t("cargo.newRequest")}</h2>
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
      <label className="field">
        <span className="field__label">{t("cargo.category")}</span>
        <select value={category} onChange={(e) => { const v = e.target.value as CargoRequest["category"] | ""; setCategory(v); if (v === "dangerous_goods") setAdrRequired(true); }}>
          <option value="">— {t("cargo.categoryRequired")} —</option>
          {CARGO_CATEGORIES.map((item) => (
            <option key={item} value={item}>{cargoCategoryLabel(item)}</option>
          ))}
        </select>
      </label>
      {category && <div className="cargo-category-preview"><MultilingualCargoCategory category={category} /></div>}
      <textarea
        placeholder={category === "other" ? t("cargo.otherName") : t("cargo.additionalDescription")}
        value={description}
        onChange={(e) => setDescription(e.target.value)}
      />

      <label className="field">
        <span className="field__label">{t("cargoLogistics.type")}</span>
        <select className="field__input" value={packaging} onChange={(e) => setPackaging(e.target.value as "packaged" | "bulk")}>
          <option value="packaged">{t("cargoLogistics.packaged")}</option>
          <option value="bulk">{t("cargoLogistics.bulk")}</option>
        </select>
      </label>

      {packaging === "packaged" && (
        <div className="field">
          <span className="field__label">{t("cargoLogistics.placesCount")}: {places.length} ({t("cargoLogistics.dimsHint")})</span>
          {places.map((p, i) => (
            <div className="form-grid form-grid--3" key={i} style={{ marginBottom: 6 }}>
              <input className="field__input" type="number" placeholder={t("cargoLogistics.length")} value={p.length_m} onChange={(e) => setPlace(i, { length_m: e.target.value })} />
              <input className="field__input" type="number" placeholder={t("cargoLogistics.width")} value={p.width_m} onChange={(e) => setPlace(i, { width_m: e.target.value })} />
              <input className="field__input" type="number" placeholder={t("cargoLogistics.height")} value={p.height_m} onChange={(e) => setPlace(i, { height_m: e.target.value })} />
            </div>
          ))}
          <div className="btn-group">
            <button type="button" className="btn btn--ghost btn--sm" onClick={() => setPlaces((p) => [...p, { length_m: "", width_m: "", height_m: "" }])}>{t("cargoLogistics.addPlace")}</button>
            {places.length > 1 && (
              <button type="button" className="btn btn--ghost btn--sm" onClick={() => setPlaces((p) => p.slice(0, -1))}>{t("cargoLogistics.removePlace")}</button>
            )}
          </div>
        </div>
      )}

      <label style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <input type="checkbox" checked={stackable} onChange={(e) => setStackable(e.target.checked)} />
        <span>{t("cargoLogistics.stackable")}</span>
      </label>
      <span className="field__hint">{t("cargoLogistics.stackableHint")}</span>

      <label style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <input type="checkbox" checked={adrRequired} onChange={(e) => setAdrRequired(e.target.checked)} />
        <span>{t("cargoLogistics.adr")}</span>
      </label>
      <span className="field__hint">{t("cargoLogistics.adrHint")}</span>

      {error && <div className="form-error">{error}</div>}
      <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
        {isSubmitting ? t("common.loading") : initial ? t("common.save") : t("cargo.submit")}
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
  onEdit,
  onCancelled,
}: {
  cargo: CargoRequest;
  onSelected: () => void;
  onEdit: () => void;
  onCancelled: () => void;
}) {
  const confirm = useConfirm();
  const offers = useAsync(() => getCargoOffers(cargo.id), [cargo.id]);
  const [reveal, setReveal] = useState<SelectOfferResult | null>(null);
  const [selectError, setSelectError] = useState<string | null>(null);
  const [isSelecting, setIsSelecting] = useState(false);
  const [isCancelling, setIsCancelling] = useState(false);
  const canSelect = cargo.status === "open" && !reveal;

  async function handleSelect(offerId: string) {
    if (!await confirm({ message: t("select.confirm"), confirmLabel: t("select.button"), danger: false })) return;
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

  async function handleCancel() {
    if (!await confirm({ message: t("cargo.cancelConfirm"), confirmLabel: t("cargo.cancelRequest") })) return;
    setSelectError(null);
    setIsCancelling(true);
    try {
      await cancelCargo(cargo.id);
      onCancelled();
    } catch (err) {
      setSelectError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsCancelling(false);
    }
  }

  return (
    <div className="detail-panel">
      <div className="detail-panel__heading-row">
        <div>
          <h2 className="detail-panel__title"><MultilingualRoute origin={cargo.origin} destination={cargo.destination} /></h2>
          <p className="panel__hint">{cargo.volume_m3} м³ · {cargo.weight_kg.toLocaleString("ru-RU")} кг</p>
          <p><MultilingualCargoCategory category={cargo.category} /></p>
          <p className="panel__hint">{pickLabel(cargo.origin.labels, cargo.origin.label)} → {pickLabel(cargo.destination.labels, cargo.destination.label)}</p>
          {cargo.description && <p><strong>{t("cargo.originalDescription")}:</strong> {cargo.description}</p>}
        </div>
        {cargo.status === "open" && (
          <div className="competition-response__actions">
            <button className="btn btn--secondary btn--sm" type="button" onClick={onEdit}>{t("cargo.editRequest")}</button>
            <button className="btn btn--ghost btn--sm" type="button" disabled={isCancelling} onClick={() => void handleCancel()}>{t("cargo.cancelRequest")}</button>
          </div>
        )}
      </div>
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
                    <DispatchOfferProgress offer={offer} />
                  </td>
                  <td>
                    <Money amount={offer.price} currency={offer.currency} />
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
          <Link className="panel__link" to="/app/chats">
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
  const confirm = useConfirm();
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
    if (!await confirm({
      message: confirmText,
      confirmLabel: action === "agree" ? t("consolidation.agree") : t("consolidation.decline"),
      danger: action !== "agree",
    })) return;
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
        {compactDirectionLabel(view.direction_label)}
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
  const confirm = useConfirm();
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

  async function handleSelect(offerId: string) {
    if (!await confirm({ message: t("consolidation.selectConfirm"), confirmLabel: t("consolidation.selectCarrier"), danger: false })) return;
    void run(() => selectConsolidatedOffer(consolidated.id, offerId));
  }

  const view = status.data;
  const accepted = view?.consolidated.invite_status === "accepted";
  const canChoose = accepted && view?.consolidated.status === "open";

  return (
    <div className="detail-panel">
      <h2 className="detail-panel__title">
        <MultilingualRoute origin={consolidated.origin} destination={consolidated.destination} />
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
          <Link className="panel__link" to="/app/chats">
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
                    <DispatchOfferProgress offer={offer} />
                  </td>
                  <td>
                    <Money amount={offer.price} currency={offer.currency} />
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
      <WarehouseOffersPanel kind="consolidated" id={consolidated.id} />
    </div>
  );
}

// CustomsSection — конкурс таможенных представителей на закрытой партии
// (ТЗ §10.2/10.3): анонимные предложения, выбор любым из клиентов
// (обсуждение — в общем чате), после выбора контакт раскрыт и представитель
// подключён к общему чату.
function CustomsSection({ consolidatedId }: { consolidatedId: string }) {
  const confirm = useConfirm();
  const offers = useAsync(() => getCustomsOffers(consolidatedId), [consolidatedId]);
  const [result, setResult] = useState<CustomsSelectResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isBusy, setIsBusy] = useState(false);

  const selectedOffer = offers.data?.find((o) => o.status === "selected") ?? null;

  async function handleSelect(offerId: string) {
    if (!await confirm({ message: t("customs.selectConfirm"), confirmLabel: t("customs.select"), danger: false })) return;
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
                      <Money amount={offer.price} currency={offer.currency} />
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

function DispatchOfferProgress({ offer }: { offer: AnonymizedOffer }) {
  if (offer.dispatch_threshold_m3 == null) return null;
  const target = offer.dispatch_threshold_m3;
  const accrued = offer.dispatch_accrued_m3 ?? 0;
  const remaining = offer.dispatch_remaining_m3 ?? Math.max(0, target - accrued);
  const percent = target > 0 ? Math.min(100, Math.max(0, (accrued / target) * 100)) : 0;
  return (
    <div className="dispatch-offer-progress">
      <strong>{remaining > 0 ? `${t("dispatch.lookingFor")} ${formatDispatchVolume(remaining)} м³` : t("dispatch.readyToSend")}</strong>
      <span>{t("dispatch.accruedShort")}: {formatDispatchVolume(accrued)} из {formatDispatchVolume(target)} м³</span>
      <span className="dispatch-offer-progress__bar"><i style={{ width: `${percent}%` }} /></span>
      {offer.dispatch_date && <span>{t("dispatch.estimatedDate")}: {new Intl.DateTimeFormat("ru-RU").format(new Date(offer.dispatch_date))}</span>}
    </div>
  );
}

function formatDispatchVolume(value: number) {
  return value.toLocaleString("ru-RU", { maximumFractionDigits: 2 });
}

// WarehouseOffersPanel — предложения складов на груз клиента. Контакты склада
// приходят только после выбора (по подписке; сейчас гейт-заглушка отключена).
function WarehouseOffersPanel({ kind, id }: { kind: "cargo" | "consolidated"; id: string }) {
  const offers = useAsync(
    () => (kind === "cargo" ? getWarehouseOffersForCargo(id) : getWarehouseOffersForConsolidated(id)),
    [kind, id]
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function pick(offerId: string) {
    setError(null);
    try {
      setBusy(true);
      if (kind === "cargo") await selectWarehouseOffer(id, offerId);
      else await selectWarehouseOfferForConsolidated(id, offerId);
      offers.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setBusy(false);
    }
  }

  if (offers.isLoading || !offers.data || offers.data.length === 0) return null;

  return (
    <section className="detail-panel" style={{ marginTop: 16 }}>
      <h3 className="panel__title">{t("warehouseOffers.title")}</h3>
      {error && <div className="form-error">{error}</div>}
      <ul className="landing-search__list">
        {offers.data.map((o: WarehouseOfferView) => (
          <li className="public-card" key={o.id}>
            <div className="public-card__route">{o.warehouse_name}</div>
            <div className="public-card__meta">
              {pickLabel(o.warehouse_address.labels, o.warehouse_address.label)} · {t("warehouseOffers.coveredShort")} {o.covered_area_m2} {t("fleet.unitM2")} · {t("warehouseOffers.upToKg")} {o.max_weight_kg.toLocaleString()} {t("fleet.unitKg")}
            </div>
            <div className="public-card__meta">
              <strong><Money amount={o.price} currency={o.currency} /></strong>{o.conditions ? ` · ${o.conditions}` : ""}
            </div>
            {o.status === "selected" && o.contact ? (
              <div className="public-card__trip-plan">
                <strong>{t("warehouseOffers.chosenContacts")}</strong>
                <span>{o.contact.warehouse_name} · {o.contact.contact_name}</span>
                <small>{o.contact.contact_phone} · {o.contact.email}</small>
                <Link className="btn btn--ghost btn--sm" to="/app/chats">{t("warehouseOffers.openChat")}</Link>
              </div>
            ) : o.status === "submitted" ? (
              <button className="btn btn--primary btn--sm" type="button" disabled={busy} onClick={() => void pick(o.id)}>
                {t("warehouseOffers.choose")}
              </button>
            ) : (
              <span className="pill pill--neutral">{t("warehouseOffers.rejected")}</span>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

// LateJoinPanel — если груз клиента подходит к уже созданному объединению
// (окно закрылось без него), даём присоединиться (Фаза 3b).
function LateJoinPanel({ cargo, onJoined }: { cargo: CargoRequest; onJoined: () => void }) {
  const matches = useAsync(
    () => (cargo.status === "open" ? getMatchingConsolidations(cargo.id) : Promise.resolve([])),
    [cargo.id, cargo.status]
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function join(consId: string) {
    setError(null);
    try {
      setBusy(true);
      await joinConsolidation(consId, cargo.id);
      matches.reload();
      onJoined();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setBusy(false);
    }
  }

  if (matches.isLoading || !matches.data || matches.data.length === 0) return null;

  return (
    <section className="detail-panel" style={{ marginTop: 16 }}>
      <h3 className="panel__title">{t("lateJoin.title")}</h3>
      {error && <div className="form-error">{error}</div>}
      <ul className="landing-search__list">
        {matches.data.map((c) => (
          <li className="public-card" key={c.id}>
            <div className="public-card__route">
              <MultilingualRoute origin={c.origin} destination={c.destination} />
            </div>
            <div className="public-card__meta">{t("lateJoin.consolidatedCargo")}: {c.total_volume_m3} {t("fleet.unitM3")} · {c.total_weight_kg} {t("fleet.unitKg")}</div>
            <button className="btn btn--primary btn--sm" type="button" disabled={busy} onClick={() => void join(c.id)}>
              {t("lateJoin.join")}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
