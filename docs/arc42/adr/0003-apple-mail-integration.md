# ADR-0003 — Apple-Mail-Integration: MailKit + AX-Overlay-Companion-App

- Status: accepted
- Datum: 2026-06-17

## Kontext

Gewünscht sind echte Buttons in Apple Mail für „Approve/Block/
Newsletter/Receipt" sowie eine Listenverwaltung. Apple bietet keine
API, um Buttons in Mails eigene Toolbar einzufügen (siehe C3/C4,
ADR-0004). Referenz Mailbutler löst das über eine angedockte Sidebar +
MailKit, nicht über echte Toolbar-Buttons.

## Entscheidung

Die Integration besteht aus drei unterstützten Bausteinen:

1. **MailKit-Extension** (`MEMessageActionHandler`) für automatisches
   Screening neuer **INBOX**-Mails.
2. **Companion-App** (`ScreenerBar`) mit Menüleisten-UI zur
   Listenverwaltung **und** einem **Accessibility-Overlay**, das ein
   eigenes, transparentes Button-Fenster über Mails Toolbar
   positioniert (sieht aus wie native Buttons).
3. **AppleScript/JXA** für die eigentliche Aktion auf der in Mail
   selektierten Nachricht (lesen des Absenders, Verschieben), plus
   API-Call an `screenerd`.

Zusätzlich bleibt **Ordner-Move-Training** als robuster, plattform-
übergreifender Pfad (funktioniert auch auf dem iPhone).

## Begründung

- Kommt „echten Buttons" optisch sehr nahe, **ohne** SIP/AMFI-Eingriff.
- Nutzt ausschließlich unterstützte APIs → update-stabil (Q3).
- Mit Developer-ID signier-/notarisierbar, kein App Store nötig (C6).

## Konsequenzen

- App braucht Accessibility- und Automation-Berechtigungen (vom Nutzer
  einmalig erteilt).
- Overlay-Anker müssen robust über die AX-Hierarchie gefunden werden,
  nicht über feste Pixelkoordinaten.
- Fallback auf reines Ordner-Move-Training, falls AppleScript/AX
  künftig eingeschränkt wird.
