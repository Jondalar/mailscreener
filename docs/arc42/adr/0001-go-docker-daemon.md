# ADR-0001 — Go + Docker for the daemon

- Status: accepted
- Date: 2026-06-17

## Context

The existing setup uses imapfilter (Lua) in a 600 s polling loop. We need a
robust, long-running daemon that runs on a NAS/Linux host via Docker, stays
small and maintainable, and speaks IMAP IDLE.

## Decision

We implement the daemon (`screenerd`) in **Go** and ship it as a **Docker
image** (multi-stage build → minimal base such as `distroless`/`scratch`).

## Rationale

- Static, CGO-free binary (`modernc.org/sqlite`) → tiny, reproducible image,
  simple deployments.
- Mature IMAP libraries with IDLE support (`go-imap/v2`).
- Strong standard library for the HTTP API and a good concurrency model for a
  long-lived IDLE connection plus an API server.
- Easy cross-compiling for NAS architectures (amd64/arm64).

## Alternatives

- **Rust**: maximum robustness, but slower iteration.
- **Python**: closer to the script origin, but larger image and more runtime
  dependencies.
- **Stay on Lua/imapfilter**: no IDLE, hard to test, limited API surface.

## Consequences

- Go toolchain + standard project layout (`cmd/`, `internal/`).
- Build is pure-Go / CGO-free, so production cross-compiles statically:
  `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/screenerd`.
- Deployed and running in production on a QNAP NAS. Because git is not
  installed on the NAS, the deploy procedure is: cross-compile on the Mac → scp
  the binary to the NAS → `docker compose build && up -d`. The container listens
  on 8443, published on host port 18443.
