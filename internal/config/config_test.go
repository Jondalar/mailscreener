package config

import (
	"testing"
	"time"
)

func TestLoadMissingRequired(t *testing.T) {
	t.Setenv("ICLOUD_USER", "")
	t.Setenv("ICLOUD_APP_PASSWORD", "")
	t.Setenv("API_TOKEN", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for missing required env")
	}
}

func TestLoadDefaultsAndOverrides(t *testing.T) {
	t.Setenv("ICLOUD_USER", "a@icloud.com")
	t.Setenv("ICLOUD_APP_PASSWORD", "pw")
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("JUNK_RETENTION", "7d")
	t.Setenv("RECEIPT_RETENTION", "0")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ScreenedFolder != "Screened" {
		t.Errorf("ScreenedFolder = %q", c.ScreenedFolder)
	}
	if c.IdleTimeout != 25*time.Minute {
		t.Errorf("IdleTimeout = %v", c.IdleTimeout)
	}
	if c.JunkRetention != 7*24*time.Hour {
		t.Errorf("JunkRetention = %v", c.JunkRetention)
	}
	if c.ReceiptRetention != 0 {
		t.Errorf("ReceiptRetention = %v, want 0 (disabled)", c.ReceiptRetention)
	}
	if c.DBPath() != "/state/screener.db" {
		t.Errorf("DBPath = %q", c.DBPath())
	}
}

func TestParseDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"0":     0,
		"7d":    7 * 24 * time.Hour,
		"10m":   10 * time.Minute,
		"1500s": 1500 * time.Second,
		"72h":   72 * time.Hour,
	}
	for in, want := range cases {
		got, err := ParseDuration(in)
		if err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v,%v want %v", in, got, err, want)
		}
	}
}
