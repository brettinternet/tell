---
id: TELL-8
title: Deliver native desktop notifications without Apprise
status: To Do
assignee: []
created_date: '2026-10-16 21:30'
labels: []
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
Why: desktop banners are the most common personal route and should not need Python, Apprise, and terminal-notifier. Tell owns thin built-in routes (stdout, desktop, Herdr) and keeps Apprise for fan-out to external services; see docs/design.md.

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
- [ ] #1 task test verifies through local fixture executables that the macOS and Linux desktop routes pass title and body as separate literal argv elements, never inside script source, preserving quotes, backslashes, newlines, and leading dashes.
- [ ] #2 Unsupported platforms and a missing osascript or notify-send return not_sent before anything runs; posting failures, timeouts, and cancellation map to the outcomes in docs/design.md.
- [ ] #3 Docs cover macOS notification permission, Linux D-Bus requirements for scheduled services, and that a posted banner is not acknowledgment; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
