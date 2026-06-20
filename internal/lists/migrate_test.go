package lists

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateCleans(t *testing.T) {
	dir := t.TempDir()
	// Dupes, junk header lines, self address, a wildcard.
	writeFile(t, dir, "newsletterlist.txt",
		"news@a.com\nnews@a.com\nmime-version: 1.0\nsubject: hi\nto: me@self.com\nme@self.com\n*@list.b.com\n")
	writeFile(t, dir, "blocklist.txt", "spam@bad.com\n")

	s := openTest(t)
	rep, err := s.Migrate(dir, []string{"me@self.com"})
	if err != nil {
		t.Fatal(err)
	}

	var nl KindReport
	for _, l := range rep.Lines {
		if l.Kind == Newsletter {
			nl = l
		}
	}
	if nl.Kept != 2 { // news@a.com + *@list.b.com
		t.Errorf("newsletter kept = %d, want 2 (%+v)", nl.Kept, nl)
	}
	if nl.Dupes != 1 {
		t.Errorf("newsletter dupes = %d, want 1", nl.Dupes)
	}
	if nl.Junk != 3 { // mime-version + subject + "to: me@self.com" (has space)
		t.Errorf("newsletter junk = %d, want 3", nl.Junk)
	}
	if nl.SelfHit != 1 {
		t.Errorf("newsletter self = %d, want 1", nl.SelfHit)
	}

	// Idempotent: second run inserts nothing new.
	rep2, _ := s.Migrate(dir, []string{"me@self.com"})
	for _, l := range rep2.Lines {
		if l.Kept != 0 {
			t.Errorf("re-migrate kept %d for %s, want 0", l.Kept, l.Kind)
		}
	}

	// Legacy group exceptions seeded.
	if ok, _ := s.Contains(GroupAllowlist, "drpong@googlegroups.com"); !ok {
		t.Error("group allowlist not seeded")
	}
}

func TestSuggest(t *testing.T) {
	s := openTest(t)
	// 3 senders on one non-freemail domain -> suggestion.
	s.Add(Blocklist, "a@spam.io", "seed")
	s.Add(Blocklist, "b@spam.io", "seed")
	s.Add(Blocklist, "c@spam.io", "seed")
	// Freemail cluster must never be suggested.
	s.Add(Blocklist, "x@gmail.com", "seed")
	s.Add(Blocklist, "y@gmail.com", "seed")
	s.Add(Blocklist, "z@gmail.com", "seed")
	// Apple relay likewise.
	s.Add(Newsletter, "a@privaterelay.appleid.com", "seed")
	s.Add(Newsletter, "b@privaterelay.appleid.com", "seed")
	s.Add(Newsletter, "c@privaterelay.appleid.com", "seed")

	sug, err := s.Suggest(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(sug) != 1 {
		t.Fatalf("got %d suggestions, want 1: %+v", len(sug), sug)
	}
	if sug[0].Wildcard != "*@spam.io" || sug[0].Kind != Blocklist {
		t.Fatalf("unexpected suggestion %+v", sug[0])
	}

	// Apply collapses exacts into the wildcard.
	if err := s.ApplySuggestion(Blocklist, "*@spam.io"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Contains(Blocklist, "d@spam.io"); !ok {
		t.Error("wildcard not active after apply")
	}
	n, _ := s.Count(Blocklist)
	if n != 4 { // *@spam.io + 3 gmail exacts
		t.Errorf("blocklist count = %d, want 4", n)
	}
}
