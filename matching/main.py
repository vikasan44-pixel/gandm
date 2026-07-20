"""Small dependency-free HTTP service for cargo consolidation matching."""

from __future__ import annotations

import argparse
import hmac
import json
import math
import os
from dataclasses import dataclass
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

EARTH_RADIUS_KM = 6371.0
MAX_REQUEST_BYTES = 2 << 20
MAX_CARGO_REQUESTS = 5000
_SHARED_SECRET = os.environ.get("MATCHING_SHARED_SECRET", "")


class ValidationError(ValueError):
    pass


@dataclass(frozen=True)
class Point:
    lat: float
    lng: float
    country: str = ""


@dataclass(frozen=True)
class CargoItem:
    id: str
    client_id: str
    origin: Point
    destination: Point
    volume_m3: float
    weight_kg: float


@dataclass(frozen=True)
class Limits:
    max_volume_m3: float
    max_weight_kg: float


@dataclass(frozen=True)
class Radii:
    cn_km: float
    kz_km: float


@dataclass(frozen=True)
class MatchRequest:
    requests: list[CargoItem]
    limits: Limits
    radii: Radii


def finite_number(value: Any, field: str, *, minimum: float | None = None,
                  maximum: float | None = None) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ValidationError(f"{field} must be a number")
    number = float(value)
    if not math.isfinite(number):
        raise ValidationError(f"{field} must be finite")
    if minimum is not None and number < minimum:
        raise ValidationError(f"{field} is below the minimum")
    if maximum is not None and number > maximum:
        raise ValidationError(f"{field} exceeds the maximum")
    return number


def required_string(value: Any, field: str, max_length: int = 100) -> str:
    if not isinstance(value, str) or not value.strip() or len(value) > max_length:
        raise ValidationError(f"{field} must be a non-empty string")
    return value.strip()


def parse_point(raw: Any, field: str) -> Point:
    if not isinstance(raw, dict):
        raise ValidationError(f"{field} must be an object")
    country = raw.get("country", "")
    if not isinstance(country, str) or len(country.strip()) > 2:
        raise ValidationError(f"{field}.country must be an ISO alpha-2 code")
    return Point(
        lat=finite_number(raw.get("lat"), f"{field}.lat", minimum=-90, maximum=90),
        lng=finite_number(raw.get("lng"), f"{field}.lng", minimum=-180, maximum=180),
        country=country.strip().lower(),
    )


def parse_match_request(raw: Any) -> MatchRequest:
    if not isinstance(raw, dict):
        raise ValidationError("request body must be an object")
    items_raw = raw.get("requests")
    if not isinstance(items_raw, list) or len(items_raw) > MAX_CARGO_REQUESTS:
        raise ValidationError(f"requests must be an array of at most {MAX_CARGO_REQUESTS} items")
    items: list[CargoItem] = []
    seen_ids: set[str] = set()
    for index, item in enumerate(items_raw):
        if not isinstance(item, dict):
            raise ValidationError(f"requests[{index}] must be an object")
        item_id = required_string(item.get("id"), f"requests[{index}].id")
        if item_id in seen_ids:
            raise ValidationError("cargo ids must be unique")
        seen_ids.add(item_id)
        items.append(CargoItem(
            id=item_id,
            client_id=required_string(item.get("client_id"), f"requests[{index}].client_id"),
            origin=parse_point(item.get("origin"), f"requests[{index}].origin"),
            destination=parse_point(item.get("destination"), f"requests[{index}].destination"),
            volume_m3=finite_number(item.get("volume_m3"), f"requests[{index}].volume_m3", minimum=0.000001),
            weight_kg=finite_number(item.get("weight_kg"), f"requests[{index}].weight_kg", minimum=0.000001),
        ))
    limits_raw, radii_raw = raw.get("limits"), raw.get("radii")
    if not isinstance(limits_raw, dict) or not isinstance(radii_raw, dict):
        raise ValidationError("limits and radii must be objects")
    return MatchRequest(
        requests=items,
        limits=Limits(
            max_volume_m3=finite_number(limits_raw.get("max_volume_m3"), "limits.max_volume_m3", minimum=0.000001),
            max_weight_kg=finite_number(limits_raw.get("max_weight_kg"), "limits.max_weight_kg", minimum=0.000001),
        ),
        radii=Radii(
            cn_km=finite_number(radii_raw.get("cn_km"), "radii.cn_km", minimum=0.000001, maximum=3000),
            kz_km=finite_number(radii_raw.get("kz_km"), "radii.kz_km", minimum=0.000001, maximum=3000),
        ),
    )


def haversine_km(lat1: float, lng1: float, lat2: float, lng2: float) -> float:
    rad = math.pi / 180.0
    d_lat = (lat2 - lat1) * rad
    d_lng = (lng2 - lng1) * rad
    h = math.sin(d_lat / 2) ** 2 + math.cos(lat1 * rad) * math.cos(lat2 * rad) * math.sin(d_lng / 2) ** 2
    return EARTH_RADIUS_KM * 2 * math.asin(math.sqrt(min(1.0, h)))


def radius_for(country: str, radii: Radii) -> float:
    return radii.cn_km if country == "cn" else radii.kz_km


def points_match(left: Point, right: Point, radii: Radii) -> bool:
    threshold = max(radius_for(left.country, radii), radius_for(right.country, radii))
    return haversine_km(left.lat, left.lng, right.lat, right.lng) <= threshold


def fits_group(group: list[CargoItem], candidate: CargoItem, volume: float, weight: float,
               clients: set[str], limits: Limits, radii: Radii) -> bool:
    if candidate.client_id in clients:
        return False
    if volume + candidate.volume_m3 > limits.max_volume_m3 or weight + candidate.weight_kg > limits.max_weight_kg:
        return False
    return all(points_match(member.origin, candidate.origin, radii)
               and points_match(member.destination, candidate.destination, radii)
               for member in group)


def match(request: MatchRequest) -> list[list[str]]:
    groups: list[list[str]] = []
    used: set[str] = set()
    for index, seed in enumerate(request.requests):
        if seed.id in used:
            continue
        group = [seed]
        clients = {seed.client_id}
        volume, weight = seed.volume_m3, seed.weight_kg
        for candidate in request.requests[index + 1:]:
            if candidate.id in used:
                continue
            if fits_group(group, candidate, volume, weight, clients, request.limits, request.radii):
                group.append(candidate)
                clients.add(candidate.client_id)
                volume += candidate.volume_m3
                weight += candidate.weight_kg
        if len(group) >= 2:
            used.update(item.id for item in group)
            groups.append([item.id for item in group])
    return groups


class MatchingHandler(BaseHTTPRequestHandler):
    server_version = "gandm-matching/1"

    def send_json(self, status: int, payload: dict[str, Any]) -> None:
        body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("X-Content-Type-Options", "nosniff")
        self.end_headers()
        self.wfile.write(body)

    def authorized(self) -> bool:
        return not _SHARED_SECRET or hmac.compare_digest(self.headers.get("X-Internal-Token", ""), _SHARED_SECRET)

    def do_GET(self) -> None:
        if self.path == "/health":
            self.send_json(200, {"status": "ok"})
        else:
            self.send_json(404, {"detail": "not found"})

    def do_POST(self) -> None:
        if self.path != "/match":
            self.send_json(404, {"detail": "not found"})
            return
        if not self.authorized():
            self.send_json(401, {"detail": "invalid internal token"})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if length <= 0 or length > MAX_REQUEST_BYTES:
                raise ValidationError("request body size is invalid")
            payload = json.loads(self.rfile.read(length))
            groups = match(parse_match_request(payload))
        except (ValueError, json.JSONDecodeError, ValidationError) as error:
            self.send_json(422, {"detail": str(error)})
            return
        self.send_json(200, {"groups": groups})

    def log_message(self, fmt: str, *args: Any) -> None:
        print(f"{self.address_string()} - {fmt % args}")


def run(host: str = "127.0.0.1", port: int = 8000) -> None:
    server = ThreadingHTTPServer((host, port), MatchingHandler)
    print(f"gandm matching listening on http://{host}:{port}")
    server.serve_forever()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8000)
    args = parser.parse_args()
    run(args.host, args.port)
