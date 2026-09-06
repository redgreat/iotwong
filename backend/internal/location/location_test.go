package location

import (
	"testing"
	"time"
)

var now = time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)

func mustJSON(s string) []byte { return []byte(s) }

func TestParseValidPosition(t *testing.T) {
	m, why := Parse(mustJSON(`{
		"schema_version": 1,
		"event_id": "boot_a-seq_42",
		"recorded_at": "2026-09-05T08:00:00.123Z",
		"position": {"longitude": 116.4, "latitude": 39.9, "crs": "WGS84", "fix": true},
		"speed_mps": 1.2, "heading_deg": 90, "accuracy_m": 5, "battery_pct": 80,
		"location_source": "gnss"
	}`), now)
	if why != "" || m == nil {
		t.Fatalf("valid rejected: %s", why)
	}
	if m.Position == nil || m.Position.Longitude != 116.4 {
		t.Fatalf("position lost")
	}
	if m.RecordedAt.UTC().Hour() != 8 {
		t.Fatalf("time wrong: %v", m.RecordedAt)
	}
}

func TestParseHeartbeatNullPositionAndFixFalse(t *testing.T) {
	hb, why := Parse(mustJSON(`{"schema_version":1,"event_id":"hb-1","recorded_at":"2026-09-05T08:00:00Z","position":null}`), now)
	if why != "" || hb.Position != nil {
		t.Fatalf("heartbeat null: %s", why)
	}
	falseFix, why := Parse(mustJSON(`{"schema_version":1,"event_id":"hb-2","recorded_at":"2026-09-05T08:00:00Z","position":{"longitude":1,"latitude":2,"crs":"WGS84","fix":false}}`), now)
	if why != "" || falseFix.Position != nil {
		t.Fatalf("fix=false must not carry position: %s", why)
	}
}

func TestParseRejections(t *testing.T) {
	cases := map[string]string{
		"bad version":      `{"schema_version":2,"event_id":"a","recorded_at":"2026-09-05T08:00:00Z"}`,
		"missing event id": `{"schema_version":1,"recorded_at":"2026-09-05T08:00:00Z"}`,
		"long event":       `{"schema_version":1,"event_id":"` + repeatStr("x", 130) + `","recorded_at":"2026-09-05T08:00:00Z"}`,
		"no tz":            `{"schema_version":1,"event_id":"a","recorded_at":"2026-09-05T08:00:00"}`,
		"crs wrong":        `{"schema_version":1,"event_id":"a","recorded_at":"2026-09-05T08:00:00Z","position":{"longitude":116,"latitude":39,"crs":"GCJ02","fix":true}}`,
		"lng out of range": `{"schema_version":1,"event_id":"a","recorded_at":"2026-09-05T08:00:00Z","position":{"longitude":200,"latitude":39,"crs":"WGS84","fix":true}}`,
		"bad speed":        `{"schema_version":1,"event_id":"a","recorded_at":"2026-09-05T08:00:00Z","speed_mps":-1}`,
		"bad source":       `{"schema_version":1,"event_id":"a","recorded_at":"2026-09-05T08:00:00Z","location_source":"glonass"}`,
		"unknown field":    `{"schema_version":1,"event_id":"a","recorded_at":"2026-09-05T08:00:00Z","tenant_id":"x"}`,
		"not json":         `not-json`,
	}
	for name, body := range cases {
		if _, why := Parse(mustJSON(body), now); why == "" {
			t.Errorf("%s: expected rejection, got ok", name)
		}
	}
}

func TestWindow(t *testing.T) {
	base := now
	if w := Window(base, base, 5*time.Minute, 7*24*time.Hour); w != "" {
		t.Fatalf("current should pass, got %s", w)
	}
	if w := Window(base.Add(6*time.Minute), base, 5*time.Minute, 7*24*time.Hour); w != "future_too_far" {
		t.Fatalf("future: %s", w)
	}
	if w := Window(base.Add(-8*24*time.Hour), base, 5*time.Minute, 7*24*time.Hour); w != "too_old" {
		t.Fatalf("old: %s", w)
	}
}

func repeatStr(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
