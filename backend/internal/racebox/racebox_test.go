package racebox

import (
	"strings"
	"testing"
	"time"
)

func TestFixedOffset(t *testing.T) {
	loc, err := FixedOffset("Asia/Shanghai")
	if err != nil || loc == nil {
		t.Fatalf("Asia/Shanghai should resolve to fixed +8, got %v err %v", loc, err)
	}
	if _, off := time.Now().In(loc).Zone(); off != 8*3600 {
		t.Fatalf("offset = %d, want %d", off, 8*3600)
	}
	if _, err := FixedOffset("Mars/Olympus"); err == nil {
		t.Fatal("unsupported zone should error")
	}
	loc2, err := FixedOffset("-05:00")
	if err != nil {
		t.Fatal(err)
	}
	if _, off := time.Now().In(loc2).Zone(); off != -5*3600 {
		t.Fatalf("offset = %d, want -18000", off)
	}
}

func TestUUIDv5Deterministic(t *testing.T) {
	a := UUIDv5("ns", "x")
	b := UUIDv5("ns", "x")
	c := UUIDv5("ns", "y")
	if a != b {
		t.Fatalf("not deterministic: %s != %s", a, b)
	}
	if a == c {
		t.Fatal("different names must differ")
	}
	parts := strings.Split(a, "-")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[4]) != 12 {
		t.Fatalf("bad uuid shape: %s", a)
	}
	if parts[2][0] != '5' {
		t.Fatalf("expected version 5, got %s", parts[2])
	}
	if parts[3][0] != '8' && parts[3][0] != '9' && parts[3][0] != 'a' && parts[3][0] != 'b' {
		t.Fatalf("expected RFC4122 variant, got %s", parts[3])
	}
}

func TestRowValidate(t *testing.T) {
	good := srcRow{rawYear: 2026, rawMo: 8, rawDay: 4, rawNs: 880371973, lng: 120.45, lat: 36.17}
	if why := good.validate(); why != "" {
		t.Fatalf("good row rejected: %s", why)
	}
	cases := []struct {
		name string
		row  srcRow
		want string
	}{
		{"bad year", srcRow{rawYear: 1999, rawMo: 8, rawDay: 4, rawNs: 1, lng: 120, lat: 36}, "invalid_year"},
		{"bad month", srcRow{rawYear: 2026, rawMo: 13, rawDay: 4, rawNs: 1, lng: 120, lat: 36}, "invalid_date"},
		{"bad day", srcRow{rawYear: 2026, rawMo: 8, rawDay: 0, rawNs: 1, lng: 120, lat: 36}, "invalid_date"},
		{"neg ns", srcRow{rawYear: 2026, rawMo: 8, rawDay: 4, rawNs: -1, lng: 120, lat: 36}, "invalid_nanoseconds"},
		{"ns over", srcRow{rawYear: 2026, rawMo: 8, rawDay: 4, rawNs: 1_000_000_000, lng: 120, lat: 36}, "invalid_nanoseconds"},
		{"lng out", srcRow{rawYear: 2026, rawMo: 8, rawDay: 4, rawNs: 1, lng: 190, lat: 36}, "invalid_coordinates"},
		{"lat out", srcRow{rawYear: 2026, rawMo: 8, rawDay: 4, rawNs: 1, lng: 120, lat: -95}, "invalid_coordinates"},
	}
	for _, c := range cases {
		if got := c.row.validate(); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
