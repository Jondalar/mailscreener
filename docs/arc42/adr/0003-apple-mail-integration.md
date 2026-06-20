# ADR-0003 — Mac client: menu-bar app + built-in web UI

- Status: accepted (supersedes the earlier MailKit + AX-overlay plan)
- Date: 2026-06-17

## Context

We want a convenient way to manage lists and review screened mail from the Mac.
Apple offers no supported API to add buttons to Mail's own toolbar (see ADR-0004).

An earlier plan tried to approximate "real buttons" in Apple Mail with a MailKit
extension (`MEMessageActionHandler`) for auto-screening the INBOX plus an
Accessibility (AX) overlay window that floated transparent buttons over Mail's
toolbar, driven by AppleScript/JXA. That approach was **dropped**: it was
complex and fragile across macOS point releases (AX hierarchies and automation
permissions keep shifting), and not worth the cost for a personal tool.

In the meantime the daemon screens the INBOX itself (ADR-0002), so no in-Mail
auto-screening component is needed at all.

## Decision

The Mac side is delivered as two thin clients over the daemon's REST API, with
**no Apple Mail integration** and **no private APIs**:

1. **`ScreenerBar`** — a SwiftUI **menu-bar app** (envelope/mail menu-bar icon)
   for status, list management, and reviewing suggestions.
2. **Built-in web UI** — a self-contained browser UI served by the daemon at the
   API root (`/`). The page is unauthenticated and carries no secrets; it stores
   the Bearer token in browser `localStorage` and sends it on every API call.

Both share a thin Swift package, **`ScreenerKit`** (typed REST client + DTOs +
Keychain), and talk to the daemon over the LAN.

Folder-move training remains the robust, cross-platform path: the user simply
moves mail between folders and the daemon learns on the next sweep. This works
from any client, including the iPhone.

## Rationale

- Zero Apple private APIs and zero in-process injection → update-stable (Q3).
- Far simpler and more robust than the AX overlay / MailKit approach.
- The built-in web UI gives a usable interface from any device with a browser,
  with no install required.
- `ScreenerBar` can be signed/notarized with a Developer ID; no App Store
  needed (personal tool, App Sandbox off).

## Consequences

- The Mac never reads or moves mail itself; it only calls the REST API. All
  classification and IMAP side effects stay in the daemon.
- The dropped overlay/MailKit approach is recorded here as superseded; see
  ADR-0004 for why private-API paths were rejected.
