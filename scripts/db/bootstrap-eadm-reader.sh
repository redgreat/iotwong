#!/usr/bin/env bash
# One-time admin bootstrap: create a READ-ONLY login role for the legacy
# eadm database (eadm.public.lc_racebox) used by the RaceBox importer.
#
# The eadm database is never modified by this project: this role only
# receives CONNECT + SELECT privileges (no DML/DDL/extension changes).
# Credentials are kept in the git-ignored .env.local, never printed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="$ROOT/.env.local"
CONTAINER="${IOTWONG_PG_CONTAINER:-postgres}"
ROLE_NAME="iotwong_eadm_ro"

[ -f "$ENV_FILE" ] || { echo "bootstrap-eadm: missing $ENV_FILE" >&2; exit 1; }
chmod 600 "$ENV_FILE"

if ! grep -q '^EADM_PASSWORD=' "$ENV_FILE"; then
    PW="$(openssl rand -hex 24)"
    printf '\nEADM_USER=%s\nEADM_PASSWORD=%s\n' "$ROLE_NAME" "$PW" >> "$ENV_FILE"
    chmod 600 "$ENV_FILE"
    echo "bootstrap-eadm: generated EADM_PASSWORD into .env.local (not printed)"
fi

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

if [ -z "${EADM_PASSWORD:-}" ]; then
    echo "bootstrap-eadm: EADM_PASSWORD empty in $ENV_FILE" >&2
    exit 1
fi

SQL_TMP="$(mktemp /tmp/iotwong-eadm-ro.XXXXXX.sql)"
trap 'rm -f "$SQL_TMP"' EXIT
cat > "$SQL_TMP" <<EOF
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${ROLE_NAME}') THEN
        CREATE ROLE ${ROLE_NAME} LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;
END
\$\$;
ALTER ROLE ${ROLE_NAME} WITH LOGIN PASSWORD '${EADM_PASSWORD}'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;
-- read-only, 限量访问：仅 CONNECT + schema USAGE + 对 lc_racebox 的 SELECT
GRANT CONNECT ON DATABASE eadm TO ${ROLE_NAME};
GRANT USAGE ON SCHEMA public TO ${ROLE_NAME};
GRANT SELECT ON TABLE public.lc_racebox TO ${ROLE_NAME};
EOF

echo "bootstrap-eadm: applying read-only role to '$CONTAINER' ..."
docker exec -i "$CONTAINER" psql -U postgres -d eadm -v ON_ERROR_STOP=1 < "$SQL_TMP"
echo "bootstrap-eadm: read-only role '${ROLE_NAME}' ready"
