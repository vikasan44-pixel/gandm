import { api } from "./client";
import type { CargoCategory, GeoPoint } from "./types";

// Гостевой (без авторизации) поиск. Точки — координатами из геокодера, поиск
// по радиусу haversine (как везде на платформе). Гость видит анонимные
// карточки без контактов; чтобы открыть объявление — вход/регистрация.

export interface PublicCargoCard {
  id: string;
  origin_label: string;
  origin_country: string;
  origin_labels?: Record<string, string>;
  destination_label: string;
  destination_country: string;
  destination_labels?: Record<string, string>;
  category: CargoCategory;
  volume_m3: number;
  weight_kg: number;
  created_at: string;
}

export interface PublicPoint {
  label: string;
  labels?: Record<string, string>;
}

export interface PublicVehicleCard {
  id: string;
  body_type: string;
  capacity_kg: number;
  capacity_m3: number;
  length_m: number;
  width_m: number;
  height_m: number;
  axles: number;
  location_label?: string;
  location_labels?: Record<string, string>;
  destinations: PublicPoint[];
  trust_percent: number;
  documents_verified: boolean;
  has_completed_trips: boolean;
  masked_plate?: string;
  active_trip?: {
    id: string;
    origin: PublicPoint;
    destination: PublicPoint;
    waypoints: PublicPoint[];
    can_pickup_en_route: boolean;
    pickup_radius_km: number;
    departure_date: string;
    free_weight_kg: number;
    free_volume_m3: number;
  };
  created_at: string;
}

export interface TransportSearchFilter {
  body_type?: string;
  min_capacity_kg?: number;
  min_capacity_m3?: number;
  min_length_m?: number;
  min_width_m?: number;
  min_height_m?: number;
  min_axles?: number;
}

// appendPoint пишет координаты точки в query под заданным префиксом
// (from/to) — ровно те параметры, что читает бэкенд.
function appendPoint(params: URLSearchParams, prefix: string, p: GeoPoint | null) {
  if (!p) return;
  params.set(`${prefix}_lat`, String(p.lat));
  params.set(`${prefix}_lng`, String(p.lng));
  params.set(`${prefix}_country`, p.country ?? "");
  params.set(`${prefix}_label`, p.label ?? "");
}

export function searchPublicCargo(from: GeoPoint | null, to: GeoPoint | null) {
  const params = new URLSearchParams();
  appendPoint(params, "from", from);
  appendPoint(params, "to", to);
  return api.get<PublicCargoCard[]>(`/public/cargo?${params.toString()}`);
}

export function searchPublicTransport(
  filter: TransportSearchFilter,
  from: GeoPoint | null,
  to: GeoPoint | null,
  isAuthenticated = false
) {
  const params = new URLSearchParams();
  if (filter.body_type) params.set("body_type", filter.body_type);
  if (filter.min_capacity_kg) params.set("min_capacity_kg", String(filter.min_capacity_kg));
  if (filter.min_capacity_m3) params.set("min_capacity_m3", String(filter.min_capacity_m3));
  if (filter.min_length_m) params.set("min_length_m", String(filter.min_length_m));
  if (filter.min_width_m) params.set("min_width_m", String(filter.min_width_m));
  if (filter.min_height_m) params.set("min_height_m", String(filter.min_height_m));
  if (filter.min_axles) params.set("min_axles", String(filter.min_axles));
  appendPoint(params, "from", from);
  appendPoint(params, "to", to);
  const endpoint = isAuthenticated ? "/transport/search" : "/public/transport";
  return api.get<PublicVehicleCard[]>(`${endpoint}?${params.toString()}`);
}
