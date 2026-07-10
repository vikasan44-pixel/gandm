"""Matching service (Stage 5): finds consolidation pair candidates.

Stateless by design: capacity limits and per-country radii arrive in every
request body — the Go backend owns all configuration. This service only does
the pairing math.

Run:
    cd matching
    pip install -r requirements.txt
    uvicorn main:app --port 8000
"""

import hmac
import math
import os

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from pydantic import BaseModel

app = FastAPI(title="gandm-matching")

# Optional shared secret: when MATCHING_SHARED_SECRET is set, every request
# except /health must carry it in X-Internal-Token. Unset = open (local dev,
# where the service listens on localhost only).
_SHARED_SECRET = os.environ.get("MATCHING_SHARED_SECRET", "")


@app.middleware("http")
async def require_shared_secret(request: Request, call_next):
    if _SHARED_SECRET and request.url.path != "/health":
        token = request.headers.get("x-internal-token", "")
        if not hmac.compare_digest(token, _SHARED_SECRET):
            return JSONResponse(status_code=401, content={"detail": "invalid internal token"})
    return await call_next(request)

EARTH_RADIUS_KM = 6371.0


class Point(BaseModel):
    lat: float
    lng: float
    # Lowercase ISO alpha-2 from the geocoder; "" = unknown (default radius).
    country: str = ""


class CargoItem(BaseModel):
    id: str
    # client_id is only used to avoid pairing a client with themselves;
    # it never leaves this service.
    client_id: str
    origin: Point
    destination: Point
    volume_m3: float
    weight_kg: float


class Limits(BaseModel):
    max_volume_m3: float
    max_weight_kg: float


class Radii(BaseModel):
    cn_km: float
    kz_km: float


class MatchRequest(BaseModel):
    requests: list[CargoItem]
    limits: Limits
    radii: Radii


class MatchResponse(BaseModel):
    # Группы консолидации (ТЗ §4.2 «два клиента и более»): каждая группа —
    # список id заявок, вместе укладывающихся в лимит. Пара — частный
    # случай группы из двух.
    groups: list[list[str]]


def haversine_km(lat1: float, lng1: float, lat2: float, lng2: float) -> float:
    """Great-circle distance; must stay consistent with the Go/SQL versions
    (R = 6371 km)."""
    rad = math.pi / 180.0
    d_lat = (lat2 - lat1) * rad
    d_lng = (lng2 - lng1) * rad
    h = (
        math.sin(d_lat / 2) ** 2
        + math.cos(lat1 * rad) * math.cos(lat2 * rad) * math.sin(d_lng / 2) ** 2
    )
    return EARTH_RADIUS_KM * 2 * math.asin(math.sqrt(min(1.0, h)))


def radius_for(country: str, radii: Radii) -> float:
    return radii.cn_km if country == "cn" else radii.kz_km


def points_match(p1: Point, p2: Point, radii: Radii) -> bool:
    """Same rule as the Go/SQL matching: per-point country picks the radius,
    cross-border pairs use the more generous of the two."""
    threshold = max(radius_for(p1.country, radii), radius_for(p2.country, radii))
    return haversine_km(p1.lat, p1.lng, p2.lat, p2.lng) <= threshold


def fits_group(seed: CargoItem, b: CargoItem, vol: float, wt: float,
               clients: set[str], limits: Limits, radii: Radii) -> bool:
    """Кандидат подходит в группу: другой клиент, направление совпадает с
    якорем группы (первой заявкой) по радиусной логике, суммарные объём и
    вес не превышают лимит."""
    if b.client_id in clients:
        return False
    if vol + b.volume_m3 > limits.max_volume_m3:
        return False
    if wt + b.weight_kg > limits.max_weight_kg:
        return False
    if not points_match(seed.origin, b.origin, radii):
        return False
    if not points_match(seed.destination, b.destination, radii):
        return False
    return True


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/match")
def match(req: MatchRequest) -> MatchResponse:
    """Greedy grouping: первая свободная заявка становится якорем группы,
    к ней добираются совместимые заявки, пока влезают в лимит. Каждая
    заявка попадает максимум в одну группу — Go-стороне не приходится
    разруливать конфликтующие предложения. O(n²) приемлем на MVP-объёмах
    (порог пересмотра — ~тысяча одновременно открытых заявок)."""
    groups: list[list[str]] = []
    used: set[str] = set()

    items = req.requests
    for i, seed in enumerate(items):
        if seed.id in used:
            continue
        group = [seed]
        clients = {seed.client_id}
        vol, wt = seed.volume_m3, seed.weight_kg
        for b in items[i + 1 :]:
            if b.id in used:
                continue
            if fits_group(seed, b, vol, wt, clients, req.limits, req.radii):
                group.append(b)
                clients.add(b.client_id)
                vol += b.volume_m3
                wt += b.weight_kg
        if len(group) >= 2:
            for item in group:
                used.add(item.id)
            groups.append([item.id for item in group])

    return MatchResponse(groups=groups)
