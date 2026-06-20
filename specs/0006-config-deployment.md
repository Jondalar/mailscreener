# Spec 0006 — Config & Deployment

Status: draft · Phase: Spec · Refs: arc42 §2/§7/§8.2, C5

## Purpose

Make the daemon a 12-factor Docker citizen: all config via env, no secrets in
the image, persistent state on a volume, reproducible build.

## Configuration (env, arc42 §8.2)

| Key | Default | Meaning |
|-----|---------|---------|
| `ICLOUD_USER` | — (required) | iCloud address |
| `ICLOUD_APP_PASSWORD` | — (required) | app-specific password (C2) |
| `SCREENED_FOLDER` | `Screened` | folder the daemon watches |
| `API_TOKEN` | — (required) | Bearer token for the REST API |
| `API_ADDR` | `127.0.0.1:8443` | API bind address |
| `IDLE_TIMEOUT` | `1500s` | IDLE renew interval |
| `SWEEP_INTERVAL` | `10m` | safety-net rescan interval |
| `JUNK_RETENTION` | `7d` | Junk→Deleted age (0=off) |
| `RECEIPT_RETENTION` | `30d` | Receipts→Archive age (0=off) |
| `STATE_DIR` | `/state` | SQLite + backups location |
| `LOG_FORMAT` | `json` | structured logging |

- Missing required key → daemon refuses to start with a clear message.
- No secret is ever baked into the image; provided via compose env / `.env` /
  secret mount.

## Deployment (arc42 §7)

- Multi-stage `Dockerfile`: `golang:alpine` build → `distroless/static` runtime,
  static `CGO_ENABLED=0` binary, non-root.
- `docker-compose.yml`: restart `unless-stopped`, `/state` volume, loopback port
  publish, env from `.env`.
- `screenerd migrate` subcommand for one-time legacy `.txt` seeding (Spec 0002).

## Observability (arc42 §8.6)

- Structured JSON logs: classification decisions (sender hash or address,
  verdict), moves, reconnects, errors.
- `GET /status` for health (Spec 0005).

## Acceptance criteria

- `docker compose up` with required env starts the daemon; missing env fails
  fast with a named-key error.
- State survives `docker compose down && up` (lists intact, QS4).
- Image contains no password/token (verified: env-only).
- Binary runs on linux/amd64 and linux/arm64 (NAS).

## Out of scope

- Reverse-proxy/TLS setup (documented, not shipped).
- Mac app distribution / notarization (Product phase, arc42 §7.2).
