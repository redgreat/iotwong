#!/usr/bin/env bash
# One-time admin bootstrap: create the iotwong_app runtime role on the
# local cluster and keep its credentials in the git-ignored .env.local.
#
# Only an admin (superuser) can create roles; iotwong_owner intentionally
# lacks CREATEROLE (docs/03-data-design.md). This script must NOT be run by
# app roles and never prints the password.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="$ROOT/.env.local"
CONTAINER="${IOTWONG_PG_CONTAINER:-postgres}"
APP_USER="iotwong_app"

[ -f "$ENV_FILE" ] || { echo "bootstrap: missing $ENV_FILE" >&2; exit 1; }
chmod 600 "$ENV_FILE"

# Generate the password once, store it only in the ignored file.
if ! grep -q '^IOTWONG_APP_PASSWORD=' "$ENV_FILE"; then
    PW="$(openssl rand -hex 24)"
    printf '\nIOTWONG_APP_USER=%s\nIOTWONG_APP_PASSWORD=%s\n' "$APP_USER" "$PW" >> "$ENV_FILE"
    chmod 600 "$ENV_FILE"
    echo "bootstrap: generated IOTWONG_APP_PASSWORD into .env.local (not printed)"
fi

# Import values for the current process without echoing the file.
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

if [ -z "${IOTWONG_APP_PASSWORD:-}" ]; then
    echo "bootstrap: IOTWONG_APP_PASSWORD empty in $ENV_FILE" >&2
    exit 1
fi

# Password charset is hex, so single-quoted SQL interpolation is safe.
SQL_TMP="$(mktemp /tmp/iotwong-app-role.XXXXXX.sql)"
trap 'rm -f "$SQL_TMP"' EXIT
cat > "$SQL_TMP" <<EOF
-- create iotwong_app if missing, then align its password with .env.local
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${APP_USER}') THEN
        CREATE ROLE ${APP_USER} LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;
END
\$\$;
ALTER ROLE ${APP_USER} WITH LOGIN PASSWORD '${IOTWONG_APP_PASSWORD}'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;
EOF

echo "bootstrap: applying role to container '$CONTAINER' ..."
docker exec -i "$CONTAINER" psql -U postgres -v ON_ERROR_STOP=1 < "$SQL_TMP"
echo "bootstrap: role '$APP_USER' ready (password kept in ignored .env.local)"
