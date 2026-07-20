import { useEffect, useState } from "react";
import { DetailModal } from "../common/DetailModal";
import { CurrencySelect } from "../common/CurrencySelect";
import { GeoPointField } from "../geo/GeoPointField";
import { getMyCargo, sendTransportProposal } from "../../api/participant";
import { ApiError } from "../../api/client";
import { DEFAULT_CURRENCY } from "../../utils/currency";
import { t } from "../../i18n";
import type { CargoRequest, GeoPoint } from "../../api/types";

interface Place {
  length_m: string;
  width_m: string;
  height_m: string;
}

const emptyPlace: Place = { length_m: "", width_m: "", height_m: "" };

// SendProposalModal lets a cargo owner send a priced inquiry to a specific
// vehicle: either reusing one of their posted cargo requests, or typing the
// cargo in by hand (name, route, volume/weight, per-place dimensions, date).
export function SendProposalModal({
  vehicleId,
  vehicleLabel,
  onClose,
  onSent,
}: {
  vehicleId: string;
  vehicleLabel: string;
  onClose: () => void;
  onSent: () => void;
}) {
  const [mode, setMode] = useState<"request" | "manual">("manual");
  const [myCargo, setMyCargo] = useState<CargoRequest[]>([]);
  const [cargoRequestId, setCargoRequestId] = useState<string>("");

  const [cargoName, setCargoName] = useState("");
  const [origin, setOrigin] = useState<GeoPoint | null>(null);
  const [destination, setDestination] = useState<GeoPoint | null>(null);
  const [volume, setVolume] = useState("");
  const [weight, setWeight] = useState("");
  const [pickupDate, setPickupDate] = useState("");
  const [currency, setCurrency] = useState<string>(DEFAULT_CURRENCY);
  const [places, setPlaces] = useState<Place[]>([{ ...emptyPlace }]);

  const [error, setError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);

  useEffect(() => {
    getMyCargo()
      .then((list) => setMyCargo(list.filter((c) => c.status === "open")))
      .catch(() => setMyCargo([]));
  }, []);

  function setPlace(i: number, patch: Partial<Place>) {
    setPlaces((prev) => prev.map((p, idx) => (idx === i ? { ...p, ...patch } : p)));
  }

  async function submit() {
    setError(null);
    const items = places
      .map((p) => ({
        length_m: Number(p.length_m) || 0,
        width_m: Number(p.width_m) || 0,
        height_m: Number(p.height_m) || 0,
      }))
      .filter((p) => p.length_m > 0 || p.width_m > 0 || p.height_m > 0);

    try {
      setSending(true);
      if (mode === "request") {
        if (!cargoRequestId) {
          setError(t("proposals.chooseRequest"));
          return;
        }
        await sendTransportProposal(vehicleId, {
          cargo_request_id: cargoRequestId,
          origin: {} as GeoPoint,
          destination: {} as GeoPoint,
          cargo_name: cargoName,
          volume_m3: 0,
          weight_kg: 0,
          pickup_date: pickupDate,
          currency,
          items,
        });
      } else {
        if (!origin || !destination) {
          setError(t("proposals.directionRequired"));
          return;
        }
        if (!(Number(volume) > 0) || !(Number(weight) > 0)) {
          setError(t("proposals.sizesPositive"));
          return;
        }
        await sendTransportProposal(vehicleId, {
          origin,
          destination,
          cargo_name: cargoName,
          volume_m3: Number(volume),
          weight_kg: Number(weight),
          pickup_date: pickupDate,
          currency,
          items,
        });
      }
      onSent();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("proposals.sendFailed"));
    } finally {
      setSending(false);
    }
  }

  return (
    <DetailModal onClose={onClose} wide>
      <h2 className="detail-panel__title">{t("proposals.modalTitle")}</h2>
      <p className="panel__hint">{vehicleLabel}</p>

      <div className="btn-group" style={{ marginBottom: 12 }}>
        <button
          type="button"
          className={"btn btn--sm " + (mode === "manual" ? "btn--primary" : "btn--ghost")}
          onClick={() => setMode("manual")}
        >
          {t("proposals.manual")}
        </button>
        <button
          type="button"
          className={"btn btn--sm " + (mode === "request" ? "btn--primary" : "btn--ghost")}
          onClick={() => setMode("request")}
        >
          {t("proposals.fromRequests")}
        </button>
      </div>

      {mode === "request" ? (
        <label className="field">
          <span className="field__label">{t("proposals.myRequest")}</span>
          <select className="field__input" value={cargoRequestId} onChange={(e) => setCargoRequestId(e.target.value)}>
            <option value="">{t("proposals.choosePlaceholder")}</option>
            {myCargo.map((c) => (
              <option key={c.id} value={c.id}>
                {c.origin.label} → {c.destination.label} · {c.volume_m3} {t("fleet.unitM3")} · {c.weight_kg} {t("fleet.unitKg")}
              </option>
            ))}
          </select>
          {myCargo.length === 0 && <span className="field__hint">{t("proposals.noOpenRequests")}</span>}
        </label>
      ) : (
        <>
          <label className="field">
            <span className="field__label">{t("proposals.cargoName")}</span>
            <input className="field__input" value={cargoName} onChange={(e) => setCargoName(e.target.value)} placeholder={t("proposals.cargoNamePlaceholder")} />
          </label>
          <div className="form-grid">
            <GeoPointField title={t("proposals.from")} value={origin} onChange={setOrigin} />
            <GeoPointField title={t("proposals.to")} value={destination} onChange={setDestination} />
          </div>
          <div className="form-grid">
            <label className="field">
              <span className="field__label">{t("proposals.volume")}</span>
              <input className="field__input" type="number" value={volume} onChange={(e) => setVolume(e.target.value)} />
            </label>
            <label className="field">
              <span className="field__label">{t("proposals.weight")}</span>
              <input className="field__input" type="number" value={weight} onChange={(e) => setWeight(e.target.value)} />
            </label>
          </div>
        </>
      )}

      <div className="form-grid">
        <label className="field">
          <span className="field__label">{t("proposals.date")}</span>
          <input className="field__input" value={pickupDate} onChange={(e) => setPickupDate(e.target.value)} placeholder={t("proposals.datePlaceholder")} />
        </label>
        <label className="field">
          <span className="field__label">{t("common.currency")}</span>
          <CurrencySelect value={currency} onChange={setCurrency} ariaLabel={t("common.currency")} />
        </label>
      </div>

      <div className="field">
        <span className="field__label">{t("proposals.places")}</span>
        {places.map((p, i) => (
          <div className="form-grid form-grid--3" key={i} style={{ marginBottom: 6 }}>
            <input className="field__input" type="number" placeholder={t("proposals.length")} value={p.length_m} onChange={(e) => setPlace(i, { length_m: e.target.value })} />
            <input className="field__input" type="number" placeholder={t("proposals.width")} value={p.width_m} onChange={(e) => setPlace(i, { width_m: e.target.value })} />
            <input className="field__input" type="number" placeholder={t("proposals.height")} value={p.height_m} onChange={(e) => setPlace(i, { height_m: e.target.value })} />
          </div>
        ))}
        <div className="btn-group">
          <button type="button" className="btn btn--ghost btn--sm" onClick={() => setPlaces((p) => [...p, { ...emptyPlace }])}>
            {t("proposals.addPlace")}
          </button>
          {places.length > 1 && (
            <button type="button" className="btn btn--ghost btn--sm" onClick={() => setPlaces((p) => p.slice(0, -1))}>
              {t("proposals.removePlace")}
            </button>
          )}
        </div>
      </div>

      {error && <div className="form-error">{error}</div>}

      <div className="btn-group" style={{ marginTop: 12 }}>
        <button type="button" className="btn btn--primary" onClick={() => void submit()} disabled={sending}>
          {sending ? t("proposals.sending") : t("proposals.send")}
        </button>
        <button type="button" className="btn btn--ghost" onClick={onClose}>
          {t("proposals.cancel")}
        </button>
      </div>
    </DetailModal>
  );
}
