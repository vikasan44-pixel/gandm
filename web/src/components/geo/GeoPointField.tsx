import { useEffect, useRef, useState } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import markerIconUrl from "leaflet/dist/images/marker-icon.png";
import markerIcon2xUrl from "leaflet/dist/images/marker-icon-2x.png";
import markerShadowUrl from "leaflet/dist/images/marker-shadow.png";
import type { GeoPoint } from "../../api/types";
import { gcj02ToWgs84, wgs84ToGcj02 } from "../../utils/gcj02";
import { getLocale, t } from "../../i18n";

// Vite doesn't resolve Leaflet's default icon paths — point them at the
// bundled assets explicitly, otherwise markers render as broken images.
const leafletIcon = L.icon({
  iconUrl: markerIconUrl,
  iconRetinaUrl: markerIcon2xUrl,
  shadowUrl: markerShadowUrl,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
});

const SEARCH_DEBOUNCE_MS = 700;
const DEFAULT_CENTER: [number, number] = [43.25, 76.9]; // Алматы (WGS-84)

// Primary basemap: CARTO Voyager (OSM data over Fastly). WGS-84.
const CARTO_URL = "https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}.png";
const CARTO_ATTRIBUTION = "© OpenStreetMap contributors © CARTO";
const CARTO_SUBDOMAINS = "abcd";

// China fallback: CARTO is blocked by the Great Firewall (verified: mainland
// nodes can't reach basemaps.cartocdn.com). Amap raster tiles load in China
// with no API key. They are GCJ-02 ("Mars"), so while this layer is active the
// map's coordinates are treated as GCJ-02 and converted to/from WGS-84 at the
// marker/click boundary (data is always WGS-84).
const AMAP_URL =
  "https://webrd0{s}.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}";
const AMAP_ATTRIBUTION = "© AutoNavi";
const AMAP_SUBDOMAINS = "1234";
const CARTO_FAIL_MS = 6000;

interface SearchResult {
  label: string;
  lat: number;
  lng: number;
  country: string;
}

// Reverse geocode the SAME coordinates in ru/en/zh so the address label reads
// in the viewer's UI language, not the submitter's. Nominatim honours
// accept-language and is global. Best-effort: missing languages just fall back
// at display time. (In China Nominatim may be blocked — the point is still
// placed by coordinates; only the text label degrades.)
async function reverseGeocodeMultilang(
  lat: number,
  lng: number
): Promise<{ labels: Record<string, string>; country: string }> {
  const langs = ["ru", "en", "zh"];
  const labels: Record<string, string> = {};
  let country = "";
  await Promise.all(
    langs.map(async (lang) => {
      try {
        const res = await fetch(
          `https://nominatim.openstreetmap.org/reverse?format=json&accept-language=${lang}&lat=${lat}&lon=${lng}`
        );
        if (!res.ok) return;
        const data = (await res.json()) as {
          display_name?: string;
          address?: { country_code?: string };
        };
        if (data.display_name) labels[lang] = data.display_name;
        if (!country && data.address?.country_code) {
          country = data.address.country_code.toLowerCase();
        }
      } catch {
        /* ignore — this language just won't be available */
      }
    })
  );
  return { labels, country };
}

interface GeoPointFieldProps {
  title: string;
  value: GeoPoint | null;
  onChange: (value: GeoPoint | null) => void;
}

// Address search + map + draggable marker. CARTO Voyager tiles by default
// (WGS-84, one coordinate system worldwide); auto-falls back to Amap tiles
// where CARTO is blocked (China), converting GCJ-02 ↔ WGS-84 only while that
// layer is active. Clicking/dragging captures coordinates directly.
export function GeoPointField({ title, value, onChange }: GeoPointFieldProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);

  const mapContainerRef = useRef<HTMLDivElement | null>(null);
  const leafletMapRef = useRef<L.Map | null>(null);
  const leafletMarkerRef = useRef<L.Marker | null>(null);
  const chinaModeRef = useRef(false);
  // Bridge for handleResultClick to place a marker via the map effect's
  // closures (which know the current display coordinate system).
  const markerApiRef = useRef<{
    setMarker: (dispLat: number, dispLng: number) => void;
    toDisplay: (wgsLat: number, wgsLng: number) => { lat: number; lng: number };
  } | null>(null);

  // Latest onChange/value without re-initializing the map on every render.
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const valueRef = useRef(value);
  valueRef.current = value;

  function emitPoint(
    latWgs: number,
    lngWgs: number,
    country: string,
    label?: string,
    labels?: Record<string, string>
  ) {
    onChangeRef.current({
      lat: latWgs,
      lng: lngWgs,
      label:
        label ??
        valueRef.current?.label ??
        `${latWgs.toFixed(5)}, ${lngWgs.toFixed(5)}`,
      source: "osm",
      country,
      labels: labels ?? valueRef.current?.labels,
    });
  }

  // Emit the point immediately (with whatever label we have), then refine it
  // with ru/en/zh labels + country from the geocoder. Skipped if the user
  // moved the point elsewhere before the reverse-geocode returns.
  function emitWithMultilang(
    latWgs: number,
    lngWgs: number,
    country: string,
    immediateLabel?: string
  ) {
    emitPoint(latWgs, lngWgs, country, immediateLabel);
    void reverseGeocodeMultilang(latWgs, lngWgs).then((r) => {
      const current = valueRef.current;
      if (!current || current.lat !== latWgs || current.lng !== lngWgs) return;
      const primary =
        r.labels[getLocale()] || immediateLabel || r.labels.en || current.label;
      emitPoint(latWgs, lngWgs, r.country || country, primary, r.labels);
    });
  }

  useEffect(() => {
    if (!mapContainerRef.current) return;

    const map = L.map(mapContainerRef.current).setView(DEFAULT_CENTER, 5);
    const carto = L.tileLayer(CARTO_URL, {
      attribution: CARTO_ATTRIBUTION,
      subdomains: CARTO_SUBDOMAINS,
      maxZoom: 20,
    }).addTo(map);

    // In China mode the map coordinates are GCJ-02; elsewhere they're WGS-84.
    const toDisplay = (latWgs: number, lngWgs: number) =>
      chinaModeRef.current ? wgs84ToGcj02(latWgs, lngWgs) : { lat: latWgs, lng: lngWgs };
    const toWgs = (dispLat: number, dispLng: number) =>
      chinaModeRef.current ? gcj02ToWgs84(dispLat, dispLng) : { lat: dispLat, lng: dispLng };

    function setMarker(dispLat: number, dispLng: number) {
      if (leafletMarkerRef.current) {
        leafletMarkerRef.current.setLatLng([dispLat, dispLng]);
      } else {
        const marker = L.marker([dispLat, dispLng], { draggable: true, icon: leafletIcon }).addTo(map);
        marker.on("dragend", () => {
          const pos = marker.getLatLng();
          const w = toWgs(pos.lat, pos.lng);
          emitWithMultilang(w.lat, w.lng, "");
        });
        leafletMarkerRef.current = marker;
      }
    }

    let cartoReachable = false;
    let switched = false;
    function switchToChina() {
      if (switched) return;
      switched = true;
      chinaModeRef.current = true;
      map.removeLayer(carto);
      L.tileLayer(AMAP_URL, {
        attribution: AMAP_ATTRIBUTION,
        subdomains: AMAP_SUBDOMAINS,
        maxZoom: 18,
      }).addTo(map);
      // Re-place the marker/view in GCJ-02 so it aligns with Amap tiles.
      const v = valueRef.current;
      const center = v ? wgs84ToGcj02(v.lat, v.lng) : wgs84ToGcj02(DEFAULT_CENTER[0], DEFAULT_CENTER[1]);
      if (v && leafletMarkerRef.current) leafletMarkerRef.current.setLatLng([center.lat, center.lng]);
      map.setView([center.lat, center.lng], map.getZoom());
    }

    carto.on("tileload", () => {
      cartoReachable = true;
    });
    carto.on("tileerror", () => {
      if (!cartoReachable) switchToChina();
    });
    const failTimer = setTimeout(() => {
      if (!cartoReachable) switchToChina();
    }, CARTO_FAIL_MS);

    map.on("click", (e: L.LeafletMouseEvent) => {
      setMarker(e.latlng.lat, e.latlng.lng);
      const w = toWgs(e.latlng.lat, e.latlng.lng);
      emitWithMultilang(w.lat, w.lng, "");
    });

    const existing = valueRef.current;
    if (existing) {
      const d = toDisplay(existing.lat, existing.lng);
      setMarker(d.lat, d.lng);
      map.setView([d.lat, d.lng], 10);
    }

    leafletMapRef.current = map;
    markerApiRef.current = { setMarker, toDisplay };
    return () => {
      clearTimeout(failTimer);
      map.remove();
      leafletMapRef.current = null;
      leafletMarkerRef.current = null;
      markerApiRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const q = query.trim();
    if (q.length < 3) {
      setResults([]);
      return;
    }

    const timer = setTimeout(async () => {
      setIsSearching(true);
      setSearchError(null);
      try {
        const res = await fetch(
          `https://nominatim.openstreetmap.org/search?format=json&limit=5&addressdetails=1&accept-language=${getLocale()}&q=${encodeURIComponent(q)}`
        );
        if (!res.ok) throw new Error(`nominatim ${res.status}`);
        const data = (await res.json()) as {
          display_name: string;
          lat: string;
          lon: string;
          address?: { country_code?: string };
        }[];
        setResults(
          data.map((item) => ({
            label: item.display_name,
            lat: Number(item.lat),
            lng: Number(item.lon),
            country: (item.address?.country_code ?? "").toLowerCase(),
          }))
        );
      } catch {
        setSearchError(t("geo.searchError"));
        setResults([]);
      } finally {
        setIsSearching(false);
      }
    }, SEARCH_DEBOUNCE_MS);

    return () => clearTimeout(timer);
  }, [query]);

  function handleResultClick(result: SearchResult) {
    setResults([]);
    setQuery("");
    const map = leafletMapRef.current;
    const api = markerApiRef.current;
    if (map && api) {
      const d = api.toDisplay(result.lat, result.lng);
      map.setView([d.lat, d.lng], 11);
      api.setMarker(d.lat, d.lng);
    }
    emitWithMultilang(result.lat, result.lng, result.country, result.label);
  }

  return (
    <div className="geo-field">
      <div className="geo-field__header">
        <span className="field__label">{title}</span>
      </div>

      <input
        placeholder={t("geo.searchPlaceholder")}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
      />
      {isSearching && <div className="geo-field__hint">{t("common.loading")}</div>}
      {searchError && <div className="form-error">{searchError}</div>}
      {results.length > 0 && (
        <ul className="geo-results">
          {results.map((result, i) => (
            <li key={i}>
              <button type="button" onClick={() => handleResultClick(result)}>
                {result.label}
              </button>
            </li>
          ))}
        </ul>
      )}

      <div ref={mapContainerRef} className="geo-map" />

      {value ? (
        <div className="geo-field__chosen">
          <input
            className="geo-field__label-input"
            value={value.label}
            placeholder={t("geo.labelPlaceholder")}
            onChange={(e) => onChange({ ...value, label: e.target.value })}
          />
          <span className="geo-field__coords">
            {value.lat.toFixed(5)}, {value.lng.toFixed(5)} ·{" "}
            {value.country ? value.country.toUpperCase() : "??"} · WGS-84
          </span>
        </div>
      ) : (
        <div className="geo-field__hint">{t("geo.pickHint")}</div>
      )}
    </div>
  );
}
