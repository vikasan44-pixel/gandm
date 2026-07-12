import { useEffect, useRef, useState } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import markerIconUrl from "leaflet/dist/images/marker-icon.png";
import markerIcon2xUrl from "leaflet/dist/images/marker-icon-2x.png";
import markerShadowUrl from "leaflet/dist/images/marker-shadow.png";
import type { GeoPoint } from "../../api/types";
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
const DEFAULT_CENTER: [number, number] = [43.25, 76.9]; // Алматы

// CARTO Voyager basemap (OSM data, Voyager style) served over Fastly's global
// CDN — unlike tile.openstreetmap.org it isn't blocked by the Great Firewall,
// so the map loads inside China too. Everything is WGS-84, so one picker and
// one coordinate system worldwide — no Amap, no GCJ-02 conversion.
const TILE_URL = "https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}.png";
const TILE_ATTRIBUTION = "© OpenStreetMap contributors © CARTO";
const TILE_SUBDOMAINS = "abcd";

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

// Address search + map + draggable marker on a single WGS-84 Leaflet map
// (CARTO Voyager tiles + Nominatim geocoder). Clicking or dragging captures
// coordinates directly; the label is refined into ru/en/zh asynchronously.
export function GeoPointField({ title, value, onChange }: GeoPointFieldProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);

  const mapContainerRef = useRef<HTMLDivElement | null>(null);
  const leafletMapRef = useRef<L.Map | null>(null);
  const leafletMarkerRef = useRef<L.Marker | null>(null);

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
    L.tileLayer(TILE_URL, {
      attribution: TILE_ATTRIBUTION,
      subdomains: TILE_SUBDOMAINS,
      maxZoom: 20,
    }).addTo(map);

    function setMarker(lat: number, lng: number) {
      if (leafletMarkerRef.current) {
        leafletMarkerRef.current.setLatLng([lat, lng]);
      } else {
        const marker = L.marker([lat, lng], { draggable: true, icon: leafletIcon }).addTo(map);
        marker.on("dragend", () => {
          const pos = marker.getLatLng();
          emitWithMultilang(pos.lat, pos.lng, "");
        });
        leafletMarkerRef.current = marker;
      }
    }

    map.on("click", (e: L.LeafletMouseEvent) => {
      setMarker(e.latlng.lat, e.latlng.lng);
      emitWithMultilang(e.latlng.lat, e.latlng.lng, "");
    });

    const existing = valueRef.current;
    if (existing) {
      setMarker(existing.lat, existing.lng);
      map.setView([existing.lat, existing.lng], 10);
    }

    leafletMapRef.current = map;
    return () => {
      map.remove();
      leafletMapRef.current = null;
      leafletMarkerRef.current = null;
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
    leafletMapRef.current?.setView([result.lat, result.lng], 11);
    if (leafletMarkerRef.current) {
      leafletMarkerRef.current.setLatLng([result.lat, result.lng]);
    } else if (leafletMapRef.current) {
      const marker = L.marker([result.lat, result.lng], {
        draggable: true,
        icon: leafletIcon,
      }).addTo(leafletMapRef.current);
      marker.on("dragend", () => {
        const pos = marker.getLatLng();
        emitWithMultilang(pos.lat, pos.lng, "");
      });
      leafletMarkerRef.current = marker;
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
