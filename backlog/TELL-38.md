---
id: TELL-38
title: Add alert route that waits for local acknowledgement
status: done
type: feature
labels: [notify, escalation]
blocked_by: [TELL-37]
created: 2026-11-02T03:29:17Z
updated: 2026-11-02T03:54:31Z
---

## Context

The desktop route's `accepted` only means the banner was posted (TELL-37 adds "someone could see it"). Escalation needs a stronger local signal: someone at this machine acknowledged it. A separate `alert` route waits for an explicit click, for a bounded time. If nobody clicks, it reports that the alert wasn't acknowledged so the cascade can escalate (for example `[alert, apprise]`, or `tell notify --via alert ... && ack.sh KEY` to cancel later Noon tiers in the TELL-36 example).

It's a separate route rather than a desktop option because it behaves differently: it's modal or persistent, it blocks until acknowledged or the wait runs out, and its success means something stronger.

| Platform | Mechanism |
| --- | --- |
| macOS | `osascript` running fixed AppleScript `display dialog (item 2 of argv) with title (item 1 of argv) buttons {"Acknowledge"} default button 1 giving up after N`; parse `gave up:` |
| Linux | `notify-send --wait --urgency critical --expire-time 0 --action ack=Acknowledge --print-id -- TITLE BODY`; stdout `ack` means acknowledged; close by id on expiry (`gdbus` `CloseNotification`) |
| other | `not_sent/unsupported_platform` |

## Outcome mapping

| Result | Outcome | Exit |
| --- | --- | --- |
| Acknowledge clicked | `accepted`, `acknowledged: true` | 0 |
| wait elapsed, dismissed, or closed without the action | `not_sent/unacknowledged` (cascade advances; resending or escalating is safe) | 3 |
| locked or unattended (TELL-37) | `not_sent/unattended`, without waiting | 3 |
| provider error | `failed/provider_failure` | 4 |
| provider start/teardown exceeded `--timeout`, or cancellation | `uncertain` | 5 |

Update the compatibility table: "0 never means acknowledged" becomes "0 means acknowledged only for the alert route". `acknowledged` is a new JSON field, which the schema allows.

## Scope

- Add an `alert` route usable in `--via` and in `notify.via` cascades, with an optional step form `alert: {wait: DURATION}`, matching how `apprise` and `herdr` steps carry options. A bare `alert` uses the default.
- `wait` defaults to `2m`: long enough to reach the keyboard from nearby, short enough that the next tier (phone) still matters. It must be a positive Go duration.
- The wait is human time and does not use the shared `--timeout` budget (default `10s`). `--timeout` still covers starting and tearing down the provider, so later steps keep their budget and nobody has to raise `--timeout` to use alert. The longest an alert step can run is `wait` plus the remaining budget. In listener notify actions, the action timeout still caps the whole step.
- Check presence through TELL-37 before showing anything.
- Add doctor support. On Linux, check that `notify-send` supports `--action` and `--wait` (libnotify 0.7.10 or newer).
- Verify whether `osascript` `display dialog` works from non-GUI parents (launchd/Noon, ssh). If it fails with -1713, decide between `tell application "System Events"` (needs Automation permission) and documenting the limit, and record the evidence.

## Non-goals

Remote acknowledgement (TELL-33), retries or repeated alerts (Noon tiers, TELL-36), state kept by Tell, native macOS banners with actions (needs a signed app bundle and UNUserNotificationCenter), multiple buttons or custom labels, sound or Focus overrides, special MCP handling beyond the existing timeout.

## Dependencies

TELL-37 (presence check).

## Modified-file boundaries

internal/delivery/alert.go (new) and alert_test.go, internal/delivery route registry, doctor, and cascade wiring, internal/config (route name and step schema) and tests, internal/cli where route names are listed, README.md, docs/design.md, docs/configuration.md, examples/escalate/** only to show alert followed by ack if TELL-36 already landed.

## Verification

On macOS: acknowledge an alert, let one expire with `wait: 15s` inside `[alert, stderr]`, and run one from a Noon or launchd job. On Linux if available, otherwise record it as unverified. Test only the critical logic: parsing the result into an outcome, validating the wait and its default, and keeping the wait out of the shared budget. Run `task ci`.

## Acceptance criteria

- [x] tell notify --via alert shows a dialog with one Acknowledge button on macOS and an action notification on Linux; title and body are passed as argv, never interpolated
  Evidence: internal/delivery/alert.go: macOS osascript display dialog with buttons {"Acknowledge"}, Linux notify-send --wait --action ack=Acknowledge; title/body/wait passed as argv after --. TestAlertWaitOutsideTimeoutBudget asserts literal argv with injection-shaped title/body. Commit 4b060c5.
- [x] Clicking Acknowledge within the wait returns accepted with acknowledged: true in JSON and exits 0; verified by hand on macOS
  Evidence: By hand on macOS 2026-11-01: tell --json notify --via alert ... -> user clicked Acknowledge; output {"acknowledged":true,...,"outcome":"accepted","route":"alert"} exit=0. TestAlertResultParsing covers clicked/gave up/-128/-1713/other.
- [x] No acknowledgement within the wait (or a Linux dismissal) returns not_sent/unacknowledged, removes the dialog or notification, and the cascade advances; verified with alert: {wait: 15s} followed by stderr
  Evidence: By hand: cascade [alert: {wait: 15s}, stderr] with default 10s timeout -> 15.5s, attempts alert not_sent/unacknowledged then stderr accepted, exit 0; dialog closed by giving up after. Same from a launchd gui agent (Noon-like) with wait 3s. Linux close-by-id via gdbus CloseNotification implemented but not verified by hand (no Linux host).
- [x] A locked or unattended screen returns not_sent/unattended at once without waiting (reuses TELL-37)
  Evidence: notifyAlert checks desktopPresence first; TestDesktopPresence asserts locked screen returns not_sent/unattended without running osascript/notify-send, with wait=1h, on darwin and linux fixtures.
- [x] alert waits 2m by default; alert: {wait: DURATION} overrides it and config rejects zero or negative waits; the wait does not use the shared --timeout budget, so with the default 10s timeout a [alert, stderr] cascade still reaches stderr after an unacknowledged alert; tell doctor --via alert reports readiness, including the Linux notify-send action support
  Evidence: DefaultAlertWait=2m; alert: {wait} validated positive Go duration (TestNotifyCascadeStrictValidation rejects 0s, -5s, soon, unknown option; accepts 15s, default 2m). Cascade budget extended by time spent waiting (TestAlertWaitOutsideTimeoutBudget: 100ms timeout, 1s wait, provider 300ms still reaches stderr; without wait it is uncertain). tell --json doctor --via alert -> ready, presence attended; Linux checks gdbus + notify-send --version >= 0.7.10 (unsupported_provider_version).
- [x] README, docs/design.md, and docs/configuration.md describe the alert route, the outcome mapping, and the compatibility change (0 means acknowledged only for alert); task ci passes
  Evidence: README.md, docs/design.md (Alert delivery section, outcomes, compatibility: 0 means acknowledged only for alert), docs/configuration.md, examples/escalate/README.md (alert && ack.sh). mise exec -- task ci passed after rebase onto e33ea33.

## Summary

Added the alert route (internal/delivery/alert.go) in commit 4b060c5 on main. macOS uses a fixed display dialog AppleScript with argv. Linux uses notify-send --wait --action and closes the notification with gdbus by ID when the wait expires. Presence is checked before showing anything. The cascade budget is extended by the time spent waiting, so the wait doesn't consume --timeout. Steps accept alert: {wait}, default 2m, rounded up to whole seconds. Doctor checks Linux libnotify 0.7.10+ and gdbus. Non-GUI evidence: dialogs work from a launchd gui-domain agent. ssh was not reachable here; -1713 maps to not_sent/no_user_interaction and is documented, not routed through System Events. Linux has not been verified by hand. Review: one self-review pass fixed sub-second wait rounding and removed an incorrect claim that MCP caps the wait. Verification: task ci, plus hand runs for acknowledge, expiry in [alert{15s}, stderr], and a launchd run.
