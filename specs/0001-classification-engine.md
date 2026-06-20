# Spec 0001 — Classification Engine

Status: draft · Phase: Spec · Refs: ADR-0005, arc42 §8.1, decision D4

## Purpose

Decide, for one incoming message, where it belongs — as a **pure function** with
no I/O and no side effects, so it is fully unit-testable.

## Interface

```go
type Verdict string // "approve" | "block" | "newsletter" | "receipt" | "unknown"

type Sender struct {
    Address string // lowercased, e.g. "a@b.com"; "" if unparseable
}

type Headers struct {
    ListID          string // raw List-Id value, "" if absent
    ListUnsubscribe string // raw List-Unsubscribe value, "" if absent
}

type Snapshot struct {
    Whitelist  Matcher
    Blocklist  Matcher
    Newsletter Matcher
    Receipts   Matcher
}

func Classify(s Sender, h Headers, lists Snapshot) Verdict
```

`Matcher` answers `Contains(addr string) bool` honoring exact address first, then
`*@domain` wildcard (and parent-domain `.domain`). The engine never reads files.

## Rules (deterministic, evaluated in order — first match wins)

1. `Whitelist.Contains(sender)` → **approve** _(always wins, Q4: a wanted sender
   never lands in junk; whitelist beats blocklist on conflict, QS3)_
2. `Blocklist.Contains(sender)` → **block**
3. `Newsletter.Contains(sender)` **OR** `ListID != ""` **OR**
   `ListUnsubscribe != ""` → **newsletter**
4. `Receipts.Contains(sender)` → **receipt** _(list)_; **or**, only when
   `ReceiptSubjectMatch` is enabled (`RECEIPT_SUBJECT_MATCH=true`, default off),
   the subject contains a receipt keyword. Whitelist still wins (rule 1).
5. otherwise → **unknown** _(stays in Screened)_

Empty/unparseable sender with no list match and no list-headers → unknown.

## Acceptance criteria

- Table-driven unit tests cover every rule and these conflicts:
  - sender on whitelist **and** blocklist → approve (QS3).
  - sender on whitelist **and** carries `List-Id` → approve (whitelist precedes
    the newsletter heuristic).
  - sender only on receipts list → receipt; same sender also whitelisted → approve.
  - wildcard `*@domain.com` matches `x@domain.com` and `x@sub.domain.com`.
  - exact address beats a conflicting wildcard.
- Zero file/network access in the package (enforced by review; no `os`/`net`
  imports in `classify`).
- Same inputs always yield the same verdict (no time, no randomness).

## Out of scope

- Where lists come from (Spec 0002), how messages move (Spec 0003), learning
  (Spec 0004).
- Subject-keyword receipt detection — explicitly excluded (D4).

## Open questions

- Should multiple `From` addresses ever appear? MVP picks the first parseable
  address upstream (imap layer); engine sees a single `Sender`.
