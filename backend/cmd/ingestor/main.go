// Command ingestor consumes MQTT telemetry (QoS1) and persists it to
// PostgreSQL, ACKing only after a successful transaction (docs/02-architecture
// ADR-003, docs/04-contracts.md MQTT 协议 v1).
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"iotwong/backend/internal/location"
	"iotwong/backend/internal/store"
)

const (
	topicPrefix    = "iotwong/v1/devices/"
	subscribeTopic = "iotwong/v1/devices/+/telemetry"
	futureLimit    = 5 * time.Minute
	lateLimit      = 7 * 24 * time.Hour
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st := openStore(ctx)

	opts := mqtt.NewClientOptions().
		AddBroker(envOr("MQTT_URL", "tcp://127.0.0.1:1883")).
		SetClientID(envOr("MQTT_CLIENT_ID", "iotwong-ingestor")).
		SetCleanSession(false).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetMaxReconnectInterval(30 * time.Second).
		SetAutoAckDisabled(true) // ack only after durable write
	if u := envOr("MQTT_USER", ""); u != "" {
		opts.SetUsername(u).SetPassword(envOr("MQTT_PASSWORD", ""))
	}

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		slog.Info("mqtt connected; (re)subscribing")
		if t := c.Subscribe(subscribeTopic, 1, nil); !t.WaitTimeout(15*time.Second) || t.Error() != nil {
			slog.Error("subscribe failed", "err", t.Error())
			os.Exit(1)
		}
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		slog.Warn("mqtt connection lost", "err", err)
	})
	opts.SetDefaultPublishHandler(func(c mqtt.Client, msg mqtt.Message) {
		if err := handleMessage(ctx, st, msg); err != nil {
			slog.Error("message not acked (will be redelivered)",
				"topic", msg.Topic(), "err", err)
		} else {
			msg.Ack()
		}
	})

	client := mqtt.NewClient(opts)
	tok := client.Connect()
	if !tok.WaitTimeout(30 * time.Second) {
		logger.Error("mqtt connect timeout")
		os.Exit(1)
	}
	if err := tok.Error(); err != nil {
		logger.Error("mqtt connect failed", "err", err)
		os.Exit(1)
	}
	logger.Info("ingestor ready", "topic", subscribeTopic, "client", opts.ClientID)

	<-ctx.Done()
	slog.Info("shutdown signal; stopping ingestion")
	client.Disconnect(1000)
}

func openStore(ctx context.Context) *store.Store {
	pool, err := store.NewPool(ctx)
	if err != nil {
		slog.Error("db pool failed", "err", err)
		os.Exit(1)
	}
	return store.NewStore(pool)
}

// handleMessage validates + persists one message; returning an error means
// NOT acked (QoS1 redelivery after reconnect; never ack unpersisted data).
func handleMessage(ctx context.Context, st *store.Store, msg mqtt.Message) error {
	raw := msg.Payload()
	digest := sha256.Sum256(raw)
	digestHex := hex.EncodeToString(digest[:])

	// topic: iotwong/v1/devices/{device_external_id}/telemetry
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) != 5 || parts[0] != "iotwong" || parts[1] != "v1" ||
		parts[2] != "devices" || parts[4] != "telemetry" || parts[3] == "" {
		slog.Warn("unexpected topic shape", "topic", msg.Topic())
		if err := st.Poison(ctx, nil, "?", "unexpected_topic", digestHex[:16]); err != nil {
			return err
		}
		return nil
	}
	deviceID := parts[3]
	if len(deviceID) > 64 {
		return nil
	}

	// registration + source binding check (topic credential/business layer)
	tenantPtr := (*string)(nil)
	deviceUUID := ""
	if tenant, duuid, terr := st.DeviceExternalInfo(ctx, deviceID); terr == nil {
		tenantPtr = &tenant
		deviceUUID = duuid
	} else if errors.Is(terr, store.ErrDeviceUnknown) {
		if perr := st.Poison(ctx, nil, deviceID, "device_unknown", digestHex[:32]); perr != nil {
			return fmt.Errorf("device unknown poison: %w", perr)
		}
		slog.Warn("device not registered", "device", deviceID)
		return nil
	} else {
		return terr // DB trouble: do not ack
	}

	now := time.Now().UTC()
	parsed, reject := location.Parse(raw, now)
	if reject != "" {
		// persist a bounded digest, then ack (poison must not block the topic)
		if err := st.Poison(ctx, tenantPtr, deviceID, reject, digestHex[:32]); err != nil {
			return fmt.Errorf("poison persist: %w", err)
		}
		slog.Warn("rejected message", "device", deviceID, "reason", reject)
		return nil
	}
	if w := location.Window(parsed.RecordedAt, now, futureLimit, lateLimit); w != "" {
		if err := st.Poison(ctx, tenantPtr, deviceID, w, digestHex[:32]); err != nil {
			return err
		}
		slog.Warn("window rejected", "device", deviceID, "event", parsed.EventID, "reason", w)
		return nil
	}

	in := store.IngestInput{
		DeviceExternalID: deviceID,
		RawPayloadHash:   digestHex,
		MessageID:        parsed.EventID,
		RecordedAt:       parsed.RecordedAt,
		ReceivedAt:       now,
		HasPosition:      parsed.Position != nil,
		LocationSource:   parsed.LocationSource,
		SpeedMPS:         parsed.SpeedMPS,
		HeadingDeg:       parsed.HeadingDeg,
		AccuracyM:        parsed.AccuracyM,
		BatteryPct:       parsed.BatteryPct,
	}
	if parsed.Position != nil {
		in.Longitude = parsed.Position.Longitude
		in.Latitude = parsed.Position.Latitude
	}

	res, err := st.Ingest(ctx, in)
	if err != nil {
		if errors.Is(err, store.ErrDeviceUnknown) {
			if perr := st.Poison(ctx, nil, deviceID, "device_unknown", digestHex[:32]); perr != nil {
				return fmt.Errorf("device unknown poison: %w", perr)
			}
			slog.Warn("device not registered", "device", deviceID)
			return nil // recorded as poison; ack
		}
		return err // DB trouble: do not ack
	}
	slog.Info("ingested", "device", deviceID, "event", parsed.EventID, "result", res)

	// Server-side geofence evaluation AFTER the durable write: works with no
	// webpage open; errors only warn (position already committed) and never
	// block message ACK.
	if res == store.IngestInserted && parsed.Position != nil && deviceUUID != "" {
		if err := st.EvalFencesPost(ctx, *tenantPtr, deviceUUID,
			parsed.RecordedAt.Format(time.RFC3339Nano),
			parsed.Position.Longitude, parsed.Position.Latitude); err != nil {
			slog.Warn("fence eval failed (position still durable)", "device", deviceID,
				"event", parsed.EventID, "err", err)
		}
	}
	return nil
}
