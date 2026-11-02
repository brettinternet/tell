---
id: TELL-43
title: Open notification origin on desktop click on macOS
status: done
type: feature
labels: [notify, desktop]
created: 2026-11-02T03:50:06Z
updated: 2026-11-02T04:44:56Z
---

## Context

On macOS the desktop route runs `osascript -e 'display notification ...'` (internal/delivery/desktop.go). macOS attributes those notifications to Script Editor, so clicking one opens Script Editor's file picker. AppleScript can't attach a click handler, so fixing this needs a different provider.

Notifications should be attributed to Tell, and clicking one should go back to where it came from. At send time the caller's environment already identifies the origin:

| Origin | Source | Click action |
| --- | --- | --- |
| Terminal app | `__CFBundleIdentifier` (e.g. `com.mitchellh.ghostty`) | `open -b ID` |
| Herdr workspace/tab | `HERDR_WORKSPACE_ID`, `HERDR_TAB_ID` | `herdr workspace focus ID`, `herdr tab focus ID` |
| Directory | working directory | passed to `on_click` only |

Options considered: `terminal-notifier` (Homebrew dependency, uses the deprecated NSUserNotification API, click handling flaky on recent macOS) and `alerter` (blocks until clicked, which breaks one-shot notify). A small native helper using UNUserNotificationCenter is the only option that gives Tell its own name, icon, and permission, and a click callback that doesn't need a long-running Tell process: macOS relaunches the helper with the stored origin when the notification is clicked.

## Scope

- Add a minimal Swift helper app bundle, `Tell.app` (own bundle ID and icon, ad-hoc signed for local builds), built by a Task target on macOS. Tell calls it with title, body, and an origin payload; it posts through UNUserNotificationCenter and exits. On click, macOS relaunches it with the stored origin and it runs the click action, then exits.
- Capture the origin in Tell at send time: terminal bundle ID, Herdr workspace and tab IDs, and working directory. Store only those identifiers in the notification's userInfo, never message text.
- Default click action: activate the terminal app, then focus the Herdr workspace and tab when present. With no origin, a click does nothing extra.
- Optional `notify.desktop.on_click: [argv...]` in personal config only (refused in shared `tell.yaml`, like `notify.command`). It replaces the default action and receives `TELL_ORIGIN_APP`, `TELL_ORIGIN_HERDR_WORKSPACE`, `TELL_ORIGIN_HERDR_TAB`, `TELL_ORIGIN_CWD`. Message content never reaches its argv or environment.
- The desktop route uses the helper when it is installed and falls back to the current osascript provider otherwise. Outcomes and TELL-37 presence checks are unchanged; if notification permission is denied, return `not_sent` with a specific reason.
- Doctor reports the active provider and the helper's notification authorization status without sending.
- Reading `HERDR_*` and `__CFBundleIdentifier` from Tell's own environment to record the origin is allowed. Keep stripping `HERDR_*` from child environments.

## Non-goals

Focusing the exact Herdr pane (Herdr 0.9.5 `pane focus` only moves by direction; needs an upstream focus-by-ID change), action buttons or replies, opening a new terminal when there's no origin, Linux click actions, notarized distribution, tmux or other multiplexers, terminal-notifier or alerter support.

## Dependencies

None. Works alongside TELL-37 presence checks.

## Modified-file boundaries

New macOS helper source (e.g. `macos/TellNotifier/`), Taskfile.yml (helper build/install target), internal/delivery/desktop.go and tests, internal/delivery/doctor.go, internal/config/ (on_click, shared-config refusal), internal/cli/ (doctor/help wiring), README.md, docs/configuration.md, docs/design.md, AGENTS.md (origin env note).

## Verification

Build and install the helper locally. From a Herdr pane in Ghostty, send a notification, switch to another workspace and app, click the notification, and record what happens. Repeat with the helper removed to confirm the osascript fallback. Run `task ci`.

## Acceptance criteria

- [x] tell notify --via desktop on macOS with Tell.app installed shows a notification attributed to Tell (not Script Editor); verified on a real Mac and recorded
  Evidence: c718aa8. task macos:install built and installed ~/Applications/Tell.app (com.brettinternet.tell, ad-hoc signed). After enabling in System Settings, bin/tell notify --via desktop from a Herdr pane in Ghostty exited 0; user confirmed the banner was attributed to Tell, not Script Editor (2026-11-01).
- [x] Clicking a notification sent from a Herdr pane in Ghostty activates Ghostty and focuses the origin Herdr workspace and tab, after tell notify has already exited; verified on a real Mac and recorded
  Evidence: Real Mac: sent from Herdr pane wQY:p5 (Ghostty, workspace wQY/tab wQY:t1); tell notify exited, user switched Herdr workspace and app, clicked the notification; user confirmed Ghostty activated on the tell workspace/tab (Works fully).
- [x] A notification sent with no origin (e.g. from Noon or a non-terminal parent) does nothing extra on click; covered by a test of origin capture
  Evidence: TestDesktopOrigin (internal/delivery/desktop_test.go): empty environment yields no click actions; Ghostty+Herdr env yields open -b, herdr workspace focus, herdr tab focus with absolute herdr path and socket. go test ./internal/delivery passes.
- [x] notify.desktop.on_click is refused in shared tell.yaml, receives origin only as TELL_ORIGIN_* environment variables, and message title/body never reach its argv or environment; covered by a test
  Evidence: TestCommandConfig refuses notify.desktop.on_click in shared tell.yaml (personal policy) and overrides are rejected; TestDesktopHelper asserts on_click argv/Dir/env are only TELL_ORIGIN_* and that title/body never appear in click actions or helper env (HERDR_* stripped). go test ./internal/... passes.
- [x] Without Tell.app, the desktop route falls back to the existing osascript behavior unchanged; tell doctor --via desktop reports which provider is active and whether notification permission is granted, without sending
  Evidence: Real Mac with ~/Applications/Tell.app moved aside: tell doctor --via desktop -> 'provider: osascript', notify exit 0 via osascript; restored -> 'provider: tell-app; permission: authorized'; denied state reported 'permission: denied' and notify returned not_sent/permission_denied exit 3. TestDesktopLiteralArguments (fallback argv unchanged) and TestDesktopHelper (doctor provider/permission) pass.
- [x] README route table, docs/configuration.md, and docs/design.md document the helper, origin capture, click behavior, and non-goals; AGENTS.md notes HERDR_* origin reads are allowed; task ci passes
  Evidence: README route tables, docs/configuration.md (settings, personal policy, doctor), docs/design.md (Desktop delivery + Click actions incl. non-goals), docs/development.md, tell.schema.json, AGENTS.md origin-env note updated in c718aa8. task ci passed (security, check, test, race, smoke).

## Notes

Worktree: .worktrees/feat-tell-43-desktop-click branch feat/tell-43-desktop-click session 4101740d-395c-46c9-a6e3-3af4d4d6535f

Done in c718aa8 (merged to main, fast-forward). Review: one self-review pass, no item-scoped defects found. Tell.app remains installed at ~/Applications/Tell.app with notifications allowed. Verified: task ci, real-Mac attribution/click/fallback with user.
