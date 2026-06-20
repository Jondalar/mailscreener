// Package lists holds the sender lists in SQLite and persists them atomically
// (Spec 0002). It exposes Add/Remove/Contains plus a Snapshot the classify
// engine consumes, the transient screened_ids index, and the snoozed table.
package lists

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Jondalar/mailscreener/internal/classify"
	_ "modernc.org/sqlite"
)

// Kind identifies one of the sender lists.
type Kind string

const (
	Whitelist      Kind = "whitelist"
	Blocklist      Kind = "blocklist"
	Newsletter     Kind = "newsletter"
	Receipts       Kind = "receipts"
	GroupAllowlist Kind = "group_allowlist"
)

// AllKinds is every valid list kind, used for schema bootstrap and iteration.
var AllKinds = []Kind{Whitelist, Blocklist, Newsletter, Receipts, GroupAllowlist}

func validKind(k Kind) bool {
	for _, x := range AllKinds {
		if x == k {
			return true
		}
	}
	return false
}

// Store is the SQLite-backed list store. All writes are serialized through mu
// and run in transactions, so the on-disk DB is always consistent (QS4).
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

const schema = `
CREATE TABLE IF NOT EXISTS lists (
  id   INTEGER PRIMARY KEY,
  kind TEXT UNIQUE NOT NULL
);
CREATE TABLE IF NOT EXISTS entries (
  id          INTEGER PRIMARY KEY,
  list_id     INTEGER NOT NULL REFERENCES lists(id),
  value       TEXT NOT NULL,
  is_wildcard INTEGER NOT NULL,
  source      TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  UNIQUE(list_id, value)
);
CREATE TABLE IF NOT EXISTS counters (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS screened_ids (mid TEXT PRIMARY KEY, seen_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS snoozed (
  uid     INTEGER NOT NULL,
  folder  TEXT NOT NULL,
  label   TEXT NOT NULL,
  mid     TEXT,
  wake_at TEXT NOT NULL,
  PRIMARY KEY (folder, uid)
);
CREATE TABLE IF NOT EXISTS folder_state (folder TEXT PRIMARY KEY, last_uid INTEGER NOT NULL);
`

// Open opens (or creates) the SQLite database at path and ensures the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// One writer; WAL for durability and concurrent reads.
	db.SetMaxOpenConns(1)
	for _, p := range []string{"PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000", "PRAGMA foreign_keys=ON"} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	s := &Store{db: db}
	if err := s.bootstrapKinds(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) bootstrapKinds() error {
	for _, k := range AllKinds {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO lists(kind) VALUES(?)`, string(k)); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// Add upserts a value into a list. value is normalized (trim+lower). It returns
// whether a new row was inserted. Invalid values are rejected.
func (s *Store) Add(k Kind, value, source string) (bool, error) {
	if !validKind(k) {
		return false, fmt.Errorf("unknown list kind %q", k)
	}
	v := Normalize(value)
	if !ValidEntry(v) {
		return false, fmt.Errorf("invalid list value %q", value)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		INSERT OR IGNORE INTO entries(list_id, value, is_wildcard, source, created_at)
		VALUES((SELECT id FROM lists WHERE kind=?), ?, ?, ?, ?)`,
		string(k), v, b2i(IsWildcard(v)), source, now())
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Remove deletes a value from a list. Returns whether a row was removed.
func (s *Store) Remove(k Kind, value string) (bool, error) {
	if !validKind(k) {
		return false, fmt.Errorf("unknown list kind %q", k)
	}
	v := Normalize(value)
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		DELETE FROM entries
		WHERE list_id=(SELECT id FROM lists WHERE kind=?) AND value=?`, string(k), v)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// All returns every value of a list, sorted.
func (s *Store) All(k Kind) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT value FROM entries
		WHERE list_id=(SELECT id FROM lists WHERE kind=?)
		ORDER BY value`, string(k))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Contains reports whether addr is covered by a list (exact or wildcard).
func (s *Store) Contains(k Kind, addr string) (bool, error) {
	vals, err := s.All(k)
	if err != nil {
		return false, err
	}
	return classify.NewMapMatcher(vals).Contains(Normalize(addr)), nil
}

// Count returns the number of entries in a list.
func (s *Store) Count(k Kind) (int, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM entries
		WHERE list_id=(SELECT id FROM lists WHERE kind=?)`, string(k)).Scan(&n)
	return n, err
}

// Snapshot builds an in-memory matcher set for the classify engine.
func (s *Store) Snapshot() (classify.Snapshot, error) {
	get := func(k Kind) (classify.Matcher, error) {
		vals, err := s.All(k)
		if err != nil {
			return nil, err
		}
		return classify.NewMapMatcher(vals), nil
	}
	var snap classify.Snapshot
	var err error
	if snap.Whitelist, err = get(Whitelist); err != nil {
		return snap, err
	}
	if snap.Blocklist, err = get(Blocklist); err != nil {
		return snap, err
	}
	if snap.Newsletter, err = get(Newsletter); err != nil {
		return snap, err
	}
	if snap.Receipts, err = get(Receipts); err != nil {
		return snap, err
	}
	if snap.GroupAllowlist, err = get(GroupAllowlist); err != nil {
		return snap, err
	}
	return snap, nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Normalize trims and lowercases a list value.
func Normalize(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
