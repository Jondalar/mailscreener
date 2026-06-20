# ADR-0004 — No private APIs / legacy bundle injection

- Status: accepted
- Date: 2026-06-17

## Context

Real toolbar buttons in Apple Mail used to be possible only via legacy Mail
bundles (code injection into the Mail.app process). Since macOS Sonoma (14)
Apple **removed** the plugin loader; Mail.app no longer loads bundles. Injection
on current macOS would require permanently disabling **SIP** and **Library
Validation/AMFI**, plus ongoing reverse-engineering after every macOS update.

The tool is private, MIT, "at own risk", so an injection path would be
technically *possible*.

## Decision

We deliberately avoid private APIs and bundle injection entirely.

## Rationale

- **System-wide security weakening** (turning SIP/AMFI off) is disproportionate
  for a quality-of-life tool.
- **High fragility**: would break on every macOS point release and require
  constant re-reversing of internal view hierarchies.
- **Crash risk inside the production mail client.**
- Quality goal Q3 (update stability) would be unachievable.

## Consequences

- The final design uses **zero private APIs**. The Mac side is a SwiftUI
  menu-bar app plus a built-in browser web UI, both thin REST clients over the
  LAN (ADR-0003). No Accessibility access is needed at all.
- There are no buttons in Apple's own Mail toolbar, and no overlay window
  either; list management and review happen in `ScreenerBar` or the web UI.
- This decision can be revisited if Apple ever offers official UI extension
  points for Mail.
