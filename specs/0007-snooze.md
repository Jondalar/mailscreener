# Spec 0007 — Snooze

Status: draft · Phase: MVP · Refs: production `snoozebox.lua` (not in arc42),
decision D6=MVP

## Purpose

Let the user park a message and have it return to `INBOX` (as unread) at a
computed time. Feature parity with the running daemon's Snoozebox, on a cleaner
per-message SQLite model.

## Production mechanism (analyzed from `snoozebox.lua`)

- User drops a message into a `Snoozed/<label>` **subfolder**.
- `scan_subfolders()`: finds `Snoozed/*` subfolders, computes a wake timestamp
  from the **label**, tags each message with an IMAP keyword flag
  `SNOOZED_<LABEL>`, moves it up into the parent `Snoozed` folder, and records
  `map["Snoozed/<label>"] = wake_ts` in `snoozed_map.txt`.
- `wakeup_due()`: for each map entry with `ts <= now`, finds parent-`Snoozed`
  messages carrying that keyword flag, moves them back to `INBOX`, clears
  `\Seen` (best-effort, several binding fallbacks).
- State is split: **per-message** in the IMAP keyword flag, **per-label** wake
  time in the map file. All messages under one label wake together; the wake
  time is set once on first scan and not updated.

### Label grammar (`compute_ts_for_label`)

| Label form | Meaning | Example |
|------------|---------|---------|
| `Nd HH` (`<days>d<hour>`) | in N days at hour HH | `1d10` = tomorrow 10:00, `0d18` = today 18:00 |
| `Ns` | now + N seconds | `60s` |
| `Nh` | now + N hours | `2h` |
| weekday `mo/di/mi/do/fr/sa/so` or `mon/tue/…` (+ optional hour) | next that weekday | `mo`, `fri18` |
| fixed | `1d10`/`1d` (tomorrow 10:00), `sat10`/`sat` (next Sat 10:00), `1w`, `2w`, `1m`, `3m` | |

Default folder set `LABELS = { 1d10, sat10, 1w, 2w, 1m, 3m }`. Times round to the
given hour (default 10:00); month math is calendar-correct (clamps day-of-month).

## v2 model (cleaner — per-message in SQLite)

v2 keeps the **same label grammar and folder UX** but stores wake state
**per message** in SQLite (Spec 0002), not per-label in a flat map — SQLite
removes the need for the IMAP-keyword-flag workaround.

- On IDLE/sweep, a message found in `Snoozed/<label>`: compute
  `wake = compute(label, now)`, upsert `{folder, uid, mid, wake_at}`, move it to
  the parent `Snoozed` folder (keep a flag too, for robustness across UID
  changes).
- On each sweep, any row with `wake_at <= now`: move the message back to INBOX,
  clear `\Seen`, delete the row. Idempotent over UID/MID.

```sql
CREATE TABLE snoozed (
  uid     INTEGER NOT NULL,
  folder  TEXT NOT NULL,    -- "Snoozed/<label>" it came from
  label   TEXT NOT NULL,
  mid     TEXT,
  wake_at TEXT NOT NULL,
  PRIMARY KEY (folder, uid)
);
```

## Seeding / migration

- Import `snoozed_map.txt` (`<folder> <wake_unix_ts>`) best-effort: preserve the
  recorded wake times so nothing currently snoozed wakes early. Because legacy
  state is per-label, migrated rows may share a wake time until re-scanned.
- Do not fail migration on a malformed line; log and skip.

Implemented (`AdoptLegacySnoozes`, runs once, counter-guarded): on first connect
the daemon lists the parent `Snoozed` folder, reads each message's
`SNOOZED_<LABEL>` keyword flag (the old daemon's per-message marker), looks up
the wake time from `snoozed_map.txt` (falling back to recomputing from the
label), and writes a per-message `snoozed` row so legacy parked mail wakes
correctly. v2 mail without the keyword is ignored.

## Acceptance criteria

- `1d10` snooze wakes the message in INBOX tomorrow ~10:00, unread, exactly once.
- `2h`, `60s`, `fri18`, `1m` all compute correct wake times (table tests on the
  label parser — pure, no IMAP).
- Restart before wake → message still wakes (SQLite, QS4).
- Re-running a sweep does not wake an already-woken message twice.
- Malformed `snoozed_map.txt` line skipped with a log line.

## Out of scope

- Per-message snooze via REST, snooze-duration UI (Mac app, Product).

## Open questions

- Confirm the exact label/folder set the user wants to keep (`1d10 sat10 1w 2w
  1m 3m` + ad-hoc `Nd HH`/`Nh`/`Ns`/weekday?).
- v2 drops the per-label-shared-wake quirk in favor of true per-message wake —
  confirm that's desired (it is a behavior improvement, not 1:1 parity).
