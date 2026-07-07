#!/usr/bin/env bash
# scripts/smoke.sh — end-to-end smoke test for Stage 1 + Stage 2 (backend
# only). Assumes the server from scripts/dev.sh is already running. Runs
# every check regardless of earlier failures and prints a PASS/FAIL summary
# at the end; exits non-zero if anything failed.
#
# Requires: curl, jq (brew install jq), go (to run cmd/seed).
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ -f "$ROOT_DIR/.env" ]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

BASE_URL="${SMOKE_BASE_URL:-http://localhost:${SERVER_PORT:-8080}/api}"

SEED_ADMIN_EMAIL="admin@platform.local"
SEED_ADMIN_PASSWORD="Admin12345!"
SEED_TOOL_KEY="view_cargo_requests"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

PASS_COUNT=0
FAIL_COUNT=0
FAILURES=()

step() { printf '\n\033[1;34m==>\033[0m %s\n' "$1"; }
pass() { PASS_COUNT=$((PASS_COUNT + 1)); printf '  \033[1;32mPASS\033[0m %s\n' "$1"; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); FAILURES+=("$1"); printf '  \033[1;31mFAIL\033[0m %s\n' "$1"; }

assert_status() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    pass "$desc (HTTP $actual)"
  else
    fail "$desc (ожидали HTTP $expected, получили HTTP $actual)"
  fi
}

# req METHOD PATH [JSON_BODY] [BEARER_TOKEN] -> prints HTTP status code, body in $TMP_DIR/resp.json
req() {
  local method="$1" path="$2" body="${3:-}" token="${4:-}"
  local args=(-s -o "$TMP_DIR/resp.json" -w '%{http_code}' -X "$method" "$BASE_URL$path" -H 'Content-Type: application/json')
  [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
  [ -n "$body" ] && args+=(-d "$body")
  curl "${args[@]}" 2>/dev/null || echo "000"
}

# req_upload PATH TOKEN FILE DOC_TYPE -> prints HTTP status code, body in $TMP_DIR/resp.json
req_upload() {
  local path="$1" token="$2" file="$3" doctype="$4"
  curl -s -o "$TMP_DIR/resp.json" -w '%{http_code}' -X POST "$BASE_URL$path" \
    -H "Authorization: Bearer $token" \
    -F "type=$doctype" \
    -F "file=@${file};type=image/png" 2>/dev/null || echo "000"
}

step "Проверяю зависимости (curl, jq)"
if ! command -v curl >/dev/null 2>&1; then
  echo "curl не найден. Прервано." >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq не найден (нужен для разбора JSON-ответов). Установите: brew install jq. Прервано." >&2
  exit 1
fi
pass "curl и jq на месте"

step "Проверяю что сервер отвечает на $BASE_URL"
PING_STATUS=$(req POST /register '{}')
if [ "$PING_STATUS" = "000" ]; then
  echo "Сервер не отвечает на $BASE_URL. Запустите scripts/dev.sh в отдельном терминале и повторите." >&2
  exit 1
fi
pass "Сервер отвечает (HTTP $PING_STATUS на пустой запрос)"

RAND="$(date +%s)$$"
EMAIL_A="smoke.user.${RAND}@example.com"
PASSWORD_A="Smoke12345!"
TOKEN_A=""
USER_ID_A=""

step "1. Регистрация участника"
BODY=$(jq -n --arg email "$EMAIL_A" --arg password "$PASSWORD_A" \
  '{email:$email, phone:"+70000000001", company_name:"Smoke Test Co", participant_type:"client", password:$password}')
STATUS=$(req POST /register "$BODY")
assert_status "Регистрация нового участника" "201" "$STATUS"
if [ "$STATUS" = "201" ]; then
  TOKEN_A="$(jq -r '.tokens.access_token // empty' "$TMP_DIR/resp.json")"
  USER_ID_A="$(jq -r '.user.id // empty' "$TMP_DIR/resp.json")"
  [ -n "$TOKEN_A" ] && [ -n "$USER_ID_A" ] || fail "В ответе регистрации нет tokens.access_token или user.id"
fi

step "1a. GET /me сразу после регистрации (статус своей заявки)"
if [ -n "$TOKEN_A" ]; then
  STATUS=$(req GET /me "" "$TOKEN_A")
  if [ "$STATUS" = "200" ]; then
    ME_USER_STATUS="$(jq -r '.user.status // empty' "$TMP_DIR/resp.json")"
    ME_VER_STATUS="$(jq -r '.verification.status // empty' "$TMP_DIR/resp.json")"
    if [ "$ME_USER_STATUS" = "pending" ] && [ "$ME_VER_STATUS" = "pending" ]; then
      pass "GET /me: user.status и verification.status = pending до одобрения"
    else
      fail "GET /me: user.status='$ME_USER_STATUS' verification.status='$ME_VER_STATUS' (ожидали pending/pending)"
    fi
  else
    fail "GET /me вернул HTTP $STATUS вместо 200"
  fi
else
  fail "Пропущено — нет токена участника"
fi

step "2. Дубликат email"
STATUS=$(req POST /register "$BODY")
assert_status "Повторная регистрация с тем же email" "409" "$STATUS"

step "3. Невалидные данные при регистрации"
STATUS=$(req POST /register '{"email":"not-an-email","phone":"+7000","company_name":"X","participant_type":"client","password":"Smoke12345!"}')
assert_status "Некорректный email" "400" "$STATUS"

STATUS=$(req POST /register "$(jq -n --arg email "smoke.short.${RAND}@example.com" '{email:$email,phone:"+7000",company_name:"X",participant_type:"client",password:"123"}')")
assert_status "Слишком короткий пароль" "400" "$STATUS"

STATUS=$(req POST /register "$(jq -n --arg email "smoke.badtype.${RAND}@example.com" '{email:$email,phone:"+7000",company_name:"X",participant_type:"alien",password:"Smoke12345!"}')")
assert_status "Неизвестный participant_type" "400" "$STATUS"

step "4. Загрузка документа (настоящий PNG)"
PNG_FILE="$TMP_DIR/test.png"
PNG_B64="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
if [[ "$(uname)" == "Darwin" ]]; then
  printf '%s' "$PNG_B64" | base64 -D >"$PNG_FILE" 2>/dev/null
else
  printf '%s' "$PNG_B64" | base64 -d >"$PNG_FILE" 2>/dev/null
fi

if [ -n "$TOKEN_A" ] && [ -s "$PNG_FILE" ]; then
  STATUS=$(req_upload /register/documents "$TOKEN_A" "$PNG_FILE" "id_card")
  assert_status "Загрузка PNG-документа участником" "201" "$STATUS"
else
  fail "Пропущено — нет токена участника или не удалось создать тестовый PNG"
fi

step "5. Полный seed (админ, инструменты, наборы, тестовые участники)"
if (cd "$ROOT_DIR" && go run ./cmd/seed) >"$TMP_DIR/seed.log" 2>&1; then
  pass "go run ./cmd/seed отработал без ошибок"
else
  fail "go run ./cmd/seed завершился с ошибкой"
  echo "  --- вывод seed ---"
  sed 's/^/  /' "$TMP_DIR/seed.log"
  echo "  -------------------"
fi

step "6. Логин админа"
STATUS=$(req POST /admin/login "$(jq -n --arg email "$SEED_ADMIN_EMAIL" --arg password "$SEED_ADMIN_PASSWORD" '{email:$email,password:$password}')")
assert_status "Логин админа" "200" "$STATUS"
ADMIN_TOKEN=""
if [ "$STATUS" = "200" ]; then
  ADMIN_TOKEN="$(jq -r '.tokens.access_token // empty' "$TMP_DIR/resp.json")"
  [ -n "$ADMIN_TOKEN" ] || fail "В ответе логина админа нет tokens.access_token"
fi

step "7. Dashboard stats"
STATUS=$(req GET /admin/dashboard/stats "" "$ADMIN_TOKEN")
assert_status "GET /admin/dashboard/stats" "200" "$STATUS"
if [ "$STATUS" = "200" ]; then
  if jq -e '(.waiting_verification != null) and (.new_today != null) and (.active_users != null) and (.visits != null)' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "Ответ dashboard содержит все 4 поля"
  else
    fail "В ответе dashboard не хватает ожидаемых полей"
  fi
fi

step "8. Очередь верификации содержит нашего участника"
VERIFICATION_ID=""
STATUS=$(req GET "/admin/verifications?status=pending" "" "$ADMIN_TOKEN")
assert_status "GET /admin/verifications?status=pending" "200" "$STATUS"
if [ "$STATUS" = "200" ] && [ -n "$USER_ID_A" ]; then
  VERIFICATION_ID="$(jq -r --arg uid "$USER_ID_A" '[.[] | select(.user_id == $uid)] | .[0].verification_id // empty' "$TMP_DIR/resp.json")"
  if [ -n "$VERIFICATION_ID" ]; then
    pass "Участник найден в очереди верификации"
  else
    fail "Участник не найден в очереди верификации"
  fi
fi

step "9. Одобрение заявки"
if [ -n "$VERIFICATION_ID" ] && [ -n "$ADMIN_TOKEN" ]; then
  STATUS=$(req POST "/admin/verifications/$VERIFICATION_ID/approve" "" "$ADMIN_TOKEN")
  assert_status "Approve заявки" "200" "$STATUS"
else
  fail "Пропущено — нет verification_id или токена админа"
fi

step "10. GET /me после approve — статус участника и статус заявки"
if [ -n "$TOKEN_A" ]; then
  STATUS=$(req GET /me "" "$TOKEN_A")
  if [ "$STATUS" = "200" ]; then
    ACTUAL_STATUS="$(jq -r '.user.status // empty' "$TMP_DIR/resp.json")"
    ACTUAL_VER_STATUS="$(jq -r '.verification.status // empty' "$TMP_DIR/resp.json")"
    if [ "$ACTUAL_STATUS" = "active" ] && [ "$ACTUAL_VER_STATUS" = "approved" ]; then
      pass "GET /me после approve: user.status=active, verification.status=approved"
    else
      fail "GET /me после approve: user.status='$ACTUAL_STATUS' verification.status='$ACTUAL_VER_STATUS' (ожидали active/approved)"
    fi
  else
    fail "GET /me вернул HTTP $STATUS вместо 200"
  fi
else
  fail "Пропущено — нет токена участника"
fi

step "11. Повторный approve -> 409"
if [ -n "$VERIFICATION_ID" ] && [ -n "$ADMIN_TOKEN" ]; then
  STATUS=$(req POST "/admin/verifications/$VERIFICATION_ID/approve" "" "$ADMIN_TOKEN")
  assert_status "Повторный approve" "409" "$STATUS"
else
  fail "Пропущено — нет verification_id или токена админа"
fi

step "12. Блокировка участника запрещает загрузку документов"
if [ -n "$USER_ID_A" ] && [ -n "$ADMIN_TOKEN" ]; then
  STATUS=$(req POST "/admin/users/$USER_ID_A/block" "" "$ADMIN_TOKEN")
  assert_status "Блокировка участника" "200" "$STATUS"

  STATUS=$(req_upload /register/documents "$TOKEN_A" "$PNG_FILE" "id_card")
  assert_status "Загрузка документа заблокированным участником" "403" "$STATUS"

  STATUS=$(req POST "/admin/users/$USER_ID_A/unblock" "" "$ADMIN_TOKEN")
  assert_status "Разблокировка участника обратно" "200" "$STATUS"
else
  fail "Пропущено — нет user_id участника или токена админа"
fi

step "13. Назначение инструмента участнику и проверка доступа по инструменту"
if [ -n "$USER_ID_A" ] && [ -n "$ADMIN_TOKEN" ] && [ -n "$TOKEN_A" ]; then
  STATUS=$(req GET /admin/tools "" "$ADMIN_TOKEN")
  TOOL_ID=""
  if [ "$STATUS" = "200" ]; then
    TOOL_ID="$(jq -r --arg key "$SEED_TOOL_KEY" '[.[] | select(.key == $key)] | .[0].id // empty' "$TMP_DIR/resp.json")"
  fi

  if [ -z "$TOOL_ID" ]; then
    fail "Не нашёл tool_id для ключа $SEED_TOOL_KEY (GET /admin/tools вернул HTTP $STATUS) — пропускаю проверку доступа"
  else
    STATUS=$(req GET "/tools/${SEED_TOOL_KEY}/access-check" "" "$TOKEN_A")
    assert_status "Доступ к инструменту ДО назначения" "403" "$STATUS"

    STATUS=$(req POST "/admin/users/$USER_ID_A/tools" "$(jq -n --arg id "$TOOL_ID" '{tool_ids:[$id]}')" "$ADMIN_TOKEN")
    assert_status "Назначение инструмента участнику" "200" "$STATUS"

    STATUS=$(req GET "/tools/${SEED_TOOL_KEY}/access-check" "" "$TOKEN_A")
    assert_status "Доступ к инструменту ПОСЛЕ назначения" "200" "$STATUS"
  fi
else
  fail "Пропущено — не хватает user_id, токена админа или токена участника"
fi

step "14. Токен участника не пускает в админку"
if [ -n "$TOKEN_A" ]; then
  STATUS=$(req GET /admin/dashboard/stats "" "$TOKEN_A")
  assert_status "GET /admin/dashboard/stats с токеном участника" "401" "$STATUS"
else
  fail "Пропущено — нет токена участника"
fi

# Координатные константы (WGS-84). Радиусы: KZ/прочее 40 км, CN 100 км.
#   Алматы (kz) 43.238949,76.889709 · Урумчи (cn) 43.825592,87.616848
#   KZ-сдвиги от Алматы по широте: +0.18°≈20 км (внутри 40), +0.315°≈35 км (внутри 40),
#     +0.45°≈50 км (вне 40)
#   CN-сдвиги от Урумчи: +0.72°≈80 км (внутри 100), +1.08°≈120 км (вне 100)
CARGO_BODY_ALMATY_URUMQI=$(jq -n '{
  origin:      {lat:43.238949, lng:76.889709, label:"Алматы", source:"osm", country:"kz"},
  destination: {lat:43.825592, lng:87.616848, label:"Урумчи", source:"osm", country:"cn"},
  volume_m3:12.5, weight_kg:800, description:"Смоук-тест груза"}')

step "15. Клиент подаёт заявку на груз (с координатами)"
CARGO_ID=""
if [ -n "$TOKEN_A" ]; then
  STATUS=$(req POST /cargo "$CARGO_BODY_ALMATY_URUMQI" "$TOKEN_A")
  assert_status "POST /cargo клиентом" "201" "$STATUS"
  if [ "$STATUS" = "201" ]; then
    CARGO_ID="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"
    [ -n "$CARGO_ID" ] || fail "В ответе POST /cargo нет id"
    if jq -e '.origin.lat == 43.238949 and .origin.source == "osm" and .origin.country == "kz" and .destination.country == "cn"' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
      pass "Координаты и страны точек сохранились в ответе"
    else
      fail "В ответе POST /cargo нет ожидаемых координат/стран (country потерялся?)"
    fi
  fi
else
  fail "Пропущено — нет токена участника"
fi

step "16. Заявка видна клиенту в /cargo/mine"
if [ -n "$TOKEN_A" ] && [ -n "$CARGO_ID" ]; then
  STATUS=$(req GET /cargo/mine "" "$TOKEN_A")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$CARGO_ID" '[.[] | select(.id == $id)] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "GET /cargo/mine содержит поданную заявку"
  else
    fail "GET /cargo/mine (HTTP $STATUS) не содержит заявку $CARGO_ID"
  fi
else
  fail "Пропущено — нет cargo_id или токена участника"
fi

step "17. Регистрация второго участника (для проверки receive_cargo_by_route/submit_offer)"
EMAIL_B="smoke.participant.${RAND}@example.com"
PASSWORD_B="Smoke12345!"
TOKEN_B=""
USER_ID_B=""
BODY=$(jq -n --arg email "$EMAIL_B" --arg password "$PASSWORD_B" \
  '{email:$email, phone:"+70000000002", company_name:"Smoke Warehouse Co", participant_type:"warehouse", password:$password}')
STATUS=$(req POST /register "$BODY")
assert_status "Регистрация второго участника" "201" "$STATUS"
if [ "$STATUS" = "201" ]; then
  TOKEN_B="$(jq -r '.tokens.access_token // empty' "$TMP_DIR/resp.json")"
  USER_ID_B="$(jq -r '.user.id // empty' "$TMP_DIR/resp.json")"
  [ -n "$TOKEN_B" ] && [ -n "$USER_ID_B" ] || fail "В ответе регистрации второго участника нет tokens/id"
fi

step "18. Без инструмента receive_cargo_by_route — /cargo/available недоступен"
if [ -n "$TOKEN_B" ]; then
  STATUS=$(req GET /cargo/available "" "$TOKEN_B")
  assert_status "GET /cargo/available без инструмента" "403" "$STATUS"
else
  fail "Пропущено — нет токена второго участника"
fi

TOOL_ID_RECEIVE=""
TOOL_ID_SUBMIT=""
if [ -n "$ADMIN_TOKEN" ]; then
  STATUS=$(req GET /admin/tools "" "$ADMIN_TOKEN")
  if [ "$STATUS" = "200" ]; then
    TOOL_ID_RECEIVE="$(jq -r '[.[] | select(.key == "receive_cargo_by_route")] | .[0].id // empty' "$TMP_DIR/resp.json")"
    TOOL_ID_SUBMIT="$(jq -r '[.[] | select(.key == "submit_offer")] | .[0].id // empty' "$TMP_DIR/resp.json")"
  fi
fi

step "19. Инструмент есть, маршрута нет — заявка НЕ видна; появился маршрут — видна"
ROUTE_ID_B=""
if [ -n "$USER_ID_B" ] && [ -n "$ADMIN_TOKEN" ] && [ -n "$TOOL_ID_RECEIVE" ] && [ -n "$TOKEN_B" ] && [ -n "$CARGO_ID" ]; then
  STATUS=$(req POST "/admin/users/$USER_ID_B/tools" "$(jq -n --arg id "$TOOL_ID_RECEIVE" '{tool_ids:[$id]}')" "$ADMIN_TOKEN")
  assert_status "Назначение receive_cargo_by_route" "200" "$STATUS"

  STATUS=$(req GET /cargo/available "" "$TOKEN_B")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$CARGO_ID" '[.[] | select(.id == $id)] | length == 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "С инструментом, но без маршрута — заявка не видна"
  else
    fail "С инструментом без маршрута заявка видна или ошибка (HTTP $STATUS) — ожидали 200 и пустое совпадение"
  fi

  # POST /routes доступен только active-участнику — одобряем B
  STATUS=$(req GET "/admin/verifications?status=pending" "" "$ADMIN_TOKEN")
  VERIFICATION_ID_B="$(jq -r --arg uid "$USER_ID_B" '[.[] | select(.user_id == $uid)] | .[0].verification_id // empty' "$TMP_DIR/resp.json")"
  if [ -n "$VERIFICATION_ID_B" ]; then
    STATUS=$(req POST "/admin/verifications/$VERIFICATION_ID_B/approve" "" "$ADMIN_TOKEN")
    assert_status "Approve второго участника (нужен active для /routes)" "200" "$STATUS"
  else
    fail "Не нашёл заявку на верификацию второго участника"
  fi

  ROUTE_BODY_B=$(jq -n '{
    origin:      {lat:43.418949, lng:76.889709, label:"Точка в 20 км от Алматы", source:"osm", country:"kz"},
    destination: {lat:43.825592, lng:87.616848, label:"Урумчи", source:"osm", country:"cn"}}')
  STATUS=$(req POST /routes "$ROUTE_BODY_B" "$TOKEN_B")
  assert_status "POST /routes — точка в ~20 км от заявки (внутри радиуса KZ 40)" "201" "$STATUS"
  if [ "$STATUS" = "201" ]; then
    ROUTE_ID_B="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"
  fi

  STATUS=$(req GET /routes "" "$TOKEN_B")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$ROUTE_ID_B" '[.[] | select(.id == $id)] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "GET /routes показывает добавленное направление"
  else
    fail "GET /routes (HTTP $STATUS) не показывает направление $ROUTE_ID_B"
  fi

  STATUS=$(req GET /cargo/available "" "$TOKEN_B")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$CARGO_ID" '[.[] | select(.id == $id)] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "С инструментом и точкой в пределах радиуса заявка видна"
  else
    fail "GET /cargo/available (HTTP $STATUS) не содержит заявку $CARGO_ID после добавления маршрута в 20 км"
  fi
else
  fail "Пропущено — не хватает tool_id/user_id/токенов/cargo_id"
fi

step "20. Без инструмента submit_offer — предложение подать нельзя"
if [ -n "$TOKEN_B" ] && [ -n "$CARGO_ID" ]; then
  BODY=$(jq -n '{price:150000, conditions:"Смоук-тест предложения", warehouse_fill_percent:40}')
  STATUS=$(req POST "/cargo/$CARGO_ID/offers" "$BODY" "$TOKEN_B")
  assert_status "POST /cargo/:id/offers без submit_offer" "403" "$STATUS"
else
  fail "Пропущено — нет токена второго участника или cargo_id"
fi

step "21. С инструментом submit_offer — предложение подаётся"
OFFER_STATUS_CODE=""
if [ -n "$USER_ID_B" ] && [ -n "$ADMIN_TOKEN" ] && [ -n "$TOOL_ID_RECEIVE" ] && [ -n "$TOOL_ID_SUBMIT" ] && [ -n "$TOKEN_B" ] && [ -n "$CARGO_ID" ]; then
  STATUS=$(req POST "/admin/users/$USER_ID_B/tools" "$(jq -n --arg r "$TOOL_ID_RECEIVE" --arg s "$TOOL_ID_SUBMIT" '{tool_ids:[$r,$s]}')" "$ADMIN_TOKEN")
  assert_status "Назначение submit_offer (вместе с receive_cargo_by_route)" "200" "$STATUS"

  BODY=$(jq -n '{price:150000, conditions:"Смоук-тест предложения", warehouse_fill_percent:40}')
  STATUS=$(req POST "/cargo/$CARGO_ID/offers" "$BODY" "$TOKEN_B")
  assert_status "POST /cargo/:id/offers с submit_offer" "201" "$STATUS"
  OFFER_STATUS_CODE="$STATUS"
else
  fail "Пропущено — не хватает tool_id/user_id/токенов/cargo_id"
fi

step "22. Клиент видит предложения анонимно"
if [ -n "$TOKEN_A" ] && [ -n "$CARGO_ID" ] && [ "$OFFER_STATUS_CODE" = "201" ]; then
  STATUS=$(req GET "/cargo/$CARGO_ID/offers" "" "$TOKEN_A")
  if [ "$STATUS" = "200" ]; then
    HAS_OFFER=$(jq -e '[.[] | select(.offer_number != null and .price != null)] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1 && echo yes || echo no)
    LEAKS_IDENTITY=$(jq -e --arg email "$EMAIL_B" --arg uid "$USER_ID_B" \
      'tostring | test($email) or test($uid)' "$TMP_DIR/resp.json" >/dev/null 2>&1 && echo yes || echo no)
    if [ "$HAS_OFFER" = "yes" ] && [ "$LEAKS_IDENTITY" = "no" ]; then
      pass "GET /cargo/:id/offers отдаёт offer_number/price и не палит email/id участника"
    else
      fail "GET /cargo/:id/offers: has_offer=$HAS_OFFER leaks_identity=$LEAKS_IDENTITY (ожидали yes/no)"
    fi
  else
    fail "GET /cargo/:id/offers вернул HTTP $STATUS вместо 200"
  fi
else
  fail "Пропущено — нет токена клиента, cargo_id или предложение не было подано"
fi

step "23. GET /api/admin/audit-log отдаёт записи по времени действия"
if [ -n "$ADMIN_TOKEN" ]; then
  STATUS=$(req GET "/admin/audit-log?limit=5" "" "$ADMIN_TOKEN")
  if [ "$STATUS" = "200" ]; then
    COUNT=$(jq 'length' "$TMP_DIR/resp.json" 2>/dev/null || echo 0)
    if [ "$COUNT" -gt 0 ]; then
      pass "GET /admin/audit-log вернул $COUNT записей"
    else
      fail "GET /admin/audit-log вернул пустой список (ожидали хотя бы одну запись действий админа)"
    fi
  else
    fail "GET /admin/audit-log вернул HTTP $STATUS вместо 200"
  fi
else
  fail "Пропущено — нет токена админа"
fi

step "24. Направления: участник видит и удаляет только свои"
if [ -n "$TOKEN_A" ] && [ -n "$ROUTE_ID_B" ]; then
  STATUS=$(req GET /routes "" "$TOKEN_A")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$ROUTE_ID_B" '[.[] | select(.id == $id)] | length == 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "GET /routes не показывает чужое направление"
  else
    fail "GET /routes (HTTP $STATUS) показывает чужое направление или ошибка"
  fi

  ROUTE_BODY_A=$(jq -n '{
    origin:      {lat:31.2304, lng:121.4737, label:"Шанхай", source:"osm", country:"cn"},
    destination: {lat:43.238949, lng:76.889709, label:"Алматы", source:"osm", country:"kz"}}')
  STATUS=$(req POST /routes "$ROUTE_BODY_A" "$TOKEN_A")
  assert_status "POST /routes своего направления" "201" "$STATUS"
  ROUTE_ID_A="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  STATUS=$(req POST /routes "$ROUTE_BODY_A" "$TOKEN_A")
  assert_status "Дубль тех же координат" "409" "$STATUS"

  STATUS=$(req DELETE "/routes/$ROUTE_ID_B" "" "$TOKEN_A")
  assert_status "DELETE чужого направления" "404" "$STATUS"

  if [ -n "$ROUTE_ID_A" ]; then
    STATUS=$(req DELETE "/routes/$ROUTE_ID_A" "" "$TOKEN_A")
    assert_status "DELETE своего направления" "200" "$STATUS"
  else
    fail "Пропущено — не получил id своего направления"
  fi
else
  fail "Пропущено — нет токена клиента или route_id второго участника"
fi

step "25. Третий участник: инструмент есть, точка в ~50 км (вне радиуса KZ 40, через админку)"
EMAIL_C="smoke.carrier.${RAND}@example.com"
TOKEN_C=""
USER_ID_C=""
BODY=$(jq -n --arg email "$EMAIL_C" '{email:$email, phone:"+70000000003", company_name:"Smoke Carrier Co", participant_type:"carrier", password:"Smoke12345!"}')
STATUS=$(req POST /register "$BODY")
assert_status "Регистрация третьего участника" "201" "$STATUS"
if [ "$STATUS" = "201" ]; then
  TOKEN_C="$(jq -r '.tokens.access_token // empty' "$TMP_DIR/resp.json")"
  USER_ID_C="$(jq -r '.user.id // empty' "$TMP_DIR/resp.json")"
fi

if [ -n "$USER_ID_C" ] && [ -n "$ADMIN_TOKEN" ] && [ -n "$TOOL_ID_RECEIVE" ]; then
  STATUS=$(req GET "/admin/verifications?status=pending" "" "$ADMIN_TOKEN")
  VERIFICATION_ID_C="$(jq -r --arg uid "$USER_ID_C" '[.[] | select(.user_id == $uid)] | .[0].verification_id // empty' "$TMP_DIR/resp.json")"
  if [ -n "$VERIFICATION_ID_C" ]; then
    STATUS=$(req POST "/admin/verifications/$VERIFICATION_ID_C/approve" "" "$ADMIN_TOKEN")
    assert_status "Approve третьего участника" "200" "$STATUS"
  else
    fail "Не нашёл заявку на верификацию третьего участника"
  fi

  STATUS=$(req POST "/admin/users/$USER_ID_C/tools" "$(jq -n --arg id "$TOOL_ID_RECEIVE" '{tool_ids:[$id]}')" "$ADMIN_TOKEN")
  assert_status "Назначение receive_cargo_by_route третьему" "200" "$STATUS"

  ROUTE_BODY_C=$(jq -n '{
    origin:      {lat:43.688949, lng:76.889709, label:"Точка в 50 км от Алматы", source:"osm", country:"kz"},
    destination: {lat:43.825592, lng:87.616848, label:"Урумчи", source:"osm", country:"cn"}}')
  STATUS=$(req POST "/admin/users/$USER_ID_C/routes" "$ROUTE_BODY_C" "$ADMIN_TOKEN")
  assert_status "Админ добавляет третьему точку в ~50 км от заявки" "201" "$STATUS"

  STATUS=$(req GET "/admin/users/$USER_ID_C/routes" "" "$ADMIN_TOKEN")
  if [ "$STATUS" = "200" ] && jq -e '[.[] | select(.origin.label == "Точка в 50 км от Алматы")] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "GET /admin/users/:id/routes показывает направление"
  else
    fail "GET /admin/users/:id/routes (HTTP $STATUS) не показывает добавленное направление"
  fi

  STATUS=$(req GET /cargo/available "" "$TOKEN_C")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$CARGO_ID" '[.[] | select(.id == $id)] | length == 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "KZ-точка в 50 км (вне радиуса 40) — заявку НЕ видит"
  else
    fail "GET /cargo/available (HTTP $STATUS): участник с точкой в 50 км видит заявку или ошибка"
  fi
else
  fail "Пропущено — не хватает user_id третьего участника, токена админа или tool_id"
fi

step "26. Уведомление о новой заявке — только участнику с точкой в радиусе"
if [ -n "$TOKEN_A" ] && [ -n "$TOKEN_B" ] && [ -n "$TOKEN_C" ] && [ -n "$CARGO_ID" ]; then
  BODY=$(jq -n '{
    origin:      {lat:43.238949, lng:76.889709, label:"Алматы", source:"osm", country:"kz"},
    destination: {lat:43.825592, lng:87.616848, label:"Урумчи", source:"osm", country:"cn"},
    volume_m3:5, weight_kg:300, description:"Смоук-тест уведомлений"}')
  STATUS=$(req POST /cargo "$BODY" "$TOKEN_A")
  assert_status "Вторая заявка на груз (после появления маршрутов)" "201" "$STATUS"
  CARGO_ID_2="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  if [ -n "$CARGO_ID_2" ]; then
    STATUS=$(req GET /notifications "" "$TOKEN_B")
    if [ "$STATUS" = "200" ]; then
      HAS_2=$(jq -e --arg id "$CARGO_ID_2" '[.[] | select(.type == "cargo_request_available") | .payload.cargo_request_id] | index($id) != null' "$TMP_DIR/resp.json" >/dev/null 2>&1 && echo yes || echo no)
      HAS_1=$(jq -e --arg id "$CARGO_ID" '[.[] | select(.type == "cargo_request_available") | .payload.cargo_request_id] | index($id) != null' "$TMP_DIR/resp.json" >/dev/null 2>&1 && echo yes || echo no)
      if [ "$HAS_2" = "yes" ] && [ "$HAS_1" = "no" ]; then
        pass "Участник с точкой в радиусе получил уведомление о новой заявке (и не получал о старой, поданной до маршрута)"
      else
        fail "Уведомления участника с маршрутом: новая=$HAS_2 старая=$HAS_1 (ожидали yes/no)"
      fi
    else
      fail "GET /notifications участника с маршрутом вернул HTTP $STATUS"
    fi

    STATUS=$(req GET /notifications "" "$TOKEN_C")
    if [ "$STATUS" = "200" ]; then
      if jq -e --arg id "$CARGO_ID_2" '[.[] | select(.type == "cargo_request_available") | .payload.cargo_request_id] | index($id) == null' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
        pass "Участник с точкой в 50 км (вне радиуса 40) уведомление НЕ получил"
      else
        fail "Участник с точкой вне радиуса получил уведомление о заявке $CARGO_ID_2"
      fi
    else
      fail "GET /notifications участника вне радиуса вернул HTTP $STATUS"
    fi
  else
    fail "Не получил id второй заявки"
  fi
else
  fail "Пропущено — не хватает токенов участников или cargo_id"
fi

step "26b. Отметка уведомлений прочитанными"
if [ -n "$TOKEN_B" ]; then
  STATUS=$(req GET /notifications "" "$TOKEN_B")
  UNREAD_BEFORE=$(jq '[.[] | select(.is_read == false)] | length' "$TMP_DIR/resp.json" 2>/dev/null || echo 0)
  if [ "$UNREAD_BEFORE" -gt 0 ]; then
    pass "До отметки есть непрочитанные ($UNREAD_BEFORE)"
  else
    fail "До отметки нет непрочитанных — предыдущие шаги не создали уведомлений?"
  fi

  STATUS=$(req POST /notifications/read "" "$TOKEN_B")
  assert_status "POST /notifications/read" "200" "$STATUS"

  STATUS=$(req GET /notifications "" "$TOKEN_B")
  if [ "$STATUS" = "200" ] && jq -e '[.[] | select(.is_read == false)] | length == 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "После отметки непрочитанных = 0, все is_read=true"
  else
    fail "GET /notifications (HTTP $STATUS): после отметки остались непрочитанные"
  fi
else
  fail "Пропущено — нет токена участника B"
fi

step "27. Haversine: известное расстояние (1° широты по меридиану ≈ 111.19 км)"
DIST_OUT="$( (cd "$ROOT_DIR" && go run ./cmd/geocheck 43 76 44 76) 2>/dev/null | tail -1 )"
if [ -n "$DIST_OUT" ] && awk -v d="$DIST_OUT" 'BEGIN{exit !(d > 110.9 && d < 111.5)}'; then
  pass "geocheck(43,76 → 44,76) = $DIST_OUT км (ожидали ~111.19)"
else
  fail "geocheck(43,76 → 44,76) вернул '$DIST_OUT' (ожидали ~111.19 км)"
fi

DIST_OUT="$( (cd "$ROOT_DIR" && go run ./cmd/geocheck 43.238949 76.889709 43.418949 76.889709) 2>/dev/null | tail -1 )"
if [ -n "$DIST_OUT" ] && awk -v d="$DIST_OUT" 'BEGIN{exit !(d > 19.5 && d < 20.5)}'; then
  pass "Точка «в 20 км» действительно в ~20 км ($DIST_OUT)"
else
  fail "Расстояние до точки «в 20 км» = '$DIST_OUT' (ожидали ~20)"
fi

DIST_OUT="$( (cd "$ROOT_DIR" && go run ./cmd/geocheck 43.825592 87.616848 44.545592 87.616848) 2>/dev/null | tail -1 )"
if [ -n "$DIST_OUT" ] && awk -v d="$DIST_OUT" 'BEGIN{exit !(d > 79.5 && d < 80.6)}'; then
  pass "CN-точка «в 80 км» действительно в ~80 км ($DIST_OUT)"
else
  fail "Расстояние до CN-точки «в 80 км» = '$DIST_OUT' (ожидали ~80)"
fi

step "28. Радиус по стране: KZ 35/50 км и CN 80/120 км"
EMAIL_D="smoke.radius.${RAND}@example.com"
TOKEN_D=""
USER_ID_D=""
BODY=$(jq -n --arg email "$EMAIL_D" '{email:$email, phone:"+70000000004", company_name:"Smoke Radius Co", participant_type:"carrier", password:"Smoke12345!"}')
STATUS=$(req POST /register "$BODY")
assert_status "Регистрация участника D" "201" "$STATUS"
if [ "$STATUS" = "201" ]; then
  TOKEN_D="$(jq -r '.tokens.access_token // empty' "$TMP_DIR/resp.json")"
  USER_ID_D="$(jq -r '.user.id // empty' "$TMP_DIR/resp.json")"
fi

if [ -n "$USER_ID_D" ] && [ -n "$ADMIN_TOKEN" ] && [ -n "$TOOL_ID_RECEIVE" ] && [ -n "$TOKEN_A" ] && [ -n "$TOKEN_D" ]; then
  STATUS=$(req GET "/admin/verifications?status=pending" "" "$ADMIN_TOKEN")
  VERIFICATION_ID_D="$(jq -r --arg uid "$USER_ID_D" '[.[] | select(.user_id == $uid)] | .[0].verification_id // empty' "$TMP_DIR/resp.json")"
  if [ -n "$VERIFICATION_ID_D" ]; then
    STATUS=$(req POST "/admin/verifications/$VERIFICATION_ID_D/approve" "" "$ADMIN_TOKEN")
    assert_status "Approve участника D" "200" "$STATUS"
  else
    fail "Не нашёл заявку на верификацию участника D"
  fi

  STATUS=$(req POST "/admin/users/$USER_ID_D/tools" "$(jq -n --arg id "$TOOL_ID_RECEIVE" '{tool_ids:[$id]}')" "$ADMIN_TOKEN")
  assert_status "Назначение receive_cargo_by_route участнику D" "200" "$STATUS"

  # Маршрут D — ровно Алматы (kz) → ровно Урумчи (cn)
  ROUTE_BODY_D=$(jq -n '{
    origin:      {lat:43.238949, lng:76.889709, label:"Алматы", source:"osm", country:"kz"},
    destination: {lat:43.825592, lng:87.616848, label:"Урумчи", source:"osm", country:"cn"}}')
  STATUS=$(req POST "/admin/users/$USER_ID_D/routes" "$ROUTE_BODY_D" "$ADMIN_TOKEN")
  assert_status "Маршрут D: Алматы → Урумчи" "201" "$STATUS"

  # Хелпер: подать заявку с заданным origin/destination и проверить видимость для D
  check_radius_case() {
    local desc="$1" body="$2" expect_visible="$3"
    local cargo_id
    STATUS=$(req POST /cargo "$body" "$TOKEN_A")
    if [ "$STATUS" != "201" ]; then
      fail "$desc — заявка не создалась (HTTP $STATUS)"
      return
    fi
    cargo_id="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"
    STATUS=$(req GET /cargo/available "" "$TOKEN_D")
    if [ "$STATUS" != "200" ]; then
      fail "$desc — GET /cargo/available вернул HTTP $STATUS"
      return
    fi
    local found
    found=$(jq -e --arg id "$cargo_id" '[.[] | select(.id == $id)] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1 && echo yes || echo no)
    if [ "$found" = "$expect_visible" ]; then
      pass "$desc"
    else
      fail "$desc (видимость=$found, ожидали $expect_visible)"
    fi
  }

  check_radius_case "KZ-точка в ~35 км (внутри 40) — совпадает" "$(jq -n '{
    origin:      {lat:43.553949, lng:76.889709, label:"Точка в 35 км от Алматы", source:"osm", country:"kz"},
    destination: {lat:43.825592, lng:87.616848, label:"Урумчи", source:"osm", country:"cn"},
    volume_m3:1, weight_kg:100, description:"Радиус-тест KZ 35"}')" "yes"

  check_radius_case "KZ-точка в ~50 км (вне 40) — НЕ совпадает" "$(jq -n '{
    origin:      {lat:43.688949, lng:76.889709, label:"Точка в 50 км от Алматы", source:"osm", country:"kz"},
    destination: {lat:43.825592, lng:87.616848, label:"Урумчи", source:"osm", country:"cn"},
    volume_m3:1, weight_kg:100, description:"Радиус-тест KZ 50"}')" "no"

  check_radius_case "CN-точка в ~80 км (внутри 100) — совпадает" "$(jq -n '{
    origin:      {lat:43.238949, lng:76.889709, label:"Алматы", source:"osm", country:"kz"},
    destination: {lat:44.545592, lng:87.616848, label:"Точка в 80 км от Урумчи", source:"osm", country:"cn"},
    volume_m3:1, weight_kg:100, description:"Радиус-тест CN 80"}')" "yes"

  check_radius_case "CN-точка в ~120 км (вне 100) — НЕ совпадает" "$(jq -n '{
    origin:      {lat:43.238949, lng:76.889709, label:"Алматы", source:"osm", country:"kz"},
    destination: {lat:44.905592, lng:87.616848, label:"Точка в 120 км от Урумчи", source:"osm", country:"cn"},
    volume_m3:1, weight_kg:100, description:"Радиус-тест CN 120"}')" "no"
else
  fail "Пропущено — не хватает участника D, токена админа/клиента или tool_id"
fi

step "29. Этап 3: предложения ДО select анонимны + получаем offer_id"
OFFER_ID_1=""
if [ -n "$TOKEN_A" ] && [ -n "$CARGO_ID" ]; then
  STATUS=$(req GET "/cargo/$CARGO_ID/offers" "" "$TOKEN_A")
  if [ "$STATUS" = "200" ]; then
    OFFER_ID_1="$(jq -r '.[0].offer_id // empty' "$TMP_DIR/resp.json")"
    LEAKS=$(jq -e --arg email "$EMAIL_B" --arg uid "$USER_ID_B" \
      'tostring | test($email) or test($uid)' "$TMP_DIR/resp.json" >/dev/null 2>&1 && echo yes || echo no)
    if [ -n "$OFFER_ID_1" ] && [ "$LEAKS" = "no" ]; then
      pass "offer_id получен, личность участника до select не раскрыта"
    else
      fail "offer_id='$OFFER_ID_1' leaks=$LEAKS (ожидали id и no)"
    fi
  else
    fail "GET /cargo/:id/offers вернул HTTP $STATUS"
  fi
else
  fail "Пропущено — нет токена клиента или cargo_id"
fi

step "30. Select без подписки: 1-й контакт открывается, чат создаётся"
CHAT_ID=""
if [ -n "$TOKEN_A" ] && [ -n "$CARGO_ID" ] && [ -n "$OFFER_ID_1" ]; then
  STATUS=$(req POST "/cargo/$CARGO_ID/select" "$(jq -n --arg id "$OFFER_ID_1" '{offer_id:$id}')" "$TOKEN_A")
  assert_status "POST /cargo/:id/select (1-й контакт)" "200" "$STATUS"
  if [ "$STATUS" = "200" ]; then
    CHAT_ID="$(jq -r '.chat_id // empty' "$TMP_DIR/resp.json")"
    CONTACT_EMAIL="$(jq -r '.contact.email // empty' "$TMP_DIR/resp.json")"
    USED="$(jq -r '.reveals_used // empty' "$TMP_DIR/resp.json")"
    if [ "$CONTACT_EMAIL" = "$EMAIL_B" ] && [ -n "$CHAT_ID" ] && [ "$USED" = "1" ]; then
      pass "Контакт исполнителя раскрыт ($CONTACT_EMAIL), чат создан, использовано 1"
    else
      fail "contact.email='$CONTACT_EMAIL' chat_id='$CHAT_ID' used='$USED' (ожидали email B / id / 1)"
    fi
  fi

  STATUS=$(req GET /cargo/mine "" "$TOKEN_A")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$CARGO_ID" '[.[] | select(.id == $id and .status == "matched")] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "Заявка после select в статусе matched"
  else
    fail "Заявка $CARGO_ID не перешла в matched (HTTP $STATUS)"
  fi
else
  fail "Пропущено — нет токена/cargo_id/offer_id"
fi

step "31. Второй контакт без подписки — лимит (429)"
OFFER_ID_2=""
if [ -n "$TOKEN_B" ] && [ -n "$CARGO_ID_2" ] && [ -n "$TOKEN_A" ]; then
  BODY=$(jq -n '{price:180000, conditions:"Оффер для теста лимита", warehouse_fill_percent:55}')
  STATUS=$(req POST "/cargo/$CARGO_ID_2/offers" "$BODY" "$TOKEN_B")
  assert_status "Оффер B на вторую заявку" "201" "$STATUS"
  OFFER_ID_2="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  if [ -n "$OFFER_ID_2" ]; then
    STATUS=$(req POST "/cargo/$CARGO_ID_2/select" "$(jq -n --arg id "$OFFER_ID_2" '{offer_id:$id}')" "$TOKEN_A")
    assert_status "Второй select без подписки" "429" "$STATUS"
  else
    fail "Не получил id второго оффера"
  fi
else
  fail "Пропущено — нет токенов или CARGO_ID_2"
fi

step "32. Подписка вручную — контактов открывается больше"
if [ -n "$USER_ID_A" ] && [ -n "$ADMIN_TOKEN" ] && [ -n "$OFFER_ID_2" ] && [ -n "$CARGO_ID_2" ]; then
  STATUS=$(req POST "/admin/users/$USER_ID_A/subscription" '{"has_subscription":true}' "$ADMIN_TOKEN")
  assert_status "Админ включает подписку клиенту" "200" "$STATUS"

  STATUS=$(req POST "/cargo/$CARGO_ID_2/select" "$(jq -n --arg id "$OFFER_ID_2" '{offer_id:$id}')" "$TOKEN_A")
  assert_status "Select с подпиской" "200" "$STATUS"
  if [ "$STATUS" = "200" ]; then
    USED="$(jq -r '.reveals_used // empty' "$TMP_DIR/resp.json")"
    LIMIT="$(jq -r '.reveals_limit // empty' "$TMP_DIR/resp.json")"
    if [ "$USED" = "2" ] && [ "$LIMIT" = "5" ]; then
      pass "С подпиской: использовано 2 из 5"
    else
      fail "reveals_used=$USED reveals_limit=$LIMIT (ожидали 2/5)"
    fi
  fi
else
  fail "Пропущено — нет user_id/токена админа/offer_id"
fi

step "33. Чат: участники пишут и читают, посторонний — нет"
if [ -n "$CHAT_ID" ] && [ -n "$TOKEN_A" ] && [ -n "$TOKEN_B" ] && [ -n "$TOKEN_C" ]; then
  STATUS=$(req GET /chats/mine "" "$TOKEN_A")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$CHAT_ID" '[.[] | select(.id == $id)] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "GET /chats/mine клиента содержит созданный чат"
  else
    fail "GET /chats/mine (HTTP $STATUS) не содержит чат $CHAT_ID"
  fi

  STATUS=$(req POST "/chats/$CHAT_ID/messages" '{"body":"Здравствуйте! Когда сможете забрать груз?"}' "$TOKEN_A")
  assert_status "Клиент пишет в чат" "201" "$STATUS"
  MSG_ID_1="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  STATUS=$(req GET "/chats/$CHAT_ID/messages" "" "$TOKEN_B")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$MSG_ID_1" '[.[] | select(.id == $id)] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "Исполнитель видит сообщение клиента"
  else
    fail "GET messages исполнителем (HTTP $STATUS) не содержит сообщение"
  fi

  STATUS=$(req POST "/chats/$CHAT_ID/messages" '{"body":"Добрый день! Завтра после обеда."}' "$TOKEN_B")
  assert_status "Исполнитель отвечает" "201" "$STATUS"
  MSG_ID_2="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  STATUS=$(req GET "/chats/$CHAT_ID/messages?after=$MSG_ID_1" "" "$TOKEN_A")
  if [ "$STATUS" = "200" ] \
     && jq -e --arg id "$MSG_ID_2" '[.[] | select(.id == $id)] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1 \
     && jq -e --arg id "$MSG_ID_1" '[.[] | select(.id == $id)] | length == 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "?after=<id> отдаёт только новые сообщения"
  else
    fail "GET messages?after (HTTP $STATUS): курсор работает неверно"
  fi

  STATUS=$(req GET "/chats/$CHAT_ID/messages" "" "$TOKEN_C")
  assert_status "Посторонний читает чужой чат" "404" "$STATUS"

  STATUS=$(req POST "/chats/$CHAT_ID/messages" '{"body":"взлом"}' "$TOKEN_C")
  assert_status "Посторонний пишет в чужой чат" "404" "$STATUS"
else
  fail "Пропущено — нет chat_id или токенов A/B/C"
fi

# --- Этап 5: консолидация ---------------------------------------------------
# Требуется запущенный Python-сервис: cd matching && uvicorn main:app --port 8000

# cargo_body lat1 lng1 label1 lat2 lng2 label2 volume weight desc
cargo_body() {
  jq -n --argjson lat1 "$1" --argjson lng1 "$2" --arg l1 "$3" \
        --argjson lat2 "$4" --argjson lng2 "$5" --arg l2 "$6" \
        --argjson vol "$7" --argjson wt "$8" --arg d "$9" '{
    origin:      {lat:$lat1, lng:$lng1, label:$l1, source:"osm", country:"kz"},
    destination: {lat:$lat2, lng:$lng2, label:$l2, source:"osm", country:"cn"},
    volume_m3:$vol, weight_kg:$wt, description:$d}'
}

# consolidation_of cargo_id token → печатает suggestion_id или "none"
consolidation_of() {
  req GET "/cargo/$1/consolidation" "" "$2" >/dev/null
  jq -r 'if . == null then "none" else (.suggestion_id // "none") end' "$TMP_DIR/resp.json" 2>/dev/null || echo "none"
}

step "34. Консолидация: Python-сервис доступен, настройки сброшены к 90/20000"
if curl -sf "${MATCHING_SERVICE_URL:-http://localhost:8000}/health" >/dev/null 2>&1; then
  pass "Matching-сервис отвечает на /health"
else
  fail "Matching-сервис НЕ запущен. Запустите: cd matching && uvicorn main:app --port 8000 — и повторите smoke"
fi

if [ -n "$ADMIN_TOKEN" ]; then
  STATUS=$(req PATCH /admin/settings '{"max_volume_m3":90,"max_weight_kg":20000}' "$ADMIN_TOKEN")
  assert_status "Сброс лимитов вместимости к 90/20000" "200" "$STATUS"
  STATUS=$(req GET /admin/settings "" "$ADMIN_TOKEN")
  if [ "$STATUS" = "200" ] && jq -e '.max_volume_m3 == 90 and .max_weight_kg == 20000' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "GET /admin/settings отдаёт 90/20000"
  else
    fail "GET /admin/settings (HTTP $STATUS) не отдаёт ожидаемые лимиты"
  fi
else
  fail "Пропущено — нет токена админа"
fi

step "35. Две заявки одного направления в пределах лимита → оба получают suggestion"
EMAIL_E="smoke.client2.${RAND}@example.com"
TOKEN_E=""
USER_ID_E=""
BODY=$(jq -n --arg email "$EMAIL_E" '{email:$email, phone:"+70000000005", company_name:"Smoke Client Two", participant_type:"client", password:"Smoke12345!"}')
STATUS=$(req POST /register "$BODY")
assert_status "Регистрация клиента E" "201" "$STATUS"
if [ "$STATUS" = "201" ]; then
  TOKEN_E="$(jq -r '.tokens.access_token // empty' "$TMP_DIR/resp.json")"
  USER_ID_E="$(jq -r '.user.id // empty' "$TMP_DIR/resp.json")"
fi
if [ -n "$USER_ID_E" ] && [ -n "$ADMIN_TOKEN" ]; then
  STATUS=$(req GET "/admin/verifications?status=pending" "" "$ADMIN_TOKEN")
  VER_E="$(jq -r --arg uid "$USER_ID_E" '[.[] | select(.user_id == $uid)] | .[0].verification_id // empty' "$TMP_DIR/resp.json")"
  if [ -n "$VER_E" ]; then
    STATUS=$(req POST "/admin/verifications/$VER_E/approve" "" "$ADMIN_TOKEN")
    assert_status "Approve клиента E" "200" "$STATUS"
  else
    fail "Не нашёл заявку на верификацию клиента E"
  fi
fi

CARGO_S1=""
CARGO_S2=""
SID_1=""
if [ -n "$TOKEN_A" ] && [ -n "$TOKEN_E" ]; then
  STATUS=$(req POST /cargo "$(cargo_body 42.3417 69.5901 "Шымкент" 39.4704 75.9898 "Кашгар" 30 5000 "Консолидация S1")" "$TOKEN_A")
  assert_status "Заявка S1 клиента A (30 м³ / 5000 кг)" "201" "$STATUS"
  CARGO_S1="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  RES=$(consolidation_of "$CARGO_S1" "$TOKEN_A")
  if [ "$RES" = "none" ]; then
    pass "До второй заявки предложения объединиться нет"
  else
    fail "Преждевременный suggestion на S1: $RES"
  fi

  STATUS=$(req POST /cargo "$(cargo_body 42.3417 69.5901 "Шымкент" 39.4704 75.9898 "Кашгар" 40 6000 "Консолидация S2")" "$TOKEN_E")
  assert_status "Заявка S2 клиента E (40 м³ / 6000 кг)" "201" "$STATUS"
  CARGO_S2="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  SID_1=$(consolidation_of "$CARGO_S1" "$TOKEN_A")
  SID_1_E=$(consolidation_of "$CARGO_S2" "$TOKEN_E")
  if [ "$SID_1" != "none" ] && [ "$SID_1" = "$SID_1_E" ]; then
    pass "Оба клиента видят один и тот же suggestion"
  else
    fail "suggestion у A='$SID_1', у E='$SID_1_E' (ожидали одинаковый id)"
  fi

  # Контакты клиентов друг другу не раскрываются
  req GET "/cargo/$CARGO_S2/consolidation" "" "$TOKEN_E" >/dev/null
  if jq -e --arg email "$EMAIL_A" --arg uid "$USER_ID_A" 'tostring | test($email) or test($uid)' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    fail "Ответ consolidation для E содержит email/id клиента A"
  else
    pass "Контакты клиента A не раскрыты клиенту E"
  fi

  STATUS=$(req GET /notifications "" "$TOKEN_E")
  if [ "$STATUS" = "200" ] && jq -e '[.[] | select(.type == "consolidation_suggested")] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "Клиент E получил уведомление consolidation_suggested"
  else
    fail "У клиента E нет уведомления consolidation_suggested (HTTP $STATUS)"
  fi
else
  fail "Пропущено — нет токенов A/E"
fi

step "36. Оба согласны → создаётся consolidated_request, участники закрыты"
CONS_ID=""
if [ -n "$SID_1" ] && [ "$SID_1" != "none" ]; then
  STATUS=$(req POST "/cargo/$CARGO_S1/consolidation/$SID_1/agree" "" "$TOKEN_A")
  assert_status "Клиент A соглашается" "200" "$STATUS"

  STATUS=$(req POST "/cargo/$CARGO_S2/consolidation/$SID_1/agree" "" "$TOKEN_E")
  assert_status "Клиент E соглашается" "200" "$STATUS"
  if [ "$STATUS" = "200" ] && jq -e '.status == "both_agreed"' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "Статус предложения — both_agreed"
  else
    fail "После второго согласия статус не both_agreed"
  fi

  STATUS=$(req GET /consolidated/mine "" "$TOKEN_A")
  if [ "$STATUS" = "200" ]; then
    CONS_ID="$(jq -r '[.[] | select(.total_volume_m3 == 70 and .total_weight_kg == 11000 and .status == "open")] | .[0].id // empty' "$TMP_DIR/resp.json")"
    if [ -n "$CONS_ID" ]; then
      pass "Объединённая заявка создана (70 м³ / 11000 кг, open)"
    else
      fail "В /consolidated/mine нет объединённой заявки 70/11000"
    fi
  else
    fail "GET /consolidated/mine вернул HTTP $STATUS"
  fi

  STATUS=$(req GET /consolidated/mine "" "$TOKEN_E")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$CONS_ID" '[.[] | select(.id == $id)] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "Объединённая заявка видна и клиенту E"
  else
    fail "Клиент E не видит объединённую заявку (HTTP $STATUS)"
  fi

  STATUS=$(req GET /cargo/mine "" "$TOKEN_A")
  if [ "$STATUS" = "200" ] && jq -e --arg id "$CARGO_S1" '[.[] | select(.id == $id and .status == "closed")] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "Заявка S1 закрыта (ушла в объединённую)"
  else
    fail "Заявка S1 не перешла в closed"
  fi
else
  fail "Пропущено — нет suggestion_id из шага 35"
fi

step "37. Один отказался → consolidated не создаётся, заявки остаются открытыми"
if [ -n "$TOKEN_A" ] && [ -n "$TOKEN_E" ]; then
  STATUS=$(req POST /cargo "$(cargo_body 42.9000 71.3667 "Тараз" 44.2107 80.4184 "Хоргос" 20 3000 "Консолидация T1")" "$TOKEN_A")
  assert_status "Заявка T1 клиента A" "201" "$STATUS"
  CARGO_T1="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  STATUS=$(req POST /cargo "$(cargo_body 42.9000 71.3667 "Тараз" 44.2107 80.4184 "Хоргос" 25 4000 "Консолидация T2")" "$TOKEN_E")
  assert_status "Заявка T2 клиента E" "201" "$STATUS"
  CARGO_T2="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  SID_2=$(consolidation_of "$CARGO_T1" "$TOKEN_A")
  if [ "$SID_2" != "none" ]; then
    pass "Suggestion по Тараз→Хоргос создан"

    STATUS=$(req POST "/cargo/$CARGO_T1/consolidation/$SID_2/decline" "" "$TOKEN_A")
    assert_status "Клиент A отказывается" "200" "$STATUS"

    RES=$(consolidation_of "$CARGO_T2" "$TOKEN_E")
    if [ "$RES" = "none" ]; then
      pass "После отказа активного suggestion у E нет"
    else
      fail "После отказа у E всё ещё есть suggestion: $RES"
    fi

    STATUS=$(req GET /cargo/mine "" "$TOKEN_A")
    if jq -e --arg id "$CARGO_T1" '[.[] | select(.id == $id and .status == "open")] | length > 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
      pass "T1 осталась открытой — едет своим конкурсом"
    else
      fail "T1 не в статусе open после отказа"
    fi

    STATUS=$(req GET /consolidated/mine "" "$TOKEN_A")
    if jq -e '[.[] | select(.origin.label == "Тараз")] | length == 0' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
      pass "Consolidated по Тараз→Хоргос не создан"
    else
      fail "Consolidated по отклонённой паре всё-таки создан"
    fi
  else
    fail "Suggestion по Тараз→Хоргос не создан — decline-ветку проверить нельзя"
  fi
else
  fail "Пропущено — нет токенов A/E"
fi

step "38. Суммарный объём выше лимита → пара НЕ предлагается"
CARGO_U1=""
CARGO_U2=""
if [ -n "$TOKEN_A" ] && [ -n "$TOKEN_E" ]; then
  STATUS=$(req POST /cargo "$(cargo_body 50.2839 57.1670 "Актобе" 43.825592 87.616848 "Урумчи" 60 8000 "Лимит U1")" "$TOKEN_A")
  assert_status "Заявка U1 (60 м³)" "201" "$STATUS"
  CARGO_U1="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  STATUS=$(req POST /cargo "$(cargo_body 50.2839 57.1670 "Актобе" 43.825592 87.616848 "Урумчи" 60 9000 "Лимит U2")" "$TOKEN_E")
  assert_status "Заявка U2 (60 м³)" "201" "$STATUS"
  CARGO_U2="$(jq -r '.id // empty' "$TMP_DIR/resp.json")"

  RES=$(consolidation_of "$CARGO_U2" "$TOKEN_E")
  if [ "$RES" = "none" ]; then
    pass "120 м³ > лимита 90 — suggestion не создан"
  else
    fail "Suggestion создан несмотря на превышение лимита: $RES"
  fi
else
  fail "Пропущено — нет токенов A/E"
fi

step "39. Админ поднимает лимит до 200 — новое значение применяется"
if [ -n "$ADMIN_TOKEN" ] && [ -n "$CARGO_U1" ] && [ -n "$TOKEN_E" ]; then
  STATUS=$(req PATCH /admin/settings '{"max_volume_m3":200,"max_weight_kg":20000}' "$ADMIN_TOKEN")
  assert_status "PATCH /admin/settings (200/20000)" "200" "$STATUS"

  STATUS=$(req GET /admin/settings "" "$ADMIN_TOKEN")
  if [ "$STATUS" = "200" ] && jq -e '.max_volume_m3 == 200' "$TMP_DIR/resp.json" >/dev/null 2>&1; then
    pass "Новый лимит 200 сохранён"
  else
    fail "GET /admin/settings не отдаёт 200"
  fi

  # Новая заявка триггерит матчинг заново: U1(60)+U2(60)=120 теперь <= 200
  STATUS=$(req POST /cargo "$(cargo_body 50.2839 57.1670 "Актобе" 43.825592 87.616848 "Урумчи" 55 7000 "Триггер U3")" "$TOKEN_E")
  assert_status "Заявка-триггер U3" "201" "$STATUS"

  RES=$(consolidation_of "$CARGO_U1" "$TOKEN_A")
  if [ "$RES" != "none" ]; then
    pass "После поднятия лимита пара по Актобе→Урумчи предложена (лимит применён без перезапуска)"
  else
    fail "После поднятия лимита suggestion на U1 так и не появился"
  fi
else
  fail "Пропущено — нет токена админа или заявок U1/U3"
fi

step "Итоги"
TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo "Всего проверок: $TOTAL, PASS: $PASS_COUNT, FAIL: $FAIL_COUNT"
if [ "$FAIL_COUNT" -gt 0 ]; then
  echo "Провалившиеся проверки:"
  for f in "${FAILURES[@]}"; do
    echo "  - $f"
  done
  exit 1
fi
echo "Все проверки прошли."
exit 0
