# ADR-0002 — IMAP IDLE statt Polling

- Status: accepted
- Datum: 2026-06-17

## Kontext

Das alte Setup pollt alle 600 s. Qualitätsziel Q1 verlangt
Klassifikation neuer Mail in < 5 s.

## Entscheidung

`screenerd` hält eine **IMAP-IDLE-Verbindung** (RFC 2177) auf den
`Screened/`-Ordner und reagiert auf Server-Push.

## Begründung

- Push statt Poll → niedrige Latenz, weniger Verbindungslast.
- iCloud unterstützt IDLE.

## Konsequenzen

- IDLE muss vor dem Server-Timeout (~29 min) erneuert werden.
- Reconnect mit Exponential Backoff bei Verbindungsabbruch.
- Zusätzlich ein periodischer „Sweep" als Sicherheitsnetz, falls ein
  IDLE-Event verloren geht (idempotent über UID).
