#!/usr/bin/env bash
# run-mqtt-chain.sh — MQTT ingest-chain acceptance against a running Mosquitto
# broker + its PostgreSQL database:
#   1. sim-send publishes ONE dev-1 frame (event id prefix acpt-<rand>);
#   2. within a few seconds ingest_events must contain exactly that event (ok);
#   3. the identical frame is published again -> still exactly one row
#      (QoS1 at-least-once idempotency / duplicate replay).
#
# Usage:
#   tests/run-mqtt-chain.sh
#
# Env:
#   MQTT_URL / MQTT_PORT   broker url (default tcp://127.0.0.1:${MQTT_PORT:-1883})
#   MQTT_USER / MQTT_PASSWORD   device credentials (sim-send env)
#   MQTT_DEVICE            device external id (default dev-1)
#   DB_VIA=host|docker     psql transport (default host)
#   DB_HOST/DB_PORT        host mode psql endpoint (defaults 127.0.0.1/5432)
#   DB_CONTAINER           docker mode container name (e.g. iotwong-standalone-db-1)
#   DB_USER / DB_NAME      psql identity (default iotwong_owner / iotwong)
#   PGPASSWORD             exported database password (never passed on argv)
#   SIM_SEND               path to sim-send binary (default <repo>/backend/bin/sim-send)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SIM_SEND="${SIM_SEND:-$ROOT/backend/bin/sim-send}"
[ -x "$SIM_SEND" ] || { echo "run-mqtt-chain: sim-send not found/executable: $SIM_SEND"; exit 2; }

MQTT_DEVICE="${MQTT_DEVICE:-dev-1}"
BROKER="${MQTT_URL:-tcp://127.0.0.1:${MQTT_PORT:-1883}}"
MQTT_USER="${MQTT_USER:-$MQTT_DEVICE}"
[ -n "${MQTT_PASSWORD:-}" ] || { echo "run-mqtt-chain: MQTT_PASSWORD required (env only)"; exit 2; }
DB_VIA="${DB_VIA:-host}"
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-iotwong_owner}"
DB_NAME="${DB_NAME:-iotwong}"
[ -n "${PGPASSWORD:-}" ] || { echo "run-mqtt-chain: PGPASSWORD required (env only)"; exit 2; }

PASSED=0; FAILED=0
ok()  { printf 'MQTT-CHAIN PASS: %s\n' "$*"; PASSED=$((PASSED+1)); }
bad() { printf 'MQTT-CHAIN FAIL: %s\n' "$*"; FAILED=$((FAILED+1)); }

psqlq() { # $1 SQL  -> stdout single value
  case "$DB_VIA" in
    docker) docker exec -e PGPASSWORD -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -tAc "$1" ;;
    *)      psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc "$1" ;;
  esac
}

# wait_until SQL EXPECTED SECONDS: poll until psqlq(SQL) == EXPECTED or timeout.
wait_until() {
  local sql=$1 want=$2 limit=$3 got="" i=0
  while [ "$i" -lt "$limit" ]; do
    got=$(psqlq "$sql" 2>/dev/null | tr -d '[:space:]' || true)
    [ "$got" = "$want" ] && return 0
    sleep 1; i=$((i+1))
  done
  return 1
}

PREFIX="acpt-$(openssl rand -hex 4)"
EVENT="$PREFIX-seq-1"
echo "run-mqtt-chain: broker=$BROKER device=$MQTT_DEVICE event=$EVENT db_via=$DB_VIA"

# ---- send frame #1 ----------------------------------------------------------
if MQTT_USER="$MQTT_USER" MQTT_PASSWORD="$MQTT_PASSWORD" \
   "$SIM_SEND" --broker "$BROKER" --device "$MQTT_DEVICE" \
   --count 1 --dup 0 --prefix "$PREFIX" >/dev/null; then
  ok "sim-send frame #1 published"
else
  bad "sim-send frame #1 publish failed"
fi

SQL="SELECT count(*) FROM ingest_events WHERE event_id='$EVENT'"
SQLOK="SELECT count(*) FROM ingest_events WHERE event_id='$EVENT' AND status='ok'"
sleep 3
if wait_until "$SQL" 1 10; then
  st=$(psqlq "$SQLOK" | tr -d '[:space:]')
  if [ "$st" = 1 ]; then
    ok "ingest_events has event '$EVENT' exactly once (status=ok) after 3s"
  else
    bad "ingest_events row for '$EVENT' has unexpected status (ok_rows=$st)"
  fi
else
  bad "ingest_events did not record '$EVENT' within 10s"
fi

# ---- resend identical frame -> idempotent duplicate --------------------------
if MQTT_USER="$MQTT_USER" MQTT_PASSWORD="$MQTT_PASSWORD" \
   "$SIM_SEND" --broker "$BROKER" --device "$MQTT_DEVICE" \
   --count 1 --dup 1 --prefix "$PREFIX" >/dev/null; then
  ok "sim-send identical frame republished (dup replay)"
else
  bad "sim-send duplicate publish failed"
fi
if wait_until "$SQL" 1 10; then
  sleep 2
  n=$(psqlq "$SQL" | tr -d '[:space:]')
  if [ "$n" = 1 ]; then
    ok "duplicate replay did not create a second row (still 1, idempotent)"
  else
    bad "duplicate replay left $n rows (want 1)"
  fi
else
  bad "after duplicate replay ingest_events count != 1"
fi

echo "run-mqtt-chain: $PASSED passed, $FAILED failed (event=$EVENT)"
[ "$FAILED" -eq 0 ]
