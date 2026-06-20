package imap

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Jondalar/mailscreener/internal/classify"
	"github.com/Jondalar/mailscreener/internal/lists"
)

// fakeBackend is an in-memory Backend for testing the Engine.
type fakeBackend struct {
	folders map[string][]Msg
	seen    map[string]map[uint32]bool
	uidSeq  uint32 // models IMAP UID reassignment on MOVE
}

func newFake() *fakeBackend {
	return &fakeBackend{folders: map[string][]Msg{}, seen: map[string]map[uint32]bool{}, uidSeq: 1000}
}

func (b *fakeBackend) add(folder string, m Msg) {
	m.Folder = folder
	b.folders[folder] = append(b.folders[folder], m)
}

func (b *fakeBackend) EnsureFolders([]string) error { return nil }

func (b *fakeBackend) List(folder string, unseenOnly bool) ([]Msg, error) {
	var out []Msg
	for _, m := range b.folders[folder] {
		if unseenOnly && b.seen[folder][m.UID] {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (b *fakeBackend) ListSince(folder string, since uint32) ([]Msg, error) {
	var out []Msg
	for _, m := range b.folders[folder] {
		if m.UID > since {
			out = append(out, m)
		}
	}
	return out, nil
}

func (b *fakeBackend) ListOlderThan(folder string, d time.Duration) ([]Msg, error) {
	cut := time.Now().Add(-d)
	var out []Msg
	for _, m := range b.folders[folder] {
		if m.Date.Before(cut) {
			out = append(out, m)
		}
	}
	return out, nil
}

func (b *fakeBackend) ListChildren(parent string) ([]string, error) {
	var out []string
	for f := range b.folders {
		if strings.HasPrefix(f, parent+"/") {
			out = append(out, f)
		}
	}
	return out, nil
}

// Move models a real IMAP MOVE: the message gets a NEW UID in the destination
// (returned as a source→dest map), and its \Seen flag travels with it.
func (b *fakeBackend) Move(folder string, ids []uint32, dest string) (map[uint32]uint32, error) {
	want := map[uint32]bool{}
	for _, id := range ids {
		want[id] = true
	}
	mapping := map[uint32]uint32{}
	var keep []Msg
	for _, m := range b.folders[folder] {
		if !want[m.UID] {
			keep = append(keep, m)
			continue
		}
		old := m.UID
		b.uidSeq++
		nu := b.uidSeq
		mapping[old] = nu
		wasSeen := b.seen[folder] != nil && b.seen[folder][old]
		if b.seen[folder] != nil {
			delete(b.seen[folder], old)
		}
		m.UID = nu
		m.Folder = dest
		b.folders[dest] = append(b.folders[dest], m)
		if wasSeen {
			if b.seen[dest] == nil {
				b.seen[dest] = map[uint32]bool{}
			}
			b.seen[dest][nu] = true
		}
	}
	b.folders[folder] = keep
	return mapping, nil
}

func (b *fakeBackend) MarkSeen(folder string, ids []uint32) error {
	if b.seen[folder] == nil {
		b.seen[folder] = map[uint32]bool{}
	}
	for _, id := range ids {
		b.seen[folder][id] = true
	}
	return nil
}

func (b *fakeBackend) ClearSeen(folder string, ids []uint32) error {
	for _, id := range ids {
		if b.seen[folder] != nil {
			delete(b.seen[folder], id)
		}
	}
	return nil
}

func newEngine(t *testing.T, be Backend) (*Engine, *lists.Store) {
	t.Helper()
	st, err := lists.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	f := DefaultFolders("Screened")
	e := NewEngine(be, st, f, EngineConfig{ScreenedIDTTL: 72 * time.Hour}, nil, func() time.Time {
		return time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	})
	return e, st
}

func folderUIDs(b *fakeBackend, folder string) []uint32 {
	var out []uint32
	for _, m := range b.folders[folder] {
		out = append(out, m.UID)
	}
	return out
}

// hasSender reports whether folder contains a message from sender (used instead
// of UID assertions for moved messages, whose UID is reassigned by Move).
func hasSender(b *fakeBackend, folder, sender string) bool {
	for _, m := range b.folders[folder] {
		if m.Sender == sender {
			return true
		}
	}
	return false
}

func TestMaintenanceAgesNewslettersAndReceipts(t *testing.T) {
	be := newFake()
	base := time.Now()
	// aged (60d) + fresh (1d) newsletters; one aged receipt.
	be.add("Newsletters", Msg{UID: 1, Date: base.Add(-60 * 24 * time.Hour)})
	be.add("Newsletters", Msg{UID: 2, Date: base.Add(-1 * 24 * time.Hour)})
	be.add("Receipts", Msg{UID: 3, Date: base.Add(-60 * 24 * time.Hour)})

	st, err := lists.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	e := NewEngine(be, st, DefaultFolders("Screened"), EngineConfig{
		NewsletterRetention: 30 * 24 * time.Hour,
		ReceiptRetention:    30 * 24 * time.Hour,
	}, nil, nil)

	if err := e.maintenance(); err != nil {
		t.Fatal(err)
	}
	if got := folderUIDs(be, "Newsletters"); len(got) != 1 || got[0] != 2 {
		t.Errorf("Newsletters = %v, want [2] (fresh stays)", got)
	}
	if got := folderUIDs(be, "Archive"); len(got) != 2 {
		t.Errorf("Archive = %v, want 2 (aged newsletter + receipt)", got)
	}
}

func TestSortScreened(t *testing.T) {
	be := newFake()
	be.add("Screened", Msg{UID: 1, Sender: "wl@x.com"})
	be.add("Screened", Msg{UID: 2, Sender: "bl@y.com"})
	be.add("Screened", Msg{UID: 3, Sender: "n@z.com", Headers: classify.Headers{ListID: "<l.z.com>"}})
	be.add("Screened", Msg{UID: 4, Sender: "shop@s.com"})
	be.add("Screened", Msg{UID: 5, Sender: "unknown@q.com", MID: "<mid5@q>"})

	e, st := newEngine(t, be)
	st.Add(lists.Whitelist, "wl@x.com", "seed")
	st.Add(lists.Blocklist, "bl@y.com", "seed")
	st.Add(lists.Receipts, "shop@s.com", "seed")

	if err := e.sortScreened(); err != nil {
		t.Fatal(err)
	}
	if got := folderUIDs(be, "INBOX"); len(got) != 1 || !hasSender(be, "INBOX", "wl@x.com") {
		t.Errorf("INBOX = %v, want [wl@x.com]", got)
	}
	if got := folderUIDs(be, "Junk"); len(got) != 1 || !hasSender(be, "Junk", "bl@y.com") {
		t.Errorf("Junk = %v, want [bl@y.com]", got)
	}
	if got := folderUIDs(be, "Newsletters"); len(got) != 1 || !hasSender(be, "Newsletters", "n@z.com") {
		t.Errorf("Newsletters = %v, want [n@z.com]", got)
	}
	if got := folderUIDs(be, "Receipts"); len(got) != 1 || !hasSender(be, "Receipts", "shop@s.com") {
		t.Errorf("Receipts = %v, want [shop@s.com]", got)
	}
	if got := folderUIDs(be, "Screened"); len(got) != 1 || got[0] != 5 {
		t.Errorf("Screened (unknown stays) = %v, want [5]", got)
	}
	if ok, _ := st.WasScreened("<mid5@q>"); !ok {
		t.Error("unknown MID not tracked")
	}
}

func TestCatchUpInbox(t *testing.T) {
	be := newFake()
	be.add("INBOX", Msg{UID: 1, Sender: "bl@y.com"})                 // block -> Junk
	be.add("INBOX", Msg{UID: 2, Sender: "wl@x.com"})                 // approve -> stay
	be.add("INBOX", Msg{UID: 3, Sender: "new@q.com", MID: "<m3@q>"}) // -> Screened

	e, st := newEngine(t, be)
	st.Add(lists.Whitelist, "wl@x.com", "seed")
	st.Add(lists.Blocklist, "bl@y.com", "seed")

	if err := e.catchUpInbox(); err != nil {
		t.Fatal(err)
	}
	if got := folderUIDs(be, "Junk"); len(got) != 1 || !hasSender(be, "Junk", "bl@y.com") {
		t.Errorf("Junk = %v, want [bl@y.com]", got)
	}
	if got := folderUIDs(be, "INBOX"); len(got) != 1 || got[0] != 2 {
		t.Errorf("INBOX = %v, want [2] (wl@x.com stays)", got)
	}
	if got := folderUIDs(be, "Screened"); len(got) != 1 || !hasSender(be, "Screened", "new@q.com") {
		t.Errorf("Screened = %v, want [new@q.com]", got)
	}
	if ok, _ := st.WasScreened("<m3@q>"); !ok {
		t.Error("screened MID not tracked")
	}
}

func TestApproveWhitelistsAndUnblocks(t *testing.T) {
	be := newFake()
	be.add("INBOX", Msg{UID: 7, Sender: "person@x.com", MID: "<m7@x>"})

	e, st := newEngine(t, be)
	st.Add(lists.Blocklist, "person@x.com", "seed") // previously blocked
	st.MarkScreened("<m7@x>")                       // was in Screened before

	if err := e.approve(); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.Contains(lists.Whitelist, "person@x.com"); !ok {
		t.Error("sender not whitelisted")
	}
	if ok, _ := st.Contains(lists.Blocklist, "person@x.com"); ok {
		t.Error("sender still on blocklist (K1: approve must un-block)")
	}
	if ok, _ := st.WasScreened("<m7@x>"); ok {
		t.Error("screened MID not cleared")
	}
}

func TestTrainJunkToBlocklist(t *testing.T) {
	be := newFake()
	be.add("Junk", Msg{UID: 1, Sender: "spam@bad.com", MID: "<m1@b>"})
	e, st := newEngine(t, be)
	st.MarkScreened("<m1@b>")

	if err := e.train(); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.Contains(lists.Blocklist, "spam@bad.com"); !ok {
		t.Error("junk sender not blocklisted")
	}
	if ok, _ := st.WasScreened("<m1@b>"); ok {
		t.Error("MID not cleared on junk training")
	}
}

func TestQuickSweepWatermark(t *testing.T) {
	be := newFake()
	be.add("Screened", Msg{UID: 10, Sender: "bl@y.com"})                      // block -> Junk
	be.add("Screened", Msg{UID: 11, Sender: "unknown@q.com", MID: "<m11@q>"}) // stays
	e, st := newEngine(t, be)
	st.Add(lists.Blocklist, "bl@y.com", "seed")

	if err := e.QuickSweep(); err != nil {
		t.Fatal(err)
	}
	if got := folderUIDs(be, "Junk"); len(got) != 1 || !hasSender(be, "Junk", "bl@y.com") {
		t.Fatalf("Junk = %v, want [bl@y.com]", got)
	}
	wm, _ := st.Watermark("Screened")
	if wm != 11 {
		t.Fatalf("watermark = %d, want 11", wm)
	}

	// A user moves the unknown mail to Junk; it arrives with a NEW higher UID
	// (above the UIDs the Move helper reassigns, mirroring real IMAP).
	be.add("Junk", Msg{UID: 5000, Sender: "later@spam.com", MID: "<m50@s>"})
	// Second quick sweep: nothing new in Screened (uid 11 <= watermark),
	// but the new Junk message trains the blocklist.
	if err := e.QuickSweep(); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.Contains(lists.Blocklist, "later@spam.com"); !ok {
		t.Error("new Junk sender not learned incrementally")
	}
	// uid 11 still sits in Screened, untouched (not re-routed).
	if got := folderUIDs(be, "Screened"); len(got) != 1 || got[0] != 11 {
		t.Fatalf("Screened = %v, want [11] untouched", got)
	}
}

func TestGroupSweepInbox(t *testing.T) {
	be := newFake()
	be.add("INBOX", Msg{UID: 1, Sender: "g@x.com", Headers: classify.Headers{XGoogleGroupID: "abc"}})  // group -> Junk
	be.add("INBOX", Msg{UID: 2, Sender: "ok@x.com", Headers: classify.Headers{XGoogleGroupID: "abc"}}) // allowlisted -> stay
	be.add("INBOX", Msg{UID: 3, Sender: "plain@x.com"})                                                // not a group -> stay
	e, st := newEngine(t, be)
	st.Add(lists.GroupAllowlist, "ok@x.com", "seed")

	if err := e.groupSweepInbox(); err != nil {
		t.Fatal(err)
	}
	if got := folderUIDs(be, "Junk"); len(got) != 1 || !hasSender(be, "Junk", "g@x.com") {
		t.Fatalf("Junk = %v, want [g@x.com]", got)
	}
	if got := folderUIDs(be, "INBOX"); len(got) != 2 {
		t.Fatalf("INBOX = %v, want 2 (ok@/plain@ stay)", got)
	}
}

func TestAdoptLegacySnoozes(t *testing.T) {
	be := newFake()
	// Legacy: parked in parent Snoozed with a SNOOZED_1W keyword, no DB row.
	be.add("Snoozed", Msg{UID: 5, MID: "<m5@x>", Flags: []string{"\\Seen", "SNOOZED_1W"}})
	// v2-style mail (no snooze keyword) must be ignored.
	be.add("Snoozed", Msg{UID: 6, MID: "<m6@x>"})

	e, st := newEngine(t, be)
	dir := t.TempDir()
	past := time.Date(2026, 6, 17, 11, 0, 0, 0, time.UTC).Unix() // before engine's fixed now
	writeFile2(t, dir, "snoozed_map.txt", "Snoozed/1w "+itoa(past)+"\n")

	if err := e.AdoptLegacySnoozes(dir + "/snoozed_map.txt"); err != nil {
		t.Fatal(err)
	}
	due, _ := st.DueSnoozes(time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC))
	if len(due) != 1 || due[0].UID != 5 {
		t.Fatalf("due = %+v, want only uid 5", due)
	}
	// Idempotent: second call adopts nothing more (counter guard).
	st.Unsnooze("Snoozed", 5)
	if err := e.AdoptLegacySnoozes(dir + "/snoozed_map.txt"); err != nil {
		t.Fatal(err)
	}
	if due, _ := st.DueSnoozes(time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)); len(due) != 0 {
		t.Fatalf("re-adopt should be a no-op, got %+v", due)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func writeFile2(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(dir+"/"+name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSnoozeScanAndWake(t *testing.T) {
	be := newFake()
	be.add("Snoozed/1w", Msg{UID: 1, Sender: "a@x.com", MID: "<m1@x>"})
	e, st := newEngine(t, be)

	if err := e.SnoozeScan(); err != nil {
		t.Fatal(err)
	}
	// Consolidated into the parent (UID reassigned by Move), child emptied.
	parent := folderUIDs(be, "Snoozed")
	if len(parent) != 1 || !hasSender(be, "Snoozed", "a@x.com") {
		t.Fatalf("Snoozed parent = %v, want one msg from a@x.com", parent)
	}
	if got := folderUIDs(be, "Snoozed/1w"); len(got) != 0 {
		t.Fatalf("child not emptied: %v", got)
	}
	// The user had read it while it sat in Snoozed.
	if err := be.MarkSeen("Snoozed", parent); err != nil {
		t.Fatal(err)
	}
	// Not due yet (wake = scan-now + 1w).
	if err := e.SnoozeWake(); err != nil {
		t.Fatal(err)
	}
	if got := folderUIDs(be, "INBOX"); len(got) != 0 {
		t.Fatalf("woke too early: %v", got)
	}
	// Force due: replace the real row (parent folder + reassigned UID) with a
	// past wake time, before the engine's fixed clock (2026-06-17 12:00).
	if err := st.Snooze("Snoozed", "1w", "<m1@x>", parent[0], time.Date(2026, 6, 17, 11, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := e.SnoozeWake(); err != nil {
		t.Fatal(err)
	}
	if got := folderUIDs(be, "INBOX"); len(got) != 1 || !hasSender(be, "INBOX", "a@x.com") {
		t.Fatalf("INBOX after wake = %v, want one msg from a@x.com", got)
	}
	// Regression: the woken mail must be UNREAD (clear \Seen happens before the
	// move; with the old clear-after-move it would arrive read).
	unread, err := be.List("INBOX", true) // unseen only
	if err != nil {
		t.Fatal(err)
	}
	if len(unread) != 1 {
		t.Fatalf("woken mail not unread: unseen INBOX = %v, want 1", unread)
	}
}
