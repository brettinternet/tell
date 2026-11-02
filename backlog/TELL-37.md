---
id: TELL-37
title: Skip desktop notifications when nobody is at the screen
status: done
type: feature
labels: [notify, escalation]
created: 2026-11-02T03:28:55Z
updated: 2026-11-02T03:42:29Z
---

## Context

The desktop route reports `accepted` once `osascript` or `notify-send` exits 0, even when the screen is locked or the user switched away. In an escalation cascade like `[desktop, apprise]`, that `accepted` stops the cascade, so the phone step never fires while nobody can see the banner. Tell should check whether anyone could see the banner before sending, and return `not_sent/unattended` otherwise so the cascade advances.

Signals (no cgo, no extra permissions):

| Signal | macOS | Linux |
| --- | --- | --- |
| locked | `IOConsoleLocked` from `ioreg -n Root -d1 -a` | `loginctl show-session` `LockedHint` |
| session on the active console | `IOConsoleUsers` entry for the current uid with `kCGSSessionOnConsoleKey` | logind `Active` / `State` for the caller's session |
| display asleep | investigate; Apple Silicon has no `IODisplayWrangler` | investigate (DPMS is compositor-specific) |

## Scope

- Check presence in the desktop step before sending. If the screen is locked or the session is inactive, return `not_sent/unattended` and don't run the provider.
- Fail open: if presence can't be read, send as before. Never suppress a notification because detection failed.
- Show the presence state in `tell doctor --via desktop`.
- Same behavior on every path that uses the desktop route: CLI notify, listener notify actions, MCP.
- Document the behavior change: `--via desktop` on a locked screen now exits 3 instead of 0.

## Non-goals

Idle-time thresholds (a possible follow-up), Focus/Do Not Disturb detection (no reliable public API; DND files need Full Disk Access), cgo or IOKit bindings, new config keys or opt-out flags, acknowledgement (see the desktop alert task).

## Dependencies

None.

## Modified-file boundaries

internal/delivery/desktop.go, internal/delivery/presence*.go (new), internal/delivery/doctor.go, internal/delivery/desktop_test.go, internal/cli/doctor.go and tests if output changes, README.md, docs/design.md, docs/configuration.md.

## Verification

On macOS, lock the screen and run `tell notify --via desktop,stderr --json "test"` from a scheduled or remote shell: desktop is `not_sent/unattended` and stderr is `accepted`. Unlocked, desktop is `accepted`. Test only the critical logic: parsing the ioreg plist and loginctl output, and failing open. Run `task ci`.

## Acceptance criteria

- [x] On macOS, a locked screen or a session not on the active console makes the desktop step return not_sent/unattended without running osascript, and the cascade advances; verified by locking the screen and running tell notify --via desktop,stderr
  Evidence: ebddeb2. Real macOS lock (Ctrl+Cmd+Q) with notify.via [desktop, stderr]: locked -> attempts desktop not_sent/unattended (osascript not run), stderr accepted; unlocked -> desktop accepted. Doctor while locked: 'desktop: unavailable here; presence: locked'. (Task's --via desktop,stderr isn't valid syntax; used config cascade.)
- [x] On Linux, LockedHint=yes or an inactive logind session returns not_sent/unattended; verified on a real session or recorded as unverified with the parser covered by tests
  Evidence: Unverified on a real Linux session (dev host is macOS). Uses loginctl show-user UID Display -> show-session ID LockedHint/Active (avoids 'auto' resolving to an SSH caller's session). Parser and end-to-end skip covered by TestDesktopPresence linux cases.
- [x] If presence cannot be read (missing tool, unknown session, parse error), the notification is sent as before; covered by a test
  Evidence: TestDesktopPresence: probe exits 1 or prints garbage on darwin/linux -> notify accepted and provider ran, doctor ready presence unknown; parser cases for missing uid entry, missing console key, malformed/empty plist return unknown. go test ./internal/delivery passes.
- [x] tell doctor --via desktop reports unattended with the signal (locked, inactive session) and the cascade readiness reflects it
  Evidence: tell doctor --via desktop prints '; presence: attended|locked|inactive_session|unknown', JSON adds presence; unattended -> ready=false reason unattended, cascade step shows 'unavailable here' (verified live while locked). Covered by TestDesktopPresence readiness asserts.
- [x] Display sleep is either detected without cgo on Apple Silicon and Linux or documented as not detected, with the finding recorded
  Evidence: Documented as not detected in docs/design.md Desktop delivery: Apple Silicon has no IODisplayWrangler (verified: ioreg -n IODisplayWrangler empty on this M-series host); per-framebuffer IOPowerManagement exists but semantics across multiple/external displays are undocumented; Linux DPMS is compositor-specific. Displays that lock on sleep are caught as locked.
- [x] README, docs/design.md (Desktop delivery), and docs/configuration.md document unattended, fail-open behavior, and that Focus/Do Not Disturb is still not detected; task ci passes
  Evidence: README (desktop route row, doctor example), docs/design.md (Desktop delivery, outcomes, reasons), docs/configuration.md (doctor table/reasons/JSON) document unattended, fail-open, Focus/DND not detected. task ci passed (fmt, vet, staticcheck, tests, race, smoke).
