---
id: TELL-37
title: Skip desktop notifications when nobody is at the screen
status: todo
type: feature
labels: [notify, escalation]
created: 2026-11-02T03:28:55Z
updated: 2026-11-02T03:28:55Z
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

- [ ] On macOS, a locked screen or a session not on the active console makes the desktop step return not_sent/unattended without running osascript, and the cascade advances; verified by locking the screen and running tell notify --via desktop,stderr
- [ ] On Linux, LockedHint=yes or an inactive logind session returns not_sent/unattended; verified on a real session or recorded as unverified with the parser covered by tests
- [ ] If presence cannot be read (missing tool, unknown session, parse error), the notification is sent as before; covered by a test
- [ ] tell doctor --via desktop reports unattended with the signal (locked, inactive session) and the cascade readiness reflects it
- [ ] Display sleep is either detected without cgo on Apple Silicon and Linux or documented as not detected, with the finding recorded
- [ ] README, docs/design.md (Desktop delivery), and docs/configuration.md document unattended, fail-open behavior, and that Focus/Do Not Disturb is still not detected; task ci passes
