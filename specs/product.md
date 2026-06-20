# Product — Full arc42 Target

Status: draft · Refs: arc42 §5.3/§6.3/§7, ADR-0003/0004

Everything beyond the MVP that realizes the full Mail Screener vision. Built on
the same daemon truth; the Mac side stays a thin client.

## Mac integration (ADR-0003, no SIP/AMFI — ADR-0004)

- **ScreenerKit** — shared Swift lib: typed REST client (DTOs, Bearer auth),
  domain models.
- **ScreenerMailExt** — MailKit `MEMessageActionHandler` auto-screening new
  INBOX mail (complements the daemon; C4: sees INBOX only).
- **ScreenerBar** — SwiftUI menu-bar companion:
  - list management UI + stats over the REST API,
  - **AX overlay**: transparent button window anchored over Mail's toolbar via
    `AXObserver` (Approve/Block/Newsletter/Receipt),
  - **AppleScript/JXA** actions on the selected message (read sender, move) plus
    a parallel API call so future mail is sorted server-side (§6.3).
  - Soft-fail: local AppleScript move works even if the daemon is briefly
    offline; resync on reconnect (QS5).

## Classification enhancements

- Regex / substring patterns as a first-class entry type (beyond exact +
  `*@domain`).
- Optional auto-remove on explicit reclassification (resolve Spec 0004 open
  question).

## Hardening & ops

- In-process TLS option (not only reverse-proxy).
- Backup/restore of `/state/screener.db`; export back to `.txt` for portability.
- Metrics endpoint; richer `/status` history.
- Multi-account support (arc42 currently single iCloud account).

## Build / release (arc42 §7.2)

- Mac: Xcode build → `codesign` (Developer ID) → `notarytool` → `stapler`.
- Daemon: tagged releases, multi-arch images (amd64/arm64).

## Quality targets carried from arc42

Q1 reactivity, Q2 data sovereignty, Q3 macOS-update robustness, Q4
classification correctness, Q5 maintainability — see `../docs/arc42`.
