---
id: TELL-48
title: Play a sound for failure notifications on macOS
status: todo
type: feature
priority: low
labels: [notify, desktop]
created: 2026-11-17T19:51:46Z
updated: 2026-11-17T19:51:46Z
---

## Context

Linux maps `--type failure` to critical urgency. macOS ignores type, and Tell.app sets no sound, so a failed build looks and sounds exactly like a success.

## Scope

- With Tell.app, `failure` notifications play the default notification sound; `info`, `success`, and `warning` stay silent.
- Tell decides: the payload carries a `sound` boolean and the helper only applies it, matching the click-action design.
- Users silence it per app in System Settings > Notifications > Tell; no Tell config.

## Non-goals

Per-type or custom sounds, config keys, time-sensitive or critical interruption levels (they need entitlements), sounds in the osascript fallback, and mapping type to other routes' priority (Pushover and ntfy priority stay personal config).

## Dependencies

None. Builds on TELL-43 (Tell.app).

## Modified-file boundaries

macos/TellNotifier/main.swift, internal/delivery/desktop.go, internal/delivery/desktop_test.go, docs/design.md (Desktop delivery table).

## Verification

`task macos:install`, then send each type on a real Mac; `task ci`.

## Acceptance criteria

- [ ] On a real Mac with Tell.app, a --type failure notification plays the default sound and info, success, and warning are silent; verified and recorded
- [ ] The Tell.app payload requests sound only for failure; covered by extending the desktop helper test
- [ ] docs/design.md Desktop delivery table documents the macOS type mapping; task ci passes
