// Package config loads the daemon configuration from environment variables
// (12-factor, Spec 0006). No secrets are baked into the image; missing required
// keys make Load fail fast with a named-key error.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the resolved daemon configuration.
type Config struct {
	ICloudUser          string
	ICloudAppPassword   string
	IMAPServer          string
	APIToken            string
	APIAddr             string
	IdleTimeout         time.Duration
	SweepInterval       time.Duration
	JunkRetention       time.Duration // 0 = disabled
	ReceiptRetention    time.Duration // 0 = disabled
	NewsletterRetention time.Duration // 0 = disabled
	ScreenedIDTTL       time.Duration
	StateDir            string
	LogFormat           string

	// Folder names. The iCloud layout is the default; override any of them via
	// env (FOLDER_* / SCREENED_FOLDER) for a differently-organised mailbox.
	ScreenedFolder    string
	JunkFolder        string
	ReceiptsFolder    string
	NewslettersFolder string
	ArchiveFolder     string
	DeletedFolder     string
	SnoozedFolder     string

	// SnoozeLabels are snooze subfolders (SNOOZE_LABELS, comma-separated, e.g.
	// "1d10,sat10,1w") pre-created under SnoozedFolder on bootstrap. Manually
	// created snooze subfolders are still picked up dynamically regardless.
	SnoozeLabels []string

	// ReceiptSubjectMatch enables the optional receipt subject heuristic
	// (RECEIPT_SUBJECT_MATCH=true). Default off.
	ReceiptSubjectMatch bool
}

// DBPath returns the SQLite path under StateDir.
func (c Config) DBPath() string { return strings.TrimRight(c.StateDir, "/") + "/screener.db" }

// Load reads and validates configuration from the environment.
func Load() (Config, error) {
	var missing []string
	req := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	c := Config{
		ICloudUser:          req("ICLOUD_USER"),
		ICloudAppPassword:   req("ICLOUD_APP_PASSWORD"),
		APIToken:            req("API_TOKEN"),
		IMAPServer:          envDefault("IMAP_SERVER", "imap.mail.me.com:993"),
		APIAddr:             envDefault("API_ADDR", "127.0.0.1:8443"),
		StateDir:            envDefault("STATE_DIR", "/state"),
		LogFormat:           envDefault("LOG_FORMAT", "json"),
		ReceiptSubjectMatch: strings.EqualFold(os.Getenv("RECEIPT_SUBJECT_MATCH"), "true"),

		ScreenedFolder:    envDefault("SCREENED_FOLDER", "Screened"),
		JunkFolder:        envDefault("FOLDER_JUNK", "Junk"),
		ReceiptsFolder:    envDefault("FOLDER_RECEIPTS", "Receipts"),
		NewslettersFolder: envDefault("FOLDER_NEWSLETTERS", "Newsletters"),
		ArchiveFolder:     envDefault("FOLDER_ARCHIVE", "Archive"),
		DeletedFolder:     envDefault("FOLDER_DELETED", "Deleted Messages"),
		SnoozedFolder:     envDefault("FOLDER_SNOOZED", "Snoozed"),
		SnoozeLabels:      envList("SNOOZE_LABELS"),
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}

	var err error
	if c.IdleTimeout, err = durDefault("IDLE_TIMEOUT", 25*time.Minute); err != nil {
		return Config{}, err
	}
	if c.SweepInterval, err = durDefault("SWEEP_INTERVAL", 10*time.Minute); err != nil {
		return Config{}, err
	}
	if c.JunkRetention, err = durDefault("JUNK_RETENTION", 7*24*time.Hour); err != nil {
		return Config{}, err
	}
	if c.ReceiptRetention, err = durDefault("RECEIPT_RETENTION", 30*24*time.Hour); err != nil {
		return Config{}, err
	}
	if c.NewsletterRetention, err = durDefault("NEWSLETTER_RETENTION", 30*24*time.Hour); err != nil {
		return Config{}, err
	}
	if c.ScreenedIDTTL, err = durDefault("SCREENED_ID_TTL", 72*time.Hour); err != nil {
		return Config{}, err
	}
	return c, nil
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envList parses a comma-separated env var into a trimmed, non-empty slice.
// Returns nil when unset/empty.
func envList(key string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func durDefault(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("env %s: %w", key, err)
	}
	return d, nil
}

// ParseDuration extends time.ParseDuration with a day suffix ("7d") and accepts
// "0" as a disabled value.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "0" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid day duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
