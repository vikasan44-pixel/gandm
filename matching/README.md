# Matching-сервис (Этап 5)

Отдельный Python-сервис (FastAPI), который находит пары заявок-кандидатов на
консолидацию: направления совпадают (радиус по стране точки, как в основном
матчинге) и суммарные объём/вес в пределах лимита вместимости.

Сервис **stateless**: лимиты вместимости и радиусы приходят в теле каждого
запроса от Go-backend'а (он читает их из БД/конфига). Здесь — только математика.

## Запуск

```
cd matching
python3 -m venv .venv && source .venv/bin/activate   # по желанию
pip install -r requirements.txt
uvicorn main:app --port 8000
```

Go-backend ожидает сервис на `http://localhost:8000` (переопределяется
переменной `MATCHING_SERVICE_URL` в корневом `.env`).

## Контракт

`POST /match`

```json
{
  "requests": [
    {"id": "...", "client_id": "...",
     "origin": {"lat": 43.2, "lng": 76.8, "country": "kz"},
     "destination": {"lat": 43.8, "lng": 87.6, "country": "cn"},
     "volume_m3": 30, "weight_kg": 5000}
  ],
  "limits": {"max_volume_m3": 90, "max_weight_kg": 20000},
  "radii": {"cn_km": 100, "kz_km": 40}
}
```

Ответ: `{"pairs": [{"a": "<id>", "b": "<id>"}]}` — жадное паросочетание,
каждая заявка максимум в одной паре, пары клиента с самим собой исключены.

`GET /health` → `{"status": "ok"}` — health-check (его использует smoke.sh).
