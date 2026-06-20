package lists

import (
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAddContainsRemove(t *testing.T) {
	s := openTest(t)

	ins, err := s.Add(Whitelist, "A@B.com", "api")
	if err != nil || !ins {
		t.Fatalf("add: ins=%v err=%v", ins, err)
	}
	// Idempotent.
	ins, _ = s.Add(Whitelist, "a@b.com", "api")
	if ins {
		t.Fatal("second add should not insert")
	}

	if ok, _ := s.Contains(Whitelist, "a@b.com"); !ok {
		t.Fatal("contains exact failed")
	}
	// Wildcard + subdomain.
	s.Add(Blocklist, "*@spam.com", "api")
	if ok, _ := s.Contains(Blocklist, "x@mail.spam.com"); !ok {
		t.Fatal("wildcard subdomain match failed")
	}

	rm, _ := s.Remove(Whitelist, "a@b.com")
	if !rm {
		t.Fatal("remove failed")
	}
	if ok, _ := s.Contains(Whitelist, "a@b.com"); ok {
		t.Fatal("still present after remove")
	}
}

func TestRejectInvalid(t *testing.T) {
	s := openTest(t)
	if _, err := s.Add(Newsletter, "mime-version: 1.0", "seed"); err == nil {
		t.Fatal("expected rejection of junk value")
	}
	if _, err := s.Add("bogus", "a@b.com", "api"); err == nil {
		t.Fatal("expected rejection of unknown kind")
	}
}

func TestSnapshot(t *testing.T) {
	s := openTest(t)
	s.Add(Whitelist, "a@b.com", "api")
	s.Add(Newsletter, "n@m.com", "api")
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Whitelist.Contains("a@b.com") || snap.Blocklist.Contains("a@b.com") {
		t.Fatal("snapshot matchers wrong")
	}
}

func TestScreenedIndexTTL(t *testing.T) {
	s := openTest(t)
	s.MarkScreened("<mid-1@x>")
	if ok, _ := s.WasScreened("<MID-1@x>"); !ok {
		t.Fatal("was screened failed")
	}
	// Nothing older than 1h yet.
	if n, _ := s.PruneScreened(time.Hour); n != 0 {
		t.Fatalf("prune removed %d, want 0", n)
	}
	// Everything older than 0 -> pruned.
	if n, _ := s.PruneScreened(0); n != 1 {
		t.Fatalf("prune removed %d, want 1", n)
	}
}

func TestSnoozeRoundtrip(t *testing.T) {
	s := openTest(t)
	past := time.Now().Add(-time.Minute)
	future := time.Now().Add(time.Hour)
	s.Snooze("Snoozed/1w", "1w", "<m1@x>", 10, past)
	s.Snooze("Snoozed/1m", "1m", "<m2@x>", 11, future)

	due, err := s.DueSnoozes(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].UID != 10 {
		t.Fatalf("due = %+v, want only uid 10", due)
	}
	s.Unsnooze("Snoozed/1w", 10)
	if due, _ := s.DueSnoozes(time.Now()); len(due) != 0 {
		t.Fatalf("still due after unsnooze: %+v", due)
	}
}
