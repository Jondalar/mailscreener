# Mail Screener — Architekturdokumentation

Diese Dokumentation beschreibt die Zielarchitektur für einen
selbst-gehosteten "HEY-Style"-Screener für iCloud Mail — als Nachfolger
des `imapfilter-icloud-hey-screener` (Lua/Polling).

## Inhalt

- [`arc42-architecture.md`](./arc42-architecture.md) — vollständige
  Architektur nach arc42 (Kap. 1–12).
- [`adr/`](./adr/) — Architecture Decision Records:
  - [ADR-0001](./adr/0001-go-docker-daemon.md) — Go + Docker für den Daemon
  - [ADR-0002](./adr/0002-imap-idle.md) — IMAP IDLE statt Polling
  - [ADR-0003](./adr/0003-apple-mail-integration.md) — Apple-Mail-Integration (MailKit + AX-Overlay)
  - [ADR-0004](./adr/0004-no-private-apis.md) — Verzicht auf private APIs
  - [ADR-0005](./adr/0005-pure-classification-engine.md) — Reine Klassifikations-Engine

## Kurzüberblick

```
ScreenerBar (Menüleiste + AX-Overlay-Buttons)  ┐
ScreenerMailExt (MailKit Auto-Screening INBOX)  ├─ Mac (macOS 14+)
Apple Mail                                       ┘
        │ HTTPS / AppleScript / Accessibility
        ▼
screenerd (Go-Daemon, Docker, IMAP IDLE)  ── NAS/Linux
        │ IMAPS
        ▼
iCloud Mail
```

## Status

Draft / v0.1 — Architektur steht, Implementierung ausstehend.
Lizenz: MIT.
