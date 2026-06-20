package imap

import (
	"log/slog"
	"time"

	"github.com/Jondalar/mailscreener/internal/classify"
	"github.com/Jondalar/mailscreener/internal/lists"
	"github.com/Jondalar/mailscreener/internal/snooze"
)

// EngineConfig carries the timing knobs.
type EngineConfig struct {
	JunkRetention       time.Duration // 0 = disabled
	ReceiptRetention    time.Duration // 0 = disabled
	NewsletterRetention time.Duration // 0 = disabled
	ScreenedIDTTL       time.Duration
	ReceiptSubjectMatch bool // enable the receipt subject heuristic

	// SnoozeLabels are pre-created as Snoozed/<label> subfolders on Bootstrap.
	// Manually created subfolders are still scanned dynamically (SnoozeScan), so
	// this only seeds the well-known set up front.
	SnoozeLabels []string
}

// Engine runs the screening orchestration over a Backend. It is the side of the
// system that decides what to move where; the verdict itself comes from the
// pure classify engine.
type Engine struct {
	be  Backend
	st  *lists.Store
	f   Folders
	cfg EngineConfig
	log *slog.Logger
	now func() time.Time
}

// NewEngine builds an Engine. now defaults to time.Now if nil.
func NewEngine(be Backend, st *lists.Store, f Folders, cfg EngineConfig, log *slog.Logger, now func() time.Time) *Engine {
	if now == nil {
		now = time.Now
	}
	if log == nil {
		log = slog.Default()
	}
	return &Engine{be: be, st: st, f: f, cfg: cfg, log: log, now: now}
}

// Bootstrap ensures all working folders exist, plus any pre-seeded snooze
// subfolders (SnoozeLabels). Invalid labels are skipped with a warning rather
// than creating a folder SnoozeScan would later reject.
func (e *Engine) Bootstrap() error {
	folders := e.f.all()
	for _, label := range e.cfg.SnoozeLabels {
		if _, ok := snooze.ParseLabel(label, e.now()); !ok {
			e.log.Warn("skipping invalid snooze label", "label", label)
			continue
		}
		folders = append(folders, e.f.Snoozed+"/"+label)
	}
	return e.be.EnsureFolders(folders)
}

// FullSweep runs the complete cycle over every folder (Spec 0003 §run_full /
// safety-net tier): catch-up, full Screened re-sort (retroactive after list
// changes), training, approve, maintenance. Expensive; runs on the periodic
// ticker, not on every IDLE trigger.
func (e *Engine) FullSweep() error {
	if err := e.groupSweepInbox(); err != nil {
		return err
	}
	if err := e.catchUpInbox(); err != nil {
		return err
	}
	if err := e.sortScreened(); err != nil {
		return err
	}
	if err := e.train(); err != nil {
		return err
	}
	if err := e.approve(); err != nil {
		return err
	}
	if err := e.maintenance(); err != nil {
		return err
	}
	if n, err := e.st.PruneScreened(e.cfg.ScreenedIDTTL); err != nil {
		return err
	} else if n > 0 {
		e.log.Info("pruned screened ids", "count", n)
	}
	return nil
}

// QuickSweep is the cheap incremental tier (Spec 0003 §on_new_mail): it touches
// only messages whose UID is above the per-folder watermark — new arrivals and
// user-moved mail — across INBOX, Screened and the training folders. It runs on
// IDLE triggers for < 5 s reactivity and never rescans whole folders.
func (e *Engine) QuickSweep() error {
	snap, err := e.snapshot()
	if err != nil {
		return err
	}
	if err := e.incInbox(snap); err != nil {
		return err
	}
	if err := e.incScreened(snap); err != nil {
		return err
	}
	if err := e.incTrain(e.f.Junk, lists.Blocklist, true); err != nil {
		return err
	}
	if err := e.incTrain(e.f.Receipts, lists.Receipts, false); err != nil {
		return err
	}
	return e.incTrain(e.f.Newsletters, lists.Newsletter, false)
}

// sinceNew fetches the messages newer than a folder's watermark and advances it.
func (e *Engine) sinceNew(folder string) ([]Msg, error) {
	wm, err := e.st.Watermark(folder)
	if err != nil {
		return nil, err
	}
	msgs, err := e.be.ListSince(folder, wm)
	if err != nil {
		return nil, err
	}
	var maxUID uint32
	for _, m := range msgs {
		if m.UID > maxUID {
			maxUID = m.UID
		}
	}
	if err := e.st.BumpWatermark(folder, maxUID); err != nil {
		return nil, err
	}
	return msgs, nil
}

// incScreened sorts only the new messages in Screened.
func (e *Engine) incScreened(snap classify.Snapshot) error {
	msgs, err := e.sinceNew(e.f.Screened)
	if err != nil {
		return err
	}
	return e.routeScreened(msgs, snap)
}

// incInbox handles new INBOX messages: a Screened->INBOX move (Message-ID known)
// is an approve; otherwise block/group -> Junk, whitelist stays, rest -> Screened.
func (e *Engine) incInbox(snap classify.Snapshot) error {
	msgs, err := e.sinceNew(e.f.Inbox)
	if err != nil {
		return err
	}
	var toJunk, toScreened []uint32
	for _, m := range msgs {
		if m.MID != "" {
			if seen, err := e.st.WasScreened(m.MID); err != nil {
				return err
			} else if seen {
				e.approveSender(m)
				continue
			}
		}
		switch e.verdict(m, snap) {
		case classify.Block:
			toJunk = append(toJunk, m.UID)
		case classify.Approve:
			// stays in INBOX
		default:
			if m.MID != "" {
				if err := e.st.MarkScreened(m.MID); err != nil {
					return err
				}
			}
			toScreened = append(toScreened, m.UID)
		}
	}
	if _, err := e.move(e.f.Inbox, toJunk, e.f.Junk); err != nil {
		return err
	}
	_, err = e.move(e.f.Inbox, toScreened, e.f.Screened)
	return err
}

// incTrain learns senders from newly moved-in training-folder messages.
func (e *Engine) incTrain(folder string, kind lists.Kind, clearMID bool) error {
	msgs, err := e.sinceNew(folder)
	if err != nil {
		return err
	}
	return e.learnMsgs(msgs, kind, clearMID)
}

// approveSender whitelists a sender, removes it from the blocklist (K1), and
// clears its screened-id row.
func (e *Engine) approveSender(m Msg) {
	if m.Sender != "" {
		if _, err := e.st.Add(lists.Whitelist, m.Sender, "training"); err == nil {
			e.st.Remove(lists.Blocklist, m.Sender)
			e.log.Info("approved sender", "sender", m.Sender)
		}
	}
	e.st.ClearScreened(m.MID)
}

// snapshot builds a list snapshot and applies engine-level classify config.
func (e *Engine) snapshot() (classify.Snapshot, error) {
	snap, err := e.st.Snapshot()
	if err != nil {
		return snap, err
	}
	snap.ReceiptSubjectMatch = e.cfg.ReceiptSubjectMatch
	return snap, nil
}

// classify resolves a verdict for a message using a fresh list snapshot.
func (e *Engine) verdict(m Msg, snap classify.Snapshot) classify.Verdict {
	return classify.Classify(classify.Sender{Address: m.Sender}, m.Headers, snap)
}

// groupSweepInbox moves mailing-list / Google-Group mail out of INBOX to Junk,
// including already-read messages (ports the production apply_groups_00A full
// scan, Spec 0008). Whitelisted and group-allowlisted senders are spared.
func (e *Engine) groupSweepInbox() error {
	snap, err := e.snapshot()
	if err != nil {
		return err
	}
	msgs, err := e.be.List(e.f.Inbox, false)
	if err != nil {
		return err
	}
	var toJunk []uint32
	for _, m := range msgs {
		if !classify.IsGroupMessage(m.Headers) {
			continue
		}
		if m.Sender != "" && (snap.Whitelist.Contains(m.Sender) || snap.GroupAllowlist.Contains(m.Sender)) {
			continue
		}
		toJunk = append(toJunk, m.UID)
	}
	_, err = e.move(e.f.Inbox, toJunk, e.f.Junk)
	return err
}

// catchUpInbox handles mail that reached INBOX directly (e.g. Hide-My-Mail):
// block/group -> Junk, whitelist stays, everything else -> Screened (Spec 0003).
func (e *Engine) catchUpInbox() error {
	snap, err := e.snapshot()
	if err != nil {
		return err
	}
	msgs, err := e.be.List(e.f.Inbox, true) // unseen only
	if err != nil {
		return err
	}
	var toJunk, toScreened []uint32
	for _, m := range msgs {
		switch e.verdict(m, snap) {
		case classify.Block:
			toJunk = append(toJunk, m.UID)
		case classify.Approve:
			// stays in INBOX
		default:
			if m.MID != "" {
				if err := e.st.MarkScreened(m.MID); err != nil {
					return err
				}
			}
			toScreened = append(toScreened, m.UID)
		}
	}
	if _, err := e.move(e.f.Inbox, toJunk, e.f.Junk); err != nil {
		return err
	}
	_, err = e.move(e.f.Inbox, toScreened, e.f.Screened)
	return err
}

// sortScreened classifies everything in Screened and routes it (full tier).
func (e *Engine) sortScreened() error {
	snap, err := e.snapshot()
	if err != nil {
		return err
	}
	msgs, err := e.be.List(e.f.Screened, false)
	if err != nil {
		return err
	}
	e.bumpFrom(e.f.Screened, msgs)
	return e.routeScreened(msgs, snap)
}

// routeScreened classifies the given Screened messages and moves them. Unknown
// stays and is tracked for later approve detection. Shared by full and quick.
func (e *Engine) routeScreened(msgs []Msg, snap classify.Snapshot) error {
	var toInbox, toJunk, toNews, toRcpt []uint32
	for _, m := range msgs {
		switch e.verdict(m, snap) {
		case classify.Approve:
			toInbox = append(toInbox, m.UID)
		case classify.Block:
			toJunk = append(toJunk, m.UID)
		case classify.Newsletter:
			toNews = append(toNews, m.UID)
		case classify.Receipt:
			toRcpt = append(toRcpt, m.UID)
		default: // unknown: stays, track MID for approve
			if m.MID != "" {
				if err := e.st.MarkScreened(m.MID); err != nil {
					return err
				}
			}
		}
	}
	for _, mv := range []struct {
		uids []uint32
		dest string
	}{
		{toInbox, e.f.Inbox}, {toJunk, e.f.Junk}, {toNews, e.f.Newsletters}, {toRcpt, e.f.Receipts},
	} {
		if _, err := e.move(e.f.Screened, mv.uids, mv.dest); err != nil {
			return err
		}
	}
	return nil
}

// train learns from manual moves into the training folders.
func (e *Engine) train() error {
	// Junk -> blocklist
	if err := e.learnFolder(e.f.Junk, lists.Blocklist, true); err != nil {
		return err
	}
	// Receipts -> receipts list
	if err := e.learnFolder(e.f.Receipts, lists.Receipts, false); err != nil {
		return err
	}
	// Newsletters -> newsletter list
	return e.learnFolder(e.f.Newsletters, lists.Newsletter, false)
}

// learnFolder adds the senders of every message in folder to a list (full tier).
func (e *Engine) learnFolder(folder string, kind lists.Kind, clearMID bool) error {
	msgs, err := e.be.List(folder, false)
	if err != nil {
		return err
	}
	e.bumpFrom(folder, msgs)
	return e.learnMsgs(msgs, kind, clearMID)
}

// learnMsgs adds the senders of the given messages to a list. When clearMID is
// set (Junk), it also drops each message's screened-id entry. Shared by full
// and quick tiers.
func (e *Engine) learnMsgs(msgs []Msg, kind lists.Kind, clearMID bool) error {
	for _, m := range msgs {
		if m.Sender != "" {
			if _, err := e.st.Add(kind, m.Sender, "training"); err != nil {
				// invalid sender is non-fatal; skip
				e.log.Debug("training add skipped", "kind", kind, "sender", m.Sender, "err", err)
			}
		}
		if clearMID && m.MID != "" {
			if err := e.st.ClearScreened(m.MID); err != nil {
				return err
			}
		}
	}
	return nil
}

// approve detects messages the user moved Screened -> INBOX and whitelists their
// senders (also removing them from the blocklist, K1).
func (e *Engine) approve() error {
	msgs, err := e.be.List(e.f.Inbox, false)
	if err != nil {
		return err
	}
	e.bumpFrom(e.f.Inbox, msgs)
	for _, m := range msgs {
		if m.MID == "" {
			continue
		}
		seen, err := e.st.WasScreened(m.MID)
		if err != nil {
			return err
		}
		if !seen {
			continue
		}
		if m.Sender != "" {
			if _, err := e.st.Add(lists.Whitelist, m.Sender, "training"); err != nil {
				e.log.Debug("approve add skipped", "sender", m.Sender, "err", err)
			} else {
				if _, err := e.st.Remove(lists.Blocklist, m.Sender); err != nil {
					return err
				}
				e.log.Info("approved sender", "sender", m.Sender)
			}
		}
		if err := e.st.ClearScreened(m.MID); err != nil {
			return err
		}
	}
	return nil
}

// maintenance applies retention: Junk seen+aged -> Deleted, Receipts aged ->
// Archive, Newsletters seen + aged -> Archive.
func (e *Engine) maintenance() error {
	if msgs, err := e.be.List(e.f.Junk, true); err != nil {
		return err
	} else if err := e.markSeen(e.f.Junk, uids(msgs)); err != nil {
		return err
	}
	if e.cfg.JunkRetention > 0 {
		old, err := e.be.ListOlderThan(e.f.Junk, e.cfg.JunkRetention)
		if err != nil {
			return err
		}
		if _, err := e.move(e.f.Junk, uids(old), e.f.Deleted); err != nil {
			return err
		}
	}
	if e.cfg.ReceiptRetention > 0 {
		old, err := e.be.ListOlderThan(e.f.Receipts, e.cfg.ReceiptRetention)
		if err != nil {
			return err
		}
		if _, err := e.move(e.f.Receipts, uids(old), e.f.Archive); err != nil {
			return err
		}
	}
	if msgs, err := e.be.List(e.f.Newsletters, true); err != nil {
		return err
	} else if err := e.markSeen(e.f.Newsletters, uids(msgs)); err != nil {
		return err
	}
	if e.cfg.NewsletterRetention > 0 {
		old, err := e.be.ListOlderThan(e.f.Newsletters, e.cfg.NewsletterRetention)
		if err != nil {
			return err
		}
		if _, err := e.move(e.f.Newsletters, uids(old), e.f.Archive); err != nil {
			return err
		}
	}
	return nil
}

// --- snooze (Spec 0007) ---

// SnoozeScan registers messages dropped into Snoozed/<label> subfolders and
// moves them up into the parent Snoozed folder with a computed wake time.
func (e *Engine) SnoozeScan() error {
	children, err := e.be.ListChildren(e.f.Snoozed)
	if err != nil {
		return err
	}
	for _, child := range children {
		label := lastSegment(child)
		if label == "" || label == e.f.Snoozed {
			continue
		}
		wake, ok := snooze.ParseLabel(label, e.now())
		if !ok {
			e.log.Warn("unknown snooze label", "folder", child, "label", label)
			continue
		}
		msgs, err := e.be.List(child, false)
		if err != nil {
			return err
		}
		// Consolidate into the parent Snoozed folder first; the move reassigns
		// UIDs, so record each row against its NEW parent UID (from COPYUID).
		// Without that, SnoozeWake would later move by a stale child UID.
		newUIDs, err := e.move(child, uids(msgs), e.f.Snoozed)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			uid := m.UID
			if nu, ok := newUIDs[m.UID]; ok {
				uid = nu
			}
			if err := e.st.Snooze(e.f.Snoozed, label, m.MID, uid, wake); err != nil {
				return err
			}
		}
		if len(msgs) > 0 {
			e.log.Info("snoozed", "folder", child, "count", len(msgs), "wake", wake)
		}
	}
	return nil
}

// AdoptLegacySnoozes is a one-time migration: messages already parked in the
// parent Snoozed folder by the old daemon carry SNOOZED_<LABEL> keyword flags
// but have no per-message wake row. This reads the legacy snoozed_map.txt for
// wake times (falling back to recomputing from the label) and creates rows so
// they wake correctly. Guarded by a counter so it runs once (Spec 0007).
func (e *Engine) AdoptLegacySnoozes(mapPath string) error {
	if done, err := e.st.GetCounter("legacy_snooze_adopted"); err != nil {
		return err
	} else if done != 0 {
		return nil
	}

	legacy, err := snooze.ParseSnoozedMap(mapPath)
	if err != nil {
		return err
	}
	msgs, err := e.be.List(e.f.Snoozed, false)
	if err != nil {
		return err
	}
	adopted := 0
	for _, m := range msgs {
		var label string
		for _, fl := range m.Flags {
			if l := snooze.LabelFromKeyword(fl); l != "" {
				label = l
				break
			}
		}
		if label == "" {
			continue // v2 snooze (DB-tracked) or unrelated mail
		}
		// The legacy map is keyed by the old child-folder string, but the message
		// physically sits in the parent Snoozed folder — store the row against the
		// parent (folder + UID) so SnoozeWake moves the right message.
		mapKey := e.f.Snoozed + "/" + label
		wake, ok := legacy[mapKey]
		if !ok {
			if wake, ok = snooze.ParseLabel(label, e.now()); !ok {
				continue
			}
		}
		if err := e.st.Snooze(e.f.Snoozed, label, m.MID, m.UID, wake); err != nil {
			return err
		}
		adopted++
	}
	if err := e.st.SetCounter("legacy_snooze_adopted", 1); err != nil {
		return err
	}
	if adopted > 0 {
		e.log.Info("adopted legacy snoozes", "count", adopted)
	}
	return nil
}

// SnoozeWake moves due messages back to INBOX as unread. Each row carries the
// folder + UID where the message currently sits (the parent Snoozed for v2
// snoozes, recorded post-consolidation; the same for legacy-adopted ones), so
// the move and the unread-flag clear both target the right message.
func (e *Engine) SnoozeWake() error {
	due, err := e.st.DueSnoozes(e.now())
	if err != nil {
		return err
	}
	woke := 0
	for _, d := range due {
		ids := []uint32{d.UID}
		// Clear \Seen *before* the move: an IMAP MOVE reassigns the UID in the
		// destination, so the source UID can't be used against INBOX afterwards.
		// The flag travels with the message, so the woken mail arrives unread.
		if err := e.be.ClearSeen(d.Folder, ids); err != nil {
			e.log.Warn("clear seen before wake failed", "folder", d.Folder, "err", err)
		}
		if _, err := e.move(d.Folder, ids, e.f.Inbox); err != nil {
			return err
		}
		if err := e.st.Unsnooze(d.Folder, d.UID); err != nil {
			return err
		}
		woke++
	}
	if woke > 0 {
		e.log.Info("woke snoozed", "count", woke)
	}
	return nil
}

// --- helpers ---

// move moves messages and returns the server's source→dest UID mapping (may be
// empty). Most callers ignore the map; the snooze path needs it to track the
// new UID after consolidation.
func (e *Engine) move(folder string, ids []uint32, dest string) (map[uint32]uint32, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	e.log.Info("move", "from", folder, "to", dest, "count", len(ids))
	return e.be.Move(folder, ids, dest)
}

func (e *Engine) markSeen(folder string, ids []uint32) error {
	if len(ids) == 0 {
		return nil
	}
	return e.be.MarkSeen(folder, ids)
}

// bumpFrom advances a folder's watermark to the highest UID among msgs, so a
// full sweep leaves the quick tier in an incremental state (best-effort).
func (e *Engine) bumpFrom(folder string, msgs []Msg) {
	var maxUID uint32
	for _, m := range msgs {
		if m.UID > maxUID {
			maxUID = m.UID
		}
	}
	if maxUID > 0 {
		if err := e.st.BumpWatermark(folder, maxUID); err != nil {
			e.log.Debug("bump watermark failed", "folder", folder, "err", err)
		}
	}
}

func uids(msgs []Msg) []uint32 {
	out := make([]uint32, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.UID)
	}
	return out
}

func lastSegment(name string) string {
	last := name
	for i := len(name) - 1; i >= 0; i-- {
		if c := name[i]; c == '/' || c == '\\' || c == '.' || c == ':' {
			last = name[i+1:]
			break
		}
	}
	return last
}
