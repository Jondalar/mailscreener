# ADR-0004 — Verzicht auf private APIs / Legacy-Bundle-Injection

- Status: accepted
- Datum: 2026-06-17

## Kontext

Echte Toolbar-Buttons in Apple Mail waren früher nur über Legacy-Mail-
Bundles (Code-Injection in den Mail.app-Prozess) möglich. Seit macOS
Sonoma (14) hat Apple den Plugin-Loader **entfernt**; Mail.app lädt
keine Bundles mehr. Injection auf aktuellem macOS erfordert das
dauerhafte Deaktivieren von **SIP** und **Library Validation/AMFI**
sowie laufendes Reverse-Engineering nach jedem macOS-Update.

Die Nutzung ist privat, MIT, „at own risk" — technisch *möglich* wäre
ein Injection-Weg also denkbar.

## Entscheidung

Wir verzichten bewusst auf private APIs und Bundle-Injection.

## Begründung

- **Systemweite Sicherheitsschwächung** (SIP/AMFI aus) ist
  unverhältnismäßig für ein QoL-Tool.
- **Hohe Fragilität**: bricht bei jedem macOS-Point-Release; ständiges
  Neu-Reversen interner View-Hierarchien.
- **Crash-Risiko im produktiven Mailprogramm.**
- Qualitätsziel Q3 (Update-Stabilität) wäre nicht erreichbar.

Der AX-Overlay-Weg (ADR-0003) liefert ~95 % des „echte Buttons"-
Erlebnisses ohne diese Kosten.

## Konsequenzen

- Keine Buttons in Apples *eigener* Toolbar — die Buttons leben im
  angedockten Overlay-Fenster.
- Diese Entscheidung kann revidiert werden, falls Apple je wieder
  offizielle UI-Erweiterungspunkte für Mail anbietet.
