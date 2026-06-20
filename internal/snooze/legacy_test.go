package snooze

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLabelFromKeyword(t *testing.T) {
	cases := map[string]string{
		"SNOOZED_1W":   "1w",
		"SNOOZED_1D10": "1d10",
		"\\Seen":       "",
		"SNOOZED_":     "",
		"random":       "",
	}
	for in, want := range cases {
		if got := LabelFromKeyword(in); got != want {
			t.Errorf("LabelFromKeyword(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseSnoozedMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snoozed_map.txt")
	os.WriteFile(path, []byte("Snoozed/2w 1781942400\nSnoozed/1m 1782979200\nbad line\n"), 0o644)

	m, err := ParseSnoozedMap(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("got %d entries, want 2: %v", len(m), m)
	}
	if m["Snoozed/2w"].Unix() != 1781942400 {
		t.Errorf("wrong ts for Snoozed/2w: %v", m["Snoozed/2w"])
	}

	// Missing file -> empty map, no error.
	m2, err := ParseSnoozedMap(filepath.Join(dir, "nope.txt"))
	if err != nil || len(m2) != 0 {
		t.Fatalf("missing file: m=%v err=%v", m2, err)
	}
}
