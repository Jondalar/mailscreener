# Spec 0004 — Training & Maintenance

Status: draft · Phase: Spec · Refs: arc42 §6.2, preserve-from-imapfilter

## Purpose

Learn from the user's manual folder moves (additive list updates) and keep the
auxiliary folders tidy — the two behaviors that make the screener self-training.

## Training (folder move → list update, §6.2)

On sweep/IDLE, reconcile the training folders. A message a user dragged into a
folder teaches the corresponding list:

| User moves message into | Effect |
|-------------------------|--------|
| `INBOX` (from `Screened/`, Message-ID in `screened_ids`) | sender → **whitelist** **and removed from blocklist**; drop the Message-ID |
| `Junk` | sender → **blocklist** (resync); drop any `screened_ids` entry |
| `Newsletters` | sender → **newsletter** list |
| `Receipts` | sender → **receipts** list |

Rules:
- **Mostly additive**, with one deliberate removal: **approve also removes the
  sender from the blocklist** (ports production `screener.lua:349` — approving a
  previously-blocked sender un-blocks them). This is the only auto-removal in
  MVP; other reclassification is explicit via API.
- Idempotent: a sender already on the target list is a no-op.
- Source recorded as `training` (Spec 0002).
- Approve detection uses `screened_ids`: a message now in INBOX whose
  Message-ID was recorded while it sat in `Screened/` means the user approved it.

## Maintenance / retention (preserve from imapfilter)

Run on a daily tick (or each sweep, guarded by age):

- `Junk`: mark unseen as seen; messages older than `JUNK_RETENTION` (default
  7 days) → `Deleted Messages`.
- `Receipts`: messages older than `RECEIPT_RETENTION` (default 30 days) →
  `Archive`.
- `Newsletters`: mark unseen as seen; messages older than `NEWSLETTER_RETENTION`
  (default 30 days) → `Archive`.

All retention windows are config-driven and may be disabled (0 = off).

## Acceptance criteria

- Moving a `Screened/` mail to INBOX adds its sender to whitelist exactly once
  and clears its `screened_ids` row.
- Moving a mail to Junk adds the sender to blocklist; a second identical move is
  a no-op.
- A Junk message aged past retention is moved to Deleted Messages; a fresh one
  is not.
- Disabling retention (window=0) leaves folders untouched.

## Out of scope

- Verdict logic (0001), connection handling (0003), API surface (0005).

## Open questions

- Conflict: user moves a whitelisted sender's mail to Junk. MVP = additive on
  the block side, so the sender ends on both lists; classify keeps approve
  (whitelist wins, K2). Symmetric to approve-un-blocks (K1), should a Junk move
  also *remove* from whitelist? Production does not; MVP keeps production
  behavior (no whitelist removal on Junk). Flag if the user wants symmetry.
