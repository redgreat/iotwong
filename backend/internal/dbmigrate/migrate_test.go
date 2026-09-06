package dbmigrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadOrdersAndSkipsNonMatching(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0002_b.sql", "b")
	writeFile(t, dir, "0001_a.sql", "a")
	writeFile(t, dir, "notes_scratch.sql", "ignore me")
	writeFile(t, dir, "0003_under_score.sql", "c")

	ms, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(ms) != 3 {
		t.Fatalf("got %d migrations, want 3: %+v", len(ms), ms)
	}
	want := []string{"0001_a.sql", "0002_b.sql", "0003_under_score.sql"}
	for i, w := range want {
		if ms[i].Path != w {
			t.Errorf("order[%d] = %s, want %s", i, ms[i].Path, w)
		}
	}
	if len(ms[0].Checksum) != 64 {
		t.Fatalf("checksum len = %d, want sha256 hex 64", len(ms[0].Checksum))
	}
	if !strings.HasPrefix(ms[0].Body, "a") {
		t.Fatalf("body mismatch: %q", ms[0].Body)
	}
}

func TestLoadRejectsBadVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "abc_bad.sql", "x")
	writeFile(t, dir, "0abc.sql", "x")
	ms, err := Load(dir)
	if err != nil {
		t.Fatalf("Load should ignore non-matching, got err %v", err)
	}
	if len(ms) != 0 {
		t.Fatalf("expected no migrations, got %d", len(ms))
	}
}

func TestLoadMissingDir(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing dir")
	}
}
