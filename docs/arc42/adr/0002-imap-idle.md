# ADR-0002 — IMAP IDLE instead of polling

- Status: accepted
- Date: 2026-06-17

## Context

The old setup polls every 600 s. Quality goal Q1 requires that new mail be
classified in under 5 s.

## Decision

`screenerd` holds persistent **IMAP IDLE connections** (RFC 2177) and reacts to
server push. It runs **two IMAP connections**: a worker connection (fetch/move
plus IDLE on the `Screened/` folder) and a dedicated INBOX IDLE watcher. Only
the worker connection ever moves mail, so moves are never concurrent.

## Rationale

- Push instead of poll → low latency, less connection load.
- iCloud supports IDLE.
- A dedicated INBOX idler lets the daemon screen new INBOX mail immediately,
  with no iCloud server-side rule required.

## Consequences

- IDLE must be renewed before the server timeout (~29 min).
- Reconnect with exponential backoff on connection drops.
- A periodic **sweep** acts as a safety net in case an IDLE event is lost. The
  sweep has two tiers, both idempotent via UID:
  - **QuickSweep** — cheap, incremental; only processes messages above a
    per-folder UID watermark (stored in the `folder_state` table). Runs on IDLE
    triggers and completes in well under 5 s.
  - **FullSweep** — full rescan with retroactive re-sort, training,
    maintenance/retention, and snooze handling. Runs on the periodic ticker and
    on reconnect.
