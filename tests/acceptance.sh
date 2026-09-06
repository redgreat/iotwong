#!/usr/bin/env bash
# acceptance.sh — top-level post-deployment acceptance orchestration for the
# standalone (all-container) deployment.
#
# Flow (docs/07-acceptance.md A10 / task):
#   0. refuse any compose project other than iotwong-standalone
#   1. docker compose -p iotwong-standalone -f compose.standalone.yaml down -v
#   2. export WEB_PORT MQTT_PORT and up -d --build (cold start)
#   3. wait db healthy + migrate finished + api/web ready (<=180s)
#   4. seed admin into the standalone DB (one-time password -> tests/.seedenv,
#      git-ignored, 0600, kept only while the stack is up for debugging)
#   5. tests/run-contract.sh   (BASE_URL http://127.0.0.1:$WEB_PORT)
#   6. tests/run-mqtt-chain.sh (broker 127.0.0.1:$MQTT_PORT, standalone DB)
#   7. tests/run-ui-e2e.sh    (APP_URL http://127.0.0.1:$WEB_PORT)
# Any failing suite prints FAIL and the script exits non-zero (no fake retry).
#
# Env: COMPOSE_PROJECT (must stay iotwong-standalone), WEB_PORT (18100),
#      MQTT_PORT (11900), DOWN_AFTER=1 to `down` (no -v) at the end.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE_PROJECT="${COMPOSE_PROJECT:-iotwong-standalone}"
COMPOSE_FILE="compose.standalone.yaml"
[ "$COMPOSE_PROJECT" = "iotwong-standalone" ] || {
  echo "acceptance: refusing to touch compose project '$COMPOSE_PROJECT' (only iotwong-standalone is allowed)"; exit 2; }
# caller-provided overrides win; repo .env is loaded below and must not clobber them
CALLER_WEB_PORT="${WEB_PORT:-}"
CALLER_MQTT_PORT="${MQTT_PORT:-}"
WEB_PORT="${CALLER_WEB_PORT:-18100}"
MQTT_PORT="${CALLER_MQTT_PORT:-11900}"
DOWN_AFTER="${DOWN_AFTER:-0}"

EVID_DIR="$ROOT/tests/evidence"
mkdir -p "$EVID_DIR"
SEEDENV="$ROOT/tests/.seedenv"
rm -f "$SEEDENV"

need() { command -v "$1" >/dev/null 2>&1 || { echo "acceptance: missing tool: $1"; exit 2; }; }
need docker; need curl; need openssl; need jq

log() { printf '%s\n' "$*"; }
step() { log ""; log "== acceptance: $* =="; }

# load compose env (repo .env) into the shell; never print values
set -a
# shellcheck disable=SC1091
. "$ROOT/.env"
set +a

# re-assert effective ports: caller-provided env wins, else acceptance defaults;
# the repo .env WEB_PORT/MQTT_PORT are NOT used by the acceptance stack.
WEB_PORT="${CALLER_WEB_PORT:-18100}"
MQTT_PORT="${CALLER_MQTT_PORT:-11900}"
export WEB_PORT MQTT_PORT

# backend image builds: module proxy mirror override (proxy.golang.org is not
# reliably reachable from this host during docker builds)
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOSUMDB="${GOSUMDB:-off}"

# Guard: never run the whole flow as "another project"
# The one-time admin password stays in git-ignored tests/.seedenv (0600) while
# the stack is up for debugging; the next acceptance run removes and re-seeds
# it (and `down -v` wipes the volumes anyway).

# ---- 1. teardown previous standalone stack (volumes included) --------------
step "teardown previous $COMPOSE_PROJECT stack"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" down -v --remove-orphans \
  >"$EVID_DIR/acceptance-down.log" 2>&1 || {
    cat "$EVID_DIR/acceptance-down.log"; echo "acceptance: compose down failed"; exit 1; }

# ---- 2. cold start -----------------------------------------------------------
step "cold start: WEB_PORT=$WEB_PORT MQTT_PORT=$MQTT_PORT up -d --build"
export WEB_PORT MQTT_PORT
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" config --quiet \
  || { echo "acceptance: compose config invalid"; exit 1; }
if ! docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" up -d --build \
     >"$EVID_DIR/acceptance-up.log" 2>&1; then
  cat "$EVID_DIR/acceptance-up.log"
  echo "acceptance: compose up --build failed"; exit 1
fi

# ---- 3. readiness: db healthy -> migrate done -> api/web ready (<=180s) ------
step "wait for standalone readiness (<=180s)"
DB_C="$COMPOSE_PROJECT-db-1"
WEB_BASE="http://127.0.0.1:$WEB_PORT"
READY_URL="$WEB_BASE/api/v1/health/ready"
deadline=$((SECONDS + 180))
db_ok=0; web_ok=0
while [ "$SECONDS" -lt "$deadline" ]; do
  if [ "$db_ok" = 0 ]; then
    st=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' \
         "$DB_C" 2>/dev/null || echo missing)
    [ "$st" = healthy ] && { db_ok=1; log "db healthy after ${SECONDS}s"; }
  fi
  if [ "$web_ok" = 0 ]; then
    code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 3 "$READY_URL" 2>/dev/null || true)
    [ "$code" = 200 ] && { web_ok=1; log "web+api ready after ${SECONDS}s (HTTP 200)"; }
  fi
  [ "$db_ok" = 1 ] && [ "$web_ok" = 1 ] && break
  sleep 3
done
if [ "$db_ok" != 1 ] || [ "$web_ok" != 1 ]; then
  echo "acceptance FAIL: standalone not ready in 180s (db_ok=$db_ok web_ok=$web_ok)"
  docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" ps >&2 || true
  docker logs --tail 40 "$DB_C" >&2 2>&1 || true
  docker logs --tail 40 "$COMPOSE_PROJECT-api-1" >&2 2>&1 || true
  exit 1
fi

# ---- 4. seed admin ------------------------------------------------------------
# 本地测试固定使用通用测试密码（admin/viewer 等测试账号通用），
# 避免每次发布都去临时文件找密码；如需覆盖可设 ACCEPT_TEST_PASSWORD。
step "seed admin into standalone DB"
NET="$COMPOSE_PROJECT"_default
docker network inspect "$NET" >/dev/null 2>&1 \
  || { echo "acceptance: network $NET not found"; exit 1; }
ADMIN_PW="${ACCEPT_TEST_PASSWORD:-123456}"
umask 077
printf 'SEED_PASSWORD=%s\n' "$ADMIN_PW" > "$SEEDENV"
chmod 600 "$SEEDENV"

# seed needs PG* of the standalone db container on the compose network;
# export PGPASSWORD only through the environment (never argv).
export SEED_PASSWORD="$ADMIN_PW"
export PGHOST=db PGPORT=5432
export PGDATABASE="$POSTGRES_DB" PGUSER="$POSTGRES_USER" PGPASSWORD="$POSTGRES_PASSWORD"
if ! docker run --rm --network "$NET" \
     -e PGHOST=db -e PGPORT=5432 -e PGDATABASE -e PGUSER -e PGPASSWORD -e SEED_PASSWORD \
     -v "$ROOT/backend/bin:/app:ro" golang:1.27.1-alpine \
     /app/seed --login admin --role admin --reset \
     >"$EVID_DIR/acceptance-seed-admin.log" 2>&1; then
  echo "acceptance FAIL: seed admin failed"
  cat "$EVID_DIR/acceptance-seed-admin.log" >&2
  exit 1
fi
cat "$EVID_DIR/acceptance-seed-admin.log"

# UI-E2E demo-data fixture: the SPA renders a 历史回放 chip only for
# source=racebox_demo devices, and web/e2e/ui-e2e.mjs expects that chip on a
# freshly deployed stack. This inserts ONE clearly-labeled, position-less
# racebox_demo fixture device into the throwaway standalone DB (wiped by the
# next `down -v`) so the UI test data matches the dev environment. It never
# touches the host/dev database or eadm.
step "insert racebox_demo UI fixture device (standalone DB only)"
if docker exec -e PGPASSWORD -i "$DB_C" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
     -v ON_ERROR_STOP=1 <<'SQL' >"$EVID_DIR/acceptance-seed-fixture.log" 2>&1
INSERT INTO projects (tenant_id, source, external_id, name)
SELECT t.id, 'racebox_demo', 'racebox-acpt-fixture', 'RaceBox 演示（验收 fixture）'
FROM tenants t WHERE t.name = 'local'
ON CONFLICT (tenant_id, source, external_id) DO NOTHING;

INSERT INTO devices (tenant_id, project_id, source, external_id, name)
SELECT p.tenant_id, p.id, p.source, p.external_id, 'RaceBox 演示设备（验收 fixture）'
FROM projects p
WHERE p.source = 'racebox_demo' AND p.external_id = 'racebox-acpt-fixture'
  AND p.tenant_id IN (SELECT id FROM tenants WHERE name = 'local')
ON CONFLICT (tenant_id, source, external_id) DO NOTHING;

SELECT 'fixture devices: ' || count(*) FROM devices d
JOIN tenants t ON t.id = d.tenant_id
WHERE t.name='local' AND d.source='racebox_demo' AND d.external_id='racebox-acpt-fixture';
SQL
then
  cat "$EVID_DIR/acceptance-seed-fixture.log"
else
  rc=$?
  echo "acceptance FAIL: racebox_demo fixture insert failed (rc=$rc)"
  cat "$EVID_DIR/acceptance-seed-fixture.log" >&2
  exit 1
fi

# viewer seeding inside run-contract goes through the same container path
VIEWER_SEED_CMD="docker run --rm --network $NET \
  -e PGHOST=db -e PGPORT=5432 -e PGDATABASE -e PGUSER -e PGPASSWORD -e SEED_PASSWORD \
  -v $ROOT/backend/bin:/app:ro golang:1.27.1-alpine /app/seed"
export VIEWER_SEED_CMD
export SEED_PASSWORD="$ADMIN_PW"

# ---- 5..7. the three acceptance suites ---------------------------------------
FAILED=0
run_suite() { # name command...
  local name=$1; shift
  local logf="$EVID_DIR/acceptance-$name.log"
  if "$@" >"$logf" 2>&1; then
    cat "$logf"
    echo "ACCEPTANCE PASS: $name"
  else
    cat "$logf"
    echo "ACCEPTANCE FAIL: $name (see $logf)"
    FAILED=1
  fi
}

step "contract suite (BASE_URL=$WEB_BASE)"
run_suite contract tests/run-contract.sh "$WEB_BASE"

step "MQTT ingest chain (broker 127.0.0.1:$MQTT_PORT, standalone DB)"
export MQTT_URL="tcp://127.0.0.1:$MQTT_PORT"
export MQTT_USER="${MQTT_DEVICE_USER:-dev-1}"
export MQTT_PASSWORD="$MQTT_DEVICE_PASSWORD"
export MQTT_DEVICE="${MQTT_DEVICE_ID:-dev-1}"
export DB_VIA=docker DB_CONTAINER="$DB_C" DB_USER="$POSTGRES_USER" DB_NAME="$POSTGRES_DB"
run_suite mqtt-chain tests/run-mqtt-chain.sh

step "UI E2E (APP_URL=$WEB_BASE)"
export SEED_LOGIN=admin
run_suite ui-e2e tests/run-ui-e2e.sh "$WEB_BASE"

# ---- summary ----------------------------------------------------------------
if [ "$FAILED" = 0 ]; then
  echo ""
  echo "ACCEPTANCE: ALL SUITES PASS (project=$COMPOSE_PROJECT web=http://127.0.0.1:$WEB_PORT mqtt=127.0.0.1:$MQTT_PORT)"
  echo "ACCEPTANCE: evidence in $EVID_DIR (per-suite logs below) "
else
  echo ""
  echo "ACCEPTANCE: FAIL — at least one suite failed; evidence in $EVID_DIR (per-suite logs below)"
  exit 1
fi

if [ "$DOWN_AFTER" = 1 ]; then
  echo "ACCEPTANCE: DOWN_AFTER=1 -> stopping stack (no -v)"
  docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" down
fi
# leave the stack running otherwise; next acceptance run starts with down -v
