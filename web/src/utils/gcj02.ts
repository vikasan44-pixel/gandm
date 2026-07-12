// GCJ-02 ("Mars coordinates", what Amap returns) → WGS-84 conversion.
// The backend stores WGS-84 ONLY and does no conversion itself (see
// internal/geo/geo.go) — every Amap coordinate must pass through here
// before being sent to the API. Standard approximate inverse: compute the
// forward offset at the point and subtract it; error is a couple of meters,
// negligible against the matching radius, which comes from backend config
// (MATCH_RADIUS_CN_KM / MATCH_RADIUS_KZ_KM) and is tens to a hundred km
// depending on the country.

const A = 6378245.0;
const EE = 0.00669342162296594323;

function outOfChina(lat: number, lng: number): boolean {
  return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271;
}

function transformLat(x: number, y: number): number {
  let ret =
    -100.0 + 2.0 * x + 3.0 * y + 0.2 * y * y + 0.1 * x * y + 0.2 * Math.sqrt(Math.abs(x));
  ret += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0;
  ret += ((20.0 * Math.sin(y * Math.PI) + 40.0 * Math.sin((y / 3.0) * Math.PI)) * 2.0) / 3.0;
  ret +=
    ((160.0 * Math.sin((y / 12.0) * Math.PI) + 320.0 * Math.sin((y * Math.PI) / 30.0)) * 2.0) /
    3.0;
  return ret;
}

function transformLng(x: number, y: number): number {
  let ret = 300.0 + x + 2.0 * y + 0.1 * x * x + 0.1 * x * y + 0.1 * Math.sqrt(Math.abs(x));
  ret += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0;
  ret += ((20.0 * Math.sin(x * Math.PI) + 40.0 * Math.sin((x / 3.0) * Math.PI)) * 2.0) / 3.0;
  ret +=
    ((150.0 * Math.sin((x / 12.0) * Math.PI) + 300.0 * Math.sin((x / 30.0) * Math.PI)) * 2.0) /
    3.0;
  return ret;
}

// offset returns the GCJ-02 shift (dLat, dLng) at a WGS-84 point.
function offset(lat: number, lng: number): { dLat: number; dLng: number } {
  let dLat = transformLat(lng - 105.0, lat - 35.0);
  let dLng = transformLng(lng - 105.0, lat - 35.0);
  const radLat = (lat / 180.0) * Math.PI;
  let magic = Math.sin(radLat);
  magic = 1 - EE * magic * magic;
  const sqrtMagic = Math.sqrt(magic);
  dLat = (dLat * 180.0) / (((A * (1 - EE)) / (magic * sqrtMagic)) * Math.PI);
  dLng = (dLng * 180.0) / ((A / sqrtMagic) * Math.cos(radLat) * Math.PI);
  return { dLat, dLng };
}

export function gcj02ToWgs84(lat: number, lng: number): { lat: number; lng: number } {
  if (outOfChina(lat, lng)) return { lat, lng };
  const { dLat, dLng } = offset(lat, lng);
  return { lat: lat - dLat, lng: lng - dLng };
}

// wgs84ToGcj02 is the forward shift — used to place a WGS-84 marker at the
// right pixel on GCJ-02 (Amap) tiles. Same crude China bbox as the inverse:
// points outside it (incl. most of Kazakhstan is INSIDE the bbox — a known
// coarse-approximation limitation) are returned unchanged.
export function wgs84ToGcj02(lat: number, lng: number): { lat: number; lng: number } {
  if (outOfChina(lat, lng)) return { lat, lng };
  const { dLat, dLng } = offset(lat, lng);
  return { lat: lat + dLat, lng: lng + dLng };
}
