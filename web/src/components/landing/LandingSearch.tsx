import { useState } from "react";
import { Link } from "react-router-dom";
import { GeoPointField } from "../geo/GeoPointField";
import {
  searchPublicCargo,
  searchPublicTransport,
  type PublicCargoCard,
  type PublicVehicleCard,
} from "../../api/public";
import { ApiError } from "../../api/client";
import { t } from "../../i18n";
import type { GeoPoint } from "../../api/types";

type Tab = "cargo" | "transport";

// Гостевой поиск на лендинге. Направление задаётся точками на карте (координаты
// из геокодера) — поиск по радиусу, без текстового сопоставления городов.
// Гость видит анонимные карточки; открыть объявление можно только после входа.
export function LandingSearch() {
  const [tab, setTab] = useState<Tab>("cargo");
  const [from, setFrom] = useState<GeoPoint | null>(null);
  const [to, setTo] = useState<GeoPoint | null>(null);

  // Характеристики транспорта (строки — свободный ввод, парсим при поиске).
  const [bodyType, setBodyType] = useState("");
  const [minCapacity, setMinCapacity] = useState("");
  const [minLength, setMinLength] = useState("");
  const [minWidth, setMinWidth] = useState("");
  const [minHeight, setMinHeight] = useState("");
  const [minAxles, setMinAxles] = useState("");

  const [cargo, setCargo] = useState<PublicCargoCard[] | null>(null);
  const [transport, setTransport] = useState<PublicVehicleCard[] | null>(null);
  const [isSearching, setIsSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function switchTab(next: Tab) {
    if (next === tab) return;
    setTab(next);
    setCargo(null);
    setTransport(null);
    setError(null);
  }

  async function handleSearch() {
    setError(null);
    setIsSearching(true);
    try {
      if (tab === "cargo") {
        setCargo(await searchPublicCargo(from, to));
      } else {
        setTransport(
          await searchPublicTransport(
            {
              body_type: bodyType.trim() || undefined,
              min_capacity_kg: Number(minCapacity) || undefined,
              min_length_m: Number(minLength) || undefined,
              min_width_m: Number(minWidth) || undefined,
              min_height_m: Number(minHeight) || undefined,
              min_axles: Number(minAxles) || undefined,
            },
            from,
            to
          )
        );
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSearching(false);
    }
  }

  const results = tab === "cargo" ? cargo : transport;

  return (
    <section className="landing__search panel">
      <h2 className="landing__card-title">{t("landing.search.heading")}</h2>

      <div className="landing-search__tabs">
        <button
          type="button"
          className={"landing-search__tab" + (tab === "cargo" ? " landing-search__tab--active" : "")}
          onClick={() => switchTab("cargo")}
        >
          {t("landing.search.tabCargo")}
        </button>
        <button
          type="button"
          className={"landing-search__tab" + (tab === "transport" ? " landing-search__tab--active" : "")}
          onClick={() => switchTab("transport")}
        >
          {t("landing.search.tabTransport")}
        </button>
      </div>

      <p className="panel__hint">{t("landing.search.hint")}</p>

      <div className="field-row">
        <GeoPointField title={t("landing.search.from")} value={from} onChange={setFrom} />
        <GeoPointField title={t("landing.search.to")} value={to} onChange={setTo} />
      </div>

      {tab === "transport" && (
        <div className="field-row">
          <label className="field">
            <span className="field__label">{t("landing.search.bodyType")}</span>
            <input
              value={bodyType}
              onChange={(e) => setBodyType(e.target.value)}
              placeholder={t("landing.search.anyBody")}
            />
          </label>
          <label className="field">
            <span className="field__label">{t("landing.search.minCapacity")}</span>
            <input type="number" min={0} step="any" value={minCapacity} onChange={(e) => setMinCapacity(e.target.value)} />
          </label>
          <label className="field">
            <span className="field__label">{t("landing.search.minAxles")}</span>
            <input type="number" min={0} value={minAxles} onChange={(e) => setMinAxles(e.target.value)} />
          </label>
          <label className="field">
            <span className="field__label">{t("landing.search.minLength")}</span>
            <input type="number" min={0} step="any" value={minLength} onChange={(e) => setMinLength(e.target.value)} />
          </label>
          <label className="field">
            <span className="field__label">{t("landing.search.minWidth")}</span>
            <input type="number" min={0} step="any" value={minWidth} onChange={(e) => setMinWidth(e.target.value)} />
          </label>
          <label className="field">
            <span className="field__label">{t("landing.search.minHeight")}</span>
            <input type="number" min={0} step="any" value={minHeight} onChange={(e) => setMinHeight(e.target.value)} />
          </label>
        </div>
      )}

      <button className="btn btn--primary" type="button" onClick={() => void handleSearch()} disabled={isSearching}>
        {isSearching ? t("landing.search.searching") : t("landing.search.submit")}
      </button>
      {error && <div className="form-error">{error}</div>}

      {results && (
        <div className="landing-search__results">
          <div className="landing-search__count">
            {tab === "cargo"
              ? `${t("landing.search.resultsCargo")}: ${results.length}`
              : `${t("landing.search.resultsTransport")}: ${results.length}`}
          </div>

          {results.length === 0 && <p className="panel__hint">{t("landing.search.empty")}</p>}

          <ul className="landing-search__list">
            {tab === "cargo"
              ? (cargo ?? []).map((c) => <CargoCard key={c.id} card={c} />)
              : (transport ?? []).map((v) => <TransportCard key={v.id} card={v} />)}
          </ul>

          {results.length > 0 && (
            <div className="landing-search__gate">
              {t("landing.search.loginToView")}{" "}
              <Link to="/login">{t("landing.ctaLogin")}</Link>
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function CargoCard({ card }: { card: PublicCargoCard }) {
  return (
    <li className="public-card">
      <div className="public-card__route">
        {card.origin_label} → {card.destination_label}
      </div>
      <div className="public-card__meta">
        {t("landing.search.volume")}: {card.volume_m3} м³ · {t("landing.search.weight")}: {card.weight_kg} кг
      </div>
      <button className="btn btn--ghost btn--sm" type="button" disabled title={t("landing.search.loginToView")}>
        {t("landing.search.details")}
      </button>
    </li>
  );
}

function TransportCard({ card }: { card: PublicVehicleCard }) {
  return (
    <li className="public-card">
      <div className="public-card__route">
        {card.body_type} · {card.capacity_kg.toLocaleString("ru-RU")} кг · {card.axles} ос.
      </div>
      <div className="public-card__meta">
        {card.length_m} × {card.width_m} × {card.height_m} м
        {card.current_location ? ` · ${t("landing.search.location")}: ${card.current_location}` : ""}
      </div>
      {card.ready_origin_label && card.ready_destination_label && (
        <div className="public-card__meta">
          {t("landing.search.direction")}: {card.ready_origin_label} → {card.ready_destination_label}
        </div>
      )}
      <button className="btn btn--ghost btn--sm" type="button" disabled title={t("landing.search.loginToView")}>
        {t("landing.search.details")}
      </button>
    </li>
  );
}
