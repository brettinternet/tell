---
id: TELL-48
title: Play a sound for failure notifications on macOS
status: done
type: feature
priority: low
labels: [notify, desktop]
created: 2026-11-17T19:51:46Z
updated: 2026-11-17T20:26:39Z
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

- [x] On a real Mac with Tell.app, a --type failure notification plays the default sound and info, success, and warning are silent; verified and recorded
  Evidence: task macos:install, then tell notify --via desktop --type info|success|warning|failure on a real Mac (Focus off): user confirmed only failure played the default sound.
- [x] The Tell.app payload requests sound only for failure; covered by extending the desktop helper test
  Evidence: Payload sends sound=true only for failure (internal/delivery/desktop.go); TestDesktopHelper asserts sound for failure and no sound key for info/success/warning; go test ./internal/delivery passes.
- [x] docs/design.md Desktop delivery table documents the macOS type mapping; task ci passes
  Evidence: docs/design.md Desktop delivery table maps failure to default sound for Tell.app; task ci passed after rebase on ee98eb6.

## Summary

Tell sends sound=true in the Tell.app payload only for --type failure; the helper sets UNNotificationSound.default. Commit 02ab018 on main. Verified by TestDesktopHelper, task ci, and a real-Mac listening test. Review: no defects. Note: Focus mode silences the sound.
