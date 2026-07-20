import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import {
  addVehicle,
  addVehicleDestination,
  createVehicleTrip,
  deleteVehicle,
  deleteVehicleDestination,
  deleteVehicleTrip,
  getVehicles,
  updateVehicleDetails,
  updateVehicleRegistration,
  updateVehicleTrip,
  updateVehicleLocation,
  uploadVehicleDocument,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { ApiError } from "../../api/client";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { DetailModal } from "../../components/common/DetailModal";
import { MultilingualRoute } from "../../components/common/MultilingualLabels";
import { useConfirm } from "../../components/common/ConfirmDialog";
import { BODY_TYPE_KEYS, bodyTypeLabel } from "../../utils/bodyType";
import { pickLabel } from "../../utils/geoLabel";
import { getLocale, t } from "../../i18n";
import type { GeoPoint, Vehicle, VehicleDocumentType, VehicleTrip, VehicleTripStatus } from "../../api/types";

const VEHICLE_DOCUMENT_TYPES: VehicleDocumentType[] = [
  "registration_certificate", "identity_document", "insurance",
  "photo_front", "photo_back", "photo_left", "photo_right",
];

type VehicleTripDraft = Pick<VehicleTrip, "origin" | "destination" | "waypoints" | "can_pickup_en_route">;

function tripHasActiveCargo(trip: VehicleTrip) {
  return trip.status === "loading" || trip.status === "departed" ||
    (trip.status === "planned" && (trip.loaded_volume_m3 > 0 || trip.loaded_weight_kg > 0));
}

// FleetPage — транспорт участника (ТЗ §11.1). Доступ гейтится бэкендом по
// инструменту manage_fleet: без него список отвечает 403 tool_required, и
// страница показывает локализованное сообщение. Местонахождение и назначения
// указываются координатами (по карте); назначений может быть несколько.
export function FleetPage() {
  const vehicles = useAsync(getVehicles, []);
  const confirm = useConfirm();

  const [addFormOpen, setAddFormOpen] = useState(false);
  const [vehicleName, setVehicleName] = useState("");
  const [axles, setAxles] = useState("2");
  const [capacityKg, setCapacityKg] = useState("");
  const [capacityM3, setCapacityM3] = useState("");
  const [length, setLength] = useState("");
  const [width, setWidth] = useState("");
  const [height, setHeight] = useState("");
  // Храним КЛЮЧ кузова, не переведённую строку — чтобы карточка следовала
  // языку интерфейса.
  const [bodyType, setBodyType] = useState<string>("bodyTented");
  const [registrationCountry, setRegistrationCountry] = useState("");
  const [plateNumber, setPlateNumber] = useState("");
  const [vin, setVin] = useState("");
  const [privacyConsent, setPrivacyConsent] = useState(false);
  const [documents, setDocuments] = useState<Partial<Record<VehicleDocumentType, File>>>({});
  const [verificationOpen, setVerificationOpen] = useState(false);
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
    if (!vehicleName.trim()) {
      setError(t("fleet.nameRequired"));
      return;
    }
    const numbers = [Number(axles), Number(capacityKg), Number(length), Number(width), Number(height)];
    if (numbers.some((n) => !Number.isFinite(n) || n <= 0)) {
      setError(t("fleet.numbersPositive"));
      return;
    }
    if (!bodyType.trim()) {
      setError(t("fleet.bodyTypeRequired"));
      return;
    }
    const verificationRequested = Boolean(
      registrationCountry.trim() || plateNumber.trim() || vin.trim() || privacyConsent || Object.keys(documents).length
    );
    if (verificationRequested && (!registrationCountry.trim() || !plateNumber.trim() || vin.trim().length !== 17 || !privacyConsent)) {
      setVerificationOpen(true);
      setError(t("fleet.registrationRequired"));
      return;
    }
    setIsSubmitting(true);
    let createdVehicleId: string | null = null;
    try {
      const vehicle = await addVehicle({
        name: vehicleName.trim(),
        axles: Number(axles),
        capacity_kg: Number(capacityKg),
        capacity_m3: Number(capacityM3) || 0,
        length_m: Number(length),
        width_m: Number(width),
        height_m: Number(height),
        body_type: bodyType,
        registration_country: registrationCountry,
        plate_number: plateNumber,
        vin,
        privacy_consent: privacyConsent,
        location,
        destinations: destinations.filter((d): d is GeoPoint => d !== null),
      });
      createdVehicleId = vehicle.id;
      for (const docType of VEHICLE_DOCUMENT_TYPES) {
        const file = documents[docType];
        if (file) await uploadVehicleDocument(vehicle.id, docType, file);
      }
      setNotice(t("fleet.added"));
      setVehicleName("");
      setCapacityKg("");
      setCapacityM3("");
      setLength("");
      setWidth("");
      setHeight("");
      setLocation(null);
      setDestinations([]);
      setRegistrationCountry("");
      setPlateNumber("");
      setVin("");
      setPrivacyConsent(false);
      setDocuments({});
      setVerificationOpen(false);
      setAddFormOpen(false);
      vehicles.reload();
    } catch (err) {
      if (createdVehicleId) vehicles.reload();
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleDelete(id: string) {
    if (!await confirm({ message: t("fleet.deleteVehicleConfirm"), confirmLabel: t("fleet.delete") })) return;
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

      <section className={`panel fleet-add-panel${addFormOpen ? " fleet-add-panel--open" : ""}`}>
        <button
          className="fleet-add-panel__summary"
          type="button"
          aria-expanded={addFormOpen}
          onClick={() => setAddFormOpen((value) => !value)}
        >
          <h2 className="panel__title">{t("fleet.addTitle")}</h2>
          <span className="fleet-add-panel__chevron" aria-hidden="true">⌄</span>
        </button>
        {addFormOpen && <form className="inline-form inline-form--stacked fleet-add-panel__form" onSubmit={handleAdd} noValidate>
          <label className="field vehicle-name-field">
            <span className="field__label">{t("fleet.name")}</span>
            <input value={vehicleName} maxLength={80} placeholder={t("fleet.namePlaceholder")} onChange={(e) => setVehicleName(e.target.value)} required />
            <small>{t("fleet.nameHint")}</small>
          </label>
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

          <details
            className="vehicle-verification-fields vehicle-verification-disclosure"
            open={verificationOpen}
            onToggle={(event) => setVerificationOpen(event.currentTarget.open)}
          >
            <summary className="vehicle-verification-disclosure__summary">
              <span className="vehicle-verification-disclosure__copy">
                <strong>{t("fleet.verificationCollapsedTitle")}</strong>
                <small>{t("fleet.verificationCollapsedHint")}</small>
              </span>
              <span className="vehicle-verification-disclosure__chevron" aria-hidden="true">⌄</span>
            </summary>
            <div className="vehicle-verification-disclosure__body">
              <p>{t("fleet.verificationPrivacyText")}</p>
              <div className="field-row">
                <label className="field"><span className="field__label">{t("fleet.registrationCountry")}</span><input value={registrationCountry} placeholder="KZ" maxLength={3} onChange={(e) => setRegistrationCountry(e.target.value.toUpperCase())} /></label>
                <label className="field"><span className="field__label">{t("fleet.plateNumber")}</span><input value={plateNumber} onChange={(e) => setPlateNumber(e.target.value.toUpperCase())} /></label>
                <label className="field"><span className="field__label">{t("fleet.vin")}</span><input value={vin} minLength={17} maxLength={17} onChange={(e) => setVin(e.target.value.toUpperCase())} /></label>
              </div>
              <div className="vehicle-document-grid">
                {VEHICLE_DOCUMENT_TYPES.map((docType) => (
                  <VehicleFileField key={docType} type={docType} file={documents[docType]} onChange={(file) => setDocuments((current) => ({ ...current, [docType]: file }))} />
                ))}
              </div>
              <p className="vehicle-verification-fields__hint">{t("fleet.documentsOptionalHint")}</p>
              <p className="vehicle-trust-scale">{t("fleet.trustScaleHint")}</p>
              <label className="vehicle-consent"><input type="checkbox" checked={privacyConsent} onChange={(e) => setPrivacyConsent(e.target.checked)} /><span>{t("fleet.privacyConsent")}</span></label>
            </div>
          </details>

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
        </form>}
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
  const confirm = useConfirm();
  const [expanded, setExpanded] = useState(false);
  const [editingDetails, setEditingDetails] = useState(false);
  const [editingLocation, setEditingLocation] = useState(false);
  const [newLocation, setNewLocation] = useState<GeoPoint | null>(vehicle.location ?? null);
  const [addingDest, setAddingDest] = useState(false);
  const [newDest, setNewDest] = useState<GeoPoint | null>(null);
  const [editingTrip, setEditingTrip] = useState<VehicleTrip | "new" | null>(null);
  const [tripDraft, setTripDraft] = useState<VehicleTripDraft | null>(null);
  const [editingVerification, setEditingVerification] = useState(false);
  const [busy, setBusy] = useState(false);
  const plannedTrips = vehicle.trips.filter((trip) => trip.status !== "completed");
  const committedTrips = plannedTrips.filter(tripHasActiveCargo);
  const committedTrip = committedTrips[0] ?? null;
  const hasActiveTripConflict = committedTrips.length > 1;

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
    <li className={`vehicle-card${expanded ? " vehicle-card--expanded" : ""}`}>
      <button className="vehicle-card__summary" type="button" aria-expanded={expanded} onClick={() => setExpanded((value) => !value)}>
        <span className="vehicle-card__identity">
          <strong>{vehicle.name || `${bodyTypeLabel(vehicle.body_type)} · ${formatNumber(vehicle.capacity_kg)} ${t("fleet.unitKg")}`}</strong>
          <small>
            {bodyTypeLabel(vehicle.body_type)} · {formatNumber(vehicle.capacity_m3)} {t("fleet.unitM3")} · {formatNumber(vehicle.capacity_kg)} {t("fleet.unitKg")}
            {vehicle.masked_plate ? ` · ${vehicle.masked_plate}` : ""}
          </small>
        </span>
        <span className="vehicle-card__route-summary">
          <small>{committedTrip ? t("fleet.activeTripPlan") : t("fleet.availablePlans")}</small>
          <strong>{committedTrip ? <MultilingualRoute origin={committedTrip.origin} destination={committedTrip.destination} /> : plannedTrips.length > 0 ? `${plannedTrips.length} ${t("fleet.directionsCount")}` : t("fleet.noActiveTrip")}</strong>
        </span>
        <span className="vehicle-card__compact-trust">{t("fleet.trust")}: <b>{vehicle.trust_percent}%</b></span>
        <span className="vehicle-card__chevron" aria-hidden="true">⌄</span>
      </button>

      {expanded && <div className="vehicle-card__body">
      <div className="vehicle-card__header">
        <div>
          <div className="tool-row__key">{vehicle.length_m} × {vehicle.width_m} × {vehicle.height_m} {t("fleet.unitM")} · {vehicle.axles} {t("fleet.unitAxles")}</div>
          <div className="tool-row__key">{t("fleet.location")}: {vehicle.location ? pickLabel(vehicle.location.labels, vehicle.location.label) : t("fleet.locationNone")}</div>
        </div>
        <div className="inline-form">
          <button className="btn btn--secondary btn--sm" type="button" onClick={() => setEditingDetails(true)}>{t("fleet.editVehicle")}</button>
          <button className="btn btn--ghost btn--sm" type="button" onClick={() => onDelete(vehicle.id)}>{t("fleet.delete")}</button>
        </div>
      </div>

      <section className="vehicle-trust">
        <div className="vehicle-trust__top">
          <div><strong>{t("fleet.trust")}: {vehicle.trust_percent}%</strong><span>{t(`fleet.verificationStatus.${vehicle.verification_status}`)}</span></div>
          <button className="btn btn--secondary btn--sm" type="button" onClick={() => setEditingVerification(true)}>{t("fleet.manageVerification")}</button>
        </div>
        <div className="vehicle-trust__progress"><span style={{ width: `${vehicle.trust_percent}%` }} /></div>
        <div className="vehicle-trust__badges">
          <span className={vehicle.documents_verified ? "pill pill--green" : "pill pill--neutral"}>{vehicle.documents_verified ? t("fleet.documentsVerified") : t("fleet.documentsNotVerified")}</span>
          <span className={vehicle.has_completed_trips ? "pill pill--green" : "pill pill--neutral"}>{vehicle.has_completed_trips ? t("fleet.historyConfirmed") : t("fleet.historyEmpty")}</span>
          {vehicle.masked_plate && <span className="pill pill--neutral">{vehicle.masked_plate}</span>}
        </div>
        {vehicle.verification_reject_reason && <p className="form-error">{t("fleet.rejectionReason")}: {vehicle.verification_reject_reason}</p>}
      </section>

      {vehicle.destinations.length > 0 && (
        <ul className="fleet-dest-list">
          {vehicle.destinations.map((d) => (
            <li key={d.id} className="fleet-dest-list__item">
              <span>→ {pickLabel(d.point.labels, d.point.label)}</span>
              <button
                className="btn btn--ghost btn--sm"
                disabled={busy}
                onClick={() => void (async () => {
                  if (!await confirm({ message: t("fleet.deleteDestinationConfirm"), confirmLabel: t("fleet.delete") })) return;
                  await run(() => deleteVehicleDestination(vehicle.id, d.id));
                })()}
              >
                ×
              </button>
            </li>
          ))}
        </ul>
      )}

      <div className="inline-form">
        <button className="btn btn--secondary btn--sm" onClick={() => setEditingLocation((v) => !v)}>
          {t("fleet.updateLocation")}
        </button>
        <button className="btn btn--secondary btn--sm" onClick={() => setAddingDest((v) => !v)}>
          {t("fleet.addDestination")}
        </button>
      </div>

      {editingLocation && (
        <div className="vehicle-card__editor">
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
        <div className="vehicle-card__editor">
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

      <section className="vehicle-trips">
        <div className="vehicle-trips__header">
          <div>
            <h3>{t("fleet.tripsTitle")}</h3>
            <p>{t("fleet.tripsHint")}</p>
          </div>
          <button className="btn btn--primary btn--sm" type="button" disabled={Boolean(committedTrip)} onClick={() => { setTripDraft(null); setEditingTrip("new"); }}>
            {t("fleet.addTrip")}
          </button>
        </div>
        {committedTrip ? <p className="vehicle-trips__active-hint">{t("fleet.activeCargoHint")}</p> : plannedTrips.length > 0 && <p className="vehicle-trips__active-hint">{t("fleet.multiplePlansHint")}</p>}
        {hasActiveTripConflict && <div className="vehicle-trip-conflict">{t("fleet.activeTripConflictNotice")}</div>}
        {vehicle.trips.length === 0 ? (
          <p className="vehicle-trips__empty">{t("fleet.tripsEmpty")}</p>
        ) : (
          <div className="vehicle-trip-list">
            {vehicle.trips.map((trip) => (
              <VehicleTripCard
                key={trip.id}
                vehicle={vehicle}
                trip={trip}
                onEdit={() => { setTripDraft(null); setEditingTrip(trip); }}
                temporarilyDisabled={Boolean(committedTrip && committedTrip.id !== trip.id && trip.status !== "completed")}
                canReverse={trip.status === "completed" && !committedTrip}
                onReverse={() => {
                  setTripDraft({
                    origin: trip.destination,
                    destination: trip.origin,
                    waypoints: [...(trip.waypoints ?? [])].reverse(),
                    can_pickup_en_route: trip.can_pickup_en_route,
                  });
                  setEditingTrip("new");
                }}
              />
            ))}
          </div>
        )}
      </section>

      {editingTrip && (
        <DetailModal onClose={() => { setEditingTrip(null); setTripDraft(null); }} wide>
          <VehicleTripForm
            vehicle={vehicle}
            item={editingTrip === "new" ? null : editingTrip}
            draft={editingTrip === "new" ? tripDraft : null}
            onSaved={() => { setEditingTrip(null); setTripDraft(null); onChanged(); }}
          />
        </DetailModal>
      )}
      {editingVerification && (
        <DetailModal onClose={() => setEditingVerification(false)} wide>
          <VehicleVerificationForm vehicle={vehicle} onSaved={() => { setEditingVerification(false); onChanged(); }} />
        </DetailModal>
      )}
      {editingDetails && (
        <DetailModal onClose={() => setEditingDetails(false)}>
          <VehicleDetailsForm vehicle={vehicle} onSaved={() => { setEditingDetails(false); onChanged(); }} />
        </DetailModal>
      )}
      </div>}
    </li>
  );
}

function VehicleDetailsForm({ vehicle, onSaved }: { vehicle: Vehicle; onSaved: () => void }) {
  const [name, setName] = useState(vehicle.name);
  const [axles, setAxles] = useState(String(vehicle.axles));
  const [capacityKg, setCapacityKg] = useState(String(vehicle.capacity_kg));
  const [capacityM3, setCapacityM3] = useState(String(vehicle.capacity_m3));
  const [length, setLength] = useState(String(vehicle.length_m));
  const [width, setWidth] = useState(String(vehicle.width_m));
  const [height, setHeight] = useState(String(vehicle.height_m));
  const [bodyType, setBodyType] = useState(vehicle.body_type);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    const values = {
      axles: Number(axles), capacityKg: Number(capacityKg), capacityM3: Number(capacityM3),
      length: Number(length), width: Number(width), height: Number(height),
    };
    if (!name.trim()) { setError(t("fleet.nameRequired")); return; }
    if (!Number.isInteger(values.axles) || values.axles < 1 || values.axles > 12 ||
      !Number.isFinite(values.capacityKg) || values.capacityKg <= 0 ||
      !Number.isFinite(values.capacityM3) || values.capacityM3 < 0 ||
      !Number.isFinite(values.length) || values.length <= 0 ||
      !Number.isFinite(values.width) || values.width <= 0 ||
      !Number.isFinite(values.height) || values.height <= 0) {
      setError(t("fleet.numbersPositive"));
      return;
    }
    if (vehicle.trips.some((trip) => trip.loaded_weight_kg > values.capacityKg || trip.loaded_volume_m3 > values.capacityM3)) {
      setError(t("fleet.capacityBelowTripLoad"));
      return;
    }
    setBusy(true);
    try {
      await updateVehicleDetails(vehicle.id, {
        name: name.trim(), axles: values.axles, capacity_kg: values.capacityKg, capacity_m3: values.capacityM3,
        length_m: values.length, width_m: values.width, height_m: values.height, body_type: bodyType,
      });
      onSaved();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally { setBusy(false); }
  }

  return (
    <form className="vehicle-details-form" onSubmit={submit}>
      <div><h2 className="detail-panel__title">{t("fleet.editVehicle")}</h2><p className="marketplace-page__hint">{t("fleet.editVehicleHint")}</p></div>
      <label className="field"><span className="field__label">{t("fleet.name")}</span><input value={name} maxLength={80} onChange={(event) => setName(event.target.value)} autoFocus /></label>
      <div className="field-row">
        <label className="field"><span className="field__label">{t("fleet.bodyType")}</span><select value={bodyType} onChange={(event) => setBodyType(event.target.value)}>{BODY_TYPE_KEYS.map((key) => <option value={key} key={key}>{t(`fleet.${key}`)}</option>)}</select></label>
        <label className="field"><span className="field__label">{t("fleet.axles")}</span><input type="number" min={1} max={12} value={axles} onChange={(event) => setAxles(event.target.value)} /></label>
      </div>
      <div className="field-row">
        <label className="field"><span className="field__label">{t("fleet.capacity")}</span><input type="number" min={1} step="any" value={capacityKg} onChange={(event) => setCapacityKg(event.target.value)} /></label>
        <label className="field"><span className="field__label">{t("fleet.capacityM3")}</span><input type="number" min={0} step="any" value={capacityM3} onChange={(event) => setCapacityM3(event.target.value)} /></label>
      </div>
      <div className="field-row">
        <label className="field"><span className="field__label">{t("fleet.length")}</span><input type="number" min={0.1} step="any" value={length} onChange={(event) => setLength(event.target.value)} /></label>
        <label className="field"><span className="field__label">{t("fleet.width")}</span><input type="number" min={0.1} step="any" value={width} onChange={(event) => setWidth(event.target.value)} /></label>
        <label className="field"><span className="field__label">{t("fleet.height")}</span><input type="number" min={0.1} step="any" value={height} onChange={(event) => setHeight(event.target.value)} /></label>
      </div>
      <p className="vehicle-trip-form__note">{t("fleet.editVehicleNote")}</p>
      {error && <div className="form-error">{error}</div>}
      <button className="btn btn--primary" type="submit" disabled={busy}>{busy ? t("common.loading") : t("common.save")}</button>
    </form>
  );
}

function VehicleFileField({ type, file, onChange }: { type: VehicleDocumentType; file?: File; onChange: (file: File | undefined) => void }) {
  return (
    <label className="vehicle-file-field">
      <span>{t(`fleet.vehicleDocument.${type}`)}</span>
      <input type="file" accept={type.startsWith("photo_") ? "image/jpeg,image/png" : "image/jpeg,image/png,application/pdf"} onChange={(event) => onChange(event.target.files?.[0])} />
      {file && <small>{file.name}</small>}
    </label>
  );
}

function VehicleVerificationForm({ vehicle, onSaved }: { vehicle: Vehicle; onSaved: () => void }) {
  const [country, setCountry] = useState(vehicle.registration_country || "KZ");
  const [plate, setPlate] = useState(vehicle.plate_number || "");
  const [vin, setVin] = useState(vehicle.vin || "");
  const [consent, setConsent] = useState(Boolean(vehicle.registration_country));
  const [documents, setDocuments] = useState<Partial<Record<VehicleDocumentType, File>>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    if (!country.trim() || !plate.trim() || vin.trim().length !== 17 || !consent) { setError(t("fleet.registrationRequired")); return; }
    setBusy(true);
    try {
      const changed = country.trim().toUpperCase() !== vehicle.registration_country || plate.trim().toUpperCase().replace(/\s/g, "") !== vehicle.plate_number || vin.trim().toUpperCase() !== vehicle.vin;
      if (changed || !vehicle.registration_country) {
        await updateVehicleRegistration(vehicle.id, { registration_country: country, plate_number: plate, vin, privacy_consent: consent });
      }
      for (const docType of VEHICLE_DOCUMENT_TYPES) {
        const file = documents[docType];
        if (file) await uploadVehicleDocument(vehicle.id, docType, file);
      }
      onSaved();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally { setBusy(false); }
  }

  return (
    <form className="vehicle-verification-form" onSubmit={submit}>
      <div><h2 className="detail-panel__title">{t("fleet.verificationTitle")}</h2><p className="marketplace-page__hint">{t("fleet.verificationPrivacyText")}</p></div>
      <div className="vehicle-trust-summary"><strong>{t("fleet.currentTrust")}: {vehicle.trust_percent}%</strong><span>{t(`fleet.verificationStatus.${vehicle.verification_status}`)}</span></div>
      <div className="field-row">
        <label className="field"><span className="field__label">{t("fleet.registrationCountry")}</span><input value={country} maxLength={3} onChange={(e) => setCountry(e.target.value.toUpperCase())} /></label>
        <label className="field"><span className="field__label">{t("fleet.plateNumber")}</span><input value={plate} onChange={(e) => setPlate(e.target.value.toUpperCase())} /></label>
        <label className="field"><span className="field__label">{t("fleet.vin")}</span><input value={vin} minLength={17} maxLength={17} onChange={(e) => setVin(e.target.value.toUpperCase())} /></label>
      </div>
      <div className="vehicle-document-grid">
        {VEHICLE_DOCUMENT_TYPES.map((docType) => (
          <div key={docType} className="vehicle-document-slot">
            <VehicleFileField type={docType} file={documents[docType]} onChange={(file) => setDocuments((current) => ({ ...current, [docType]: file }))} />
            {vehicle.uploaded_document_types.includes(docType) && <span className="pill pill--green">{t("fleet.fileUploaded")}</span>}
          </div>
        ))}
      </div>
      <p className="vehicle-trust-scale">{t("fleet.trustScaleHint")}</p>
      <label className="vehicle-consent"><input type="checkbox" checked={consent} onChange={(e) => setConsent(e.target.checked)} /><span>{t("fleet.privacyConsent")}</span></label>
      {error && <div className="form-error">{error}</div>}
      <button className="btn btn--primary" type="submit" disabled={busy}>{busy ? t("common.loading") : t("common.save")}</button>
    </form>
  );
}

function VehicleTripCard({ vehicle, trip, onEdit, onReverse, canReverse, temporarilyDisabled }: { vehicle: Vehicle; trip: VehicleTrip; onEdit: () => void; onReverse: () => void; canReverse: boolean; temporarilyDisabled: boolean }) {
  const volumePercent = vehicle.capacity_m3 > 0 ? (trip.loaded_volume_m3 / vehicle.capacity_m3) * 100 : 0;
  const weightPercent = vehicle.capacity_kg > 0 ? (trip.loaded_weight_kg / vehicle.capacity_kg) * 100 : 0;
  const loadPercent = Math.min(100, Math.max(volumePercent, weightPercent));
  const freeVolume = Math.max(0, vehicle.capacity_m3 - trip.loaded_volume_m3);
  const freeWeight = Math.max(0, vehicle.capacity_kg - trip.loaded_weight_kg);
  return (
    <article className={`vehicle-trip-card${temporarilyDisabled ? " vehicle-trip-card--disabled" : ""}`}>
      <button className="vehicle-trip-card__main" type="button" onClick={onEdit}>
        <div className="vehicle-trip-card__top">
          <strong><MultilingualRoute origin={trip.origin} destination={trip.destination} /></strong>
          <span className={`vehicle-trip-card__status vehicle-trip-card__status--${temporarilyDisabled ? "disabled" : trip.status}`}>{temporarilyDisabled ? t("fleet.planTemporarilyDisabled") : t(`fleet.tripStatus.${trip.status}`)}</span>
        </div>
        {trip.waypoints?.length > 0 && <div className="vehicle-trip-card__waypoints">{t("fleet.routeCities")}: {trip.waypoints.map((point) => pickLabel(point.labels, point.label)).join(" → ")}</div>}
        {trip.can_pickup_en_route && <div className="vehicle-trip-card__pickup"><span className="pill pill--green">{t("fleet.enRoutePickupBadge")}</span><span>{t("fleet.pickupRadius")}</span></div>}
        <div className="vehicle-trip-card__date">{t("fleet.departureDate")}: {formatTripDate(trip.departure_date)}</div>
        <div className="vehicle-trip-card__numbers">
          {vehicle.capacity_m3 > 0 && <span>{t("fleet.loaded")}: <b>{formatNumber(trip.loaded_volume_m3)} / {formatNumber(vehicle.capacity_m3)} м³</b></span>}
          <span>{t("fleet.loadedWeight")}: <b>{formatNumber(trip.loaded_weight_kg)} / {formatNumber(vehicle.capacity_kg)} {t("fleet.unitKg")}</b></span>
        </div>
        <div className="vehicle-trip-card__progress"><span style={{ width: `${loadPercent}%` }} /></div>
        <div className="vehicle-trip-card__free">
          <strong>{t("fleet.freeSpace")}</strong>
          {vehicle.capacity_m3 > 0 && <span>{formatNumber(freeVolume)} м³</span>}
          <span>{formatNumber(freeWeight)} {t("fleet.unitKg")}</span>
        </div>
        {temporarilyDisabled && <div className="vehicle-trip-card__disabled-note">{t("fleet.planDisabledHint")}</div>}
      </button>
      {trip.status === "completed" && (
        <button className="btn btn--secondary btn--sm vehicle-trip-card__reverse" type="button" disabled={!canReverse} onClick={onReverse} title={!canReverse ? t("fleet.reverseBlocked") : undefined}>
          <span aria-hidden="true">⇄</span> {t("fleet.reverseTrip")}
        </button>
      )}
    </article>
  );
}

function VehicleTripForm({ vehicle, item, draft, onSaved }: { vehicle: Vehicle; item: VehicleTrip | null; draft: VehicleTripDraft | null; onSaved: () => void }) {
  const confirm = useConfirm();
  const [origin, setOrigin] = useState<GeoPoint | null>(item?.origin ?? draft?.origin ?? vehicle.location ?? null);
  const [destination, setDestination] = useState<GeoPoint | null>(item?.destination ?? draft?.destination ?? vehicle.destinations[0]?.point ?? null);
  const [canPickupEnRoute, setCanPickupEnRoute] = useState(item?.can_pickup_en_route ?? draft?.can_pickup_en_route ?? false);
  const [waypoints, setWaypoints] = useState<(GeoPoint | null)[]>((item?.waypoints ?? draft?.waypoints ?? []).map((point) => point));
  const [departureDate, setDepartureDate] = useState(item?.departure_date.slice(0, 10) ?? "");
  const [loadedVolume, setLoadedVolume] = useState(item ? String(item.loaded_volume_m3) : "0");
  const [loadedWeight, setLoadedWeight] = useState(item ? String(item.loaded_weight_kg) : "0");
  const [status, setStatus] = useState<VehicleTripStatus>(item?.status ?? "planned");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    const volume = Number(loadedVolume);
    const weight = Number(loadedWeight);
    if (!origin || !destination || !departureDate) { setError(t("fleet.tripRequired")); return; }
    const willCommit = status === "loading" || status === "departed" || (status === "planned" && (volume > 0 || weight > 0));
    if (willCommit && vehicle.trips.some((trip) => trip.id !== item?.id && tripHasActiveCargo(trip))) {
      setError(t("fleet.activeTripConflict"));
      return;
    }
    if (!Number.isFinite(volume) || !Number.isFinite(weight) || volume < 0 || weight < 0 || volume > vehicle.capacity_m3 || weight > vehicle.capacity_kg) {
      setError(t("fleet.tripCapacityError")); return;
    }
    setBusy(true);
    try {
      const input = {
        origin, destination,
        waypoints: canPickupEnRoute ? waypoints.filter((point): point is GeoPoint => point !== null) : [],
        can_pickup_en_route: canPickupEnRoute,
        departure_date: departureDate, loaded_volume_m3: volume, loaded_weight_kg: weight, status,
      };
      if (item) await updateVehicleTrip(vehicle.id, item.id, input); else await createVehicleTrip(vehicle.id, input);
      onSaved();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally { setBusy(false); }
  }

  async function remove() {
    if (!item || !await confirm({ message: t("fleet.deleteTripConfirm"), confirmLabel: t("fleet.deleteTrip") })) return;
    setBusy(true);
    try { await deleteVehicleTrip(vehicle.id, item.id); onSaved(); }
    catch (err) { setError(err instanceof ApiError ? err.message : t("common.unexpectedError")); }
    finally { setBusy(false); }
  }

  return (
    <form className="vehicle-trip-form" onSubmit={submit}>
      <div><h2 className="detail-panel__title">{item ? t("fleet.editTrip") : t("fleet.addTrip")}</h2><p className="marketplace-page__hint">{bodyTypeLabel(vehicle.body_type)} · {formatNumber(vehicle.capacity_m3)} м³ · {formatNumber(vehicle.capacity_kg)} {t("fleet.unitKg")}</p></div>
      <div className="field-row"><GeoPointField title={t("fleet.tripOrigin")} value={origin} onChange={setOrigin} /><GeoPointField title={t("fleet.tripDestination")} value={destination} onChange={setDestination} /></div>
      <p className="vehicle-trip-form__note">{t("fleet.tripOriginRadiusHint")}</p>
      <section className="vehicle-trip-pickup">
        <label className="vehicle-trip-pickup__toggle">
          <input type="checkbox" checked={canPickupEnRoute} onChange={(event) => setCanPickupEnRoute(event.target.checked)} />
          <span><strong>{t("fleet.canPickupEnRoute")}</strong><small>{t("fleet.canPickupEnRouteHint")}</small></span>
        </label>
        {canPickupEnRoute && (
          <div className="vehicle-trip-waypoints">
            <div className="vehicle-trip-waypoints__header"><div><strong>{t("fleet.routeCities")}</strong><p>{t("fleet.routeCitiesHint")}</p></div><button className="btn btn--secondary btn--sm" type="button" disabled={waypoints.length >= 12} onClick={() => setWaypoints((current) => [...current, null])}>{t("fleet.addRouteCity")}</button></div>
            {waypoints.map((point, index) => (
              <div className="vehicle-trip-waypoint" key={index}>
                <GeoPointField title={`${t("fleet.routeCity")} ${index + 1}`} value={point} onChange={(value) => setWaypoints((current) => current.map((item, itemIndex) => itemIndex === index ? value : item))} />
                <button className="btn btn--ghost btn--sm" type="button" onClick={() => setWaypoints((current) => current.filter((_, itemIndex) => itemIndex !== index))}>{t("fleet.removeRouteCity")}</button>
              </div>
            ))}
            {waypoints.length === 0 && <p className="vehicle-trip-waypoints__empty">{t("fleet.routeCitiesEmpty")}</p>}
          </div>
        )}
      </section>
      <div className="field-row">
        <label className="field"><span className="field__label">{t("fleet.departureDate")}</span><input type="date" value={departureDate} onChange={(e) => setDepartureDate(e.target.value)} /></label>
        <label className="field"><span className="field__label">{t("fleet.loadedVolume")}</span><input type="number" min="0" max={vehicle.capacity_m3} step="any" value={loadedVolume} onChange={(e) => setLoadedVolume(e.target.value)} /></label>
        <label className="field"><span className="field__label">{t("fleet.loadedWeightField")}</span><input type="number" min="0" max={vehicle.capacity_kg} step="any" value={loadedWeight} onChange={(e) => setLoadedWeight(e.target.value)} /></label>
      </div>
      <p className="vehicle-trip-form__note">{t("fleet.activeTripRule")}</p>
      <label className="field"><span className="field__label">{t("fleet.tripStatusLabel")}</span><select value={status} onChange={(e) => setStatus(e.target.value as VehicleTripStatus)}><option value="planned">{t("fleet.tripStatus.planned")}</option><option value="loading">{t("fleet.tripStatus.loading")}</option><option value="departed">{t("fleet.tripStatus.departed")}</option><option value="completed">{t("fleet.tripStatus.completed")}</option></select></label>
      <p className="vehicle-trip-form__note">{t("fleet.tripLoadHint")}</p>
      {error && <div className="form-error">{error}</div>}
      <div className="inline-form"><button className="btn btn--primary" type="submit" disabled={busy}>{busy ? t("common.loading") : t("common.save")}</button>{item && <button className="btn btn--ghost" type="button" disabled={busy} onClick={() => void remove()}>{t("fleet.deleteTrip")}</button>}</div>
    </form>
  );
}

function formatNumber(value: number) { return value.toLocaleString(getLocale() === "zh" ? "zh-CN" : getLocale() === "en" ? "en-US" : "ru-RU", { maximumFractionDigits: 2 }); }
function formatTripDate(value: string) { return new Intl.DateTimeFormat(getLocale() === "zh" ? "zh-CN" : getLocale() === "en" ? "en-US" : "ru-RU").format(new Date(value)); }
