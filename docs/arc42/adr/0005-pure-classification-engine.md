# ADR-0005 — Reine, deterministische Klassifikations-Engine

- Status: accepted
- Datum: 2026-06-17

## Kontext

Die Korrektheit der Klassifikation ist ein Top-Qualitätsziel (Q4: kein
freigegebener Absender landet je im Müll). Logik, die mit IMAP-I/O
vermischt ist, lässt sich schlecht testen.

## Entscheidung

Die Klassifikation (`internal/classify`) ist eine **reine Funktion**
ohne Seiteneffekte:

```
Classify(sender, lists) -> Verdict
```

I/O (IMAP, Persistenz, API) liegt ausschließlich in den umgebenden
Modulen.

## Regeln (deterministisch)

1. **Whitelist** trifft zu → `approve` (hat Vorrang vor Blocklist).
2. **Blocklist** trifft zu → `block`.
3. **Newsletter-Liste** trifft zu → `newsletter`.
4. **Receipts-Liste** trifft zu → `receipt`.
5. sonst → `unknown` (bleibt im Screener).

Matching: exakte Adresse vor optionaler Domain-Regel.

## Begründung

- Voll unit-testbar (Tabellen-Tests), inkl. Konfliktfälle (QS3).
- Verhalten unabhängig von Netzwerk/Server reproduzierbar.

## Konsequenzen

- Engine bekommt eine `Lists`-Momentaufnahme injiziert, liest nicht
  selbst von der Platte.
- Neue Verdict-Typen erfordern nur Erweiterung der Engine + Tests.
