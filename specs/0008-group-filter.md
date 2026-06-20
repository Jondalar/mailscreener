# Spec 0008 — Mailing-List / Google-Group Filter

Status: draft · Phase: MVP · Refs: production `screener.lua:apply_groups_00A`
(not in arc42), decision G1=MVP generalized

## Purpose

Route mailing-list / Google-Group traffic to Junk by default, with an explicit
**allowlist** for the groups the user actually wants. Replaces the production
rule's two hardcoded exceptions with a real, configurable list.

## Production behavior (ported)

`apply_groups_00A` runs on INBOX and SCREENED: a message is a "group" message if

- header `X-Google-Group-Id` is present, **OR**
- `List-Id` is present **AND** (`List-Post` **OR** `List-Help`) is present.

Group messages → `Junk`, **except** a hardcoded `GROUP_WHITELIST`
(`drpong@googlegroups.com`, `haus-k@googlegroups.com`).

## v2 model (generalized)

A new list kind **`group_allowlist`** (Spec 0002, same SQLite/REST machinery):

- Detection heuristic = same headers (deterministic → lives in the classify
  layer as a pre-check, or as a dedicated filter stage before list sorting).
- If a message matches the group heuristic:
  - sender / `List-Id` on `group_allowlist` → **not** filtered; continues through
    normal classification (whitelist etc. still apply).
  - otherwise → **block** (move to Junk).
- The two legacy exceptions are seeded into `group_allowlist` on migration.

Precedence: an explicit **whitelist** entry still wins over the group filter
(Q4 — a wanted sender is never junked), mirroring K2.

Implemented: the quick/sort path junks new group mail via `Classify`; the full
sweep additionally runs a full INBOX scan (`groupSweepInbox`) so already-read
group mail is moved to Junk too, porting the production `apply_groups_00A`.

## REST (extends Spec 0005)

- `group_allowlist` is exposed like the other lists: `GET/POST/DELETE
  /lists/group_allowlist`.

## Acceptance criteria

- A Google-Group message (carries `X-Google-Group-Id`) from a sender not on any
  allowlist → Junk.
- Same message from a `group_allowlist` (or whitelist) sender → not junked.
- `List-Id` + `List-Post` present, no allowlist → Junk; `List-Id` alone (no
  List-Post/List-Help) → not treated as a group (falls through to newsletter
  heuristic, Spec 0001).
- Legacy `drpong@`/`haus-k@googlegroups.com` are present in `group_allowlist`
  after migration.

## Open questions

- Should the group filter emit verdict `block` or a distinct `group` verdict?
  MVP: reuse `block` (→ Junk) to keep the verdict set small; revisit if the user
  wants groups in a separate folder.
