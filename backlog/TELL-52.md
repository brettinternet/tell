---
id: TELL-52
title: Click actions for Linux desktop notifications via D-Bus activation
status: done
type: feature
priority: medium
labels: [desktop]
created: 2026-11-17T20:40:05Z
updated: 2026-11-17T21:14:52Z
---

## Context

On macOS, Tell.app makes a click work after `tell notify` exits: macOS relaunches the app, which replays the stored click actions (TELL-43, TELL-45). Linux `notify-send` cannot do this: a freedesktop `ActionInvoked` signal goes to the posting connection, which has already exited, so Linux desktop notifications have no click action and `--url` is only a body line.

The equivalent is D-Bus activation: the notification names an application with a `DBusActivatable=true` desktop entry, and the desktop shell activates that application on click, passing the action and its parameter. GNOME implements this for `org.gtk.Notifications` and the XDG notification portal; KDE supports the portal; lightweight daemons (dunst, mako) generally do not. A detached waiting `notify-send --wait` child per notification was rejected: it is a lingering background process.

## Scope

- First, confirm on a real GNOME session (and KDE if available) that an unsandboxed CLI can post a notification whose default action activates a D-Bus-activatable helper after the sender exits, carrying the click actions in the action parameter so no state is stored. If no such mechanism works on GNOME, stop, record the evidence in this item, and leave behavior unchanged.
- If confirmed: a helper (preferably `tell` itself, reached through a D-Bus service file) that runs the stored click actions with the same rules as Tell.app: literal argv, bounded to 10 seconds each, output discarded, message text never stored in actions.
- `task linux:install` installs the desktop entry, D-Bus service file, and icon under `~/.local/share` (no root, no services started), mirroring `task macos:install`.
- The desktop route uses the activatable path only when the helper is installed and the session supports it; otherwise `notify-send` with the URL as the last body line, unchanged. `tell doctor --via desktop` reports which provider is used.
- Click behavior matches macOS: `--url` opens the URL (`xdg-open`), otherwise the origin action where one exists on Linux (Herdr workspace and tab), and `notify.desktop.on_click` still wins with `TELL_ORIGIN_*` and `TELL_URL`.

## Non-goals

Release packaging of the Linux files (follow-up after this lands), daemons without activation support, action buttons, focusing a terminal app on Linux, Windows.

## Dependencies

None. Related: TELL-43, TELL-45, and the Tell.app release item.

## Modified-file boundaries

internal/delivery (desktop.go, presence or doctor files, tests), internal/cli only if a helper entry point is needed, a new linux/ resource directory, .taskfiles (new linux.yaml, Taskfile include), go.mod if a D-Bus library is required, README.md, docs/design.md, docs/development.md.

## Verification

Real GNOME session: install with `task linux:install`, send `tell notify --url https://example.com`, confirm `tell` has exited, click, and the browser opens; repeat with a Herdr origin and with `on_click`. Confirm a session without activation support still sends via `notify-send` with the URL as the last line. `task ci`.

## Acceptance criteria

- [x] Activation mechanism verified or refuted on a real GNOME session, with evidence recorded in this item before any implementation
  Evidence: Verified on GNOME Shell 46.0 (Ubuntu 24.04 Lima VM, headless Wayland shell, user session bus). Desktop entry io.github.brettinternet.Tell.desktop (DBusActivatable=true) + session D-Bus service file under ~/.local/share. An unsandboxed gdbus call to org.gtk.Notifications.AddNotification(app_id, id, {title, body, default-action 'app.click', default-action-target <actions JSON>}) returned and exited; helper was not running. Activating the notification in the shell (Notification.activate(), the banner click path, via Shell.Eval) D-Bus-activated the helper, which received ActivateAction('click', [<actions JSON>], {activation-token}). No state stored. KDE not available to test; Plasma has no org.gtk.Notifications and its portal needs host-app registration, so only GNOME is supported.
- [x] task linux:install installs the desktop entry, D-Bus service, and icon under ~/.local/share without root or starting services
  Evidence: GNOME 46 VM: task linux:install (as non-root user) wrote ~/.local/share/applications/com.brettinternet.tell.desktop (DBusActivatable=true, Exec="<PATH tell>" desktop-click), ~/.local/share/dbus-1/services/com.brettinternet.tell.service, ~/.local/share/icons/hicolor/scalable/apps/com.brettinternet.tell.svg; only side effect besides files is a session-bus ReloadConfig; no service started (helper ran only on click).
- [x] On GNOME with the helper installed, clicking a --url notification after tell exits opens the URL; on_click wins and receives TELL_ORIGIN_* and TELL_URL; recorded
  Evidence: GNOME Shell 46 VM (headless Wayland; click = Notification.activate() via Shell.Eval, the banner click path): tell notify --via desktop --url https://example.com exited 0 with no helper running; click D-Bus-activated 'tell desktop-click', which ran <abs xdg-open> https://example.com with XDG_ACTIVATION_TOKEN, then exited after idle. Herdr origin (no URL): click ran herdr workspace focus wQY, herdr tab focus wQY:t1 with HERDR_SOCKET_PATH. on_click config: ran on_click --flag in origin cwd with TELL_ORIGIN_HERDR_WORKSPACE/TAB, TELL_ORIGIN_CWD, TELL_URL=https://example.com/run/1. xdg-open/herdr were recording stand-ins (no browser in headless VM); not repeated on a physical GNOME desktop.
- [x] Without the helper or activation support, desktop still uses notify-send with the URL as the last body line; tell doctor --via desktop reports the provider; covered by tests
  Evidence: VM: tell doctor --via desktop reports provider gtk-notifications with helper+GNOME, notify-send with service file removed, with no session bus, and with GNOME Shell stopped. Tests: TestDesktopLiteralArguments (helper files installed, no session bus -> notify-send argv with URL last body line; doctor provider notify-send), TestDesktopOrigin (Linux xdg-open URL action, no terminal activation).
- [x] README.md and docs/design.md document Linux click actions and supported desktops; task ci passes
  Evidence: README.md (routes, --url) and docs/design.md (Desktop delivery, Click actions, Links) document GNOME click helper, task linux:install, unsupported KDE/dunst/mako; docs/development.md lists linux:install. task ci exit 0 on b9a811f (fast-forwarded to main).

## Notes

Commit b9a811f fast-forwarded to main. tell posts via org.gtk.Notifications when task linux:install's helper is installed and the session bus has an owner for it; hidden 'tell desktop-click' is the D-Bus-activated helper. Added github.com/godbus/dbus/v5. Review: one pass; switched bus connection to inherited DBUS_SESSION_BUS_ADDRESS only (no discovery/autolaunch). Verified in a Lima Ubuntu 24.04 GNOME 46 headless VM (VM 'gnome' left for reuse; limactl delete gnome to remove). Physical-desktop click not performed.
