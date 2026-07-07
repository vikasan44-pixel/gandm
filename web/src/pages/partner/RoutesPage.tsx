import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import { addRoute, deleteRoute, getRoutes } from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { ApiError } from "../../api/client";
import { t } from "../../i18n";
import type { GeoPoint } from "../../api/types";

export function RoutesPage() {
  const routes = useAsync(getRoutes, []);
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
        {routes.data && routes.data.length > 0 && (
          <ul className="tool-group__list">
            {routes.data.map((route) => (
              <li key={route.id} className="tool-row">
                <div>
                  <div className="tool-row__name">
                    {route.origin.label} → {route.destination.label}
                  </div>
                  <div className="tool-row__key">
                    {route.origin.lat.toFixed(4)}, {route.origin.lng.toFixed(4)} →{" "}
                    {route.destination.lat.toFixed(4)}, {route.destination.lng.toFixed(4)}
                  </div>
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
