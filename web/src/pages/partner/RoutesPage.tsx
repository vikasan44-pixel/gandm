import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import {
  addRoute,
  deleteRoute,
  deleteDispatchThreshold,
  getDispatchThresholds,
  getRoutes,
  setDispatchThreshold,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { ApiError } from "../../api/client";
import { t } from "../../i18n";
import type { DispatchThreshold, GeoPoint } from "../../api/types";

export function RoutesPage() {
  const routes = useAsync(getRoutes, []);
  // Пороги отправки видят только участники с manage_warehouse_slots — на
  // 403 tool_required секция просто не показывается.
  const thresholds = useAsync(getDispatchThresholds, []);
  const hasSlotsTool = thresholds.data !== null;
  const thresholdByRoute = new Map(
    (thresholds.data ?? [])
      .filter((row) => row.threshold)
      .map((row) => [row.route.id, row.threshold as DispatchThreshold])
  );
  const [origin, setOrigin] = useState<GeoPoint | null>(null);
  const [destination, setDestination] = useState<GeoPoint | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  // Remounts the two pickers after a successful add so maps/markers reset.
  const [formEpoch, setFormEpoch] = useState(0);

  async function handleAdd(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (!origin || !destination || !origin.label.trim() || !destination.label.trim()) {
      setError(t("geo.pointsRequired"));
      return;
    }
    setIsSubmitting(true);
    try {
      await addRoute(origin, destination);
      setOrigin(null);
      setDestination(null);
      setFormEpoch((v) => v + 1);
      routes.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleDelete(routeId: string) {
    setError(null);
    try {
      await deleteRoute(routeId);
      routes.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    }
  }

  return (
    <div className="page">
      <h1 className="page__title">{t("routes.title")}</h1>

      <section className="panel">
        <h2 className="panel__title">{t("routes.addTitle")}</h2>
        <form className="inline-form inline-form--stacked" onSubmit={handleAdd}>
          <GeoPointField
            key={`origin-${formEpoch}`}
            title={t("geo.originPoint")}
            value={origin}
            onChange={setOrigin}
          />
          <GeoPointField
            key={`destination-${formEpoch}`}
            title={t("geo.destinationPoint")}
            value={destination}
            onChange={setDestination}
          />
          <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("common.loading") : t("routes.add")}
          </button>
        </form>
        {error && <div className="form-error">{error}</div>}
      </section>

      <section className="panel">
        {routes.isLoading && <LoadingState />}
        {routes.error && <ErrorState message={routes.error} onRetry={routes.reload} />}
        {routes.data && routes.data.length === 0 && (
          <EmptyState message={t("routes.empty")} />
        )}
        {hasSlotsTool && routes.data && routes.data.length > 0 && (
          <p className="panel__hint">{t("dispatch.hint")}</p>
        )}
        {routes.data && routes.data.length > 0 && (
          <ul className="tool-group__list">
            {routes.data.map((route) => (
              <li key={route.id} className="tool-row" style={{ alignItems: "flex-start" }}>
                <div style={{ flex: 1 }}>
                  <div className="tool-row__name">
                    {route.origin.label} → {route.destination.label}
                  </div>
                  <div className="tool-row__key">
                    {route.origin.lat.toFixed(4)}, {route.origin.lng.toFixed(4)} →{" "}
                    {route.destination.lat.toFixed(4)}, {route.destination.lng.toFixed(4)}
                  </div>
                  {hasSlotsTool && (
                    <ThresholdEditor
                      routeId={route.id}
                      threshold={thresholdByRoute.get(route.id)}
                      onChanged={thresholds.reload}
                    />
                  )}
                </div>
                <button
                  className="btn btn--ghost btn--sm"
                  onClick={() => handleDelete(route.id)}
                >
                  {t("routes.delete")}
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

// ThresholdEditor — «порог отправки» склада на направлении (ТЗ §5.2):
// сколько м³ нужно набрать и сколько уже есть. Значения видны клиентам в
// анонимных предложениях этого склада.
function ThresholdEditor({
  routeId,
  threshold,
  onChanged,
}: {
  routeId: string;
  threshold?: DispatchThreshold;
  onChanged: () => void;
}) {
  const [thresholdM3, setThresholdM3] = useState(threshold ? String(threshold.threshold_m3) : "");
  const [accruedM3, setAccruedM3] = useState(threshold ? String(threshold.accrued_m3) : "0");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isBusy, setIsBusy] = useState(false);

  async function handleSave(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    const th = Number(thresholdM3);
    const acc = Number(accruedM3);
    if (!Number.isFinite(th) || th <= 0 || !Number.isFinite(acc) || acc < 0) {
      setError(t("dispatch.numbersInvalid"));
      return;
    }
    setIsBusy(true);
    try {
      await setDispatchThreshold(routeId, th, acc);
      setNotice(t("dispatch.saved"));
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  async function handleRemove() {
    setError(null);
    setNotice(null);
    setIsBusy(true);
    try {
      await deleteDispatchThreshold(routeId);
      setThresholdM3("");
      setAccruedM3("0");
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  return (
    <form className="inline-form" style={{ marginTop: 8 }} onSubmit={handleSave}>
      <input
        type="number"
        min={1}
        step="any"
        value={thresholdM3}
        onChange={(e) => setThresholdM3(e.target.value)}
        placeholder={t("dispatch.threshold")}
      />
      <input
        type="number"
        min={0}
        step="any"
        value={accruedM3}
        onChange={(e) => setAccruedM3(e.target.value)}
        placeholder={t("dispatch.accrued")}
      />
      <button className="btn btn--secondary btn--sm" type="submit" disabled={isBusy}>
        {isBusy ? t("common.loading") : t("dispatch.save")}
      </button>
      {threshold && (
        <button
          className="btn btn--ghost btn--sm"
          type="button"
          disabled={isBusy}
          onClick={() => void handleRemove()}
        >
          {t("dispatch.remove")}
        </button>
      )}
      {notice && <span className="panel__hint">{notice}</span>}
      {error && <span className="form-error">{error}</span>}
    </form>
  );
}
