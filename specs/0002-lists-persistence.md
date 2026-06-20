# Spec 0002 — Lists, Persistence & Seeding

Status: draft · Phase: Spec · Refs: arc42 §8.4, decisions D2, D3, D5

## Purpose

Own the four sender lists, persist them atomically in SQLite, seed them from the
legacy `imapfilter` `.txt` files, and offer (but never auto-apply) wildcard
compaction suggestions.

## Data model (SQLite, `/state/screener.db`)

```sql
CREATE TABLE lists (
  id   INTEGER PRIMARY KEY,
  kind TEXT UNIQUE NOT NULL          -- whitelist|blocklist|newsletter|receipts|group_allowlist
);

CREATE TABLE entries (
  id          INTEGER PRIMARY KEY,
  list_id     INTEGER NOT NULL REFERENCES lists(id),
  value       TEXT NOT NULL,         -- lowercased addr, or "*@domain"
  is_wildcard INTEGER NOT NULL,      -- 0 exact, 1 "*@domain"
  source      TEXT NOT NULL,         -- seed|training|api|suggest
  created_at  TEXT NOT NULL,
  UNIQUE(list_id, value)
);

CREATE TABLE counters     (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
CREATE TABLE screened_ids (mid TEXT PRIMARY KEY, seen_at TEXT NOT NULL);
```

`screened_ids` is **not a sender list** — it is the transient index of
Message-IDs seen while a mail sat in `Screened/`, used to detect a later approve
(Spec 0004). Production stores it as `<mid>` + `date:` header blocks and compacts
entries older than **72 h** (`state.lua`, feature G2). v2 keeps that bound:
`seen_at` TTL, rows older than `SCREENED_ID_TTL` (default 72h) are pruned each
sweep. The table never feeds the classify engine.

MVP entry types: **exact** + **`*@domain` wildcard** only. Regex/substring
patterns are deferred to Product (D5).

## Operations

```go
Add(kind, value, source) error      // upsert, idempotent
Remove(kind, value) error
Contains(kind, addr) bool            // exact first, then *@domain, then parent .domain
Snapshot() classify.Snapshot         // in-memory matcher set for the engine
```

- All writes go through a single SQLite transaction; the daemon serializes via
  the DB (WAL mode), so writes are atomic and survive restart (QS4).
- `Snapshot()` is cheap and called per classification; lists are small, kept in
  memory and refreshed on change.

## Seeding / migration (D3)

One-time, **idempotent** importer (`screenerd migrate --from <dir>` or auto on
first run if DB has no entries):

- Reads `whitelist.txt`, `blocklist.txt`, `newsletterlist.txt`,
  `receiptlist.txt`, `snoozed_map.txt` (Spec 0007), and the legacy group
  exceptions into `group_allowlist` (Spec 0008).
- `screened_ids.txt` is **not** imported as a sender list (K3): it is transient
  approve-tracking and its legacy format is `<mid>` + `date:` blocks. Skip it on
  seeding; the daemon repopulates `screened_ids` live from `Screened/`.
- Format: one entry per line; `#` comment; blank lines skipped; trimmed;
  lowercased. `*@domain` recognized as wildcard.
- Each entry inserted with `source = "seed"`. Re-running imports nothing new
  (UNIQUE upsert).
- **Flat / lossless** — no automatic collapsing (D5).

### Cleaning (required — real NAS data is dirty)

The running daemon appends without file-level dedup and its parser leaked mail
**header continuation lines** into the lists. Observed real data:

| list | raw | unique | junk lines |
|------|-----|--------|------------|
| newsletter | 199,378 | 339 | ~36 |
| receipt | 21,419 | 219 | ~39 |
| block | 14,791 | 1,362 | ~36 |
| whitelist | 224 | 224 | 0 |

The importer therefore must:

- **Dedup** to unique values (UNIQUE constraint handles it; ~99 % shrink).
- **Validate** each value as an email or `*@domain`; drop anything that fails
  (e.g. `mime-version: 1.0`, `subject: …`, `x-clientproxiedby: …`).
- **Drop the user's own address** (`ICLOUD_USER` and observed `alex@damhuis.me`)
  — leaked via `to:` header lines; must never enter any list.
- Log a per-list summary: read / kept / deduped / dropped-junk.

NOTE: the repo snapshot's `.txt` are empty; the cleaning rules above are derived
from the real NAS export (download_2026-06-19).

## Compaction suggestions (D5)

Separate, read-only analysis — never mutates lists on its own:

```go
type Suggestion struct {
    Kind    string   // which list
    Wildcard string  // "*@domain.com"
    Covers  []string // exact entries it would replace
}
Suggest(minCluster int) []Suggestion
```

- Groups exact entries by domain; emits a `*@domain` suggestion when a
  **non-freemail** domain has `>= minCluster` exact entries (default N=5).
- **Freemail / relay domains are never suggested** (built-in denylist:
  gmail.com, googlemail.com, icloud.com, me.com, mac.com,
  **privaterelay.appleid.com** (Hide-My-Mail relay — collapsing it would block
  every relayed sender), outlook.com, hotmail.com, live.com, yahoo.*, gmx.*,
  web.de, t-online.de, proton.me, …). The real NAS newsletter list had
  `icloud.com` (19) and `privaterelay.appleid.com` (12) as top "domains" — both
  must be excluded.
- Applying a suggestion is an explicit action (API/CLI) that inserts the
  wildcard (`source="suggest"`) and removes the covered exacts, in one tx.

## Acceptance criteria

- Re-running migrate twice yields identical DB state (idempotent).
- Wildcard `*@d.com` + exact `a@d.com` both present → `Contains` works, exact
  match still wins where it matters (classify precedence handled in 0001).
- Kill -9 mid-write leaves DB consistent (WAL/transaction; no half-written list).
- `Suggest` never returns a freemail domain, even with 100 entries.
- Applying a suggestion is reversible from a DB backup; the op is logged.

## Out of scope

- Verdict logic (0001). REST exposure of these ops (0005).
