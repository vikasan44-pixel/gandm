import { useEffect, useRef, useState } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import markerIconUrl from "leaflet/dist/images/marker-icon.png";
import markerIcon2xUrl from "leaflet/dist/images/marker-icon-2x.png";
import markerShadowUrl from "leaflet/dist/images/marker-shadow.png";
import type { GeoPoint } from "../../api/types";
import { gcj02ToWgs84 } from "../../utils/gcj02";
import { t } from "../../i18n";

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
const OSM_DEFAULT_CENTER: [number, number] = [43.25, 76.9]; // Алматы
const AMAP_DEFAULT_CENTER: [number, number] = [43.82, 87.61]; // Урумчи (lat, lng)

interface SearchResult {
  label: string;
  lat: number;
  lng: number;
  country: string;
}

// Reverse geocode via Nominatim to get an address label and, critically,
// the country code — the backend picks the matching radius by country.
// Failure degrades to unknown country ("" → default radius), never blocks
// point placement.
async function reverseGeocodeOsm(
  lat: number,
  lng: number
): Promise<{ label: string | null; country: string }> {
  try {
    const res = await fetch(
      `https://nominatim.openstreetmap.org/reverse?format=json&lat=${lat}&lon=${lng}`
    );
    if (!res.ok) throw new Error(`nominatim ${res.status}`);
    const data = (await res.json()) as {
      display_name?: string;
      address?: { country_code?: string };
    };
    return {
      label: data.display_name ?? null,
      country: (data.address?.country_code ?? "").toLowerCase(),
    };
  } catch {
    return { label: null, country: "" };
  }
}

type Provider = "osm" | "amap";

interface GeoPointFieldProps {
  title: string;
  value: GeoPoint | null;
  onChange: (value: GeoPoint | null) => void;
}

// Address search + map + draggable marker. OSM (Leaflet + Nominatim) for
// Kazakhstan and everything else; Amap for China. Amap returns GCJ-02 —
// converted to WGS-84 right here, before the value ever leaves the
// component. The API receives WGS-84 only.
export function GeoPointField({ title, value, onChange }: GeoPointFieldProps) {
  const [provider, setProvider] = useState<Provider>("osm");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);

  const amapKey = import.meta.env.VITE_AMAP_KEY ?? "";

  const mapContainerRef = useRef<HTMLDivElement | null>(null);
  const leafletMapRef = useRef<L.Map | null>(null);
  const leafletMarkerRef = useRef<L.Marker | null>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const amapMapRef = useRef<any>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const amapMarkerRef = useRef<any>(null);

  // Latest onChange/value without re-initializing maps on every render.
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const valueRef = useRef(value);
  valueRef.current = value;

  function emitPoint(
    latWgs: number,
    lngWgs: number,
    source: Provider,
    country: string,
    label?: string
  ) {
    onChangeRef.current({
      lat: latWgs,
      lng: lngWgs,
      label:
        label ??
        valueRef.current?.label ??
        `${latWgs.toFixed(5)}, ${lngWgs.toFixed(5)}`,
      source,
      country,
    });
  }

  // Place the point immediately (country unknown), then refine label and
  // country asynchronously from the reverse geocoder. Only the label the
  // user hasn't overridden is refreshed — coords stay as placed.
  function emitOsmPointWithReverse(latWgs: number, lngWgs: number) {
    emitPoint(latWgs, lngWgs, "osm", "");
    void reverseGeocodeOsm(latWgs, lngWgs).then((r) => {
      const current = valueRef.current;
      // Skip the update if the user already moved the point elsewhere.
      if (!current || current.lat !== latWgs || current.lng !== lngWgs) return;
      emitPoint(latWgs, lngWgs, "osm", r.country, r.label ?? undefined);
    });
  }

  // --- OSM (Leaflet) ---
  useEffect(() => {
    if (provider !== "osm" || !mapContainerRef.current) return;

    const map = L.map(mapContainerRef.current).setView(OSM_DEFAULT_CENTER, 5);
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      attribution: "© OpenStreetMap contributors",
    }).addTo(map);

    function setMarker(lat: number, lng: number) {
      if (leafletMarkerRef.current) {
        leafletMarkerRef.current.setLatLng([lat, lng]);
      } else {
        const marker = L.marker([lat, lng], { draggable: true, icon: leafletIcon }).addTo(map);
        marker.on("dragend", () => {
          const pos = marker.getLatLng();
          emitOsmPointWithReverse(pos.lat, pos.lng);
        });
        leafletMarkerRef.current = marker;
      }
    }

    map.on("click", (e: L.LeafletMouseEvent) => {
      setMarker(e.latlng.lat, e.latlng.lng);
      emitOsmPointWithReverse(e.latlng.lat, e.latlng.lng);
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
  }, [provider]);

  // --- Amap ---
  useEffect(() => {
    if (provider !== "amap" || !amapKey || !mapContainerRef.current) return;

    let disposed = false;
    loadAmap(amapKey)
      .then((AMap) => {
        if (disposed || !mapContainerRef.current) return;

        const map = new AMap.Map(mapContainerRef.current, {
          zoom: 5,
          center: [AMAP_DEFAULT_CENTER[1], AMAP_DEFAULT_CENTER[0]],
        });

        function setMarker(lngGcj: number, latGcj: number) {
          if (amapMarkerRef.current) {
            amapMarkerRef.current.setPosition([lngGcj, latGcj]);
          } else {
            const marker = new AMap.Marker({ position: [lngGcj, latGcj], draggable: true });
            marker.on("dragend", () => {
              const pos = marker.getPosition();
              const wgs = gcj02ToWgs84(pos.getLat(), pos.getLng());
              // Amap maps China only — country is always cn.
              emitPoint(wgs.lat, wgs.lng, "amap", "cn");
            });
            map.add(marker);
            amapMarkerRef.current = marker;
          }
        }

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        map.on("click", (e: any) => {
          const latGcj = e.lnglat.getLat();
          const lngGcj = e.lnglat.getLng();
          setMarker(lngGcj, latGcj);
          const wgs = gcj02ToWgs84(latGcj, lngGcj);
          emitPoint(wgs.lat, wgs.lng, "amap", "cn");
        });

        amapMapRef.current = { map, setMarker };
      })
      .catch(() => {
        if (!disposed) setSearchError(t("geo.amapLoadError"));
      });

    return () => {
      disposed = true;
      if (amapMapRef.current) {
        amapMapRef.current.map.destroy();
        amapMapRef.current = null;
        amapMarkerRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [provider, amapKey]);

  // --- Debounced search (both providers) ---
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
        if (provider === "osm") {
          const res = await fetch(
            `https://nominatim.openstreetmap.org/search?format=json&limit=5&addressdetails=1&q=${encodeURIComponent(q)}`
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
        } else {
          if (!amapKey) return;
          const AMap = await loadAmap(amapKey);
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          AMap.plugin("AMap.PlaceSearch", () => {
            const placeSearch = new AMap.PlaceSearch({ pageSize: 5 });
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            placeSearch.search(q, (status: string, result: any) => {
              setIsSearching(false);
              if (status !== "complete" || !result?.poiList?.pois) {
                setResults([]);
                return;
              }
              setResults(
                // Amap POIs are GCJ-02; keep raw here — conversion happens
                // in handleResultClick via the shared amap path. Amap only
                // covers China, so country is always cn.
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                result.poiList.pois.map((poi: any) => ({
                  label: [poi.name, poi.address].filter(Boolean).join(", "),
                  lat: poi.location.lat,
                  lng: poi.location.lng,
                  country: "cn",
                }))
              );
            });
          });
          return; // isSearching handled in the callback
        }
      } catch {
        setSearchError(t("geo.searchError"));
        setResults([]);
      } finally {
        if (provider === "osm") setIsSearching(false);
      }
    }, SEARCH_DEBOUNCE_MS);

    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, provider]);

  function handleResultClick(result: SearchResult) {
    setResults([]);
    setQuery("");
    if (provider === "osm") {
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
          emitOsmPointWithReverse(pos.lat, pos.lng);
        });
        leafletMarkerRef.current = marker;
      }
      emitPoint(result.lat, result.lng, "osm", result.country, result.label);
    } else {
      if (amapMapRef.current) {
        amapMapRef.current.map.setZoomAndCenter(11, [result.lng, result.lat]);
        amapMapRef.current.setMarker(result.lng, result.lat);
      }
      const wgs = gcj02ToWgs84(result.lat, result.lng);
      emitPoint(wgs.lat, wgs.lng, "amap", "cn", result.label);
    }
  }

  function switchProvider(next: Provider) {
    if (next === provider) return;
    setProvider(next);
    setResults([]);
    setQuery("");
    setSearchError(null);
  }

  return (
    <div className="geo-field">
      <div className="geo-field__header">
        <span className="field__label">{title}</span>
        <div className="geo-toggle">
          <button
            type="button"
            className={"geo-toggle__btn" + (provider === "osm" ? " geo-toggle__btn--active" : "")}
            onClick={() => switchProvider("osm")}
          >
            {t("geo.providerOsm")}
          </button>
          <button
            type="button"
            className={"geo-toggle__btn" + (provider === "amap" ? " geo-toggle__btn--active" : "")}
            onClick={() => switchProvider("amap")}
          >
            {t("geo.providerAmap")}
          </button>
        </div>
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

      {provider === "amap" && !amapKey ? (
        <div className="geo-field__placeholder">{t("geo.noAmapKey")}</div>
      ) : (
        // key forces a fresh container per provider — Leaflet and Amap can't
        // share an initialized div.
        <div key={provider} ref={mapContainerRef} className="geo-map" />
      )}

      {value ? (
        <div className="geo-field__chosen">
          <input
            className="geo-field__label-input"
            value={value.label}
            placeholder={t("geo.labelPlaceholder")}
            onChange={(e) => onChange({ ...value, label: e.target.value })}
          />
          <span className="geo-field__coords">
            {value.lat.toFixed(5)}, {value.lng.toFixed(5)} · {value.source} ·{" "}
            {value.country ? value.country.toUpperCase() : "??"} · WGS-84
          </span>
        </div>
      ) : (
        <div className="geo-field__hint">{t("geo.pickHint")}</div>
      )}
    </div>
  );
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
let amapPromise: Promise<any> | null = null;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function loadAmap(key: string): Promise<any> {
  if (!amapPromise) {
    amapPromise = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = `https://webapi.amap.com/maps?v=2.0&key=${encodeURIComponent(key)}&plugin=AMap.PlaceSearch`;
      script.onload = () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const AMap = (window as any).AMap;
        if (AMap) {
          resolve(AMap);
        } else {
          reject(new Error("AMap global missing after script load"));
        }
      };
      script.onerror = () => {
        amapPromise = null; // allow retry (e.g. after fixing the key)
        reject(new Error("failed to load Amap script"));
      };
      document.head.appendChild(script);
    });
  }
  return amapPromise;
}
