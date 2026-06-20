# Mail Screener — Architecture (arc42)

> Self-hosted "HEY-style" Screener für iCloud Mail: ein nativer Daemon
> (Go, Docker, IMAP IDLE) als Kern, plus eine Apple-Mail-Integration
> (MailKit-Extension + angedockte Companion-App mit echten Buttons via
> Accessibility-Overlay).
>
> Diese Dokumentation folgt der [arc42](https://arc42.org)-Vorlage.
> Status: **Draft / v0.1** · Letzte Änderung: 2026-06-17

---

## 1. Einführung und Ziele

### 1.1 Aufgabenstellung

Eingehende iCloud-Mail soll wie bei [HEY.com](https://hey.com) oder
SaneBox automatisch in einen **Screener** laufen: unbekannte Absender
landen nicht direkt im Posteingang, sondern werden geprüft. Bekannte/
freigegebene Absender kommen in die INBOX, Blockierte in den Müll,
Newsletter und Belege in eigene Ordner. Das Training erfolgt natürlich
durch Verschieben von Mails — und über echte Buttons direkt am
Mail-Fenster.

Das bestehende Projekt (`imapfilter-icloud-hey-screener`, Lua +
imapfilter, 600 s Polling) wird durch einen echten Daemon und eine
native Mac-Integration ersetzt.

### 1.2 Qualitätsziele (Top 5)

| # | Qualitätsziel | Szenario |
|---|---------------|----------|
| Q1 | **Reaktivität** | Neue Mail im Screened-Ordner wird in < 5 s klassifiziert (IMAP IDLE statt Polling). |
| Q2 | **Datenhoheit** | Mailinhalte verlassen nie das eigene System; keine Cloud-Dritte, keine Telemetrie. |
| Q3 | **Robustheit gegen macOS-Updates** | Die Mac-Integration überlebt macOS-Point-Releases ohne SIP/AMFI-Eingriffe. |
| Q4 | **Korrektheit der Klassifikation** | Kein freigegebener Absender landet je im Müll (deterministische Listen-Regeln, testbar). |
| Q5 | **Wartbarkeit** | Klare Modulgrenzen, statisches Binary, reproduzierbares Docker-Image, getestete Klassifikationslogik. |

### 1.3 Stakeholder

| Rolle | Erwartung |
|-------|-----------|
| Betreiber/Nutzer (Du) | Läuft auf NAS/Linux-Host, einfache Bedienung in Apple Mail, kein Cloud-Lock-in. |
| Entwickler | Lesbarer Go-Code, testbare Klassifikation, klare API zwischen Daemon und Mac-App. |
| Apple Mail | Bleibt unangetastet; Integration nur über unterstützte APIs (MailKit, Accessibility, AppleScript). |

---

## 2. Randbedingungen

### 2.1 Technische Randbedingungen

| # | Randbedingung | Konsequenz |
|---|---------------|------------|
| C1 | iCloud Mail bietet **nur IMAP/SMTP**, kein offizielles API. | Klassifikation über IMAP-Ordnerbewegungen; App-spezifisches Passwort nötig. |
| C2 | iCloud erlaubt **app-spezifische Passwörter**, kein OAuth für IMAP. | Secret-Handling im Daemon; kein Token-Refresh. |
| C3 | Apple hat den **Legacy-Mail-Plugin-Loader seit macOS Sonoma (14) entfernt**. | Keine echten Toolbar-Buttons via Bundle-Injection mehr ohne SIP+AMFI-Deaktivierung → bewusst ausgeschlossen (siehe ADR-0004). |
| C4 | **MailKit** liefert nur Handler (Message-Action, Compose, Security, Content-Blocker), **keine UI in Mails Chrome**, und `MEMessageActionHandler` sieht **nur INBOX-Mails**. | Auto-Screening im INBOX via MailKit; Buttons via separater App/Overlay. |
| C5 | Deployment des Daemons als **Docker-Container** auf NAS/Linux. | 12-Factor-Konfiguration, persistenter Volume-State. |
| C6 | Verteilung der Mac-App **außerhalb des App Store** (vorhandener Apple Developer Account). | **Developer-ID-Signatur + Notarisierung**, kein Sandboxing-Zwang des App Store. |

### 2.2 Organisatorische Randbedingungen

- **Lizenz:** MIT, Open Source, kein kommerzieller Vertrieb.
- **Nutzung:** persönlich, „at own risk".
- **Sprache Daemon:** Go (Entscheidung des Betreibers).

### 2.3 Konventionen

- arc42 für Architektur, ADRs (MADR-Format) unter `docs/arc42/adr/`.
- Go: Standard-Layout (`cmd/`, `internal/`), `gofmt`/`golangci-lint`.
- Swift: SwiftUI, Swift Concurrency.

---

## 3. Kontextabgrenzung

### 3.1 Fachlicher Kontext

```
                 ┌───────────────────────────────────────────┐
                 │                  Nutzer                     │
                 └───────────────┬───────────────┬────────────┘
                                 │               │
                  verschiebt Mail / klickt Button │ verwaltet Listen
                                 │               │
        ┌────────────────────────▼───┐   ┌───────▼─────────────────┐
        │      Apple Mail (Mac)       │   │  Companion-App (Mac)    │
        │  + MailKit-Extension        │   │  Menüleiste + Overlay   │
        └────────────┬───────────────┘   └───────────┬─────────────┘
                     │ IMAP (Ordner)                  │ HTTPS (REST)
                     │                                │
        ┌────────────▼────────────────────────────────▼─────────────┐
        │                    screenerd (Go-Daemon, Docker)           │
        │   IMAP IDLE · Klassifikation · Listen · Training · API     │
        └────────────┬───────────────────────────────────────────────┘
                     │ IMAP/IMAPS
            ┌────────▼─────────┐
            │  iCloud Mail     │
            │  (imap.mail.me.com)│
            └──────────────────┘
```

| Externe Schnittstelle | Richtung | Inhalt |
|-----------------------|----------|--------|
| iCloud IMAP | ↔ screenerd | Mails lesen, Flags setzen, zwischen Ordnern verschieben |
| Nutzer ↔ Apple Mail | ↔ | Mails lesen, in Trainingsordner ziehen, Overlay-Buttons klicken |
| Nutzer ↔ Companion-App | ↔ | Listen ansehen/bearbeiten, Statistik, Aktionen |

### 3.2 Technischer Kontext

| Kanal | Protokoll | Beschreibung |
|-------|-----------|--------------|
| screenerd ↔ iCloud | IMAPS (993) | IDLE-Verbindung auf `Screened/`, Move/Copy/Flag |
| Companion-App ↔ screenerd | HTTPS/REST (oder gRPC) | Listen-CRUD, Status, Aktion „klassifiziere Mail X als Y" |
| MailKit-Extension ↔ Apple Mail | MailKit IPC | `MEMessageActionHandler` für INBOX-Auto-Screening |
| Companion-App ↔ Apple Mail | Accessibility (AXUIElement) + AppleScript/JXA | Overlay-Position, selektierte Mail lesen/verschieben |

---

## 4. Lösungsstrategie

| Problem | Strategie | Begründung / ADR |
|---------|-----------|------------------|
| Latenz des alten Polling-Ansatzes | **IMAP IDLE** im Daemon | Push statt Poll → < 5 s (Q1). ADR-0002 |
| Sprache & Deployment | **Go + statisches Binary + Docker** | Kleines Image, robuster Daemon (Q5, C5). ADR-0001 |
| Echte Buttons in Apple Mail | **AX-Overlay-Companion-App + AppleScript-Aktionen + MailKit-Auto-Screening** | Keine SIP/AMFI-Eingriffe, update-stabil (Q3, C3/C4). ADR-0003, ADR-0004 |
| Korrektheit & Testbarkeit | **Reine, deterministische Klassifikations-Engine** ohne I/O | Unit-testbar (Q4). ADR-0005 |
| Trennung Logik/Transport | **Daemon hält Wahrheit + API**, Mac-Apps sind dünne Clients | Eine Quelle der Wahrheit, mehrere Frontends. |

---

## 5. Bausteinsicht

### 5.1 Whitebox Gesamtsystem (Ebene 1)

```
Mail Screener
├── screenerd            (Go-Daemon, Docker)        ── Kern
├── ScreenerKit          (Swift-Lib, API-Client)    ── geteilt von Mac-Komponenten
├── ScreenerMailExt      (MailKit App-Extension)     ── Auto-Screening INBOX
└── ScreenerBar          (SwiftUI Companion-App)     ── Menüleiste + AX-Overlay-Buttons
```

| Baustein | Verantwortung |
|----------|---------------|
| **screenerd** | IMAP-Anbindung, Klassifikation, Listen-Persistenz, Training, REST-API. Einzige Quelle der Wahrheit. |
| **ScreenerKit** | Gemeinsamer Swift-Code: API-Client (DTOs, Auth), Domänenmodelle. Von Extension und App genutzt. |
| **ScreenerMailExt** | MailKit-Extension; klassifiziert neue INBOX-Mails über `MEMessageActionHandler` (verschieben/flaggen). |
| **ScreenerBar** | Companion-App: Menüleisten-UI zur Listenverwaltung + AX-Overlay mit Buttons über Mails Toolbar; ruft AppleScript für Aktionen auf selektierter Mail. |

### 5.2 Whitebox `screenerd` (Ebene 2)

```
screenerd
├── cmd/screenerd/main.go        Bootstrap, Konfig, Lifecycle
├── internal/imap/               IDLE-Loop, Move/Flag, Reconnect/Backoff
├── internal/classify/           REINE Engine: Sender → Verdict (kein I/O)
├── internal/lists/              Whitelist/Blocklist/Newsletter/Receipts + Persistenz
├── internal/training/           Ordner-Move → Listen-Update (lernen)
├── internal/api/                REST-Handler (Listen-CRUD, Status, Aktion)
└── internal/config/             ENV/Datei-Konfig (12-Factor)
```

| Modul | Verantwortung | Wichtige Schnittstellen |
|-------|---------------|--------------------------|
| `imap` | Verbindung halten (IDLE), Events liefern, Mails verschieben/flaggen. | `Watch(folder) <-chan Message`, `Move(uid, folder)`, `Flag(uid, flag)` |
| `classify` | Aus Absender + Listen ein **Verdict** ableiten. Deterministisch, ohne Seiteneffekte. | `Classify(sender, lists) Verdict` |
| `lists` | Vier Listen + atomare Persistenz (Datei/SQLite). | `Add/Remove/Contains`, `Snapshot()` |
| `training` | Erkennt manuelle Moves in Trainingsordner → passende Liste pflegen. | `OnMove(uid, fromFolder, toFolder)` |
| `api` | REST: `GET/POST/DELETE /lists/...`, `GET /status`, `POST /classify`. | HTTP/JSON |
| `config` | Konfiguration aus ENV + optionaler Datei. | `Load() Config` |

### 5.3 Whitebox Mac-Seite (Ebene 2)

| Modul | Verantwortung |
|-------|---------------|
| `ScreenerBar/MenuBar` | NSStatusItem + SwiftUI-Popover: Listen, Statistik, Einstellungen. |
| `ScreenerBar/Overlay` | Beobachtet Mail via `AXObserver`; positioniert ein transparentes Fenster mit Buttons über Mails Toolbar. |
| `ScreenerBar/Actions` | Führt „Approve/Block/Newsletter/Receipt" auf der in Mail selektierten Nachricht aus (AppleScript/JXA → Move; parallel API-Call an Daemon). |
| `ScreenerKit/APIClient` | Typsicherer Client gegen `screenerd`. |

---

## 6. Laufzeitsicht

### 6.1 Neue Mail wird gescreent (Daemon-Pfad, Standardfall)

```
iCloud           screenerd(imap)      classify        lists        iCloud
  │  neue Mail in    │                   │              │            │
  │  Screened/  ───► │ IDLE wakeup       │              │            │
  │                  │ fetch sender ───► Classify(s) ──► Contains? ─►│
  │                  │                   │   Verdict     │            │
  │                  │ ◄─────────────────┘               │            │
  │                  │ Move(uid → INBOX|Junk|News|Recpt) ───────────► │
```

1. IDLE meldet neue Nachricht in `Screened/`.
2. `imap` holt Absender (+ ggf. Header).
3. `classify` liefert Verdict aus den Listen.
4. Bei `unknown`: bleibt im Screener (oder „Pending"), Nutzer entscheidet.
5. `imap` verschiebt die Mail in den Zielordner.

### 6.2 Training durch Ordner-Verschieben

```
Nutzer            iCloud           screenerd(imap/training)     lists
  │ zieht Mail in     │                    │                      │
  │ "Approve/" ────►  │ ── IDLE: Move ───► OnMove(from,to)        │
  │                   │                    │  to == Approve ─────► Add(sender→whitelist)
  │                   │ ◄── Move zurück in INBOX ─────────────────┘
```

### 6.3 Button-Klick im Overlay (Companion-App)

```
Nutzer        ScreenerBar(Overlay)   AppleScript/Mail        screenerd(API)
  │ klickt "Block"   │                    │                      │
  │ ───────────────► │ aktuelle Mail? ──► get selected message   │
  │                  │ ◄── sender ────────┘                      │
  │                  │ move message → Junk (AppleScript)         │
  │                  │ POST /lists/block {sender} ─────────────► Add(blocklist)
```

> Hinweis: Die Aktion wirkt sofort lokal (AppleScript-Move) **und**
> meldet den Absender an den Daemon, damit künftige Mails serverseitig
> korrekt landen.

---

## 7. Verteilungssicht

### 7.1 Infrastruktur

```
┌─────────────────────────┐         ┌──────────────────────────────┐
│  NAS / Linux-Host        │         │  Mac (macOS 14+)             │
│  ┌────────────────────┐  │  HTTPS  │  ┌────────────┐ ┌─────────┐  │
│  │ Docker: screenerd  │◄─┼─────────┼──┤ ScreenerBar│ │MailExt  │  │
│  │  Volume: /state    │  │         │  └─────┬──────┘ └────┬────┘  │
│  └─────────┬──────────┘  │         │        │ AX/AppleScript│     │
└────────────┼─────────────┘         │   ┌────▼─────────────▼────┐  │
             │ IMAPS                  │   │      Apple Mail       │  │
       ┌─────▼──────┐                 │   └───────────────────────┘  │
       │ iCloud Mail│                 └──────────────────────────────┘
       └────────────┘
```

| Knoten | Artefakt | Konfiguration |
|--------|----------|---------------|
| NAS/Linux | `screenerd` Container | ENV: `ICLOUD_USER`, `ICLOUD_APP_PASSWORD`, `SCREENED_FOLDER`, `API_TOKEN`, `IDLE_TIMEOUT`. Volume für State. |
| Mac | `ScreenerBar.app` (Developer-ID, notarisiert) | API-URL + Token. Berechtigung: Accessibility, Automation (Mail). |
| Mac | `ScreenerMailExt` (in App eingebettet) | In Mail aktivieren (Einstellungen → Erweiterungen). |

### 7.2 Build/Release

- Daemon: Multi-Stage-Dockerfile (`golang:alpine` → `scratch`/`distroless`), `docker-compose.yml`.
- Mac: Xcode-Build → `codesign` (Developer ID) → `notarytool` → `stapler`.

---

## 8. Querschnittliche Konzepte

### 8.1 Domänenmodell

- **Verdict** = `approve | block | newsletter | receipt | unknown`.
- **List** = Menge von Absender-Identitäten (E-Mail; optional Domain-Regeln).
- **Sender-Matching**: exakte Adresse zuerst, danach optionale Domain-Regel; Whitelist hat Vorrang vor Blocklist (Q4: nie versehentlich blocken).

### 8.2 Konfiguration (12-Factor)

Alles über ENV/Datei; keine Secrets im Image. App-spezifisches iCloud-Passwort und API-Token via Secret/`.env`.

### 8.3 Sicherheit

- API nur lokal/VPN/LAN; **Bearer-Token** Pflicht; TLS empfohlen (Reverse-Proxy).
- Keine Mailinhalte persistieren — nur Absender/Listen.
- Mac-App: minimale Entitlements; Accessibility/Automation nur, was nötig ist.
- **Kein** SIP/AMFI-Eingriff (ADR-0004).

### 8.4 Persistenz

Listen + Zähler atomar speichern (Datei mit temp+rename oder SQLite). Überlebt Container-Neustart (Volume).

### 8.5 Fehlerbehandlung & Resilienz

- IMAP-Reconnect mit Exponential Backoff; IDLE-Renew vor Server-Timeout (~29 min).
- Idempotente Moves (UID-basiert); doppelte Events schaden nicht.
- API-Soft-Fail: Mac-Aktion (AppleScript-Move) wirkt auch, wenn Daemon kurz nicht erreichbar — Resync später.

### 8.6 Logging/Observability

Strukturiertes Log (JSON), `GET /status` (letzte Klassifikationen, Listengrößen, Verbindungszustand).

---

## 9. Architekturentscheidungen

Siehe `docs/arc42/adr/`:

- **ADR-0001** — Go + Docker für den Daemon.
- **ADR-0002** — IMAP IDLE statt Polling.
- **ADR-0003** — Apple-Mail-Integration: MailKit + AX-Overlay-Companion-App.
- **ADR-0004** — Verzicht auf private APIs / Legacy-Bundle-Injection.
- **ADR-0005** — Reine, deterministische Klassifikations-Engine.

---

## 10. Qualitätsanforderungen

### 10.1 Qualitätsbaum (Auszug)

- **Performance** → Reaktivität (Q1), geringer Ressourcenverbrauch.
- **Sicherheit/Datenschutz** → Datenhoheit (Q2), Token-Auth.
- **Wartbarkeit** → Testbarkeit (Q4/Q5), klare Module.
- **Portabilität/Robustheit** → Update-Stabilität der Mac-Integration (Q3).

### 10.2 Qualitätsszenarien

| ID | Szenario | Messgröße |
|----|----------|-----------|
| QS1 | Mail trifft in `Screened/` ein → klassifiziert | < 5 s (p95) |
| QS2 | macOS-Point-Update installiert | Mac-App funktioniert ohne Reinstall/SIP-Eingriff |
| QS3 | Absender steht auf Whitelist **und** Blocklist | landet in INBOX (Whitelist gewinnt), durch Test abgedeckt |
| QS4 | Daemon-Neustart | Listen vollständig erhalten |
| QS5 | Daemon kurz offline, Nutzer klickt „Block" | lokaler Move wirkt; Liste wird bei Reconnect aktualisiert |

---

## 11. Risiken und technische Schulden

| Risiko | Bewertung | Gegenmaßnahme |
|--------|-----------|---------------|
| AppleScript-Unterstützung in Mail wird eingeschränkt | mittel | Aktionen kapseln; Fallback auf reines Ordner-Move-Training (ADR-0003). |
| AX-Overlay verrutscht bei Mail-Layout-Änderung | mittel | Anker robust über AX-Hierarchie suchen, nicht über feste Pixel; Selbsttest beim Start. |
| iCloud drosselt/verändert IMAP-Verhalten | niedrig–mittel | Backoff, konservatives IDLE-Renew, Mehrfach-Login vermeiden. |
| App-spezifisches Passwort wird widerrufen | niedrig | klare Fehlermeldung + Re-Auth-Anleitung. |
| MailKit `MEMessageActionHandler` sieht nur INBOX | bekannt (C4) | Screening primär im Daemon; Extension nur als Ergänzung. |
| Notarisierungs-/Signatur-Aufwand bei jedem Release | niedrig | Build-Skript automatisiert (`notarytool`). |

---

## 12. Glossar

| Begriff | Bedeutung |
|---------|-----------|
| **Screener** | Vorprüfung eingehender Mail unbekannter Absender vor dem Posteingang. |
| **IMAP IDLE** | IMAP-Erweiterung (RFC 2177) für serverseitige Push-Benachrichtigung bei neuen Mails. |
| **MailKit** | Apples seit macOS 12 unterstütztes Framework für Mail-Erweiterungen. |
| **MEMessageActionHandler** | MailKit-Handler, der Regeln auf eingehende INBOX-Mails anwendet. |
| **AX / Accessibility API** | macOS-API (`AXUIElement`) zum Beobachten/Steuern fremder App-UIs. |
| **AX-Overlay** | Eigenes transparentes Fenster, das per AX über fremde UI positioniert wird (hier: Buttons über Mails Toolbar). |
| **SIP / AMFI** | System Integrity Protection / Apple Mobile File Integrity — Schutzmechanismen; hier bewusst nicht deaktiviert. |
| **Developer ID / Notarisierung** | Apple-Signatur + -Prüfung für Verteilung außerhalb des App Store. |
| **Verdict** | Klassifikationsergebnis für einen Absender. |
| **screenerd** | Der Go-Daemon (Kernkomponente). |
