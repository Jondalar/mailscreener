package lists

import "time"

// --- screened_ids: transient approve-tracking index (Spec 0002 / K3) ---

// MarkScreened records that a Message-ID was seen while in Screened/.
func (s *Store) MarkScreened(mid string) error {
	mid = Normalize(mid)
	if mid == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR IGNORE INTO screened_ids(mid, seen_at) VALUES(?, ?)`, mid, now())
	return err
}

// WasScreened reports whether a Message-ID is in the screened index.
func (s *Store) WasScreened(mid string) (bool, error) {
	mid = Normalize(mid)
	var x int
	err := s.db.QueryRow(`SELECT 1 FROM screened_ids WHERE mid=?`, mid).Scan(&x)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ClearScreened removes a Message-ID from the index (after approve handling).
func (s *Store) ClearScreened(mid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM screened_ids WHERE mid=?`, Normalize(mid))
	return err
}

// PruneScreened deletes index rows older than ttl (G2 — bound the table).
func (s *Store) PruneScreened(ttl time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-ttl).Format(time.RFC3339)
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM screened_ids WHERE seen_at <= ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- snoozed: per-message wake state (Spec 0007) ---

// Snooze records a snoozed message and its wake time.
func (s *Store) Snooze(folder, label, mid string, uid uint32, wakeAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO snoozed(uid, folder, label, mid, wake_at)
		VALUES(?, ?, ?, ?, ?)`,
		uid, folder, label, mid, wakeAt.UTC().Format(time.RFC3339))
	return err
}

// DueSnooze is one message ready to wake.
type DueSnooze struct {
	UID    uint32
	Folder string
	MID    string
}

// DueSnoozes returns messages whose wake time has passed.
func (s *Store) DueSnoozes(at time.Time) ([]DueSnooze, error) {
	rows, err := s.db.Query(`
		SELECT uid, folder, mid FROM snoozed WHERE wake_at <= ?`,
		at.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueSnooze
	for rows.Next() {
		var d DueSnooze
		var mid *string
		if err := rows.Scan(&d.UID, &d.Folder, &mid); err != nil {
			return nil, err
		}
		if mid != nil {
			d.MID = *mid
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Unsnooze clears a woken message's row.
func (s *Store) Unsnooze(folder string, uid uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM snoozed WHERE folder=? AND uid=?`, folder, uid)
	return err
}

// --- counters: small persistent key/value integers ---

// GetCounter returns a counter value (0 if unset).
func (s *Store) GetCounter(key string) (int64, error) {
	var v int64
	err := s.db.QueryRow(`SELECT value FROM counters WHERE key=?`, key).Scan(&v)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0, nil
		}
		return 0, err
	}
	return v, nil
}

// SetCounter sets a counter value.
func (s *Store) SetCounter(key string, v int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO counters(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, v)
	return err
}

// --- folder watermarks: highest processed UID per folder (Spec 0003 quick tier) ---

// Watermark returns the highest processed UID for a folder (0 if none).
func (s *Store) Watermark(folder string) (uint32, error) {
	var v int64
	err := s.db.QueryRow(`SELECT last_uid FROM folder_state WHERE folder=?`, folder).Scan(&v)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0, nil
		}
		return 0, err
	}
	return uint32(v), nil
}

// BumpWatermark raises a folder's watermark to uid (never lowers it).
func (s *Store) BumpWatermark(folder string, uid uint32) error {
	if uid == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO folder_state(folder, last_uid) VALUES(?, ?)
		ON CONFLICT(folder) DO UPDATE SET last_uid=MAX(last_uid, excluded.last_uid)`,
		folder, int64(uid))
	return err
}
