# MVP — Daemon + REST API

Status: draft · Refs: decision D1

The smallest slice that screens real iCloud mail end-to-end via the Go daemon,
remotely inspectable over REST. No Mac integration yet.

## In scope

- **classify engine** (Spec 0001) — pure, deterministic, table-tested.
- **lists + SQLite + seeding** (Spec 0002) — atomic storage, idempotent legacy
  import, suggestion pass (suggest-only, no auto-collapse).
- **IMAP IDLE + sweep + catch-up** (Spec 0003) — < 5 s screening, reconnect
  backoff, INBOX Hide-My-Mail catch-up.
- **training + maintenance** (Spec 0004) — folder-move learning, retention.
- **REST API** (Spec 0005) — list CRUD, `/status`, `/classify`, suggestions,
  Bearer auth.
- **config + Docker** (Spec 0006) — 12-factor env, distroless image, `/state`
  volume, `migrate` subcommand.
- **snooze** (Spec 0007) — `Snoozed/<label>` folders (rich label grammar),
  per-message wake back to INBOX, legacy `snoozed_map` migration.
- **group filter** (Spec 0008) — mailing-list/Google-Group heuristic → Junk,
  with a configurable `group_allowlist`.

## Build order (suggested)

1. `classify` + tests (no I/O — fastest to land, de-risks correctness).
2. `lists` (SQLite) + `migrate` seeding (dedup + junk-filter + self-address
   drop) + suggestion pass + tests.
3. `config` loader.
4. `imap` IDLE/sweep/catch-up wired to classify + lists.
5. `training` + maintenance.
6. `api` REST over the above.
7. Dockerfile/compose end-to-end on the NAS.

## Definition of Done

- New mail in `Screened/` is classified and moved in < 5 s (p95, QS1).
- Whitelisted sender never lands in Junk, even if also blocklisted (QS3, test).
- Lists survive container restart (QS4).
- Legacy `.txt` import is idempotent; real NAS data pulled, cleaned and seeded
  (199k→339 newsletter etc.; header-junk and self-address dropped).
- REST API up, Bearer-protected; `/classify` matches engine tests.
- `go test ./...` green; daemon runs in Docker on the NAS via compose.

## Explicitly deferred to Product

Mac MailKit extension, ScreenerBar companion + AX overlay, AppleScript actions,
regex/substring pattern matching, auto-remove on reclassification, gRPC, TLS
termination in-process, per-message snooze via REST / snooze duration UI.
