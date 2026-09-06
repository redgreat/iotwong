// Package location validates the self-hosted MQTT location message v1
// (docs/protocol/location-v1.schema.json + docs/04-contracts.md).
//
// Rules enforced here:
//   - schema_version must be 1; event_id <=128 printable chars; recorded_at
//     must carry a timezone (RFC3339);
//   - position: null means heartbeat without fix; object requires
//     longitude/latitude/crs/fix; fix=false must not produce a position;
//   - NaN/Infinity is impossible in JSON; numeric bounds are checked;
//   - unknown/extra fields are rejected (strict contract);
//   - units are SI (m/s, m, degrees [0,360), battery %); absent means NULL.
package location

import (
	"encoding/json"
	"fmt"
	"time"
)

// Position is an effective location.
type Position struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	CRS       string  `json:"crs"`
	Fix       bool    `json:"fix"`
}

// Message is the validated v1 payload.
type Message struct {
	SchemaVersion  int       `json:"schema_version"`
	EventID        string    `json:"event_id"`
	RecordedAt     time.Time `json:"recorded_at"`
	Position       *Position `json:"position"`
	SpeedMPS       *float64  `json:"speed_mps"`
	HeadingDeg     *float64  `json:"heading_deg"`
	AccuracyM      *float64  `json:"accuracy_m"`
	BatteryPct     *float64  `json:"battery_pct"`
	LocationSource string    `json:"location_source"`
}

type rawMsg struct {
	SchemaVersion json.Number `json:"schema_version"`
	EventID       string      `json:"event_id"`
	RecordedAt    string      `json:"recorded_at"`
	Position      *struct {
		Longitude float64 `json:"longitude"`
		Latitude  float64 `json:"latitude"`
		CRS       string  `json:"crs"`
		Fix       bool    `json:"fix"`
	} `json:"position"`
	SpeedMPS       *float64 `json:"speed_mps"`
	HeadingDeg     *float64 `json:"heading_deg"`
	AccuracyM      *float64 `json:"accuracy_m"`
	BatteryPct     *float64 `json:"battery_pct"`
	LocationSource *string  `json:"location_source"`
}

// Parse validates raw bytes and returns a Message or a rejection reason.
// Position fix=false is accepted (heartbeat) with Position==nil output.
func Parse(data []byte, now time.Time) (*Message, string) {
	dec := json.NewDecoder(newBytesReader(data))
	dec.DisallowUnknownFields()
	var r rawMsg
	if err := dec.Decode(&r); err != nil {
		return nil, "invalid_json"
	}
	if r.SchemaVersion.String() != "1" {
		return nil, "unsupported_schema_version"
	}
	if r.EventID == "" || len(r.EventID) > 128 {
		return nil, "invalid_event_id"
	}
	for _, c := range r.EventID {
		if c < 0x21 || c > 0x7e {
			return nil, "invalid_event_id"
		}
	}
	t, err := time.Parse(time.RFC3339Nano, r.RecordedAt)
	if err != nil || t.Location() == nil {
		return nil, "invalid_recorded_at"
	}

	m := &Message{
		SchemaVersion:  1,
		EventID:        r.EventID,
		RecordedAt:     t.UTC(),
		SpeedMPS:       r.SpeedMPS,
		HeadingDeg:     r.HeadingDeg,
		AccuracyM:      r.AccuracyM,
		BatteryPct:     r.BatteryPct,
		LocationSource: "unknown",
	}
	if r.LocationSource != nil {
		switch *r.LocationSource {
		case "gnss", "lbs", "wifi", "unknown":
			m.LocationSource = *r.LocationSource
		default:
			return nil, "invalid_location_source"
		}
	}
	if r.Position != nil {
		if !r.Position.Fix {
			// fix=false: effective heartbeat, no location written.
			return m, ""
		}
		if r.Position.CRS != "WGS84" {
			return nil, "invalid_crs"
		}
		if r.Position.Longitude < -180 || r.Position.Longitude > 180 ||
			r.Position.Latitude < -90 || r.Position.Latitude > 90 {
			return nil, "invalid_coordinates"
		}
		m.Position = &Position{
			Longitude: r.Position.Longitude,
			Latitude:  r.Position.Latitude,
			CRS:       r.Position.CRS,
			Fix:       true,
		}
	}
	if m.SpeedMPS != nil && *m.SpeedMPS < 0 {
		return nil, "invalid_speed"
	}
	if m.HeadingDeg != nil && (*m.HeadingDeg < 0 || *m.HeadingDeg >= 360) {
		return nil, "invalid_heading"
	}
	if m.AccuracyM != nil && *m.AccuracyM < 0 {
		return nil, "invalid_accuracy"
	}
	if m.BatteryPct != nil && (*m.BatteryPct < 0 || *m.BatteryPct > 100) {
		return nil, "invalid_battery"
	}
	return m, ""
}

// Window checks the recorded_at window rules:
//   - more than futureLimit in the future -> "future_too_far"
//   - older than lateLimit -> "too_old"
//   - otherwise ""
func Window(recordedAt, now time.Time, futureLimit, lateLimit time.Duration) string {
	if d := recordedAt.Sub(now); d > futureLimit {
		return "future_too_far"
	}
	if d := now.Sub(recordedAt); d > lateLimit {
		return "too_old"
	}
	return ""
}

func (m *Message) String() string {
	return fmt.Sprintf("event=%s at=%s pos=%v", m.EventID, m.RecordedAt.Format(time.RFC3339Nano), m.Position != nil)
}

type bytesReader struct {
	b []byte
	i int
}

func newBytesReader(b []byte) *bytesReader { return &bytesReader{b: b} }
func (r *bytesReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, errEOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

var errEOF = fmt.Errorf("EOF")
