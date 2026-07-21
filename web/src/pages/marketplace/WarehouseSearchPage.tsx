import { useState } from "react";
import { GeoPointField } from "../../components/geo/GeoPointField";
import { DetailModal } from "../../components/common/DetailModal";
import { LoadingState } from "../../components/common/LoadingState";
import { EmptyState } from "../../components/common/EmptyState";
import { Pagination, SEARCH_PAGE_SIZE } from "../../components/common/Pagination";
import { searchWarehouses } from "../../api/participant";
import { ApiError } from "../../api/client";
import { pickLabel } from "../../utils/geoLabel";
import { t } from "../../i18n";
import type { GeoPoint, PaginatedResponse, PublicWarehouseCard } from "../../api/types";

export function WarehouseSearchPage() {
  const [point, setPoint] = useState<GeoPoint | null>(null);
  const [radius, setRadius] = useState("100");
	const [results, setResults] = useState<PaginatedResponse<PublicWarehouseCard> | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [upsell, setUpsell] = useState(false);
  const [page, setPage] = useState(1);

	async function search(requestedPage = 1) {
    setError(null);
    if (!point) {
      setError(t("warehouseSearch.pointRequired"));
      return;
    }
    const r = Number(radius);
    if (!(r > 0)) {
      setError(t("warehouseSearch.radiusPositive"));
      return;
    }
    setLoading(true);
		try {
			const response = await searchWarehouses(point, r, requestedPage);
			setResults(response);
			setPage(response.page);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("warehouseSearch.searchFailed"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="page marketplace-page">
      <div>
        <h1 className="page__title">{t("marketplace.warehouseTitle")}</h1>
        <p className="marketplace-page__hint">
          {t("warehouseSearch.hint")}
        </p>
      </div>

      <section className="panel warehouse-search__form">
        <GeoPointField title={t("warehouseSearch.searchPoint")} value={point} onChange={setPoint} />
        <div className="warehouse-search__actions">
          <label className="field warehouse-search__radius">
            <span className="field__label">{t("warehouseSearch.radius")}</span>
            <input className="field__input" type="number" value={radius} onChange={(e) => setRadius(e.target.value)} />
          </label>
		<button className="btn btn--primary warehouse-search__submit" type="button" onClick={() => void search(1)} disabled={loading}>
            {loading ? t("warehouseSearch.searching") : t("warehouseSearch.search")}
          </button>
        </div>
        {error && <div className="form-error">{error}</div>}
      </section>

      {loading && <LoadingState />}
	  {results && results.items.length === 0 && !loading && (
        <EmptyState message={t("warehouseSearch.empty")} />
      )}

	  {results && results.items.length > 0 && (
        <section className="landing-search__results">
		  <div className="landing-search__count">{t("warehouseSearch.found")}: {results.total}</div>
          <ul className="landing-search__list">
			{results.items.map((wh) => (
              <WarehouseCard key={wh.id} wh={wh} onWantContacts={() => setUpsell(true)} />
            ))}
          </ul>
		  <Pagination page={page} pageSize={SEARCH_PAGE_SIZE} totalItems={results.total} onPageChange={(next) => void search(next)} />
        </section>
      )}

      {upsell && <SubscriptionUpsell onClose={() => setUpsell(false)} />}
    </div>
  );
}

function WarehouseCard({ wh, onWantContacts }: { wh: PublicWarehouseCard; onWantContacts: () => void }) {
  const [showAllServices, setShowAllServices] = useState(false);
  const services = wh.services ?? [];
  const visibleServices = showAllServices ? services : services.slice(0, 6);

  return (
    <li className="public-card">
      <div className="public-card__route">{wh.name}</div>
      <div className="public-card__meta">{pickLabel(wh.address.labels, wh.address.label)}</div>
      <div className="public-card__meta">
        {t("warehouseSearch.coveredArea")}: {wh.available_covered_area_m2}/{wh.covered_area_m2} {t("fleet.unitM2")} · {t("warehouseSearch.openArea")}: {wh.available_open_area_m2}/{wh.open_area_m2} {t("fleet.unitM2")}
      </div>
      <div className="public-card__meta">
        {t("warehouseSearch.max")}: {wh.max_weight_kg.toLocaleString()} {t("fleet.unitKg")} · {wh.max_volume_m3} {t("fleet.unitM3")}
        {wh.work_hours ? ` · ${wh.work_hours}` : ""}
      </div>
      {services.length > 0 && (
        <div className="warehouse-search__services">
          <strong>{t("warehouseSearch.services")}</strong>
          <div className="warehouse-search__service-list">
            {visibleServices.map((service) => (
              <span className="pill pill--neutral" key={service}>{warehouseServiceLabel(service)}</span>
            ))}
            {services.length > 6 && (
              <button className="warehouse-search__services-toggle" type="button" onClick={() => setShowAllServices((value) => !value)}>
                {showAllServices
                  ? t("warehouseSearch.hideServices")
                  : `${t("warehouseSearch.showAllServices")} (+${services.length - 6})`}
              </button>
            )}
          </div>
        </div>
      )}
      <div className="public-card__trust">
        {wh.consolidation_enabled && <span className="pill pill--green">{t("warehouseSearch.consolidation")}</span>}
        {wh.pickup_enabled && <span className="pill pill--green">{t("warehouseSearch.pickup")} {wh.pickup_radius_km} {t("fleet.unitKm")}</span>}
        {wh.own_transport && <span className="pill pill--neutral">{t("warehouseSearch.ownTransport")}</span>}
      </div>
      <button className="btn btn--ghost btn--sm" type="button" onClick={onWantContacts}>
        {t("warehouseSearch.showContacts")}
      </button>
    </li>
  );
}

function warehouseServiceLabel(service: string) {
  const key = `warehouses.services.${service}`;
  const label = t(key);
  return label === key ? t("warehouseSearch.otherService") : label;
}

// SubscriptionUpsell — заглушка вместо контактов склада: контакты по подписке.
function SubscriptionUpsell({ onClose }: { onClose: () => void }) {
  return (
    <DetailModal onClose={onClose}>
      <h2 className="detail-panel__title">{t("warehouseSearch.upsellTitle")}</h2>
      <p className="panel__hint">{t("warehouseSearch.upsellText")}</p>
      <ul className="feature-list">
        <li>{t("warehouseSearch.upsellBullet1")}</li>
        <li>{t("warehouseSearch.upsellBullet2")}</li>
        <li>{t("warehouseSearch.upsellBullet3")}</li>
      </ul>
      <div className="btn-group" style={{ marginTop: 12 }}>
        <button className="btn btn--primary" type="button" disabled title={t("warehouseSearch.subscribeSoon")}>
          {t("warehouseSearch.subscribeSoon")}
        </button>
        <button className="btn btn--ghost" type="button" onClick={onClose}>
          {t("warehouseSearch.close")}
        </button>
      </div>
    </DetailModal>
  );
}
