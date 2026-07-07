#!/usr/bin/env bash
# scripts/dev.sh — brings up local Postgres + MinIO (starting them if
# they're not already running), applies migrations, and starts the API
# server in the foreground. Run from anywhere; paths are resolved relative
# to the repo root.
#
# First time on this machine, you'll also need:
#   go mod tidy   (pulls in golang-migrate, minio-go, etc.)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

MINIO_DATA_DIR="$HOME/minio-data"
MINIO_LOG_FILE="$HOME/.gandm-minio.log"
MINIO_PID_FILE="$HOME/.gandm-minio.pid"

step() { printf '\n\033[1;34m==>\033[0m %s\n' "$1"; }
ok()   { printf '  \033[1;32m✓\033[0m %s\n' "$1"; }
fail() { printf '  \033[1;31m✗\033[0m %s\n' "$1" >&2; exit 1; }

step "Проверяю .env"
if [ ! -f .env ]; then
  fail ".env не найден в $ROOT_DIR. Скопируйте .env.example в .env и подставьте свои значения, затем запустите снова."
fi
set -a
# shellcheck disable=SC1091
source .env
set +a
ok ".env загружен"

step "Проверяю Postgres"
PG_HOST="localhost"
PG_PORT="5432"
if [ -n "${DATABASE_DSN:-}" ]; then
  HOSTPORT="$(printf '%s' "$DATABASE_DSN" | sed -E 's#^[a-zA-Z0-9]+://([^@]*@)?([^/]+)/.*#\2#')"
  if [ -n "$HOSTPORT" ]; then
    PG_HOST="${HOSTPORT%%:*}"
    case "$HOSTPORT" in
      *:*) PG_PORT="${HOSTPORT##*:}" ;;
    esac
  fi
fi

if command -v pg_isready >/dev/null 2>&1; then
  if pg_isready -h "$PG_HOST" -p "$PG_PORT" >/dev/null 2>&1; then
    ok "Postgres уже отвечает на $PG_HOST:$PG_PORT"
  else
    echo "  Postgres не отвечает на $PG_HOST:$PG_PORT, пробую поднять через Homebrew..."
    if command -v brew >/dev/null 2>&1; then
      STARTED=""
      for formula in postgresql postgresql@17 postgresql@16 postgresql@15 postgresql@14 postgresql@13; do
        if brew list --formula 2>/dev/null | grep -qx "$formula"; then
          brew services start "$formula" >/dev/null 2>&1 || true
          STARTED="$formula"
          break
        fi
      done
      if [ -z "$STARTED" ]; then
        fail "Postgres не установлен через Homebrew (проверил formula postgresql*). Запустите свой Postgres вручную и повторите запуск dev.sh."
      fi
      echo "  Запущен через brew ($STARTED), жду готовности..."
      READY=""
      for _ in $(seq 1 15); do
        if pg_isready -h "$PG_HOST" -p "$PG_PORT" >/dev/null 2>&1; then
          READY=1
          break
        fi
        sleep 1
      done
      [ -n "$READY" ] || fail "Postgres так и не ответил на $PG_HOST:$PG_PORT после запуска через brew."
      ok "Postgres поднят ($STARTED)"
    else
      fail "Postgres не отвечает и Homebrew не найден в PATH. Запустите Postgres вручную и повторите запуск."
    fi
  fi
else
  echo "  pg_isready не найден в PATH — пропускаю автопроверку, полагаюсь на то, что Postgres уже запущен на $PG_HOST:$PG_PORT."
fi

step "Проверяю MinIO"
if curl -sf http://localhost:9000/minio/health/live >/dev/null 2>&1; then
  ok "MinIO уже отвечает на :9000"
else
  if ! command -v minio >/dev/null 2>&1; then
    fail "MinIO не найден в PATH. Установите: brew install minio/stable/minio"
  fi
  mkdir -p "$MINIO_DATA_DIR"
  echo "  Стартую MinIO в фоне (данные: $MINIO_DATA_DIR, консоль http://localhost:9001, minioadmin/minioadmin)..."
  MINIO_ROOT_USER=minioadmin MINIO_ROOT_PASSWORD=minioadmin \
    nohup minio server "$MINIO_DATA_DIR" --console-address ":9001" >"$MINIO_LOG_FILE" 2>&1 &
  echo $! > "$MINIO_PID_FILE"
  READY=""
  for _ in $(seq 1 15); do
    if curl -sf http://localhost:9000/minio/health/live >/dev/null 2>&1; then
      READY=1
      break
    fi
    sleep 1
  done
  [ -n "$READY" ] || fail "MinIO не поднялся за 15 секунд — смотрите $MINIO_LOG_FILE"
  ok "MinIO запущен (pid $(cat "$MINIO_PID_FILE"), лог: $MINIO_LOG_FILE)"
fi

step "Применяю миграции"
if go run ./cmd/migrate up; then
  ok "Миграции применены"
else
  fail "Миграции не применились — см. вывод выше. Если жалуется на отсутствующие модули, выполните: go mod tidy"
fi

step "Запускаю API-сервер (Ctrl+C чтобы остановить)"
exec go run ./cmd/api
