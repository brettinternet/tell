---
id: TELL-47
title: Group macOS desktop notifications by project
status: todo
type: feature
priority: low
labels: [notify, desktop]
created: 2026-11-17T19:51:46Z
updated: 2026-11-17T19:51:46Z
---

## Context

Tell.app posts every notification with a random identifier and no thread (macos/TellNotifier/main.swift), so Notification Center stacks every repository into one Tell group. Working across several repositories, one stack mixes them.

## Scope

- The desktop route passes the canonical project path as a `thread` field in the Tell.app payload; Tell.app sets it as `threadIdentifier`. Tell computes the value and the helper only applies it, matching the click-action design.
- The path is never displayed and message text never becomes the thread.
- A Tell.app built before this change ignores the field and still posts.

## Non-goals

Replacing earlier notifications, summary text, custom group names or config, Linux, and the osascript fallback.

## Dependencies

None. Builds on TELL-43 (Tell.app).

## Modified-file boundaries

macos/TellNotifier/main.swift, internal/delivery/desktop.go, internal/delivery/desktop_test.go, docs/design.md (Desktop delivery).

## Verification

`task macos:install`, then send from two projects and inspect Notification Center on a real Mac; `task ci`.

## Acceptance criteria

- [ ] On a real Mac with Tell.app, notifications from two projects appear as two groups under Tell in Notification Center and notifications from one project group together; verified and recorded
- [ ] The Tell.app payload carries the canonical project path as thread and never message text; covered by extending the desktop helper test
- [ ] A Tell.app built before this change still posts when sent the new payload; verified on a real Mac
- [ ] docs/design.md Desktop delivery documents grouping; task ci passes
