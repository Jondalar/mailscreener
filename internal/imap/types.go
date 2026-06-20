// Package imap drives the IMAP side: a testable Engine that holds the screening
// orchestration (catch-up, sort, training, maintenance, snooze) over a Backend
// abstraction, plus a go-imap/v2 Backend implementation. See Specs 0003/0004/
// 0007/0008.
package imap

import (
	"time"

	"github.com/Jondalar/mailscreener/internal/classify"
)

// Msg is a message as the Engine needs to see it: identity plus the header
// inputs for classification. The Backend fills it; the Engine never parses raw
// IMAP itself.
type Msg struct {
	UID     uint32
	Folder  string
	Sender  string // parsed From/Sender/Return-Path, lowercased
	MID     string // Message-ID without angle brackets, lowercased
	Date    time.Time
	Flags   []string // IMAP flags/keywords (e.g. legacy SNOOZED_1W)
	Headers classify.Headers
}

// Folders names the mailboxes the Engine works with.
type Folders struct {
	Inbox       string
	Screened    string
	Junk        string
	Receipts    string
	Newsletters string
	Archive     string
	Deleted     string
	Snoozed     string
}

// DefaultFolders is the iCloud layout.
func DefaultFolders(screened string) Folders {
	if screened == "" {
		screened = "Screened"
	}
	return Folders{
		Inbox:       "INBOX",
		Screened:    screened,
		Junk:        "Junk",
		Receipts:    "Receipts",
		Newsletters: "Newsletters",
		Archive:     "Archive",
		Deleted:     "Deleted Messages",
		Snoozed:     "Snoozed",
	}
}

func (f Folders) all() []string {
	return []string{f.Screened, f.Receipts, f.Newsletters, f.Snoozed, f.Junk, f.Archive, f.Deleted}
}

// Backend is the IMAP surface the Engine depends on. The go-imap/v2 client
// implements it; tests use a fake.
type Backend interface {
	// EnsureFolders creates/subscribes the given mailboxes (idempotent).
	EnsureFolders(names []string) error
	// List returns messages (with headers) in a folder; unseenOnly limits to
	// the \Unseen set.
	List(folder string, unseenOnly bool) ([]Msg, error)
	// ListSince returns messages whose UID is greater than sinceUID — the
	// incremental "new or moved-in" set for the quick tier (Spec 0003).
	ListSince(folder string, sinceUID uint32) ([]Msg, error)
	// ListOlderThan returns messages whose INTERNALDATE is older than d.
	ListOlderThan(folder string, d time.Duration) ([]Msg, error)
	// ListChildren returns child mailbox names under parent (e.g. "Snoozed/1w").
	ListChildren(parent string) ([]string, error)
	// Move moves messages by UID to dest. It returns a source→dest UID map from
	// the server's COPYUID/MOVE response (UIDPLUS); the map may be empty if the
	// server doesn't report it.
	Move(folder string, uids []uint32, dest string) (map[uint32]uint32, error)
	// MarkSeen / ClearSeen toggle the \Seen flag.
	MarkSeen(folder string, uids []uint32) error
	ClearSeen(folder string, uids []uint32) error
}
