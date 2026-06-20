package snooze

import (
	"testing"
	"time"
)

func TestParseLabel(t *testing.T) {
	loc := time.UTC
	// A Wednesday.
	now := time.Date(2026, 6, 17, 14, 30, 0, 0, loc)

	cases := []struct {
		label string
		want  time.Time
		ok    bool
	}{
		{"1d10", time.Date(2026, 6, 18, 10, 0, 0, 0, loc), true},
		{"1d", time.Date(2026, 6, 18, 10, 0, 0, 0, loc), true},
		{"0d18", time.Date(2026, 6, 17, 18, 0, 0, 0, loc), true},
		{"0d10", time.Date(2026, 6, 18, 10, 0, 0, 0, loc), true}, // 10:00 already passed -> next day
		{"60s", now.Add(60 * time.Second), true},
		{"2h", now.Add(2 * time.Hour), true},
		{"1w", time.Date(2026, 6, 24, 10, 0, 0, 0, loc), true},
		{"2w", time.Date(2026, 7, 1, 10, 0, 0, 0, loc), true},
		{"1m", time.Date(2026, 7, 17, 10, 0, 0, 0, loc), true},
		{"3m", time.Date(2026, 9, 17, 10, 0, 0, 0, loc), true},
		{"fr", time.Date(2026, 6, 19, 8, 0, 0, 0, loc), true},   // next Friday 08:00
		{"fri18", time.Date(2026, 6, 19, 18, 0, 0, 0, loc), true},
		{"mi", time.Date(2026, 6, 24, 8, 0, 0, 0, loc), true},   // Wed -> next Wed, not today
		{"bogus123x", time.Time{}, false},
		{"", time.Time{}, false},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			got, ok := ParseLabel(c.label, now)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if ok && !got.Equal(c.want) {
				t.Errorf("ParseLabel(%q) = %s, want %s", c.label, got, c.want)
			}
		})
	}
}

func TestAddMonthsClamp(t *testing.T) {
	jan31 := time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC)
	got := addMonths(jan31, 1)
	want := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("addMonths(Jan31,1) = %s, want %s", got, want)
	}
}
