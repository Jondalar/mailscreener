# Spec 0003 — IMAP IDLE, Sweep & Catch-up

Status: draft · Phase: Spec · Refs: ADR-0002, arc42 §6.1/§8.5, Q1/QS1

## Purpose

Keep a live connection to iCloud, react to new mail in `Screened/` within < 5 s,
move/flag messages, and recover gracefully — without the old 5 s polling loop.

## Behavior

### Connection
- IMAPS to `imap.mail.me.com:993`, app-specific password (C2).
- Hold an **IMAP IDLE** connection on the `Screened/` folder.
- Renew IDLE before the server timeout (~29 min); config `IDLE_TIMEOUT`
  default 1500 s (25 min) for safety margin.
- Reconnect with **exponential backoff** (e.g. 1s→2s→…→cap 5 min) on drop;
  avoid hammering / multiple parallel logins (arc42 risk: iCloud throttling).

### On new-message event (the screening path, §6.1)
1. IDLE wakeup → fetch headers (From/Sender/Return-Path, List-Id,
   List-Unsubscribe, Message-ID) for new UIDs in `Screened/`.
2. Parse a single sender address (From → Sender → Return-Path, first parseable,
   lowercased).
3. `classify.Classify(sender, headers, lists.Snapshot())` → Verdict.
4. Act on verdict:
   - approve → move to `INBOX`
   - block → move to `Junk`
   - newsletter → move to `Newsletters`
   - receipt → move to `Receipts`
   - unknown → leave in `Screened/`; record Message-ID in `screened_ids`
     (so a later approve via INBOX can be detected — see 0004).
5. Moves are **idempotent over UID**; duplicate events cause no harm.

### Two-tier sweeps (implemented)

To avoid rescanning whole folders (the Newsletters folder has thousands of
messages), the daemon runs two tiers:

- **Quick sweep** — fires on an IDLE trigger (new mail). Processes only messages
  whose UID is above a per-folder **watermark** (`folder_state` table) across
  INBOX, Screened, and the training folders. IMAP UIDs are monotonic and a
  moved-in message gets a fresh higher UID, so the watermark catches both new
  arrivals and user-moved mail (training) cheaply. Measured ~4 s (< 5 s, Q1).
- **Full sweep** — fires on the `SWEEP_INTERVAL` ticker (default 10 min) and on
  (re)connect. Rescans everything: retroactive Screened re-sort after list
  changes, training, approve, maintenance. It also bumps the watermarks so the
  next quick sweep stays incremental. Measured ~15 s.

### IDLE topology (implemented)
- **Two connections**: conn1 idles `Screened/` and does all fetch/move work;
  conn2 idles `INBOX`. Both, plus the ticker, feed a single trigger; only conn1
  mutates mail (no concurrent moves).

### INBOX catch-up (preserve from imapfilter — Hide-My-Mail leaks)
- Some mail reaches `INBOX` directly despite the iCloud server rule (e.g.
  Hide-My-Mail). On sweep, scan **unseen INBOX**:
  - blocklist sender → `Junk`
  - whitelist sender → stays in INBOX
  - everything else → record Message-ID, move to `Screened/` for screening.
- Whitelist is checked **before** block so a wanted sender is never demoted.

### Folder bootstrap
- Ensure folders exist/subscribed at startup: `Screened`, `Newsletters`,
  `Receipts`, plus system `Junk`, `Archive`, `Deleted Messages`. "Already
  exists" is fine.

## Acceptance criteria

- New mail dropped into `Screened/` is classified and moved in < 5 s (p95, QS1)
  under normal IDLE.
- Killing the connection mid-session → daemon reconnects with backoff and
  resumes IDLE; no crash, no duplicate moves.
- A message manually placed in INBOX from a blocklisted sender is moved to Junk
  on the next sweep; a whitelisted one is left alone.
- Replaying the same IDLE event twice does not move a message twice or corrupt
  `screened_ids`.

## Out of scope

- Verdict rules (0001), list storage (0002), learning from manual moves (0004),
  maintenance/retention (0004 §maintenance).

## Open questions

- Library choice: resolved — `emersion/go-imap/v2` (IDLE + UID MOVE).
- **UIDVALIDITY**: the watermark assumes stable UIDs. If iCloud changes a
  folder's UIDVALIDITY, watermarks become invalid. MVP ignores this; a later
  increment should store UIDVALIDITY per folder and reset the watermark on
  change (the worst case today is a one-time full rescan, which is safe).
- Further quick-tier trim: scan only INBOX+Screened on quick, leave the training
  folders (Junk/Receipts/Newsletters) to the full tier — cuts quick to ~2 s.
