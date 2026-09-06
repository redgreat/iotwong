#!/usr/bin/env bash
# run-contract.sh — HTTP contract acceptance against ANY deployed instance.
#
# Usage:
#   tests/run-contract.sh [BASE_URL]            # default http://127.0.0.1:8080
#
# Required env:
#   SEED_PASSWORD        password of the admin login (also used for the viewer)
# Optional env:
#   ADMIN_LOGIN          admin login name          (default: admin)
#   VIEWER_LOGIN         unique viewer login       (default: viewer-acpt-<pid>)
#   VIEWER_SEED_CMD      command prefix that runs cmd/seed for the viewer;
#                        the script appends: --login <VIEWER_LOGIN>
#                        --role viewer --reset. seed reads SEED_PASSWORD and
#                        PG* from its own environment (never from argv).
#                        Default: <repo>/backend/bin/seed (host PG via PG* env).
#
# Checks (every step echoes PASS/FAIL, failure -> non-zero exit):
#   1. GET /api/v1/health/live   200
#   2. GET /api/v1/health/ready  200
#   3. GET /api/v1/devices       no session -> 401
#   4. admin login -> /auth/me   role == admin
#   5. GET /devices              200, contains local device dev-1
#   6. admin circle-fence create -> listed -> deleted (real API)
#   7. seed viewer -> /devices 200; POST /fences 403; POST /alarms/{id}/ack 403
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_URL="${1:-${BASE_URL:-http://127.0.0.1:8080}}"
BASE_URL="${BASE_URL%/}"
ADMIN_LOGIN="${ADMIN_LOGIN:-admin}"
ADMIN_PASSWORD="${SEED_PASSWORD:?run-contract: SEED_PASSWORD is required (env only)}"
VIEWER_LOGIN="${VIEWER_LOGIN:-viewer-acpt-$$}"
VIEWER_SEED_CMD="${VIEWER_SEED_CMD:-$ROOT/backend/bin/seed}"
SIM="$(command -v jq || true)"
[ -n "$SIM" ] || { echo 'run-contract: jq is required'; exit 2; }

TMPD="$(mktemp -d)"
trap 'rm -rf "$TMPD"' EXIT
ADMIN_JAR="$TMPD/admin.cookies"
VIEWER_JAR="$TMPD/viewer.cookies"
API="$BASE_URL/api/v1"

PASSED=0; FAILED=0
ok()  { printf 'CONTRACT PASS: %s\n' "$*"; PASSED=$((PASSED+1)); }
bad() { printf 'CONTRACT FAIL: %s\n' "$*"; FAILED=$((FAILED+1)); }

# http_req METHOD URL JAR OUTFILE [JSON-BODY]  -> echoes HTTP status code
http_req() {
  local m=$1 u=$2 jar=$3 out=$4 data=${5:-}
  if [ -n "$data" ]; then
    curl -sS -o "$out" -w '%{http_code}' -b "$jar" -c "$jar" -X "$m" \
      -H 'Content-Type: application/json' --data "$data" "$u"
  else
    curl -sS -o "$out" -w '%{http_code}' -b "$jar" -c "$jar" -X "$m" "$u"
  fi
}

# ---- 1. liveness ------------------------------------------------------------
code=$(curl -sS -o "$TMPD/live.json" -w '%{http_code}' "$API/health/live")
if [ "$code" = 200 ] && jq -e '.data.status == "ok"' "$TMPD/live.json" >/dev/null 2>&1; then
  ok "health/live -> 200 ok"
else
  bad "health/live -> HTTP $code (want 200 ok)"
fi

# ---- 2. readiness -----------------------------------------------------------
code=$(curl -sS -o "$TMPD/ready.json" -w '%{http_code}' "$API/health/ready")
if [ "$code" = 200 ] && jq -e '.data.ready == true' "$TMPD/ready.json" >/dev/null 2>&1; then
  ok "health/ready -> 200 ready"
else
  bad "health/ready -> HTTP $code (want 200 ready)"
fi

# ---- 3. unauthenticated /devices -> 401 ------------------------------------
code=$(curl -sS -o "$TMPD/unauth.json" -w '%{http_code}' "$API/devices")
if [ "$code" = 401 ] && jq -e '.error.code == "unauthorized"' "$TMPD/unauth.json" >/dev/null 2>&1; then
  ok "GET /devices without session -> 401 unauthorized"
else
  bad "GET /devices without session -> HTTP $code (want 401 unauthorized)"
fi

# ---- 4. admin login + /auth/me role ----------------------------------------
body=$(jq -nc --arg l "$ADMIN_LOGIN" --arg p "$ADMIN_PASSWORD" '{login:$l,password:$p}')
code=$(http_req POST "$API/auth/login" "$ADMIN_JAR" "$TMPD/login.json" "$body")
if [ "$code" = 200 ]; then
  ok "admin login -> 200"
else
  bad "admin login -> HTTP $code (want 200)"
fi
code=$(http_req GET "$API/auth/me" "$ADMIN_JAR" "$TMPD/me.json")
role=""
if [ "$code" = 200 ]; then
  role=$(jq -r '.data.tenants[]? | select(.role == "admin") | .role' "$TMPD/me.json" 2>/dev/null | head -1)
fi
if [ "$code" = 200 ] && [ "$role" = admin ]; then
  ok "/auth/me -> 200 with admin role"
else
  bad "/auth/me -> HTTP $code role='${role:-<none>}' (want admin)"
fi

# ---- 5. /devices contains local dev-1 --------------------------------------
code=$(http_req GET "$API/devices?limit=100" "$ADMIN_JAR" "$TMPD/devices.json")
dev_id=""
if [ "$code" = 200 ]; then
  dev_id=$(jq -r '.data.items[]? | select(.source == "local" and .external_id == "dev-1") | .id' \
    "$TMPD/devices.json" 2>/dev/null | head -1)
fi
if [ "$code" = 200 ] && [ -n "$dev_id" ]; then
  ok "GET /devices -> 200, contains local device dev-1 ($dev_id)"
else
  bad "GET /devices -> HTTP $code (want 200 with local dev-1)"
fi

# ---- 6. admin fence lifecycle (circle + bind dev-1) -------------------------
FENCE_NAME="acpt-$(date +%s)-$(openssl rand -hex 3)"
FLNG=$(awk -v r="$RANDOM" 'BEGIN { printf "%.6f", 116.40 + (r % 2000) / 100000 }')
fbody=$(jq -nc --arg n "$FENCE_NAME" --argjson lng "$FLNG" --arg id "$dev_id" \
  '{name:$n,kind:"circle",enabled:true,center_longitude:$lng,center_latitude:39.90,radius_m:300.5,device_ids:[$id]}')
code=$(http_req POST "$API/fences" "$ADMIN_JAR" "$TMPD/fence-create.json" "$fbody")
fid=""
if [ "$code" = 200 ]; then
  fid=$(jq -r '.data.id // empty' "$TMPD/fence-create.json" 2>/dev/null)
fi
if [ "$code" = 200 ] && [ -n "$fid" ]; then
  ok "POST /fences (circle, bound dev-1) -> 200 id=$fid"
else
  bad "POST /fences -> HTTP $code (want 200 with id)"
fi
found=""
if [ -n "$fid" ]; then
  code=$(http_req GET "$API/fences" "$ADMIN_JAR" "$TMPD/fences.json")
  if [ "$code" = 200 ]; then
    found=$(jq -r --arg id "$fid" '.data.items[]? | select(.id == $id) | .name' \
      "$TMPD/fences.json" 2>/dev/null | head -1)
  fi
fi
if [ -n "$fid" ] && [ "$found" = "$FENCE_NAME" ]; then
  ok "GET /fences lists the new fence"
else
  bad "GET /fences missing fence $fid (HTTP ${code:-n/a}, found='${found:-<none>}')"
fi
gone=""
if [ -n "$fid" ]; then
  code=$(http_req DELETE "$API/fences/$fid" "$ADMIN_JAR" "$TMPD/fence-del.json")
  if [ "$code" = 200 ]; then
    code2=$(http_req GET "$API/fences" "$ADMIN_JAR" "$TMPD/fences2.json")
    gone=$(jq -r --arg id "$fid" '[.data.items[]? | select(.id == $id)] | length' \
      "$TMPD/fences2.json" 2>/dev/null)
  fi
fi
if [ -n "$fid" ] && [ "$code" = 200 ] && [ "$gone" = 0 ]; then
  ok "DELETE /fences/$fid -> 200 and fence removed"
else
  bad "DELETE /fences/$fid -> HTTP ${code:-n/a} (gone=${gone:-<unknown>}, want 0)"
fi

# ---- 7. viewer: read ok, writes 403 -----------------------------------------
if [ -n "${SEED_SKIP_VIEWER:-}" ]; then
  ok "viewer seeding skipped (SEED_SKIP_VIEWER=1)"
else
  seed_log="$TMPD/seed-viewer.log"
  if SEED_PASSWORD="$ADMIN_PASSWORD" $VIEWER_SEED_CMD \
       --login "$VIEWER_LOGIN" --role viewer --reset >"$seed_log" 2>&1; then
    ok "seed viewer '$VIEWER_LOGIN' (role viewer)"
  else
    bad "seed viewer '$VIEWER_LOGIN' failed (see log)"
    sed 's/^/    seed> /' "$seed_log" >&2
  fi

  vbody=$(jq -nc --arg l "$VIEWER_LOGIN" --arg p "$ADMIN_PASSWORD" '{login:$l,password:$p}')
  code=$(http_req POST "$API/auth/login" "$VIEWER_JAR" "$TMPD/vlogin.json" "$vbody")
  if [ "$code" = 200 ]; then
    ok "viewer login -> 200"
  else
    bad "viewer login -> HTTP $code (want 200)"
  fi
  code=$(http_req GET "$API/devices?limit=10" "$VIEWER_JAR" "$TMPD/vdevices.json")
  if [ "$code" = 200 ]; then
    ok "viewer GET /devices -> 200"
  else
    bad "viewer GET /devices -> HTTP $code (want 200)"
  fi
  code=$(http_req POST "$API/fences" "$VIEWER_JAR" "$TMPD/vfence.json" "$fbody")
  if [ "$code" = 403 ] && jq -e '.error.code == "forbidden"' "$TMPD/vfence.json" >/dev/null 2>&1; then
    ok "viewer POST /fences -> 403 forbidden"
  else
    bad "viewer POST /fences -> HTTP $code (want 403 forbidden)"
  fi
  code=$(http_req POST "$API/alarms/00000000-0000-0000-0000-000000000000/ack" \
    "$VIEWER_JAR" "$TMPD/vack.json")
  if [ "$code" = 403 ] && jq -e '.error.code == "forbidden"' "$TMPD/vack.json" >/dev/null 2>&1; then
    ok "viewer POST /alarms/{id}/ack -> 403 forbidden"
  else
    bad "viewer POST /alarms/{id}/ack -> HTTP $code (want 403 forbidden)"
  fi
fi

echo "run-contract: $PASSED passed, $FAILED failed (BASE_URL=$BASE_URL)"
[ "$FAILED" -eq 0 ]
