import { useState } from "react";
import { useAsync } from "../../hooks/useAsync";
import {
  getAvailableCargoForWarehouses,
  getAvailableConsolidatedForWarehouses,
  getMyWarehouses,
  submitWarehouseOffer,
  submitWarehouseOfferForConsolidated,
} from "../../api/participant";
import { ApiError } from "../../api/client";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { MultilingualRoute } from "../../components/common/MultilingualLabels";
import { CurrencySelect } from "../../components/common/CurrencySelect";
import { DEFAULT_CURRENCY } from "../../utils/currency";
import { t } from "../../i18n";
import type { CargoRequest, GeoPoint } from "../../api/types";

type Warehouse = { id: string; name: string; status: string; pickup_enabled: boolean };

export function WarehouseCargoPage() {
  const cargo = useAsync(getAvailableCargoForWarehouses, []);
  const consolidated = useAsync(getAvailableConsolidatedForWarehouses, []);
  const warehouses = useAsync(getMyWarehouses, []);
  const pickable = (warehouses.data ?? []).filter((w: Warehouse) => w.status === "published" && w.pickup_enabled);

  const nothing =
    cargo.data && consolidated.data && cargo.data.length === 0 && consolidated.data.length === 0;

  return (
    <div className="page">
      <h1 className="page__title">{t("warehouseCargo.title")}</h1>
      <p className="marketplace-page__hint">
        {t("warehouseCargo.hint")}
      </p>

      {(cargo.isLoading || consolidated.isLoading || warehouses.isLoading) && <LoadingState />}
      {cargo.error && <ErrorState message={cargo.error} />}
      {nothing && !cargo.isLoading && (
        <EmptyState message={t("warehouseCargo.empty")} />
      )}

      {consolidated.data && consolidated.data.length > 0 && (
        <section>
          <h2 className="panel__title">{t("warehouseCargo.consolidatedSection")}</h2>
          <div className="stack">
            {consolidated.data.map((c) => (
              <BidCard
                key={c.id}
                origin={c.origin}
                destination={c.destination}
                meta={`${c.total_volume_m3} ${t("fleet.unitM3")} · ${c.total_weight_kg} ${t("fleet.unitKg")} · ${t("warehouseCargo.consolidatedCargo")}`}
                badge={t("warehouseSearch.consolidation")}
                warehouses={pickable}
                onSubmit={(warehouseId, price, currency, conditions) =>
                  submitWarehouseOfferForConsolidated(c.id, { warehouse_id: warehouseId, price, currency, conditions })
                }
                onDone={consolidated.reload}
              />
            ))}
          </div>
        </section>
      )}

      {cargo.data && cargo.data.length > 0 && (
        <section>
          <h2 className="panel__title">{t("warehouseCargo.singleSection")}</h2>
          <div className="stack">
            {cargo.data.map((c: CargoRequest) => (
              <BidCard
                key={c.id}
                origin={c.origin}
                destination={c.destination}
                meta={
                  `${c.volume_m3} ${t("fleet.unitM3")} · ${c.weight_kg} ${t("fleet.unitKg")}` +
                  (c.packaging === "bulk" ? ` · ${t("cargoLogistics.bulkTag")}` : c.places_count > 0 ? ` · ${t("cargoLogistics.placesShort")}: ${c.places_count}` : "") +
                  (c.adr_required ? ` · ${t("cargoLogistics.adrTag")}` : "") +
                  (c.stackable ? "" : ` · ${t("cargoLogistics.noStackTag")}`)
                }
                warehouses={pickable}
                onSubmit={(warehouseId, price, currency, conditions) =>
                  submitWarehouseOffer(c.id, { warehouse_id: warehouseId, price, currency, conditions })
                }
                onDone={cargo.reload}
              />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

function BidCard({
  origin,
  destination,
  meta,
  badge,
  warehouses,
  onSubmit,
  onDone,
}: {
  origin: GeoPoint;
  destination: GeoPoint;
  meta: string;
  badge?: string;
  warehouses: { id: string; name: string }[];
  onSubmit: (warehouseId: string, price: number, currency: string, conditions: string) => Promise<unknown>;
  onDone: () => void;
}) {
  const [warehouseId, setWarehouseId] = useState(warehouses[0]?.id ?? "");
  const [price, setPrice] = useState("");
  const [currency, setCurrency] = useState<string>(DEFAULT_CURRENCY);
  const [conditions, setConditions] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sent, setSent] = useState(false);

  async function submit() {
    setError(null);
    if (!warehouseId) {
      setError(t("warehouseCargo.chooseWarehouse"));
      return;
    }
    if (!(Number(price) > 0)) {
      setError(t("warehouseCargo.pricePositive"));
      return;
    }
    try {
      setBusy(true);
      await onSubmit(warehouseId, Number(price), currency, conditions);
      setSent(true);
      onDone();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("warehouseCargo.sendFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="public-card">
      <div className="public-card__route">
        <MultilingualRoute origin={origin} destination={destination} />
        {badge && <span className="pill pill--yellow" style={{ marginLeft: 8 }}>{badge}</span>}
      </div>
      <div className="public-card__meta">{meta}</div>

      {sent ? (
        <div className="public-card__meta" style={{ color: "var(--color-success, #2e7d32)" }}>
          {t("warehouseCargo.sent")}
        </div>
      ) : (
        <>
          {error && <div className="form-error">{error}</div>}
          <div className="form-grid form-grid--3" style={{ alignItems: "end" }}>
            <label className="field">
              <span className="field__label">{t("warehouseCargo.warehouse")}</span>
              <select className="field__input" value={warehouseId} onChange={(e) => setWarehouseId(e.target.value)}>
                {warehouses.length === 0 && <option value="">{t("warehouseCargo.noPublished")}</option>}
                {warehouses.map((w) => (
                  <option key={w.id} value={w.id}>{w.name}</option>
                ))}
              </select>
            </label>
            <label className="field">
              <span className="field__label">{t("warehouseCargo.price")}</span>
              <div style={{ display: "flex", gap: 6 }}>
                <input className="field__input" type="number" value={price} onChange={(e) => setPrice(e.target.value)} style={{ flex: 1 }} />
                <CurrencySelect value={currency} onChange={setCurrency} ariaLabel={t("common.currency")} className="field__input" />
              </div>
            </label>
            <button className="btn btn--primary btn--sm" type="button" onClick={() => void submit()} disabled={busy}>
              {t("warehouseCargo.offer")}
            </button>
          </div>
          <input className="field__input" placeholder={t("warehouseCargo.conditions")} value={conditions} onChange={(e) => setConditions(e.target.value)} />
        </>
      )}
    </div>
  );
}
