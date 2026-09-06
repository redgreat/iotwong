#!/usr/bin/env bash
# run-ui-e2e.sh — wrapper around web/e2e/ui-e2e.mjs (headless Chromium).
#
# Usage:
#   tests/run-ui-e2e.sh [APP_URL]      # default http://127.0.0.1:18100
#
# Env:
#   SEED_PASSWORD          required; admin password the UI logs in with
#   SEED_LOGIN             default admin
#   PUPPETEER_EXEC_PATH    default /usr/bin/chromium
#   NODE                   default: node from PATH, fallback /usr/local/bin/node
#
# Prints each UI-E2E PASS/FAIL line plus a summary; non-zero exit on failure.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_URL="${1:-${APP_URL:-http://127.0.0.1:18100}}"

export SEED_PASSWORD="${SEED_PASSWORD:?run-ui-e2e: SEED_PASSWORD is required (env only)}"
export SEED_LOGIN="${SEED_LOGIN:-admin}"
export APP_URL
export PUPPETEER_EXEC_PATH="${PUPPETEER_EXEC_PATH:-/usr/bin/chromium}"

NODE="${NODE:-$(command -v node || true)}"
[ -n "$NODE" ] || NODE=/usr/local/bin/node

if [ ! -x "$PUPPETEER_EXEC_PATH" ]; then
  echo "UI-E2E FAIL: chromium not found at $PUPPETEER_EXEC_PATH"
  exit 1
fi
if [ ! -f "$ROOT/web/e2e/ui-e2e.mjs" ] || [ ! -d "$ROOT/web/node_modules/puppeteer-core" ]; then
  echo "UI-E2E FAIL: web/e2e/ui-e2e.mjs or puppeteer-core missing (run npm install in web/)"
  exit 1
fi

echo "run-ui-e2e: APP_URL=$APP_URL login=$SEED_LOGIN exec=$PUPPETEER_EXEC_PATH"
LOG_DIR="$ROOT/tests/evidence"
mkdir -p "$LOG_DIR"
LOG="$LOG_DIR/ui-e2e-$(date +%Y%m%d-%H%M%S).log"

set +e
"$NODE" "$ROOT/web/e2e/ui-e2e.mjs" 2>&1 | tee "$LOG"
rc=${PIPESTATUS[0]}
set -e

np=$(grep -c 'UI-E2E PASS' "$LOG" 2>/dev/null || true)
nf=$(grep -c 'UI-E2E FAIL' "$LOG" 2>/dev/null || true)
if [ "$rc" -eq 0 ] && [ "$nf" -eq 0 ]; then
  echo "run-ui-e2e: PASS ($np checks) — log $LOG"
  exit 0
fi
echo "run-ui-e2e: FAIL ($nf failure(s), $np passed) — log $LOG"
exit 1
