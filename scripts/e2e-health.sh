#!/usr/bin/env bash
# End-to-end health smoke for the iotwong API.
# Prerequisite: `make build` has produced backend/bin/server.
# Usage: scripts/e2e-health.sh   (E2E_ADDR overrides the listen address)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/backend/bin/server"
ADDR="${E2E_ADDR:-127.0.0.1:18081}"
BASE="http://$ADDR/api/v1"

if [ ! -x "$BIN" ]; then
	echo "e2e-health: missing $BIN — run \`make build\` first" >&2
	exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
	echo "e2e-health: curl is required on the host that issues HTTP checks" >&2
	exit 1
fi

"$BIN" -addr "$ADDR" >/tmp/iotwong-e2e-server.log 2>&1 &
SERVER_PID=$!
cleanup() {
	kill "$SERVER_PID" 2>/dev/null || true
	wait "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT

# Wait until live endpoint responds (max 5s).
ready=""
for _ in $(seq 1 50); do
	if ready="$(curl -sf "$BASE/health/live" 2>/dev/null)"; then
		break
	fi
	sleep 0.1
done
if [ -z "$ready" ]; then
	echo "e2e-health: server did not become live; log:" >&2
	cat /tmp/iotwong-e2e-server.log >&2
	exit 1
fi

check() {
	local name="$1" expected="$2" actual="$3"
	if ! printf '%s' "$actual" | grep -q "$expected"; then
		echo "e2e-health FAIL: $name — expected /$expected/, got: $actual" >&2
		return 1
	fi
	echo "e2e-health PASS: $name"
}

live="$(curl -sf "$BASE/health/live")"
check "live envelope data.status=ok" '"status":"ok"' "$live"
check "live has request_id" '"request_id":"' "$live"

ready_body="$(curl -sf "$BASE/health/ready")"
check "ready data.ready=true" '"ready":true' "$ready_body"
check "ready has request_id" '"request_id":"' "$ready_body"

notfound_code="$(curl -s -o /dev/null -w '%{http_code}' "$BASE/definitely-not-a-route")"
check "unknown route 404" '^404$' "$notfound_code"
notfound_body="$(curl -s "$BASE/definitely-not-a-route")"
check "unknown route error.code=not_found" '"code":"not_found"' "$notfound_body"

echo "e2e-health: all checks passed against $BASE"
