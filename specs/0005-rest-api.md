# Spec 0005 — REST API

Status: draft · Phase: Spec · Refs: arc42 §3.2/§8.3, decision D1

## Purpose

Expose the daemon's truth to thin clients (later: the Mac companion app):
inspect and edit lists, read status, classify a sender on demand. Part of the
MVP scope (D1).

## Transport & auth

- HTTP/JSON. Bind `127.0.0.1` by default; LAN/VPN only, TLS via a reverse proxy
  (arc42 §8.3).
- **Bearer token mandatory** (`Authorization: Bearer <API_TOKEN>`); missing/wrong
  token → `401`. Token from `API_TOKEN` env (D-config).
- No request body persists mail content — only addresses/lists (Q2).

## Endpoints (MVP)

| Method & path | Body / params | Returns |
|---------------|---------------|---------|
| `GET /status` | — | connection state, list sizes, last N classifications, version, uptime |
| `GET /lists/{kind}` | kind ∈ whitelist\|blocklist\|newsletter\|receipts | entries (value, is_wildcard, source) |
| `POST /lists/{kind}` | `{ "value": "a@b.com" }` | 201; idempotent upsert |
| `DELETE /lists/{kind}/{value}` | — | 204; 404 if absent |
| `POST /classify` | `{ "sender": "a@b.com", "listId": "...", "listUnsubscribe": "..." }` | `{ "verdict": "approve" }` (pure engine, no move) |
| `GET /suggestions?min=5` | — | wildcard compaction suggestions (Spec 0002) |
| `POST /suggestions/apply` | `{ "kind": "...", "wildcard": "*@d.com" }` | applies one suggestion (replaces covered exacts) |

- `POST /classify` calls `classify.Classify` only — never moves a message.
  It is the testable, side-effect-free probe of the engine over HTTP.
- All mutating endpoints go through Spec 0002's atomic ops.

## Errors

- `401` no/invalid token · `400` malformed body / bad address · `404` unknown
  kind or missing entry · `409` never (upserts are idempotent) · `500` storage
  error (logged, no content leaked).

## Acceptance criteria

- Every endpoint rejects a missing/invalid Bearer token with `401`.
- `POST /lists/whitelist {a@b.com}` then `GET /lists/whitelist` shows the entry;
  posting again returns 201 and does not duplicate.
- `POST /classify` returns the same verdict the daemon would apply, with no
  message moved (verified against engine unit tests).
- `GET /status` reflects a live vs reconnecting IMAP state.
- API binds loopback by default; binding elsewhere requires explicit config.

## Out of scope

- gRPC (arc42 lists it as an option; MVP is REST).
- Mac client implementation (Product phase).
