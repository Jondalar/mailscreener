# Mail Screener — Architecture Documentation

This documentation describes the architecture of a self-hosted, "HEY-style"
screener for iCloud Mail — the successor to `imapfilter-icloud-hey-screener`
(Lua/polling).

## Contents

- [`arc42-architecture.md`](./arc42-architecture.md) — full architecture
  following arc42 (chapters 1–12).
- [`adr/`](./adr/) — Architecture Decision Records:
  - [ADR-0001](./adr/0001-go-docker-daemon.md) — Go + Docker for the daemon
  - [ADR-0002](./adr/0002-imap-idle.md) — IMAP IDLE instead of polling
  - [ADR-0003](./adr/0003-apple-mail-integration.md) — Mac client: menu-bar app + built-in web UI
  - [ADR-0004](./adr/0004-no-private-apis.md) — No private APIs
  - [ADR-0005](./adr/0005-pure-classification-engine.md) — Pure classification engine

## Overview

```
ScreenerBar (SwiftUI menu-bar app)  ┐
Built-in browser web UI             ├─ thin REST clients
                                     ┘
        │ HTTPS (Bearer token) over LAN
        ▼
screenerd (Go daemon, Docker, IMAP IDLE)  ── QNAP NAS / Linux
        │ IMAPS
        ▼
iCloud Mail
```

`screenerd` screens the INBOX itself: unknown senders are moved to the
`Screened/` folder, so no iCloud server-side rule is required. The Mac side is
purely optional, thin clients over the REST API — there is no Apple Mail
integration (no Accessibility overlay, no MailKit extension; see ADR-0003).

## Status

In production. The daemon runs on a QNAP NAS via Docker (host port 18443 →
container 8443). License: MIT.
