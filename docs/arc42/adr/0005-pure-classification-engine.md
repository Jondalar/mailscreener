# ADR-0005 — Pure, deterministic classification engine

- Status: accepted
- Date: 2026-06-17

## Context

Classification correctness is a top quality goal (Q4: an approved sender must
never end up in the junk). Logic that is intertwined with IMAP I/O is hard to
test.

## Decision

Classification (`internal/classify`) is a **pure function** with no side
effects:

```
Classify(sender, headers, Snapshot) -> Verdict
```

All I/O (IMAP, persistence, API) lives exclusively in the surrounding modules.
The engine is handed an in-memory `Snapshot` of the lists; it never reads from
disk itself.

## Rules (deterministic, first match wins)

1. **Whitelist** matches → `approve` (always wins over the blocklist).
2. **Group** (mailing-list / `List-*` / `X-Google-Group` headers) → `block`,
   unless the sender is on the `group_allowlist`.
3. **Blocklist** matches → `block`.
4. **Newsletter** (list membership OR `List-Id`/`List-Unsubscribe` header) →
   `newsletter`.
5. **Receipt** (list membership) → `receipt`.
6. otherwise → `unknown` (stays in the screener).

Matching is exact address plus optional `*@domain` wildcard. There is no
subject-keyword routing by default (determinism).

## Rationale

- Fully unit-testable (table tests), including conflict cases.
- Behavior is reproducible independent of network/server.

## Consequences

- New routing signals belong in headers/lists, not in subject heuristics.
- New verdict types require only extending the engine plus tests.
