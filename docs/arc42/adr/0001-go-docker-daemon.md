# ADR-0001 — Go + Docker für den Daemon

- Status: accepted
- Datum: 2026-06-17

## Kontext

Das bestehende Setup nutzt imapfilter (Lua) in einem 600-s-Polling-Loop.
Gesucht ist ein robuster, lange laufender Daemon, der auf NAS/Linux per
Docker betrieben wird, klein und wartbar ist und IMAP IDLE beherrscht.

## Entscheidung

Wir implementieren den Daemon (`screenerd`) in **Go** und verteilen ihn
als **Docker-Image** (Multi-Stage-Build → minimaler Base wie
`distroless`/`scratch`).

## Begründung

- Statisches Binary → winziges, reproduzierbares Image, einfache Deployments.
- Ausgereifte IMAP-Bibliotheken inkl. IDLE (z. B. `go-imap`).
- Starke Standardbibliothek für HTTP-API, gutes Nebenläufigkeitsmodell
  für eine dauerhafte IDLE-Verbindung + API-Server.
- Einfaches Cross-Compiling für NAS-Architekturen (amd64/arm64).

## Alternativen

- **Rust**: maximale Robustheit, aber langsamere Iteration.
- **Python**: nah am Skript-Ursprung, aber größeres Image, mehr
  Laufzeitabhängigkeiten.
- **Bei Lua/imapfilter bleiben**: kein IDLE, schwer testbar, begrenzte API.

## Konsequenzen

- Go-Toolchain + Standard-Projektlayout (`cmd/`, `internal/`).
- CI für Build/Test/Lint und Image-Build nötig.
