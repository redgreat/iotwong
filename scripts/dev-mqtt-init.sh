#!/usr/bin/env bash
# Dev-only MQTT secret bootstrap: creates broker users (ingestor + device
# dev-1), writes mosquitto passwd/acl into deploy/mosquitto/secret (git-ignored)
# and stores credentials in the ignored .env.local. Never prints passwords.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT/.env.local"
SECRET_DIR="$ROOT/deploy/mosquitto/secret"
IMG="${MOSQUITTO_IMAGE:-eclipse-mosquitto:2.0.21}"

[ -f "$ENV_FILE" ] && chmod 600 "$ENV_FILE"

mkdir -p "$SECRET_DIR"
chmod 700 "$SECRET_DIR"

add_or_keep() { # key
  if ! grep -q "^$1=" "$ENV_FILE"; then
    printf '%s=%s\n' "$1" "$(openssl rand -hex 16)" >> "$ENV_FILE"
  fi
}
for k in MQTT_INGESTOR_USER MQTT_INGESTOR_PASSWORD MQTT_DEVICE_DEV1_USER MQTT_DEVICE_DEV1_PASSWORD; do
  add_or_keep "$k"
done

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

ING="${MQTT_INGESTOR_USER:-iotwong_ingestor}"
DEVU="${MQTT_DEVICE_DEV1_USER:-dev-1}"

docker run --rm -v "$SECRET_DIR:/mosquitto/secret" "$IMG" sh -c \
  "mosquitto_passwd -c -b /mosquitto/secret/passwd '${ING}' '${MQTT_INGESTOR_PASSWORD}' \
   && mosquitto_passwd -b /mosquitto/secret/passwd '${DEVU}' '${MQTT_DEVICE_DEV1_PASSWORD}' \
   && chmod 600 /mosquitto/secret/passwd"

cat > "$SECRET_DIR/acl" <<EOF
user ${ING}
topic read iotwong/v1/devices/+/telemetry

user ${DEVU}
topic write iotwong/v1/devices/${DEVU}/telemetry
EOF
echo "dev-mqtt-init: broker secrets ready (users: ${ING}, ${DEVU})"
