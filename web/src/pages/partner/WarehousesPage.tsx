import { useState, type FormEvent } from "react";
import {
  createWarehouse,
  deleteDispatchThreshold,
  deleteWarehouse,
  getDispatchThresholds,
  getMyWarehouses,
  setDispatchThreshold,
  updateWarehouse,
  type WarehouseInput,
} from "../../api/participant";
import { ApiError } from "../../api/client";
import { useAsync } from "../../hooks/useAsync";
import { DetailModal } from "../../components/common/DetailModal";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { MultilingualRoute } from "../../components/common/MultilingualLabels";
import { useConfirm } from "../../components/common/ConfirmDialog";
import { t } from "../../i18n";
import type { DispatchPlanStatus, GeoPoint, RouteWithThreshold, Warehouse } from "../../api/types";
import { cityLabel } from "../../utils/locationLabel";

const SERVICE_KEYS = ["responsible_storage", "open_storage", "loading", "packing", "marking", "palletizing", "sorting", "temporary_storage", "cross_docking", "consolidation", "order_picking", "delivery", "factory_pickup", "customs_warehouse"];

type FormState = Omit<WarehouseInput, "address"> & { address: GeoPoint | null };

const emptyForm: FormState = {
  name: "", address: null, contact_name: "", contact_phone: "", description: "", work_hours: "",
  covered_area_m2: 0, open_area_m2: 0, available_covered_area_m2: 0, available_open_area_m2: 0,
  max_weight_kg: 0, max_volume_m3: 0, services: [], consolidation_enabled: false,
  consolidation_min_volume_m3: 0, consolidation_frequency: "", pickup_enabled: false, pickup_cities: [],
  pickup_radius_km: 0, own_transport: false, pickup_max_weight_kg: 0, pickup_max_volume_m3: 0,
  pickup_price_mode: "", dispatch_routes: [], status: "draft",
};

export function WarehousesPage() {
  const confirm = useConfirm();
  const warehouses = useAsync(getMyWarehouses, []);
  const dispatchPlans = useAsync(getDispatchThresholds, []);
  const [editing, setEditing] = useState<Warehouse | "new" | null>(null);
  const [editingPlan, setEditingPlan] = useState<{ warehouse: Warehouse; row?: RouteWithThreshold } | null>(null);

  async function remove(item: Warehouse) {
    if (!await confirm({ message: t("warehouses.deleteConfirm"), confirmLabel: t("common.delete") })) return;
    await deleteWarehouse(item.id);
    warehouses.reload();
  }

  return (
    <div className="page">
      <div className="panel__header">
        <div><h1 className="page__title">{t("warehouses.title")}</h1><p className="marketplace-page__hint">{t("warehouses.hint")}</p></div>
        <button className="btn btn--primary" type="button" onClick={() => setEditing("new")}>{t("warehouses.add")}</button>
      </div>
      {warehouses.isLoading && <LoadingState />}
      {warehouses.error && <ErrorState message={warehouses.error} onRetry={warehouses.reload} />}
      {warehouses.data?.length === 0 && <EmptyState message={t("warehouses.empty")} />}
      <div className="warehouse-cards">
        {(warehouses.data ?? []).map((item) => (
          <article className="warehouse-card" key={item.id}>
            <div className="warehouse-card__header"><h2>{item.name}</h2><span className={`pill ${item.status === "published" ? "pill--green" : "pill--yellow"}`}>{t(`warehouses.status.${item.status}`)}</span></div>
            <p>{cityLabel(item.address)}</p>
            <div className="warehouse-card__areas">
              <span>{t("warehouses.covered")}: {item.available_covered_area_m2}/{item.covered_area_m2} м²</span>
              <span>{t("warehouses.open")}: {item.available_open_area_m2}/{item.open_area_m2} м²</span>
            </div>
            <div className="warehouse-card__services">{item.services.slice(0, 6).map((key) => <span className="pill pill--neutral" key={key}>{t(`warehouses.services.${key}`)}</span>)}</div>
            <WarehouseDispatchPlans
              warehouse={item}
              rows={dispatchPlans.data ?? []}
              onEdit={(row) => setEditingPlan({ warehouse: item, row })}
              onAdd={() => setEditingPlan({ warehouse: item })}
            />
            <div className="warehouse-card__actions">
              <button className="btn btn--ghost btn--sm" type="button" onClick={() => setEditing(item)}>{t("warehouses.edit")}</button>
              <button className="btn btn--ghost btn--sm" type="button" onClick={() => void remove(item)}>{t("common.delete")}</button>
            </div>
          </article>
        ))}
      </div>
      {editing && <DetailModal onClose={() => setEditing(null)} wide><WarehouseForm item={editing === "new" ? null : editing} onSaved={() => { setEditing(null); warehouses.reload(); dispatchPlans.reload(); }} /></DetailModal>}
      {editingPlan && (
        <DetailModal onClose={() => setEditingPlan(null)}>
          <DispatchPlanForm
            warehouse={editingPlan.warehouse}
            rows={dispatchPlans.data ?? []}
            item={editingPlan.row}
            onSaved={() => { setEditingPlan(null); dispatchPlans.reload(); }}
          />
        </DetailModal>
      )}
    </div>
  );
}

function WarehouseDispatchPlans({ warehouse, rows, onEdit, onAdd }: {
  warehouse: Warehouse;
  rows: RouteWithThreshold[];
  onEdit: (row: RouteWithThreshold) => void;
  onAdd: () => void;
}) {
  const warehouseRouteIDs = new Set(warehouse.dispatch_routes.map((route) => route.id).filter(Boolean));
  const plans = rows.filter((row) => warehouseRouteIDs.has(row.route.id) && row.warehouse_id === warehouse.id && row.threshold?.warehouse_id === warehouse.id);
  return (
    <section className="dispatch-plans">
      <div className="dispatch-plans__header">
        <div>
          <h3>{t("dispatch.cardTitle")}</h3>
          <p>{t("dispatch.cardHint")}</p>
        </div>
        <button className="btn btn--secondary btn--sm" type="button" onClick={onAdd}>{t("dispatch.addPlan")}</button>
      </div>
      {plans.length === 0 && <div className="dispatch-plans__empty">{t("dispatch.noPlans")}</div>}
      {plans.map((row) => {
        const plan = row.threshold!;
        const percent = Math.min(100, Math.max(0, plan.threshold_m3 ? (plan.accrued_m3 / plan.threshold_m3) * 100 : 0));
        return (
          <button className="dispatch-plan" type="button" key={row.route.id} onClick={() => onEdit(row)}>
            <div className="dispatch-plan__top">
              <strong><MultilingualRoute origin={row.route.origin} destination={row.route.destination} /></strong>
              <span className={`dispatch-plan__status dispatch-plan__status--${plan.status}`}>{t(`dispatch.status.${plan.status}`)}</span>
            </div>
            <div className="dispatch-plan__search">
              {plan.remaining_m3 > 0 ? `${t("dispatch.lookingFor")} ${formatVolume(plan.remaining_m3)} м³` : t("dispatch.readyToSend")}
            </div>
            <div className="dispatch-plan__numbers">
              <span>{t("dispatch.accruedShort")}: <b>{formatVolume(plan.accrued_m3)} м³</b></span>
              <span>{t("dispatch.targetShort")}: <b>{formatVolume(plan.threshold_m3)} м³</b></span>
            </div>
            <div className="dispatch-plan__progress"><span style={{ width: `${percent}%` }} /></div>
            {plan.estimated_dispatch_date && <span className="dispatch-plan__date">{t("dispatch.estimatedDate")}: {formatPlanDate(plan.estimated_dispatch_date)}</span>}
          </button>
        );
      })}
    </section>
  );
}

function DispatchPlanForm({ warehouse, rows, item, onSaved }: {
  warehouse: Warehouse;
  rows: RouteWithThreshold[];
  item?: RouteWithThreshold;
  onSaved: () => void;
}) {
  const confirm = useConfirm();
  const warehouseRouteIDs = new Set(warehouse.dispatch_routes.map((route) => route.id).filter(Boolean));
  const warehouseRows = rows.filter((row) => warehouseRouteIDs.has(row.route.id) && row.warehouse_id === warehouse.id);
  const availableRows = item ? warehouseRows : warehouseRows.filter((row) => !row.threshold);
  const [routeId, setRouteId] = useState(item?.route.id ?? availableRows[0]?.route.id ?? "");
  const [target, setTarget] = useState(item?.threshold ? String(item.threshold.threshold_m3) : "120");
  const [manualAccrued, setManualAccrued] = useState(item?.threshold ? String(item.threshold.manual_accrued_m3) : "0");
  const [estimatedDate, setEstimatedDate] = useState(item?.threshold?.estimated_dispatch_date?.slice(0, 10) ?? "");
  const [status, setStatus] = useState<DispatchPlanStatus>(item?.threshold?.status ?? "collecting");
  const [error, setError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    const thresholdM3 = Number(target);
    const manualAccruedM3 = Number(manualAccrued);
    if (!routeId) { setError(t("dispatch.routeRequired")); return; }
    if (!Number.isFinite(thresholdM3) || thresholdM3 <= 0 || !Number.isFinite(manualAccruedM3) || manualAccruedM3 < 0) {
      setError(t("dispatch.numbersInvalid")); return;
    }
    setIsSaving(true);
    try {
      await setDispatchThreshold(routeId, {
        warehouse_id: warehouse.id,
        threshold_m3: thresholdM3,
        manual_accrued_m3: manualAccruedM3,
        estimated_dispatch_date: estimatedDate,
        status,
      });
      onSaved();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally { setIsSaving(false); }
  }

  async function remove() {
    if (!item || !await confirm({ message: t("dispatch.deleteConfirm"), confirmLabel: t("dispatch.deletePlan") })) return;
    setIsSaving(true); setError(null);
    try { await deleteDispatchThreshold(item.route.id); onSaved(); }
    catch (err) { setError(err instanceof ApiError ? err.message : t("common.unexpectedError")); }
    finally { setIsSaving(false); }
  }

  const selected = warehouseRows.find((row) => row.route.id === routeId);
  const platformAccrued = selected?.active_cargo_m3 ?? item?.threshold?.platform_accrued_m3 ?? 0;
  const totalAccrued = platformAccrued + Number(manualAccrued || 0);
  const remaining = Math.max(0, Number(target || 0) - totalAccrued);
  return (
    <form className="warehouse-form dispatch-plan-form" onSubmit={submit}>
      <div><h2 className="detail-panel__title">{item ? t("dispatch.editPlan") : t("dispatch.addPlan")}</h2><p className="marketplace-page__hint">{warehouse.name}</p></div>
      {availableRows.length === 0 && !item ? (
        <div className="dispatch-plans__empty">{t("dispatch.noAvailableRoutes")}</div>
      ) : (
        <>
          <label className="field"><span className="field__label">{t("dispatch.direction")}</span><select value={routeId} onChange={(e) => setRouteId(e.target.value)} disabled={Boolean(item)}>{availableRows.map((row) => <option value={row.route.id} key={row.route.id}>{cityLabel(row.route.origin)} → {cityLabel(row.route.destination)}</option>)}</select></label>
          {selected && <div className="dispatch-plan-form__direction"><MultilingualRoute origin={selected.route.origin} destination={selected.route.destination} /></div>}
          <div className="field-row"><NumberField label={t("dispatch.targetVolume")} value={Number(target)} onChange={(value) => setTarget(String(value))} /><ReadOnlyNumberField label={t("dispatch.accruedVolume")} value={totalAccrued} /></div>
          <div className="dispatch-plan-form__breakdown"><span>{t("dispatch.fromMyCargo")}: <b>{formatVolume(platformAccrued)} м³</b></span><NumberField label={t("dispatch.manualAccruedVolume")} value={Number(manualAccrued)} onChange={(value) => setManualAccrued(String(value))} /></div>
          <div className="dispatch-plan-form__result">{remaining > 0 ? <><span>{t("dispatch.lookingFor")}</span><strong>{formatVolume(remaining)} м³</strong></> : <strong>{t("dispatch.readyToSend")}</strong>}</div>
          <div className="field-row"><label className="field"><span className="field__label">{t("dispatch.estimatedDate")}</span><input type="date" value={estimatedDate} onChange={(e) => setEstimatedDate(e.target.value)} /></label><label className="field"><span className="field__label">{t("dispatch.statusLabel")}</span><select value={status} onChange={(e) => setStatus(e.target.value as DispatchPlanStatus)}><option value="collecting">{t("dispatch.status.collecting")}</option><option value="ready">{t("dispatch.status.ready")}</option><option value="paused">{t("dispatch.status.paused")}</option><option value="dispatched">{t("dispatch.status.dispatched")}</option></select></label></div>
          <p className="dispatch-plan-form__note">{t("dispatch.separateAreaNote")}</p>
          {error && <div className="form-error">{error}</div>}
          <div className="dispatch-plan-form__actions"><button className="btn btn--primary" type="submit" disabled={isSaving}>{isSaving ? t("common.loading") : t("common.save")}</button>{item && <button className="btn btn--ghost" type="button" disabled={isSaving} onClick={() => void remove()}>{t("dispatch.deletePlan")}</button>}</div>
        </>
      )}
    </form>
  );
}

function formatVolume(value: number) { return value.toLocaleString("ru-RU", { maximumFractionDigits: 2 }); }
function formatPlanDate(value: string) { return new Intl.DateTimeFormat("ru-RU").format(new Date(value)); }

function WarehouseForm({ item, onSaved }: { item: Warehouse | null; onSaved: () => void }) {
  const [form, setForm] = useState<FormState>(item ? { ...item } : emptyForm);
  const [pickupCity, setPickupCity] = useState<GeoPoint | null>(null);
  const [routeOrigin, setRouteOrigin] = useState<GeoPoint | null>(null);
  const [routeDestination, setRouteDestination] = useState<GeoPoint | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const set = <K extends keyof FormState>(key: K, value: FormState[K]) => setForm((current) => ({ ...current, [key]: value }));

  function toggleService(key: string) {
    set("services", form.services.includes(key) ? form.services.filter((value) => value !== key) : [...form.services, key]);
  }

  async function submit(event: FormEvent) {
    event.preventDefault(); setError(null);
    if (!form.name.trim() || !form.address) { setError(t("warehouses.required")); return; }
    setIsSaving(true);
    try {
      const input = form as WarehouseInput;
      if (item) await updateWarehouse(item.id, input); else await createWarehouse(input);
      onSaved();
    } catch (err) { setError(err instanceof ApiError ? err.message : t("common.unexpectedError")); }
    finally { setIsSaving(false); }
  }

  return (
    <form className="warehouse-form" onSubmit={submit}>
      <h2 className="detail-panel__title">{item ? t("warehouses.editTitle") : t("warehouses.addTitle")}</h2>
      <FormSection title={t("warehouses.basic")}>
        <div className="field-row"><Field label={t("warehouses.name")} value={form.name} onChange={(v) => set("name", v)} /><Field label={t("warehouses.workHours")} value={form.work_hours} onChange={(v) => set("work_hours", v)} /></div>
        <GeoPointField title={t("warehouses.address")} value={form.address} onChange={(v) => set("address", v)} />
        <div className="field-row"><Field label={t("warehouses.contactName")} value={form.contact_name} onChange={(v) => set("contact_name", v)} /><Field label={t("warehouses.contactPhone")} value={form.contact_phone} onChange={(v) => set("contact_phone", v)} /></div>
        <label className="field"><span className="field__label">{t("warehouses.description")}</span><textarea value={form.description} onChange={(e) => set("description", e.target.value)} /></label>
      </FormSection>
      <FormSection title={t("warehouses.capacity")}>
        <div className="warehouse-form__numbers"><NumberField label={t("warehouses.coveredTotal")} value={form.covered_area_m2} onChange={(v) => set("covered_area_m2", v)} /><NumberField label={t("warehouses.coveredAvailable")} value={form.available_covered_area_m2} onChange={(v) => set("available_covered_area_m2", v)} /><NumberField label={t("warehouses.openTotal")} value={form.open_area_m2} onChange={(v) => set("open_area_m2", v)} /><NumberField label={t("warehouses.openAvailable")} value={form.available_open_area_m2} onChange={(v) => set("available_open_area_m2", v)} /><NumberField label={t("warehouses.maxWeight")} value={form.max_weight_kg} onChange={(v) => set("max_weight_kg", v)} /><NumberField label={t("warehouses.maxVolume")} value={form.max_volume_m3} onChange={(v) => set("max_volume_m3", v)} /></div>
      </FormSection>
      <FormSection title={t("warehouses.servicesTitle")}><div className="warehouse-service-grid">{SERVICE_KEYS.map((key) => <label className="warehouse-check" key={key}><input type="checkbox" checked={form.services.includes(key)} onChange={() => toggleService(key)} /><span>{t(`warehouses.services.${key}`)}</span></label>)}</div></FormSection>
      <FormSection title={t("warehouses.consolidationTitle")}><Toggle label={t("warehouses.consolidationEnabled")} checked={form.consolidation_enabled} onChange={(v) => set("consolidation_enabled", v)} />{form.consolidation_enabled && <div className="field-row"><NumberField label={t("warehouses.minBatch")} value={form.consolidation_min_volume_m3} onChange={(v) => set("consolidation_min_volume_m3", v)} /><Field label={t("warehouses.frequency")} value={form.consolidation_frequency} onChange={(v) => set("consolidation_frequency", v)} /></div>}</FormSection>
      <FormSection title={t("warehouses.routesTitle")}>
        <p className="marketplace-page__hint">{t("warehouses.routesHint")}</p>
        <div className="field-row">
          <GeoPointField title={t("warehouses.routeOrigin")} value={routeOrigin} onChange={setRouteOrigin} />
          <GeoPointField title={t("warehouses.routeDestination")} value={routeDestination} onChange={setRouteDestination} />
        </div>
        <button
          className="btn btn--secondary btn--sm"
          type="button"
          disabled={!routeOrigin || !routeDestination}
          onClick={() => {
            if (!routeOrigin || !routeDestination) return;
            const duplicate = form.dispatch_routes.some((route) => route.origin.lat === routeOrigin.lat && route.origin.lng === routeOrigin.lng && route.destination.lat === routeDestination.lat && route.destination.lng === routeDestination.lng);
            if (!duplicate) set("dispatch_routes", [...form.dispatch_routes, { origin: routeOrigin, destination: routeDestination }]);
            setRouteOrigin(null);
            setRouteDestination(null);
          }}
        >
          {t("warehouses.addRoute")}
        </button>
        {form.dispatch_routes.length === 0 ? (
          <p className="marketplace-page__hint">{t("warehouses.routesEmpty")}</p>
        ) : (
          <div className="warehouse-routes-list">
            {form.dispatch_routes.map((route, index) => (
              <div className="warehouse-route-row" key={route.id ?? `${route.origin.lat}-${route.destination.lat}-${index}`}>
                <MultilingualRoute origin={route.origin} destination={route.destination} />
                <button className="btn btn--ghost btn--sm" type="button" onClick={() => set("dispatch_routes", form.dispatch_routes.filter((_, routeIndex) => routeIndex !== index))}>{t("common.delete")}</button>
              </div>
            ))}
          </div>
        )}
      </FormSection>
      <FormSection title={t("warehouses.pickupTitle")}><Toggle label={t("warehouses.pickupEnabled")} checked={form.pickup_enabled} onChange={(v) => set("pickup_enabled", v)} />{form.pickup_enabled && <><GeoPointField title={t("warehouses.pickupCity")} value={pickupCity} onChange={setPickupCity} /><button className="btn btn--ghost btn--sm" type="button" disabled={!pickupCity} onClick={() => { if (pickupCity) { set("pickup_cities", [...form.pickup_cities, pickupCity]); setPickupCity(null); } }}>{t("warehouses.addCity")}</button><div className="warehouse-pickup-cities">{form.pickup_cities.map((city, index) => <span className="pill pill--neutral" key={`${city.lat}-${city.lng}`}><button type="button" onClick={() => set("pickup_cities", form.pickup_cities.filter((_, i) => i !== index))}>×</button>{cityLabel(city)}</span>)}</div><div className="warehouse-form__numbers"><NumberField label={t("warehouses.pickupRadius")} value={form.pickup_radius_km} onChange={(v) => set("pickup_radius_km", v)} /><NumberField label={t("warehouses.pickupWeight")} value={form.pickup_max_weight_kg} onChange={(v) => set("pickup_max_weight_kg", v)} /><NumberField label={t("warehouses.pickupVolume")} value={form.pickup_max_volume_m3} onChange={(v) => set("pickup_max_volume_m3", v)} /></div><Toggle label={t("warehouses.ownTransport")} checked={form.own_transport} onChange={(v) => set("own_transport", v)} /><Field label={t("warehouses.pickupPrice")} value={form.pickup_price_mode} onChange={(v) => set("pickup_price_mode", v)} /></>}</FormSection>
      <FormSection title={t("warehouses.publication")}><label className="field"><span className="field__label">{t("warehouses.statusLabel")}</span><select value={form.status} onChange={(e) => set("status", e.target.value as FormState["status"])}><option value="draft">{t("warehouses.status.draft")}</option><option value="published">{t("warehouses.status.published")}</option><option value="paused">{t("warehouses.status.paused")}</option></select></label></FormSection>
      {error && <div className="form-error">{error}</div>}<button className="btn btn--primary" type="submit" disabled={isSaving}>{isSaving ? t("common.loading") : t("common.save")}</button>
    </form>
  );
}

function FormSection({ title, children }: { title: string; children: React.ReactNode }) { return <section className="warehouse-form__section"><h3>{title}</h3>{children}</section>; }
function Field({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) { return <label className="field"><span className="field__label">{label}</span><input value={value} onChange={(e) => onChange(e.target.value)} /></label>; }
function NumberField({ label, value, onChange }: { label: string; value: number; onChange: (value: number) => void }) { return <label className="field"><span className="field__label">{label}</span><input type="number" min="0" step="any" value={value || ""} onChange={(e) => onChange(Number(e.target.value) || 0)} /></label>; }
function ReadOnlyNumberField({ label, value }: { label: string; value: number }) { return <label className="field"><span className="field__label">{label}</span><input type="number" value={value} readOnly /></label>; }
function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) { return <label className="warehouse-check warehouse-check--toggle"><input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} /><span>{label}</span></label>; }
