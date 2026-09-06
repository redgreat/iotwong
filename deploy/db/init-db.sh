#!/bin/sh
# One-shot standalone DB bootstrap (runs inside the db container as its
# superuser via /docker-entrypoint-initdb.d). Creates the runtime app role
# and ensures required extensions exist. Never creates iotwong_owner here:
# the container's POSTGRES_USER is the owner/superuser for migrations.
set -eu

psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname "${POSTGRES_DB}" <<SQL
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS timescaledb;
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${IOTWONG_APP_USER}') THEN
    CREATE ROLE ${IOTWONG_APP_USER} LOGIN PASSWORD '${IOTWONG_APP_PASSWORD}'
      NOSUPERUSER NOCREATEDB NOCREATEROLE;
  END IF;
END
\$\$;
ALTER ROLE ${IOTWONG_APP_USER} WITH LOGIN PASSWORD '${IOTWONG_APP_PASSWORD}'
  NOSUPERUSER NOCREATEDB NOCREATEROLE;
SQL
echo "iotwong db init: extensions + app role ready"
