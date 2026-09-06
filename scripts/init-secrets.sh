#!/usr/bin/env bash
# Generate/refresh runtime secrets into .env (compose) and broker secret files
# under .secrets/. Existing values are never overwritten (docs/06-deployment).
# Reads existing local credentials from .env.local where present.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT/.env"
ENV_LOCAL="$ROOT/.env.local"
SECRETS_DIR="$ROOT/.secrets/mqtt"
BROKER_IMG="${MOSQUITTO_IMAGE:-eclipse-mosquitto:2.0.21}"

add_or_keep() { # key value
  if ! grep -q "^$1=" "$ENV_FILE" 2>/dev/null; then
    printf '%s=%s\n' "$1" "$2" >> "$ENV_FILE"
  fi
}
from_local() { # key default
  if [ -f "$ENV_LOCAL" ]; then
    v=$(awk -F= -v k="$1" '$1==k{sub(/^[^=]*=/,"");print}' "$ENV_LOCAL")
    [ -n "${v:-}" ] && { echo "$v"; return; }
  fi
  echo "$2"
}

umask 077
[ -f "$ENV_FILE" ] || : > "$ENV_FILE"
chmod 600 "$ENV_FILE"

# DB / MIGRATION (external mode defaults; host gateway)
add_or_keep DB_HOST "host.docker.internal"
add_or_keep DB_PORT "8432"
add_or_keep DB_NAME "iotwong"
add_or_keep DB_USER "$(from_local IOTWONG_APP_USER iotwong_app)"
add_or_keep DB_PASSWORD "$(from_local IOTWONG_APP_PASSWORD "$(openssl rand -hex 16)")"
add_or_keep MIGRATION_DB_USER "$(from_local PGUSER iotwong_owner)"
add_or_keep MIGRATION_DB_PASSWORD "$(from_local PGPASSWORD "$(openssl rand -hex 16)")"
add_or_keep APP_ENV "development"
add_or_keep WEB_PORT "8080"
add_or_keep MQTT_PORT "1883"
add_or_keep SESSION_KEY "$(openssl rand -hex 24)"

# MQTT users
add_or_keep MQTT_INGESTOR_USER "$(from_local MQTT_INGESTOR_USER iotwong_ingestor)"
add_or_keep MQTT_INGESTOR_PASSWORD "$(from_local MQTT_INGESTOR_PASSWORD "$(openssl rand -hex 16)")"
add_or_keep MQTT_DEVICE_USER "$(from_local MQTT_DEVICE_DEV1_USER dev-1)"
add_or_keep MQTT_DEVICE_PASSWORD "$(from_local MQTT_DEVICE_DEV1_PASSWORD "$(openssl rand -hex 16)")"
add_or_keep MQTT_DEVICE_ID "dev-1"

# standalone db
add_or_keep POSTGRES_USER "$(from_local PGUSER iotwong_owner)"
add_or_keep POSTGRES_PASSWORD "$(from_local PGPASSWORD "$(openssl rand -hex 16)")"
add_or_keep POSTGRES_DB "iotwong"
add_or_keep IOTWONG_APP_USER "iotwong_app"
add_or_keep IOTWONG_APP_PASSWORD "$(from_local IOTWONG_APP_PASSWORD "$(openssl rand -hex 16)")"

# broker password/acl files (ignored .secrets/)
mkdir -p "$SECRETS_DIR"
set -a; . "$ENV_FILE"; set +a
docker run --rm -v "$SECRETS_DIR:/secret" "$BROKER_IMG" sh -c \
  "mosquitto_passwd -c -b /secret/passwd '${MQTT_INGESTOR_USER}' '${MQTT_INGESTOR_PASSWORD}' \
   && mosquitto_passwd -b /secret/passwd '${MQTT_DEVICE_USER}' '${MQTT_DEVICE_PASSWORD}' \
   && chmod 600 /secret/passwd" >/dev/null
cat > "$SECRETS_DIR/acl" <<EOF
user ${MQTT_INGESTOR_USER}
topic read iotwong/v1/devices/+/telemetry

user ${MQTT_DEVICE_USER}
topic write iotwong/v1/devices/${MQTT_DEVICE_ID}/telemetry
EOF
chmod 644 "$SECRETS_DIR/acl"
echo "init-secrets: .env + broker secrets ready (never printed)"
