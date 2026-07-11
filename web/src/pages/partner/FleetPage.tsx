import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import {
  addVehicle,
  addVehicleDestination,
  deleteVehicle,
  deleteVehicleDestination,
  getVehicles,
  updateVehicleLocation,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { ApiError } from "../../api/client";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { BODY_TYPE_KEYS, bodyTypeLabel } from "../../utils/bodyType";
import { t } from "../../i18n";
import type { GeoPoint, Vehicle } from "../../api/types";

// FleetPage — транспорт участника (ТЗ §11.1). Доступ гейтится бэкендом по
// инструменту manage_fleet: без него список отвечает 403 tool_required, и
// страница показывает локализованное сообщение. Местонахождение и назначения
// указываются координатами (по карте); назначений может быть несколько.
export function FleetPage() {
  const vehicles = useAsync(getVehicles, []);

  const [axles, setAxles] = useState("2");
  const [capacityKg, setCapacityKg] = useState("");
  const [capacityM3, setCapacityM3] = useState("");
  const [length, setLength] = useState("");
  const [width, setWidth] = useState("");
  const [height, setHeight] = useState("");
  // Храним КЛЮЧ кузова, не переведённую строку — чтобы карточка следовала
  // языку интерфейса.
  const [bodyType, setBodyType] = useState<string>("bodyTented");
  // Местонахождение (по карте) и назначения — всё опционально.
  const [location, setLocation] = useState<GeoPoint | null>(null);
  const [destinations, setDestinations] = useState<(GeoPoint | null)[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleAdd(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    const numbers = [Number(axles), Number(capacityKg), Number(length), Number(width), Number(height)];
    if (numbers.some((n) => !Number.isFinite(n) || n <= 0)) {
      setError(t("fleet.numbersPositive"));
      return;
    }
    if (!bodyType.trim()) {
      setError(t("fleet.bodyTypeRequired"));
      return;
    }
    setIsSubmitting(true);
    try {
      await addVehicle({
        axles: Number(axles),
        capacity_kg: Number(capacityKg),
        capacity_m3: Number(capacityM3) || 0,
        length_m: Number(length),
        width_m: Number(width),
        height_m: Number(height),
        body_type: bodyType,
        location,
        destinations: destinations.filter((d): d is GeoPoint => d !== null),
      });
      setNotice(t("fleet.added"));
      setCapacityKg("");
      setCapacityM3("");
      setLength("");
      setWidth("");
      setHeight("");
      setLocation(null);
      setDestinations([]);
      vehicles.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleDelete(id: string) {
    setError(null);
    try {
      await deleteVehicle(id);
      vehicles.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    }
  }

  function setDestinationAt(i: number, v: GeoPoint | null) {
    setDestinations((prev) => prev.map((d, idx) => (idx === i ? v : d)));
  }

  return (
    <div className="page">
      <h1 className="page__title">{t("fleet.title")}</h1>

      <section className="panel">
        <h2 className="panel__title">{t("fleet.addTitle")}</h2>
        <form className="inline-form inline-form--stacked" onSubmit={handleAdd}>
          <div className="field-row">
            <label className="field">
              <span className="field__label">{t("fleet.axles")}</span>
              <input type="number" min={1} max={12} value={axles} onChange={(e) => setAxles(e.target.value)} required />
            </label>
            <label className="field">
              <span className="field__label">{t("fleet.capacity")}</span>
              <input type="number" min={1} step="any" value={capacityKg} onChange={(e) => setCapacityKg(e.target.value)} required />
            </label>
            <label className="field">
              <span className="field__label">{t("fleet.capacityM3")}</span>
              <input type="number" min={0} step="any" value={capacityM3} onChange={(e) => setCapacityM3(e.target.value)} />
            </label>
          </div>
          <div className="field-row">
            <label className="field">
              <span className="field__label">{t("fleet.bodyType")}</span>
              <select value={bodyType} onChange={(e) => setBodyType(e.target.value)}>
                {BODY_TYPE_KEYS.map((key) => (
                  <option key={key} value={key}>
                    {t(`fleet.${key}`)}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span className="field__label">{t("fleet.length")}</span>
              <input type="number" min={0.1} step="any" value={length} onChange={(e) => setLength(e.target.value)} required />
            </label>
            <label className="field">
              <span className="field__label">{t("fleet.width")}</span>
              <input type="number" min={0.1} step="any" value={width} onChange={(e) => setWidth(e.target.value)} required />
            </label>
            <label className="field">
              <span className="field__label">{t("fleet.height")}</span>
              <input type="number" min={0.1} step="any" value={height} onChange={(e) => setHeight(e.target.value)} required />
            </label>
          </div>

          <p className="panel__hint">{t("fleet.locationHint")}</p>
          <GeoPointField title={t("fleet.location")} value={location} onChange={setLocation} />

          <p className="panel__hint">{t("fleet.destinationsHint")}</p>
          {destinations.map((d, i) => (
            <div key={i} className="fleet-destination">
              <GeoPointField title={`${t("fleet.destination")} ${i + 1}`} value={d} onChange={(v) => setDestinationAt(i, v)} />
              <button
                type="button"
                className="btn btn--ghost btn--sm"
                onClick={() => setDestinations((prev) => prev.filter((_, idx) => idx !== i))}
              >
                {t("fleet.removeDestination")}
              </button>
            </div>
          ))}
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => setDestinations((prev) => [...prev, null])}>
            {t("fleet.addDestination")}
          </button>

          <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("common.loading") : t("fleet.add")}
          </button>
        </form>
        {notice && <p className="panel__hint">{notice}</p>}
        {error && <div className="form-error">{error}</div>}
      </section>

      <section className="panel">
        {vehicles.isLoading && <LoadingState />}
        {vehicles.error && <ErrorState message={vehicles.error} onRetry={vehicles.reload} />}
        {vehicles.data && vehicles.data.length === 0 && <EmptyState message={t("fleet.empty")} />}
        {vehicles.data && vehicles.data.length > 0 && (
          <ul className="tool-group__list">
            {vehicles.data.map((v) => (
              <VehicleRow key={v.id} vehicle={v} onDelete={handleDelete} onChanged={vehicles.reload} />
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function VehicleRow({
  vehicle,
  onDelete,
  onChanged,
}: {
  vehicle: Vehicle;
  onDelete: (id: string) => void;
  onChanged: () => void;
}) {
  const [editingLocation, setEditingLocation] = useState(false);
  const [newLocation, setNewLocation] = useState<GeoPoint | null>(vehicle.location ?? null);
  const [addingDest, setAddingDest] = useState(false);
  const [newDest, setNewDest] = useState<GeoPoint | null>(null);
  const [busy, setBusy] = useState(false);

  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    try {
      await fn();
      onChanged();
    } finally {
      setBusy(false);
    }
  }

  return (
    <li className="tool-row">
      <div>
        <div className="tool-row__name">
          {bodyTypeLabel(vehicle.body_type)} · {vehicle.capacity_kg.toLocaleString("ru-RU")} кг
          {vehicle.capacity_m3 > 0 ? ` · ${vehicle.capacity_m3} м³` : ""} · {vehicle.axles} ос.
        </div>
        <div className="tool-row__key">
          {vehicle.length_m} × {vehicle.width_m} × {vehicle.height_m} м
        </div>

        <div className="tool-row__key">
          {t("fleet.location")}: {vehicle.location ? vehicle.location.label : t("fleet.locationNone")}
        </div>

        {/* Назначения (несколько), каждое можно удалить. */}
        {vehicle.destinations.length > 0 && (
          <ul className="fleet-dest-list">
            {vehicle.destinations.map((d) => (
              <li key={d.id} className="fleet-dest-list__item">
                <span>→ {d.point.label}</span>
                <button
                  className="btn btn--ghost btn--sm"
                  disabled={busy}
                  onClick={() => void run(() => deleteVehicleDestination(vehicle.id, d.id))}
                >
                  ×
                </button>
              </li>
            ))}
          </ul>
        )}

        <div className="inline-form" style={{ marginTop: 6 }}>
          <button className="btn btn--secondary btn--sm" onClick={() => setEditingLocation((v) => !v)}>
            {t("fleet.updateLocation")}
          </button>
          <button className="btn btn--secondary btn--sm" onClick={() => setAddingDest((v) => !v)}>
            {t("fleet.addDestination")}
          </button>
        </div>

        {editingLocation && (
          <div style={{ marginTop: 8 }}>
            <GeoPointField title={t("fleet.location")} value={newLocation} onChange={setNewLocation} />
            <button
              className="btn btn--primary btn--sm"
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  await updateVehicleLocation(vehicle.id, newLocation);
                  setEditingLocation(false);
                })
              }
            >
              {t("common.save")}
            </button>
          </div>
        )}

        {addingDest && (
          <div style={{ marginTop: 8 }}>
            <GeoPointField title={t("fleet.destination")} value={newDest} onChange={setNewDest} />
            <button
              className="btn btn--primary btn--sm"
              disabled={busy || !newDest}
              onClick={() =>
                void run(async () => {
                  if (newDest) await addVehicleDestination(vehicle.id, newDest);
                  setNewDest(null);
                  setAddingDest(false);
                })
              }
            >
              {t("common.save")}
            </button>
          </div>
        )}
      </div>
      <button className="btn btn--ghost btn--sm" onClick={() => onDelete(vehicle.id)}>
        {t("fleet.delete")}
      </button>
    </li>
  );
}
