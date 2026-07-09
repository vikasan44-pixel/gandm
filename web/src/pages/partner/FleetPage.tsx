import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import {
  addVehicle,
  deleteVehicle,
  getVehicles,
  updateVehicleLocation,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { ApiError } from "../../api/client";
import { t } from "../../i18n";
import type { Vehicle } from "../../api/types";

const BODY_TYPES = [
  "bodyTented",
  "bodyFlatbed",
  "bodyLowboy",
  "bodyReefer",
  "bodyContainer",
  "bodyOther",
] as const;

// FleetPage — транспорт участника (ТЗ §11.1). Доступ гейтится бэкендом по
// инструменту manage_fleet: без него список отвечает 403 tool_required, и
// страница показывает локализованное сообщение.
export function FleetPage() {
  const vehicles = useAsync(getVehicles, []);

  const [axles, setAxles] = useState("2");
  const [capacity, setCapacity] = useState("");
  const [length, setLength] = useState("");
  const [width, setWidth] = useState("");
  const [height, setHeight] = useState("");
  const [bodyType, setBodyType] = useState<string>(t("fleet.bodyTented"));
  const [location, setLocation] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleAdd(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    const numbers = [Number(axles), Number(capacity), Number(length), Number(width), Number(height)];
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
        capacity_kg: Number(capacity),
        length_m: Number(length),
        width_m: Number(width),
        height_m: Number(height),
        body_type: bodyType,
        current_location: location,
      });
      setNotice(t("fleet.added"));
      setCapacity("");
      setLength("");
      setWidth("");
      setHeight("");
      setLocation("");
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
              <input type="number" min={1} step="any" value={capacity} onChange={(e) => setCapacity(e.target.value)} required />
            </label>
            <label className="field">
              <span className="field__label">{t("fleet.bodyType")}</span>
              <select value={bodyType} onChange={(e) => setBodyType(e.target.value)}>
                {BODY_TYPES.map((key) => (
                  <option key={key} value={t(`fleet.${key}`)}>
                    {t(`fleet.${key}`)}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <div className="field-row">
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
          <label className="field">
            <span className="field__label">{t("fleet.location")}</span>
            <input value={location} onChange={(e) => setLocation(e.target.value)} placeholder={t("fleet.locationPlaceholder")} />
          </label>
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
  const [location, setLocation] = useState(vehicle.current_location);
  const [isSaving, setIsSaving] = useState(false);

  async function saveLocation() {
    setIsSaving(true);
    try {
      await updateVehicleLocation(vehicle.id, location);
      onChanged();
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <li className="tool-row">
      <div>
        <div className="tool-row__name">
          {vehicle.body_type} · {vehicle.capacity_kg.toLocaleString("ru-RU")} кг ·{" "}
          {vehicle.axles} ос.
        </div>
        <div className="tool-row__key">
          {vehicle.length_m} × {vehicle.width_m} × {vehicle.height_m} м
        </div>
        <div className="inline-form" style={{ marginTop: 6 }}>
          <input
            value={location}
            onChange={(e) => setLocation(e.target.value)}
            placeholder={t("fleet.locationPlaceholder")}
          />
          <button className="btn btn--secondary btn--sm" onClick={() => void saveLocation()} disabled={isSaving}>
            {isSaving ? t("common.loading") : t("fleet.updateLocation")}
          </button>
        </div>
      </div>
      <button className="btn btn--ghost btn--sm" onClick={() => onDelete(vehicle.id)}>
        {t("fleet.delete")}
      </button>
    </li>
  );
}
