#!/bin/sh
# Generates mosquitto secrets (password_file + ACL) from environment at
# container start, then execs mosquitto. Files are written under
# /mosquitto/secrets (persistent volume) only when absent; ownership is
# granted to the mosquitto user. Non-root final process is mosquitto.
set -eu

SECRETS=/mosquitto/secrets
PASSWD=$SECRETS/passwd
ACL=$SECRETS/acl

INGESTOR_USER=${MQTT_INGESTOR_USER:-iotwong_ingestor}
DEVICE_USER=${MQTT_DEVICE_USER:-dev-1}
DEVICE_ID=${MQTT_DEVICE_ID:-dev-1}

mkdir -p "$SECRETS"

if [ ! -f "$PASSWD" ]; then
  mosquitto_passwd -c -b "$PASSWD" "$INGESTOR_USER" "$MQTT_INGESTOR_PASSWORD"
  mosquitto_passwd -b "$PASSWD" "$DEVICE_USER" "$MQTT_DEVICE_PASSWORD"
fi

cat > "$ACL" <<EOF
user ${INGESTOR_USER}
topic read iotwong/v1/devices/+/telemetry

user ${DEVICE_USER}
topic write iotwong/v1/devices/${DEVICE_ID}/telemetry
EOF

chown -R mosquitto:mosquitto "$SECRETS" 2>/dev/null || true
chmod 700 "$SECRETS"
chmod 600 "$PASSWD" "$ACL" 2>/dev/null || true

echo "iotwong mosquitto init: secrets ready"
exec mosquitto -c /mosquitto/config/mosquitto.conf
