# External Integrations

**Analysis Date:** 2026-07-11

## APIs & External Services

**None** — No third-party APIs, SDKs, or cloud services.

## Data Storage

**Databases:** None
**File Storage:** None (no persistence)
**Caching:** None

## Authentication & Identity

**None** — No authentication required.

## Monitoring & Observability

**Error Tracking:** None
**Logs:** Console only (stdout/stderr via stdlib `log` package)

## CI/CD & Deployment

**Hosting:** None
**CI Pipeline:** None

## System Integrations

**macOS CoreGraphics API (via cgo):**
- `CGEventCreate(NULL)` — Get current mouse position
- `CGEventGetLocation()` — Extract cursor coordinates
- `CGEventCreateMouseEvent()` — Create synthetic mouse-moved event
- `CGEventPost(kCGHIDEventTap, event)` — Post event to system (triggers idle-timer reset)
- `CFRelease()` — Memory management for Core Foundation objects

**Frameworks (linked at compile time):**
- `ApplicationServices.framework` — High-level system services
- `CoreGraphics.framework` — Low-level graphics and event APIs

## Authentication & Access Control

**macOS TCC (Transparency, Consent, Compliance):**
- Requires **Accessibility** permission for the terminal running sysmon
- Location: **System Settings → Privacy & Security → Accessibility**
- Terminal (Terminal.app / iTerm) must be explicitly granted permission
- Without permission: process runs but mouse events fail silently (OS behavior, not application bug)

## Environment Configuration

**Required env vars:** None

**Secrets location:** Not applicable

## Webhooks & Callbacks

**Incoming:** None
**Outgoing:** None

---

*Integration audit: 2026-07-11*
