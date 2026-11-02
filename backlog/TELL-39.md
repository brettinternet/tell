---
id: TELL-39
title: Add iMessage notify route on macOS
status: done
type: feature
labels: [notify]
created: 2026-11-02T03:39:44Z
updated: 2026-11-02T03:51:29Z
---

### Context

On a Mac signed in to Messages, Tell can text a person (you or someone else) with nothing to install: run `osascript` with a fixed AppleScript and pass the recipient and body as argv, the same way the desktop route does. This is a phone-reaching step for cascades like `[desktop, imessage]` with no third-party account. xoba/imsg uses the same AppleScript; there's no need to depend on it. mautrix/imessage is a long-running Matrix bridge and doesn't fit one-shot notify.

```applescript
on run argv
  tell application "Messages"
    send (item 2 of argv) to participant (item 1 of argv) of (1st account whose service type = iMessage)
  end tell
end run
```

### Scope

- Add an `imessage` route usable in `--via` and in `notify.via` cascades, configured by an `imessage: {to: HANDLE}` step or a top-level `imessage.to`. HANDLE is an E.164 phone number or an Apple ID email.
- `to` is personal routing data: allowed in user and local config, refused in shared `tell.yaml` like other routing.
- The body goes through existing redaction; the title, when set, is prepended as the first line.
- Run the script with `osascript -` (script on stdin) or a fixed `-e` script with argv, never interpolating the recipient or body into script text.
- `accepted` means Messages took the message, not that it was delivered. Say so in docs, like desktop.
- Add doctor support: macOS only, `osascript` present, Messages has an iMessage account, and Automation permission isn't denied (detect via a no-op script that reads the account without sending).

### Outcome mapping

| Result | Outcome | Exit |
| --- | --- | --- |
| osascript exits 0 | `accepted` | 0 |
| not macOS | `not_sent/unsupported_platform` | 3 |
| no `to` configured, or invalid handle | config error before sending | 2 |
| no iMessage account, or Automation permission denied (osascript error -1743) | `not_sent` with a specific reason; cascade advances | 3 |
| other osascript error | `failed/provider_failure` | 4 |
| `--timeout` exceeded or cancelled after start | `uncertain` | 5 |

Diagnostics never print the body or the recipient handle.

### Non-goals

SMS fallback, group chats, attachments, multiple recipients per step, reading `chat.db` for delivery status (needs Full Disk Access), receiving messages, a listener source.

### Files

`internal/delivery/` (new `imessage.go`, `doctor.go`, cascade wiring), `internal/config/`, `internal/cli/` (doctor/notify wiring and help), README.md route table, docs/configuration.md, docs/design.md.

### Dependencies

None. TELL-37 presence does not apply: a text should reach the phone even when nobody is at the Mac.

### Notes

Messages to your own handle appear on all your devices but may not alert your phone. Record what you observe during verification and document it.

## Acceptance criteria

- [x] tell notify --via imessage "hi" with imessage.to set in user config delivers a message on a real Mac; result recorded, including whether a message to your own handle alerts the phone
  Evidence: Real Mac: tell notify --config user-config-with-imessage.to=own-phone --json 'tell TELL-39 test' returned accepted, exit 0; user confirmed the phone received it with an alert. An Apple ID email handle also returned accepted but did not reach the phone (user's devices do not receive iMessage at that email); documented in docs/design.md.
- [x] Recipient and body are passed as argv and never interpolated into AppleScript; a body containing quotes and AppleScript syntax arrives verbatim (covered by a test of the built argv)
  Evidence: Real Mac (macOS 27.2): tell notify --config <user cfg with imessage.to: +1661…> --json 'tell TELL-39 test' -> accepted, exit 0; user confirmed the phone received it WITH an alert (own phone number). An Apple ID email handle also returned accepted but did not reach the phone because the user's devices do not receive iMessage at that email; documented in docs/design.md.
- [x] imessage.to in shared tell.yaml is refused; missing or malformed to is a config error before sending; covered by a test
  Evidence: internal/delivery/imessage_test.go TestIMessageArgvAndOutcomes: recipient and title+body (quotes, AppleScript, $(...)) arrive verbatim after -- and never appear in -e script args; go test ./internal/delivery passes.
- [x] Denied Automation permission or no iMessage account returns not_sent with a specific reason and the cascade advances; non-macOS returns not_sent/unsupported_platform
  Evidence: internal/config/notify_test.go TestNotifyCascadeStrictValidation: shared tell.yaml imessage.to refused as personal routing; missing to (single and cascade), '-e', bare digits, a@b, unknown step option all fail config load. Real CLI: tell notify --via imessage x -> 'imessage.to is required' (config, exit 1 per repo outcome table; task table's '2' conflicts with repo convention where config=1).
- [x] tell doctor --via imessage reports ready/not ready with the reason, without sending
  Evidence: TestIMessageArgvAndOutcomes maps osascript -1743 -> not_sent/automation_denied, 7001 (script-raised no account) -> not_sent/no_account, -1712 -> uncertain/timeout, other -> failed/provider_failure; platform linux -> not_sent/unsupported_platform without starting osascript. not_sent advances cascades via the existing fallback_on default.
- [x] README route table, docs/configuration.md, and docs/design.md document the route, that accepted means handed to Messages, and the non-goals; task ci passes
  Evidence: Real: tell doctor --via imessage --imessage-to +15550000000 -> 'imessage (selected): ready' (reads accounts with no send script); doctor maps automation_denied/no_account/unsupported_platform/missing_executable with fixed guidance in internal/cli/doctor.go; covered by checkIMessage assertions in the delivery test.

## Summary

Added the imessage notify route: osascript with a fixed script, recipient and text passed as argv; imessage.to (plus --imessage-to/TELL_IMESSAGE_TO and step to:) validated as E.164 or email, refused in shared tell.yaml; doctor reads the account list without sending. Commit c6cbe9a on main. Verified: task ci, a real send to own phone number (alerted), real doctor. Review: one self-review pass found no defects. Config errors exit 1 per the repo outcome table, not the 2 in the task table.
