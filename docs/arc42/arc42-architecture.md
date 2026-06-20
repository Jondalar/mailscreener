# Mail Screener — Architecture (arc42)

> Self-hosted "HEY-style" screener for iCloud Mail: a native daemon
> (Go, Docker, IMAP IDLE) as the core, managed by a built-in browser UI and a
> macOS menu-bar app. No Apple private APIs.
>
> This documentation follows the [arc42](https://arc42.org) template.
> Status: **Deployed / running in production** · Last change: 2026-06-20

---

## 1. Introduction and Goals

### 1.1 Requirements Overview

Incoming iCloud mail should run through a **screener**, like
[HEY.com](https://hey.com) or SaneBox: mail from unknown senders does not land
in the inbox directly but waits in a `Screened/` folder. Known/approved senders
go to INBOX, blocked ones to Junk, newsletters and receipts into their own
folders. The user **trains** the system naturally by moving mail between
folders; the daemon learns the senders and sorts automatically from then on.

This replaces the previous project (`imapfilter-icloud-hey-screener`, Lua +
imapfilter, 600 s polling) with a real event-based daemon and thin clients.

### 1.2 Quality Goals (Top 5)

| # | Quality goal | Scenario |
|---|--------------|----------|
| Q1 | **Responsiveness** | A new mail is classified within seconds (IMAP IDLE instead of polling). |
| Q2 | **Data sovereignty** | Mail content never leaves the user's own system; no third-party cloud, no telemetry. |
| Q3 | **No fragile OS hooks** | The macOS client uses only the public REST API — no Apple private APIs, no SIP/AMFI changes, nothing that breaks on a macOS update. |
| Q4 | **Classification correctness** | An approved sender never ends up in Junk (deterministic, testable list rules; whitelist always wins). |
| Q5 | **Maintainability** | Clear module boundaries, a static CGO-free binary, a reproducible Docker image, and a fully tested classification core. |

### 1.3 Stakeholders

| Role | Expectation |
|------|-------------|
| Operator/user | Runs on a NAS/Linux host; simple to operate from a browser or menu-bar app; no cloud lock-in. |
| Developer | Readable Go code, testable classification, a clear REST API between daemon and clients. |
| Apple Mail / iCloud | Left untouched; integration only over standard IMAP. |

---

## 2. Constraints

### 2.1 Technical Constraints

| # | Constraint | Consequence |
|---|------------|-------------|
| C1 | iCloud Mail offers **only IMAP/SMTP**, no official API. | Classification happens over IMAP folder moves; an app-specific password is required. |
| C2 | iCloud allows **app-specific passwords**, no OAuth for IMAP. | Secret handling in the daemon; no token refresh. |
| C3 | Deployment of the daemon as a **Docker container** on a NAS/Linux host. | 12-factor configuration, a persistent volume for state. |
| C4 | The build must stay **CGO-free** to cross-compile a static linux/amd64 binary. | SQLite via `modernc.org/sqlite` (pure Go). |
| C5 | The macOS app is a **personal tool**, distributed outside the App Store. | App Sandbox off; LAN plain-HTTP allowed; signing/notarization optional. |

### 2.2 Organizational Constraints

- **License:** MIT, open source, no commercial distribution.
- **Use:** personal, "at your own risk".
- **Daemon language:** Go. **Client language:** Swift (SwiftUI).

### 2.3 Conventions

- arc42 for architecture, ADRs (MADR format) under `docs/arc42/adr/`.
- Go: standard layout (`cmd/`, `internal/`), `gofmt` / `golangci-lint`.
- Swift: SwiftUI + Swift Concurrency (strict).

---

## 3. Context and Scope

### 3.1 Business Context

```
              ┌─────────────────────────────────────────────┐
              │                    User                      │
              └───┬───────────────────┬───────────────────┬──┘
                  │ moves mail        │ manages lists     │ manages lists
                  │ (trains)          │ (browser)         │ (menu bar)
        ┌─────────▼────────┐  ┌───────▼────────┐  ┌───────▼────────┐
        │  Apple Mail      │  │  Web UI        │  │  ScreenerBar   │
        │  / icloud.com    │  │  (in daemon)   │  │  (macOS app)   │
        └────────┬─────────┘  └───────┬────────┘  └───────┬────────┘
                 │ IMAP               │ HTTP/REST         │ HTTP/REST
                 │                    │ (Bearer)          │ (Bearer)
        ┌────────▼────────────────────▼───────────────────▼────────┐
        │             screenerd (Go daemon, Docker)                 │
        │   IMAP IDLE · classify · lists · training · API · web UI  │
        └────────────────────────┬──────────────────────────────────┘
                                 │ IMAPS (993)
                         ┌───────▼────────┐
                         │  iCloud Mail   │
                         │ imap.mail.me.com│
                         └────────────────┘
```

| External interface | Direction | Content |
|--------------------|-----------|---------|
| iCloud IMAP | ↔ screenerd | read mail, set flags, move between folders |
| User ↔ Apple Mail | ↔ | read mail, drag into training folders (= train) |
| User ↔ Web UI / ScreenerBar | ↔ | view/edit lists, status, wildcard suggestions |

### 3.2 Technical Context

| Channel | Protocol | Description |
|---------|----------|-------------|
| screenerd ↔ iCloud | IMAPS (993) | two IDLE connections (worker on `Screened/`, watcher on `INBOX`); move/flag |
| Web UI ↔ screenerd | HTTP/REST (Bearer) | served by the daemon at `/`; list CRUD, status, suggestions |
| ScreenerBar ↔ screenerd | HTTP/REST (Bearer) | same REST API; token in the macOS Keychain |

---

## 4. Solution Strategy

| Problem | Strategy | Rationale / ADR |
|---------|----------|-----------------|
| Latency of the old polling approach | **IMAP IDLE** in the daemon, two-tier sweeps | Push instead of poll → seconds (Q1). ADR-0002 |
| Language & deployment | **Go + static binary + Docker** | Small image, robust daemon (Q5, C3). ADR-0001 |
| Managing it from the Mac | **Built-in web UI + a thin menu-bar app**, both over REST | No Apple private APIs, nothing breaks on macOS updates (Q3). ADR-0003 / ADR-0004 |
| Correctness & testability | **Pure, deterministic classification engine** with no I/O | Unit-testable; whitelist always wins (Q4). ADR-0005 |
| Logic/transport separation | **Daemon owns the truth + API**, clients are thin | One source of truth, multiple frontends. |

---

## 5. Building Block View

### 5.1 Whitebox Overall System (Level 1)

```
Mail Screener
├── screenerd     (Go daemon, Docker)          ── core: IMAP, classify, lists, API, web UI
├── ScreenerKit   (Swift package, REST client) ── shared by the macOS app
└── ScreenerBar   (SwiftUI menu-bar app)        ── status, list editing, suggestions, settings
```

| Block | Responsibility |
|-------|----------------|
| **screenerd** | IMAP connection, classification, list persistence, training, retention, snooze, REST API, and the embedded browser UI. The single source of truth. |
| **ScreenerKit** | Shared Swift code: typed REST client (DTOs, auth), domain models, Keychain helper. |
| **ScreenerBar** | macOS menu-bar app (envelope icon): live status, list view/edit, wildcard suggestions, settings (server URL + token). A thin REST client. |

> The earlier idea of a MailKit extension and an Accessibility (AX) overlay with
> buttons docked onto Mail's window was **dropped** (see ADR-0003). The web UI
> and menu-bar app cover management without any private APIs.

### 5.2 Whitebox `screenerd` (Level 2)

```
screenerd
├── cmd/screenerd/main.go     bootstrap, config, two-connection IDLE lifecycle, migrate subcommand
├── internal/imap/            Engine: catch-up, sort, training, maintenance, snooze; go-imap/v2 adapter
├── internal/classify/        PURE engine: sender + headers → Verdict (no I/O)
├── internal/lists/           SQLite store, legacy import, wildcard suggestions, snooze/screened tables
├── internal/snooze/          snooze label grammar parser + legacy map reader
├── internal/config/          12-factor env loading
└── internal/api/             REST handlers (Bearer auth) + embedded web UI (//go:embed)
```

| Module | Responsibility | Key interfaces |
|--------|----------------|----------------|
| `imap` | Hold IDLE connections, run quick/full sweeps, move/flag mail. Depends on a small `Backend` interface (testable with a fake). | `QuickSweep()`, `FullSweep()`, `SnoozeScan()`, `Bootstrap()` |
| `classify` | Derive a **Verdict** from sender + headers + a list `Snapshot`. Deterministic, no side effects. | `Classify(sender, headers, snapshot) Verdict` |
| `lists` | The five lists + atomic persistence (SQLite, WAL, single writer). | `Add/Remove/Contains`, `Snapshot()`, `Suggest(min)` |
| `snooze` | Parse snooze labels (`1d10`, `sat10`, `1w`, …). | `ParseLabel(label, now)` |
| `api` | REST: `/status`, `/lists/...`, `/classify`, `/suggestions`; serves the web UI at `/`. | HTTP/JSON, Bearer auth |
| `config` | Configuration from environment. | `Load() Config` |

> Training is **not** a separate package — it lives inside the `imap` Engine
> (a `Screened→INBOX` move approves and whitelists, `→Junk/Receipts/Newsletters`
> updates the matching list on the next sweep).

### 5.3 Whitebox macOS Side (Level 2)

| Module | Responsibility |
|--------|----------------|
| `ScreenerBar` (app) | `MenuBarExtra` (window style): status header, Lists / Suggestions / Settings tabs. |
| `ScreenerKit/ScreenerClient` | Typed async REST client against `screenerd`. |
| `ScreenerKit/Keychain` | Stores the API token in the macOS Keychain. |

---

## 6. Runtime View

### 6.1 A new mail is screened (daemon path, default case)

```
iCloud            screenerd(imap)      classify        lists
  │ new mail in INBOX │                   │              │
  │ ───────────────►  │ IDLE wakeup       │              │
  │                   │ fetch sender ───► Classify(s,h) ─► Snapshot
  │                   │ ◄──── Verdict ─────┘              │
  │                   │ approve → stays in INBOX          │
  │                   │ block   → move to Junk            │
  │                   │ unknown → move to Screened/ (+ track Message-ID)
```

The daemon screens INBOX itself, so **no iCloud server-side rule is required**.
Unknown senders are moved to `Screened/`; the user triages from there.

### 6.2 Training by moving mail

```
User              iCloud            screenerd (next sweep)        lists
  │ drags mail        │                    │                       │
  │ Screened→INBOX ─► │ ── sweep sees it ─► approve                 │
  │                   │                    │  → Add(whitelist) + Remove(blocklist)
  │ →Junk/Receipts/Newsletters             │  → Add(matching list)
```

### 6.3 Snooze

```
User              iCloud                screenerd
  │ drags mail into     │                    │
  │ Snoozed/<label> ──► │ ── SnoozeScan ────► parse label → wake time
  │                     │                    │  park in Snoozed/, record row (SQLite)
  │                     │ ◄── at wake time: move back to INBOX (unread)
```

---

## 7. Deployment View

### 7.1 Infrastructure

```
┌──────────────────────────────┐        ┌───────────────────────────┐
│  QNAP NAS (Docker)           │        │  Mac / any browser        │
│  ┌────────────────────────┐  │  HTTP  │  ┌──────────┐ ┌─────────┐ │
│  │ screenerd container    │◄─┼────────┼──┤ Web UI   │ │ScreenerBar│
│  │  host 18443 → 8443     │  │ (Bearer)│  │ (browser)│ │(menu bar) │
│  │  volume: ./state       │  │        │  └──────────┘ └─────────┘ │
│  └───────────┬────────────┘  │        └───────────────────────────┘
└──────────────┼───────────────┘
               │ IMAPS
        ┌──────▼──────┐
        │ iCloud Mail │
        └─────────────┘
```

| Node | Artifact | Configuration |
|------|----------|---------------|
| NAS / Linux | `screenerd` container | env via `.env` (`ICLOUD_USER`, `ICLOUD_APP_PASSWORD`, `API_TOKEN`, optional folder/retention/snooze keys); `API_ADDR=0.0.0.0:8443`; volume for `/state`. |
| Mac / browser | Web UI at `http://<host>:18443/` | API token entered in Settings (browser localStorage). |
| Mac | `ScreenerBar.app` | server URL + token (Keychain). |

### 7.2 Build / Release

- **Daemon image (production):** wraps a prebuilt static binary — cross-compile
  on the Mac (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`), `scp` it to the NAS, then
  `docker compose build && up -d`. (Git is not installed on the NAS.)
- **Daemon image (from source):** the root `Dockerfile` is a multi-stage build
  (`golang:alpine` → distroless) for building straight from a checkout.
- **Mac:** Xcode build via an XcodeGen-generated project; signing/notarization
  optional for personal use.

---

## 8. Crosscutting Concepts

### 8.1 Domain Model

- **Verdict** = `approve | block | newsletter | receipt | unknown`.
- **List** = a set of sender identities (exact address or `*@domain` wildcard).
  Kinds: `whitelist`, `blocklist`, `newsletter`, `receipts`, `group_allowlist`.
- **Matching & order**: exact address first, then `*@domain` wildcard. Rule order
  is **whitelist → group → blocklist → newsletter → receipt → unknown**, and the
  whitelist always wins (Q4). Newsletters are also detected from `List-Id` /
  `List-Unsubscribe` headers; group/mailing-list mail (`List-*` / `X-Google-Group`)
  goes to Junk unless the sender is on `group_allowlist`.

### 8.2 Configuration (12-Factor)

Everything via environment; no secrets in the image. The app-specific iCloud
password and the API token come from `.env`. Folder names
(`SCREENED_FOLDER`, `FOLDER_JUNK/RECEIPTS/NEWSLETTERS/ARCHIVE/DELETED/SNOOZED`)
and retention windows (`JUNK_RETENTION` 7d, `RECEIPT_RETENTION` 30d,
`NEWSLETTER_RETENTION` 30d; `0` disables) are overridable; defaults live in code.
`SNOOZE_LABELS` pre-creates `Snoozed/<label>` subfolders (manually created ones
are still detected dynamically).

### 8.3 Security

- The REST API requires a **Bearer token** (constant-time compare) on every
  request. The web UI page itself is served unauthenticated (it carries no
  secrets) and sends the token, kept in browser `localStorage`, on each call.
- The API binds the LAN; the token is the only guard — keep it secret and do not
  expose the port to the public internet.
- No mail **content** is persisted — only sender addresses and lists.
- No Apple private APIs, no SIP/AMFI changes (ADR-0004).

### 8.4 Persistence

Lists, watermarks, snooze rows, and screened-id tracking live in SQLite
(`/state/screener.db`, WAL, one writer serialized through a mutex). Survives
container restarts via the volume.

### 8.5 Error Handling & Resilience

- IMAP reconnect with exponential backoff; IDLE renew before the server timeout.
- Two connections, but **only the worker moves mail**, so moves are never
  concurrent. Moves are UID-based and idempotent; duplicate events do no harm.
- Auto-migration: on an empty DB with a `legacy-seed/` dir, lists are seeded
  once (idempotent, deduped, cleaned).

### 8.6 Logging / Observability

Structured logging (JSON; text in dev). `GET /status` reports version, uptime,
connection state, last sweep, last error, and per-list sizes.

---

## 9. Architecture Decisions

See `docs/arc42/adr/`:

- **ADR-0001** — Go + Docker for the daemon.
- **ADR-0002** — IMAP IDLE instead of polling.
- **ADR-0003** — Mac integration: a built-in web UI + a menu-bar app (the AX
  overlay / MailKit extension idea was dropped).
- **ADR-0004** — No private APIs / no legacy bundle injection.
- **ADR-0005** — Pure, deterministic classification engine.

---

## 10. Quality Requirements

### 10.1 Quality Tree (excerpt)

- **Performance** → responsiveness (Q1), low resource use.
- **Security/privacy** → data sovereignty (Q2), token auth.
- **Maintainability** → testability (Q4/Q5), clear modules.
- **Robustness** → no fragile OS hooks; clients use only the public API (Q3).

### 10.2 Quality Scenarios

| ID | Scenario | Measure |
|----|----------|---------|
| QS1 | Mail arrives → classified | within seconds (IDLE) |
| QS2 | A macOS point update is installed | the Mac app keeps working (no private APIs to break) |
| QS3 | A sender is on whitelist **and** blocklist | lands in INBOX (whitelist wins); covered by a test |
| QS4 | Daemon restart | lists and watermarks fully preserved |
| QS5 | iCloud briefly throttles IMAP | backoff + reconnect, no data loss |

---

## 11. Risks and Technical Debt

| Risk | Assessment | Mitigation |
|------|------------|------------|
| iCloud throttles / changes IMAP behavior | low–medium | backoff, conservative IDLE renew, avoid multiple logins. |
| App-specific password is revoked | low | clear error in `/status`; re-auth by issuing a new password. |
| The API is exposed with only a Bearer token on the LAN | low–medium | keep the token secret; do not publish the port; optional reverse-proxy TLS. |
| The go-imap/v2 adapter is not integration-tested against every iCloud quirk | low | the Engine is fully unit-tested over a fake `Backend`; the live adapter is thin. |
| Build pushed to the NAS by `scp` (no git there) | low | documented deploy procedure; a backup of the previous binary is kept for rollback. |

---

## 12. Glossary

| Term | Meaning |
|------|---------|
| **Screener** | Pre-check of incoming mail from unknown senders before the inbox. |
| **IMAP IDLE** | IMAP extension (RFC 2177) for server-side push notification of new mail. |
| **QuickSweep / FullSweep** | Incremental sweep (only above a per-folder UID watermark) vs. full rescan + retroactive re-sort + training + maintenance + snooze. |
| **Watermark** | The highest processed UID per folder, stored in `folder_state`, driving incremental sweeps. |
| **Snooze** | Parking a mail in `Snoozed/<label>` so it returns to INBOX at a computed time. |
| **Verdict** | The classification result for a sender. |
| **ScreenerBar** | The macOS menu-bar app. |
| **ScreenerKit** | The shared Swift package (REST client + DTOs + Keychain). |
| **screenerd** | The Go daemon (core component). |
