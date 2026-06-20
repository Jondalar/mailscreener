# Specs & Roadmap

Delivery flow: **Spec → Refinement → MVP → Product.**

| Phase | Artifact | Goal |
|-------|----------|------|
| Spec | `specs/NNNN-*.md` | What each capability must do, with acceptance criteria. Behavior, not code. |
| Refinement | open-questions + decisions logged in each spec | Resolve ambiguity, size the work, lock scope. |
| MVP | `specs/mvp.md` | Smallest slice that screens real iCloud mail end-to-end via the daemon. |
| Product | `specs/product.md` | Full arc42 target: REST API, Mac integration, hardening. |

Specs are numbered and live next to this file. Architecture context: `../docs/arc42/`.

## Capability specs

- [0001 — Classification Engine](0001-classification-engine.md) — pure `Classify(sender, headers, lists) → Verdict`
- [0002 — Lists, Persistence & Seeding](0002-lists-persistence.md) — SQLite, idempotent `.txt` import, suggest-only wildcard compaction
- [0003 — IMAP IDLE, Sweep & Catch-up](0003-imap-idle-sweep.md) — < 5 s screening, reconnect, Hide-My-Mail catch-up
- [0004 — Training & Maintenance](0004-training-maintenance.md) — folder-move learning, retention
- [0005 — REST API](0005-rest-api.md) — list CRUD, /status, /classify, Bearer auth
- [0006 — Config & Deployment](0006-config-deployment.md) — 12-factor env, distroless Docker
- [0007 — Snooze](0007-snooze.md) — `Snoozed/<label>` folders, wake back to INBOX (MVP)
- [0008 — Group Filter](0008-group-filter.md) — mailing-list/Google-Group → Junk + `group_allowlist` (MVP)

## Phase docs

- [mvp.md](mvp.md) — Daemon + REST API slice, build order, Definition of Done
- [product.md](product.md) — full arc42 target (Mac integration, hardening)

## Locked decisions (refinement)

| # | Decision |
|---|----------|
| D1 | MVP = Go daemon + REST API; Mac deferred to Product |
| D2 | SQLite at `/state/screener.db` (atomic, survives restart) |
| D3 | Idempotent seed/import from legacy `.txt` |
| D4 | Pure classify; whitelist > blocklist > newsletter(list/List-Id/List-Unsubscribe) > receipt(list) > unknown; no subject heuristics |
| D5 | Flat lossless import + suggest-only `*@domain` compaction; freemail never collapsed; regex/patterns → Product |
