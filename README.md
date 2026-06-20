# Mail Screener for iCloud (`screenerd` + Docker)

A self-hosted alternative to services like **HEY.com Screener** or **SaneBox** —
mail from senders you've never heard from waits in a `Screened/` folder instead
of buzzing your inbox. You triage and train it by simply **moving mail between
folders**; it learns and does the sorting from then on.

Built for **iCloud Mail**, runs as a small Go daemon (`screenerd`) in Docker
with a live IMAP connection. Manage it from a **browser UI**, a **macOS menu-bar
app**, or a REST API.

> This is the **v2 rebuild** of [`imapfilter-icloud-hey-screener`](https://github.com/Jondalar/imapfilter-icloud-hey-screener)
> (Lua + imapfilter, 600 s polling). v2 is event-based (IMAP IDLE, near-instant),
> screens INBOX itself (**no iCloud server-side rule needed anymore**), and adds
> SQLite, snooze, retention, a REST API, and a UI.

---

## ✨ Features

- **Real-time screening** — IMAP IDLE, not a slow polling loop. Unknown mail is
  moved to `Screened/` within seconds of arriving.
- **No iCloud rule required** — the daemon redirects unknown INBOX senders to
  `Screened/` itself. (v1 needed a server-side rule; v2 does not.)
- **Train by moving mail** — drag a message and the daemon remembers the sender:
  → INBOX = whitelist, → Junk = block, → Receipts / Newsletters = those lists.
- **Whitelist always wins** — a trusted sender is never re-screened or blocked.
- **Snooze** — move a mail into `Snoozed/<when>` and it comes back later.
- **Auto-retention** — old Junk / Receipts / Newsletters are tidied away on a
  schedule (configurable).
- **Wildcard suggestions** — proposes collapsing many addresses into `*@domain`
  (never auto-applied, freemail domains never collapsed).
- **Three ways to manage it** — built-in web UI, macOS menu-bar app, REST API.
- **Dockerized & CGO-free** — one static binary, easy on a NAS or Linux host.
  State (SQLite + lists) lives under `/state`.

---

## 📦 Requirements

- A server/NAS with **Docker + docker compose** (tested on QNAP).
- An **iCloud Mail** account with an [app-specific password](https://support.apple.com/en-us/HT204397)
  (Apple blocks plain IMAP logins).
- That's it — **no iCloud server-side rules** to configure.

---

## 🚀 Setup

### 1. Create an app-specific password

1. Sign in at <https://appleid.apple.com>.
2. **Sign-In and Security → App-Specific Passwords → +**.
3. Name it `screenerd`, copy the password (looks like `abcd-efgh-ijkl-mnop`).

This is **not** your Apple ID password and can be revoked any time.

### 2. Clone & configure

```bash
git clone https://github.com/Jondalar/mailscreener.git
cd mailscreener
cp .env.example .env
```

Edit `.env` and fill in the three required values (everything else has sane
defaults — leave the commented lines alone unless you want to tweak folders,
retention, or snooze):

```ini
ICLOUD_USER=you@icloud.com
ICLOUD_APP_PASSWORD=abcd-efgh-ijkl-mnop   # the app-specific password
API_TOKEN=pick-a-long-random-secret       # protects the API + web UI
```

`.env` is git-ignored — your secrets never get committed.

### 3. Run

```bash
docker compose up -d        # build + run in the background
docker compose logs -f      # watch it connect (Ctrl-C to stop watching)
```

On first start the daemon **creates the folders it needs** if they're missing:
`Screened`, `Junk`, `Receipts`, `Newsletters`, `Archive`, `Snoozed`. Within a few
seconds of healthy logs, unknown mail starts landing in `Screened/`.

You should see logs like:

```
INFO imap connected server=imap.mail.me.com:993 connections=2
INFO swept kind=full took=812ms
```

---

## 📂 Workflow

**New mail** from an unknown sender → daemon moves it to `Screened/`.
Known-good senders → straight to INBOX. Known junk → `Junk`.

**Triage** the `Screened/` waiting room whenever you like. Every move teaches it,
so the waiting room gets quieter over time:

| You move a message to… | …the daemon learns |
|------------------------|--------------------|
| **INBOX** (from `Screened`) | sender → **whitelist** (and removed from block list) |
| **Junk** | sender → **block list** |
| **Newsletters** | sender → **newsletter list** |
| **Receipts** | sender → **receipts list** |

Verdict order is fixed and predictable: **whitelist → group → blocklist →
newsletter → receipt → unknown**, *whitelist always wins*. Newsletters are also
detected from `List-Id` / `List-Unsubscribe` headers automatically.

### 💤 Snooze

Move a mail into a **subfolder of `Snoozed`** named with a time label; it
vanishes and reappears in your INBOX (unread) at that time.

| Folder | Wakes up |
|--------|----------|
| `Snoozed/1d10` | tomorrow at 10:00 |
| `Snoozed/sat10` | next Saturday at 10:00 |
| `Snoozed/1w` | in one week |

Create these by hand whenever you need one, or pre-create a set via
`SNOOZE_LABELS` in `.env`. Both work. (Full grammar: `specs/0007-snooze.md`.)

### 🗑 Retention

Old mail is moved out of the way automatically (set any window to `0` to disable):

| Folder | After | Goes to |
|--------|-------|---------|
| **Junk** | `JUNK_RETENTION` (default 7 days) | Deleted Messages |
| **Receipts** | `RECEIPT_RETENTION` (default 30 days) | Archive |
| **Newsletters** | `NEWSLETTER_RETENTION` (default 30 days) | Archive |

---

## 🖥 Web UI & Mac app

- **Web UI** — open `http://<your-server>:8443/`, go to **Settings**, paste your
  `API_TOKEN`. Live status, list editing, wildcard suggestions. The token stays
  in that browser only (localStorage).
- **macOS menu-bar app** — the **ScreenerBar** app in [`mac/`](mac/README.md)
  does the same from a native menu-bar icon.

> The API is reachable on your LAN guarded only by the Bearer token — keep
> `API_TOKEN` long and secret, and don't expose port 8443 to the public internet.

---

## 🔧 Customization

All settings are environment variables — the full commented list with defaults
is in [`.env.example`](.env.example). The ones you're most likely to touch:

| Key | Default | Meaning |
|-----|---------|---------|
| `SCREENED_FOLDER` | `Screened` | the waiting-room folder |
| `FOLDER_JUNK` / `FOLDER_RECEIPTS` / `FOLDER_NEWSLETTERS` / `FOLDER_ARCHIVE` / `FOLDER_DELETED` / `FOLDER_SNOOZED` | iCloud names | override if your mailbox is organised differently |
| `JUNK_RETENTION` / `RECEIPT_RETENTION` / `NEWSLETTER_RETENTION` | `7d` / `30d` / `30d` | age-out windows (`0` = off) |
| `SNOOZE_LABELS` | — | snooze subfolders to pre-create, e.g. `1d10,1w,sat10` |
| `SWEEP_INTERVAL` | `10m` | how often the full re-sort runs |
| `RECEIPT_SUBJECT_MATCH` | `false` | opt-in: also route receipts by subject keywords |

Change a value, then `docker compose up -d` to apply it.

**Importing existing lists** — coming from the old Lua daemon? Drop the `.txt`
lists in `state/legacy-seed/` and they're imported on first start (deduped,
cleaned, idempotent). Manual run:

```bash
docker compose run --rm screenerd migrate /state/legacy-seed
```

---

## ⚠️ Notes

- iCloud may still fire a push notification in the instant a mail first hits
  INBOX, before the daemon moves it to `Screened/`. The window is seconds (IDLE),
  not minutes — but it's not zero.
- One iCloud account per daemon instance.

---

## 🛠 Build from source

Pure Go, no CGO (`modernc.org/sqlite`) — cross-compiles to a static binary.

```bash
make build        # -> bin/screenerd
make test         # go test ./...
make docker       # build the image
make dev          # local run: loads .env, STATE_DIR=./state, text logs
```

```
cmd/screenerd/      daemon entrypoint (2-conn IDLE loop, quick/full sweeps, migrate)
internal/classify/  pure engine: sender + headers -> verdict (no I/O, fully tested)
internal/imap/      Engine: catch-up, sort, training, maintenance, snooze
internal/lists/     SQLite store, legacy import, wildcard suggestions
internal/snooze/    snooze label grammar parser
internal/config/    12-factor env loading
internal/api/       REST API (Bearer auth) + embedded web UI
mac/                macOS menu-bar app (ScreenerBar) + shared ScreenerKit package
docs/arc42/         architecture documentation + ADRs
specs/              feature specs + roadmap
```

REST API (all behind the Bearer token): `GET /status`, `GET|POST /lists/{kind}`,
`DELETE /lists/{kind}/{value}`, `POST /classify`, `GET /suggestions`,
`POST /suggestions/apply` — `kind` ∈ `whitelist`, `blocklist`, `newsletter`,
`receipts`, `group_allowlist`.

---

## 💡 Credits

- v1 built with [imapfilter](https://github.com/lefcha/imapfilter); v2 is a
  ground-up Go rewrite.
- Inspired by HEY.com Screener and SaneBox.
- Tested on QNAP NAS via Container Station.

## License

MIT — see [LICENSE](LICENSE). Personal use, at your own risk.
