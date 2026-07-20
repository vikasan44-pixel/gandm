import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import {
  cancelDriverCompetition,
  createDriverBid,
  createDriverCompetition,
  getMyDriverCompetitions,
  getOpenDriverCompetitions,
  getRoutes,
  selectDriverBid,
  updateDriverCompetition,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { useConfirm } from "../../components/common/ConfirmDialog";
import { ApiError } from "../../api/client";
import { t } from "../../i18n";
import { formatDateTime } from "../../utils/date";
import { DEFAULT_CURRENCY } from "../../utils/currency";
import type {
  DriverCompetitionView,
  DriverSelectResult,
  OpenDriverCompetition,
  ParticipantRoute,
} from "../../api/types";
import { cityLabel, compactDirectionLabel } from "../../utils/locationLabel";
import { CurrencySelect } from "../../components/common/CurrencySelect";
import { Money } from "../../components/common/Money";

// DriverCompetitionsPage объединяет обе стороны конкурса (ТЗ §11.4):
// секция склада (manage_warehouse_slots) и секция водителя (manage_fleet).
// Каждая секция скрывается, если бэкенд отвечает 403 tool_required —
// участник без соответствующего инструмента её просто не видит.
export function DriverCompetitionsPage() {
  const mine = useAsync(getMyDriverCompetitions, []);
  const open = useAsync(getOpenDriverCompetitions, []);

  const showWarehouse = mine.data !== null;
  const showDriver = open.data !== null;
  const bothForbidden = mine.error !== null && open.error !== null;

  return (
    <div className="page">
      <h1 className="page__title">{t("driverComp.title")}</h1>

      {(mine.isLoading || open.isLoading) && <LoadingState />}
      {bothForbidden && <ErrorState message={mine.error ?? ""} onRetry={mine.reload} />}

      {showWarehouse && (
        <WarehouseSection competitions={mine.data ?? []} onChanged={mine.reload} />
      )}
      {showDriver && <DriverSection competitions={open.data ?? []} onChanged={open.reload} />}
    </div>
  );
}

// --- сторона склада ---

function WarehouseSection({
  competitions,
  onChanged,
}: {
  competitions: DriverCompetitionView[];
  onChanged: () => void;
}) {
  const routes = useAsync(getRoutes, []);
  const [routeId, setRouteId] = useState("");
  const [volume, setVolume] = useState("");
  const [date, setDate] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleAnnounce(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    if (!routeId) {
      setError(t("driverComp.routeRequired"));
      return;
    }
    const volumeNum = Number(volume);
    if (!Number.isFinite(volumeNum) || volumeNum <= 0) {
      setError(t("driverComp.volumePositive"));
      return;
    }
    setIsSubmitting(true);
    try {
      await createDriverCompetition(routeId, volumeNum, date);
      setNotice(t("driverComp.announced"));
      setVolume("");
      setDate("");
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <section className="panel">
      <h2 className="panel__title">{t("driverComp.myTitle")}</h2>
      <p className="panel__hint">{t("driverComp.myHint")}</p>

      <form className="inline-form" onSubmit={handleAnnounce}>
        <select value={routeId} onChange={(e) => setRouteId(e.target.value)}>
          <option value="">{t("driverComp.route")}…</option>
          {(routes.data ?? []).map((r) => (
            <option key={r.id} value={r.id}>
              {cityLabel(r.origin)} → {cityLabel(r.destination)}
            </option>
          ))}
        </select>
        <input
          type="number"
          min={1}
          step="any"
          value={volume}
          onChange={(e) => setVolume(e.target.value)}
          placeholder={t("driverComp.volume")}
        />
        <input
          value={date}
          onChange={(e) => setDate(e.target.value)}
          placeholder={t("driverComp.datePlaceholder")}
        />
        <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
          {isSubmitting ? t("common.loading") : t("driverComp.announce")}
        </button>
      </form>
      {notice && <p className="panel__hint">{notice}</p>}
      {error && <div className="form-error">{error}</div>}

      <div className="competition-results">
        {competitions.length === 0 ? (
          <p className="panel__hint">{t("driverComp.myEmpty")}</p>
        ) : (
          <ul className="tool-group__list">
            {competitions.map((view) => (
              <CompetitionCard key={view.competition.id} view={view} routes={routes.data ?? []} onChanged={onChanged} />
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

function CompetitionCard({
  view,
  routes,
  onChanged,
}: {
  view: DriverCompetitionView;
  routes: ParticipantRoute[];
  onChanged: () => void;
}) {
  const confirm = useConfirm();
  const [result, setResult] = useState<DriverSelectResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isBusy, setIsBusy] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [routeId, setRouteId] = useState(view.competition.route_id);
  const [volume, setVolume] = useState(String(view.competition.volume_m3));
  const [date, setDate] = useState(view.competition.dispatch_date);
  const comp = view.competition;
  const selectedBid = view.bids.find((b) => b.status === "selected") ?? null;

  async function handleSelect(bidId: string) {
    if (!await confirm({ message: t("driverComp.selectConfirm"), confirmLabel: t("driverComp.selectDriver"), danger: false })) return;
    setError(null);
    setIsBusy(true);
    try {
      const res = await selectDriverBid(comp.id, bidId);
      setResult(res);
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  async function handleUpdate(event: FormEvent) {
    event.preventDefault();
    const volumeValue = Number(volume);
    if (!routeId || !Number.isFinite(volumeValue) || volumeValue <= 0) {
      setError(t("driverComp.volumePositive"));
      return;
    }
    setError(null);
    setIsBusy(true);
    try {
      await updateDriverCompetition(comp.id, routeId, volumeValue, date);
      setIsEditing(false);
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  async function handleCancel() {
    if (!await confirm({ message: t("driverComp.cancelConfirm"), confirmLabel: t("driverComp.cancel") })) return;
    setError(null);
    setIsBusy(true);
    try {
      await cancelDriverCompetition(comp.id);
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  return (
    <li className="tool-row" style={{ alignItems: "flex-start" }}>
      <div style={{ flex: 1 }}>
        <div className="tool-row__name">
          {compactDirectionLabel(view.direction_label)} · {comp.volume_m3} м³
          {comp.dispatch_date && ` · ${comp.dispatch_date}`}{" "}
          <span className={comp.status === "open" ? "pill pill--green" : "pill pill--neutral"}>
            {comp.status === "open" ? t("driverComp.statusOpen") : t("driverComp.statusClosed")}
          </span>
        </div>
        <div className="tool-row__key">{formatDateTime(comp.created_at)}</div>

        {comp.status === "open" && (
          <div className="competition-response__actions">
            <button className="btn btn--secondary btn--sm" type="button" disabled={isBusy} onClick={() => setIsEditing((value) => !value)}>{t("common.edit")}</button>
            <button className="btn btn--ghost btn--sm" type="button" disabled={isBusy} onClick={() => void handleCancel()}>{t("driverComp.cancel")}</button>
          </div>
        )}
        {isEditing && (
          <form className="inline-form" onSubmit={handleUpdate}>
            <select value={routeId} onChange={(event) => setRouteId(event.target.value)}>
              {routes.map((route) => <option key={route.id} value={route.id}>{cityLabel(route.origin)} → {cityLabel(route.destination)}</option>)}
            </select>
            <input type="number" min={1} step="any" value={volume} onChange={(event) => setVolume(event.target.value)} />
            <input value={date} onChange={(event) => setDate(event.target.value)} placeholder={t("driverComp.datePlaceholder")} />
            <button className="btn btn--primary btn--sm" type="submit" disabled={isBusy}>{t("common.save")}</button>
          </form>
        )}

        {view.bids.length === 0 ? (
          <p className="panel__hint">{t("driverComp.bidsEmpty")}</p>
        ) : (
          <table className="table table--compact" style={{ marginTop: 8 }}>
            <thead>
              <tr>
                <th>{t("cargo.offerNumber")}</th>
                <th>{t("cargo.rating")}</th>
                <th>{t("cargo.price")}</th>
                <th>{t("driverComp.bidComment")}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {view.bids.map((bid) => (
                <tr key={bid.bid_id}>
                  <td>№{bid.bid_number}</td>
                  <td>{bid.rating !== null ? `★ ${bid.rating}` : "—"}</td>
                  <td>
                    <Money amount={bid.price} currency={bid.currency} />
                  </td>
                  <td>{bid.comment || "—"}</td>
                  <td>
                    {comp.status === "open" ? (
                      <button
                        className="btn btn--primary btn--sm"
                        disabled={isBusy}
                        onClick={() => void handleSelect(bid.bid_id)}
                      >
                        {t("driverComp.selectDriver")}
                      </button>
                    ) : bid.status === "selected" && !result ? (
                      <button
                        className="btn btn--secondary btn--sm"
                        disabled={isBusy}
                        onClick={() => void handleSelect(bid.bid_id)}
                      >
                        {t("driverComp.driverTitle")} →
                      </button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {selectedBid && result && (
          <div className="contact-card">
            <h3 className="detail-panel__subtitle">{t("driverComp.driverTitle")}</h3>
            <p className="panel__hint">{t("driverComp.driverNote")}</p>
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
          </div>
        )}
        {error && <div className="form-error">{error}</div>}
      </div>
    </li>
  );
}

// --- сторона водителя ---

function DriverSection({
  competitions,
  onChanged,
}: {
  competitions: OpenDriverCompetition[];
  onChanged: () => void;
}) {
  return (
    <section className="panel">
      <h2 className="panel__title">{t("driverComp.openTitle")}</h2>
      <p className="panel__hint">{t("driverComp.openHint")}</p>
      <div className="competition-results competition-results--compact">
        {competitions.length === 0 ? (
          <p className="panel__hint">{t("driverComp.openEmpty")}</p>
        ) : (
          <ul className="tool-group__list">
            {competitions.map((c) => (
              <OpenCompetitionRow key={c.competition_id} competition={c} onChanged={onChanged} />
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

function OpenCompetitionRow({
  competition,
  onChanged,
}: {
  competition: OpenDriverCompetition;
  onChanged: () => void;
}) {
  const [price, setPrice] = useState(competition.my_bid ? String(competition.my_bid.price) : "");
  const [currency, setCurrency] = useState<string>(competition.my_bid?.currency || DEFAULT_CURRENCY);
  const [comment, setComment] = useState(competition.my_bid?.comment ?? "");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleBid(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const priceNum = Number(price);
    if (!Number.isFinite(priceNum) || priceNum <= 0) {
      setError(t("driverComp.pricePositive"));
      return;
    }
    setIsSubmitting(true);
    try {
      await createDriverBid(competition.competition_id, priceNum, currency, comment);
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
        <div className="tool-row__name">
          {compactDirectionLabel(competition.direction_label)} · {competition.volume_m3} м³
          {competition.dispatch_date && ` · ${competition.dispatch_date}`}
        </div>
        <div className="tool-row__key">{formatDateTime(competition.created_at)}</div>

        {competition.my_bid && competition.my_bid.status !== "withdrawn" ? (
          <p className="panel__hint">
            {t("driverComp.myBid")}: <Money amount={competition.my_bid.price} currency={competition.my_bid.currency} />
            {competition.my_bid.comment && ` — ${competition.my_bid.comment}`}
          </p>
        ) : (
          <form className="inline-form" style={{ marginTop: 8 }} onSubmit={handleBid}>
            <input
              type="number"
              min={1}
              step="any"
              value={price}
              onChange={(e) => setPrice(e.target.value)}
              placeholder={t("driverComp.bidPrice")}
              required
            />
            <CurrencySelect value={currency} onChange={setCurrency} ariaLabel={t("common.currency")} />
            <input
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              placeholder={t("driverComp.bidCommentPlaceholder")}
            />
            <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
              {isSubmitting ? t("common.loading") : t("driverComp.submitBid")}
            </button>
          </form>
        )}
        {error && <div className="form-error">{error}</div>}
      </div>
    </li>
  );
}
