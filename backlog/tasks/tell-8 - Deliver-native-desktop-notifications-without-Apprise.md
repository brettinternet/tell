---
id: TELL-8
title: Deliver native desktop notifications without Apprise
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 21:30'
updated_date: '2026-10-16 23:20'
labels:
  - reviewed
dependencies:
  - TELL-2
documentation:
  - docs/design.md
  - ../bump/docs/hooks.md
type: feature
ordinal: 2500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: desktop banners are the most common personal route and should not need Python, Apprise, and terminal-notifier. Tell owns thin built-in routes (stderr, desktop, Herdr) and keeps Apprise for fan-out to external services; see docs/design.md.

Context:
- macOS: osascript posts a banner with text passed as argv, never interpolated into AppleScript source, for example: osascript -e 'on run argv' -e 'display notification (item 2 of argv) with title (item 1 of argv)' -e 'end run' -- TITLE BODY. Banners are attributed to Script Editor and may need notification permission once. ../bump/docs/hooks.md uses the same argv pattern.
- Linux: notify-send -- TITLE BODY. It needs a session D-Bus; scheduled services need DBUS_SESSION_BUS_ADDRESS.
- Focus or Do Not Disturb can suppress a posted banner, so accepted never means seen.
- Reuse TELL-2's notify contract, subprocess helper, outcome mapping, and notification type where the platform supports it.

Scope: desktop route in internal/delivery, local fixture executables, tests, docs, examples.

Non-goals: Apprise or terminal-notifier for desktop, Windows, notification actions or replies, sound configuration.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 task test verifies through local fixture executables that the macOS and Linux desktop routes pass title and body as separate literal argv elements, never inside script source, preserving quotes, backslashes, newlines, and leading dashes.
- [x] #2 Unsupported platforms and a missing osascript or notify-send return not_sent before anything runs; posting failures, timeouts, and cancellation map to the outcomes in docs/design.md.
- [x] #3 Docs cover macOS notification permission, Linux D-Bus requirements for scheduled services, and that a posted banner is not acknowledgment; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add desktop dispatch using the existing subprocess lifecycle helper: fixed AppleScript argv on macOS, notify-send argv and urgency on Linux, explicit unsupported-platform failure.
2. Extend local fixture checks for literal argv, unavailable tools, failures and interruption; update route documentation and personal example.
3. Run focused tests and task ci, perform one scoped review, commit implementation, merge main, finalize task and clean owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Verification: mise exec -- go test ./internal/delivery ./internal/cli passed. mise exec -- task ci passed (unit tests, race, vet, Staticcheck, build, smoke, Gitleaks, govulncheck). TestDesktopLiteralArguments exercised both macOS and Linux using local fixture executables, preserving literal quotes/backslashes/newlines/leading dashes and rejecting unsupported platforms/missing tools. TestDesktopNotify and TestNotifyInterruption verify provider failures and timeout/cancellation outcomes without leaking raw output. LSP diagnostics clean. README, design/configuration docs and personal example explain permissions, session D-Bus, urgency and acknowledgment limits. One scoped self-review found no concrete defects. Implementation committed as 8bf8dbef2bc9ac711674938752e9db48667409c8 and fast-forward merged into main; no push. Remaining delivery step: finalize task record and remove verified session-owned worktree.

Review: no findings. osascript and notify-send receive title/body as literal argv after --; checked directly that osascript keeps leading-dash and '--' titles as argv items. Unsupported platform and missing executable return not_sent before any process starts; exit/timeout/cancel mapping goes through the shared process helper.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented native desktop notifications using literal argv through osascript/notify-send and the shared bounded subprocess helper. Verified both platform contracts with fixtures and full task ci. Commit 8bf8dbe merged to main; no push. Owned tell-8-desktop worktree and branch removed, and Herdr workspace wDY disappearance verified. No blockers or remaining item-scoped work.
<!-- SECTION:FINAL_SUMMARY:END -->
