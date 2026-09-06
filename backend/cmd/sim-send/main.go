// Command sim-send publishes synthetic local telemetry (MQTT protocol v1)
// for T04 verification: configurable count, duplicate, conflict, out-of-order
// and future/late window messages.
//
// Example:
//
//	MQTT_USER=dev-1 MQTT_PASSWORD=... \
//	  go run ./cmd/sim-send --device dev-1 --count 5 --dup 3
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type payload struct {
	SchemaVersion int    `json:"schema_version"`
	EventID       string `json:"event_id"`
	RecordedAt    string `json:"recorded_at"`
	Position      *struct {
		Longitude float64 `json:"longitude"`
		Latitude  float64 `json:"latitude"`
		CRS       string  `json:"crs"`
		Fix       bool    `json:"fix"`
	} `json:"position"`
	SpeedMPS       float64 `json:"speed_mps"`
	HeadingDeg     float64 `json:"heading_deg"`
	AccuracyM      float64 `json:"accuracy_m"`
	BatteryPct     float64 `json:"battery_pct"`
	LocationSource string  `json:"location_source"`
}

func main() {
	broker := flag.String("broker", envOr("MQTT_URL", "tcp://127.0.0.1:1883"), "broker url")
	device := flag.String("device", "dev-1", "device external id (topic segment)")
	count := flag.Int("count", 3, "distinct sequential events to send")
	dup := flag.Int("dup", 0, "repeat the FIRST event this many extra times (identical bytes)")
	conflict := flag.Bool("conflict", false, "send a DIFFERENT payload under the first event id")
	old := flag.Bool("old", false, "first event recorded 10 minutes ago (history, within late window)")
	future := flag.Bool("future", false, "first event recorded 10 minutes in the future (window reject)")
	intervalMS := flag.Int("interval-ms", 100, "pause between sends")
	prefix := flag.String("prefix", "sim", "event id prefix")
	flag.Parse()

	user := envOr("MQTT_USER", "")
	pass := envOr("MQTT_PASSWORD", "")
	opts := mqtt.NewClientOptions().
		AddBroker(*broker).
		SetClientID(fmt.Sprintf("sim-%s-%d", *device, os.Getpid())).
		SetCleanSession(true).
		SetConnectRetry(true)
	if user != "" {
		opts.SetUsername(user).SetPassword(pass)
	}
	client := mqtt.NewClient(opts)
	if tok := client.Connect(); !tok.WaitTimeout(15*time.Second) || tok.Error() != nil {
		log.Fatalf("connect: %v", tok.Error())
	}
	defer client.Disconnect(100)

	topic := fmt.Sprintf("iotwong/v1/devices/%s/telemetry", *device)
	now := time.Now().UTC()

	mk := func(seq int, eventID string, t time.Time, lng, lat float64, speed float64) []byte {
		p := payload{
			SchemaVersion:  1,
			EventID:        eventID,
			RecordedAt:     t.Format(time.RFC3339Nano),
			SpeedMPS:       speed,
			HeadingDeg:     90,
			AccuracyM:      3,
			BatteryPct:     80,
			LocationSource: "gnss",
		}
		p.Position = &struct {
			Longitude float64 `json:"longitude"`
			Latitude  float64 `json:"latitude"`
			CRS       string  `json:"crs"`
			Fix       bool    `json:"fix"`
		}{Longitude: lng, Latitude: lat, CRS: "WGS84", Fix: true}
		b, err := json.Marshal(p)
		if err != nil {
			log.Fatal(err)
		}
		return b
	}

	var first []byte
	var firstEvent string
	send := func(b []byte, label string) {
		t := client.Publish(topic, 1, false, b)
		if !t.WaitTimeout(10*time.Second) || t.Error() != nil {
			log.Fatalf("publish %s: %v", label, t.Error())
		}
		fmt.Printf("sent %-8s %d bytes\n", label, len(b))
		time.Sleep(time.Duration(*intervalMS) * time.Millisecond)
	}

	base := now
	if *old {
		base = now.Add(-10 * time.Minute)
	}
	if *future {
		base = now.Add(10 * time.Minute)
	}
	for i := 0; i < *count; i++ {
		ev := fmt.Sprintf("%s-seq-%d", *prefix, i+1)
		ts := base.Add(time.Duration(i) * time.Second)
		lng := 116.40 + 0.001*float64(i)
		lat := 39.90 + 0.0005*float64(i)
		b := mk(i, ev, ts, lng, lat, 1.5+float64(i))
		if i == 0 {
			first, firstEvent = b, ev
		}
		send(b, ev)
	}
	for i := 0; i < *dup; i++ {
		send(first, firstEvent+"-dup")
	}
	if *conflict {
		// same event id, different payload -> conflict
		cb := mk(999, firstEvent, now.Add(2*time.Second), 116.45, 39.95, 30)
		send(cb, firstEvent+"-conflict")
	}
	fmt.Println("sim-send done")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
