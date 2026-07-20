import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  acceptTransportProposal,
  counterTransportProposal,
  finalTransportProposal,
  getIncomingTransportProposals,
  getMyTransportProposals,
  quoteTransportProposal,
  rejectTransportProposal,
} from "../../api/participant";
import { ApiError } from "../../api/client";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { pickLabel } from "../../utils/geoLabel";
import { Money } from "../../components/common/Money";
import { t } from "../../i18n";
import type { TransportProposalView } from "../../api/types";

type Tab = "mine" | "incoming";

const STATUS_KEY: Record<string, string> = {
  sent: "proposals.statusSent",
  carrier_quoted: "proposals.statusCarrierQuoted",
  client_countered: "proposals.statusClientCountered",
  carrier_final: "proposals.statusCarrierFinal",
  agreed: "proposals.statusAgreed",
  rejected: "proposals.statusRejected",
};

export function TransportProposalsPage() {
  const [tab, setTab] = useState<Tab>("mine");
  const [items, setItems] = useState<TransportProposalView[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = tab === "mine" ? await getMyTransportProposals() : await getIncomingTransportProposals();
      setItems(data);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("proposals.loadError"));
    } finally {
      setLoading(false);
    }
  }, [tab]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="page">
      <h1 className="page__title">{t("proposals.title")}</h1>
      <div className="btn-group" style={{ marginBottom: 16 }}>
        <button type="button" className={"btn btn--sm " + (tab === "mine" ? "btn--primary" : "btn--ghost")} onClick={() => setTab("mine")}>
          {t("proposals.tabMine")}
        </button>
        <button type="button" className={"btn btn--sm " + (tab === "incoming" ? "btn--primary" : "btn--ghost")} onClick={() => setTab("incoming")}>
          {t("proposals.tabIncoming")}
        </button>
      </div>

      {loading && <LoadingState />}
      {error && <ErrorState message={error} />}
      {items && items.length === 0 && !loading && (
        <EmptyState message={tab === "mine" ? t("proposals.emptyMine") : t("proposals.emptyIncoming")} />
      )}

      <div className="stack">
        {(items ?? []).map((p) => (
          <ProposalCard key={p.id} p={p} onChanged={load} />
        ))}
      </div>
    </div>
  );
}

function ProposalCard({ p, onChanged }: { p: TransportProposalView; onChanged: () => void }) {
  const [price, setPrice] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const isCarrier = p.viewer_role === "carrier";
  const isClient = p.viewer_role === "client";

  async function act(fn: () => Promise<unknown>) {
    setErr(null);
    try {
      setBusy(true);
      await fn();
      onChanged();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : t("proposals.actionFailed"));
    } finally {
      setBusy(false);
    }
  }

  const needPrice = () => {
    const n = Number(price);
    if (!(n > 0)) {
      setErr(t("proposals.pricePositive"));
      return null;
    }
    return n;
  };

  return (
    <div className="public-card">
      <div className="public-card__route">
        {pickLabel(p.origin.labels, p.origin.label)} → {pickLabel(p.destination.labels, p.destination.label)}
      </div>
      <div className="public-card__meta">
        {p.cargo_name ? `${p.cargo_name} · ` : ""}
        {p.volume_m3} {t("fleet.unitM3")} · {p.weight_kg} {t("fleet.unitKg")} · {t("proposals.placesCount")}: {p.places_count}
        {p.pickup_date ? ` · ${p.pickup_date}` : ""}
      </div>
      {p.items.length > 0 && (
        <div className="public-card__meta">
          {t("proposals.places")}: {p.items.map((it, i) => `${i + 1}) ${it.length_m}×${it.width_m}×${it.height_m}${t("fleet.unitM")}`).join("; ")}
        </div>
      )}

      <div className="public-card__meta">
        <strong>{t(STATUS_KEY[p.status] ?? "") || p.status}</strong>
        {p.current_price != null && (
          <> · {t("proposals.price")}: <strong><Money amount={p.current_price} currency={p.currency} /></strong>
            {p.last_price_by ? ` (${p.last_price_by === "carrier" ? t("proposals.byCarrier") : t("proposals.byClient")})` : ""}</>
        )}
      </div>

      {p.status === "agreed" && p.counterpart && (
        <div className="public-card__trip-plan">
          <strong>{t("proposals.agreedContacts")}</strong>
          <span>{p.counterpart.company_name}</span>
          <small>{p.counterpart.email} · {p.counterpart.phone}</small>
          {p.chat_id && <Link className="btn btn--ghost btn--sm" to="/app/chats">{t("proposals.openChat")}</Link>}
        </div>
      )}

      {err && <div className="form-error">{err}</div>}

      {/* Действия по роли и стадии торга */}
      <div className="btn-group" style={{ marginTop: 8 }}>
        {isCarrier && p.status === "sent" && (
          <PriceAction label={t("proposals.quote")} price={price} setPrice={setPrice} busy={busy} onGo={() => { const n = needPrice(); if (n) void act(() => quoteTransportProposal(p.id, n)); }} />
        )}
        {isClient && p.status === "carrier_quoted" && (
          <>
            <button className="btn btn--primary btn--sm" disabled={busy} onClick={() => void act(() => acceptTransportProposal(p.id))}>{t("proposals.agree")}</button>
            <PriceAction label={t("proposals.counter")} price={price} setPrice={setPrice} busy={busy} onGo={() => { const n = needPrice(); if (n) void act(() => counterTransportProposal(p.id, n)); }} />
          </>
        )}
        {isCarrier && p.status === "client_countered" && (
          <>
            <button className="btn btn--primary btn--sm" disabled={busy} onClick={() => void act(() => acceptTransportProposal(p.id))}>{t("proposals.agree")}</button>
            <PriceAction label={t("proposals.final")} price={price} setPrice={setPrice} busy={busy} onGo={() => { const n = needPrice(); if (n) void act(() => finalTransportProposal(p.id, n)); }} />
          </>
        )}
        {isClient && p.status === "carrier_final" && (
          <button className="btn btn--primary btn--sm" disabled={busy} onClick={() => void act(() => acceptTransportProposal(p.id))}>{t("proposals.agree")}</button>
        )}
        {p.status !== "agreed" && p.status !== "rejected" && (
          <button className="btn btn--ghost btn--sm" disabled={busy} onClick={() => void act(() => rejectTransportProposal(p.id))}>{t("proposals.reject")}</button>
        )}
      </div>
    </div>
  );
}

function PriceAction({ label, price, setPrice, busy, onGo }: { label: string; price: string; setPrice: (v: string) => void; busy: boolean; onGo: () => void }) {
  return (
    <span className="inline-price">
      <input className="field__input field__input--inline" type="number" placeholder={t("proposals.pricePlaceholder")} value={price} onChange={(e) => setPrice(e.target.value)} style={{ width: 120 }} />
      <button className="btn btn--secondary btn--sm" disabled={busy} onClick={onGo}>{label}</button>
    </span>
  );
}
