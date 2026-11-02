# Design

This page describes implemented behavior. The listener contract is in
[listening](listening.md).

## Boundary

```text
person / script / scheduler hook / MCP client
          │
         tell
          ├── notify → one route or an ordered fallback cascade
          ├── send   → hcom mailbox | idle Herdr agent pane
          ├── targets → read-only list of send recipients
          ├── listen SOURCE → bounded run | notify | send actions
          └── mcp    → client-owned stdio: notify | send | doctor
```

| Owner | Owns |
| --- | --- |
| scheduler (cron, launchd, CI, Noon) | schedules, queues, concurrency, cancellation, run history |
| repo scripts | which events matter, what agent work means |
| Tell | routing, safe argv, bounded subprocesses, honest results |
| provider | destinations, credentials, native config |

Tell keeps no durable state and never reads scheduler-specific environment or
payloads; callers pass arguments or explicit stdin.

Outbound delivery does not throttle, deduplicate, or retry the same route.
Only the caller knows whether a message repeats, and a dropped alert costs more
than a duplicate. Notify-on-change belongs in the caller (see
[examples/noon](../examples/noon/README.md)). There is no agent spawning or
plugin registry.

## Commands

`notify` asks a person for attention. stderr (the default) keeps stdout and
`--json` clean; desktop and Herdr need no extra runtime; Apprise fans out to
services; Shoutrrr fans out in-process with no extra runtime; iMessage texts a
phone through macOS Messages; command runs one personal executable.

`send` gives one existing agent information (`--intent inform`) or an
instruction (`--intent request`) over one transport. It never falls through to
another agent and never waits for a reply. Tracked agent work belongs to the
scheduler's agent jobs (such as Noon's); ask-and-wait belongs to Unharness.
Listener routes may only `inform`, because event authors are untrusted.

`targets` lists recipients for the configured `send.via` only, never sending,
prompting, or starting services. It never selects or defaults a recipient:
`send` still requires one explicit `send.to`. See
[recipient listing](#recipient-listing).

## Notify fallback cascades

```yaml
notify:
  via:
    - apprise: {tag: phone}
    - apprise: {config: ../apprise/chat.yaml}
    - herdr: {session: work}
    - desktop
  fallback_on: [not_sent]
  fallback_notice: title
```

| Rule | Why |
| --- | --- |
| accepted stops | delivered |
| `not_sent` advances | it promises nothing was delivered |
| `failed` advances only if listed in `fallback_on` | a partial failure may have delivered |
| `uncertain` always stops | a slow provider may have delivered |
| each step runs at most once, sharing one `--timeout` | no retries; exhausted budget is `not_sent/timeout_budget` |
| one-shot `via` (flag, env, MCP) disables the list | explicit choice wins |
| higher layers replace the whole list | no merged cascades |

Per-step options override top-level provider settings; flag, environment, and
permitted MCP selectors override every matching step.

`fallback_notice: title` prefixes later titles with the first step's label and
reason. Labels are the Apprise tag, config basename, or `urls_env` name
(`shoutrrr:NAME` for Shoutrrr), never URLs. JSON receipts list attempts:

```json
{"attempts":[{"route":"apprise","label":"apprise:phone","outcome":"not_sent","reason":"no_match"},{"route":"desktop","label":"desktop","outcome":"accepted","reason":""}]}
```

Apprise tags are ordered alternatives, not comma-separated fan-out. A tagged
step removes inherited `APPRISE_URLS` so the tag is honored. `urls_env` passes
a variable's value only as the child's `APPRISE_URLS`. Tell never parses
Apprise config or URLs.

`tell doctor` lists steps in order and is ready if any step is. Listener
`notify` actions without `via` use the cascade within the action timeout.

### Escalation

Tell sends once and keeps no state, so repeat-until-acknowledged escalation
belongs to the provider or the scheduler. See
[examples/escalate](../examples/escalate/README.md).

A final Apprise step that selects a Pushover `priority=emergency` URL makes
Pushover repeat the alert until it is acknowledged in the app or expires;
PagerDuty and Opsgenie through Apprise work the same way. `accepted` means the
provider accepted the alert, not that anyone acknowledged it; Tell does not
poll receipts. Use Apprise, not Shoutrrr, for this: Shoutrrr `v0.22.1` drops
`priority=2` and sends a normal Pushover notification that still reports
`accepted`.

Timed tiers spread separate calls over time. Schedule every later tier before
waiting on anything, and let acknowledgement cancel them: if the raising
process dies, the tiers still fire. Escalation that only advances while a
process waits fails silently. A short `alert` wait is a fine acknowledgement
source (`tell notify --via alert … && ack.sh KEY`), not the escalation timer.

Tiers need a scheduler that can run a command later and survive a restart,
cancel it by a name derived from the incident, and not duplicate a re-raised
incident. Noon temporary jobs meet this, as do `systemd-run --on-active` units.
Tell never calls or reads a scheduler: each tier is one `tell notify`, and
acknowledgement is a user command (`ack.sh`). Without a scheduler, a
provider-native step (Pushover emergency, PagerDuty) is the single tier.

| Case | Behavior |
| --- | --- |
| ack before a tier is due | never sent |
| ack while a tier runs | Noon sends TERM; the alert is uncertain |
| ack after delivery | cannot be recalled |
| scheduler down across a due time | the tier runs late, once (Noon `0.15.0`) |

## Listeners

`tell listen slack` and `tell listen github` are explicit foreground processes
with bounded in-memory ingress, duplicate suppression, and a four-slot action
executor. Acknowledgments mean transport receipt, not durability: restart,
sleep, or overflow can lose events, and actions never retry. Inbound
credentials are stripped from every child. See [listening](listening.md).

## MCP

`tell mcp` adapts the existing config and delivery packages; it adds no routes
or providers. Each tool call reloads layered config (including `tell mcp`
project, config, flags, and environment), dispatches at most one delivery, and
returns a body-free receipt like the CLI's. Provider output never enters
results. stdout carries only protocol messages; the stderr route writes to
stderr. There are no resources, prompts, HTTP, or retries.

By default the notify/send schemas omit route and recipient fields, and the
server rejects them if sent anyway. Personal `mcp.agent_targets: true` allows
choosing configured selectors only; see
[configuration](configuration.md#mcp-agent-target-overrides) and the
[README](../README.md#mcp-server). Timeout or cancellation after a provider
starts is `uncertain`.

## Compatibility

| Surface | Promise within the current major version |
| --- | --- |
| config `version: 1` | valid keys keep meaning, precedence, and trust boundaries; unknown keys stay errors |
| JSON `schema_version: 1` | existing fields keep type and meaning; new fields may appear |
| exit codes | stable; 0 means acknowledged only for the alert route, 5 never means safe to resend |
| human wording | not an API |

Ignore unknown JSON fields and treat unknown reasons and routes
conservatively. Security fixes may tighten validation or redaction. Provider
behavior is outside the guarantee, and Tell never turns `uncertain` into
success.

## Message preparation

Every route gets the same prepared title and body, in order:

1. **Reject NUL**: exit 1, `config/invalid_message`.
2. **Strip controls**: C0, DEL, C1, CR (CRLF → LF). Newline and tab stay.
3. **Strip terminal sequences**: CSI, OSC, DCS, SOS, PM, APC, and ESC forms, through BEL/ST or end of input.
4. **Strip bidi controls** (U+202A–U+202E, U+2066–U+2069). Invalid UTF-8 becomes U+FFFD. No Markdown/HTML sanitizing.
5. **Scan for secrets** offline with Gitleaks v8 default rules. Matches become `[REDACTED]` everywhere they appear; `secrets.policy: reject` exits 1 `config/secret_detected`; scanner errors fail closed (`config/secret_scan_failed`). `gitleaks:allow` does not bypass.
6. **Truncate** titles to 256 bytes and bodies to 16,384 bytes on a UTF-8 boundary, ending in `[truncated]`.
7. **Reject empty** bodies: exit 1, `config/empty_message`.

```console
$ tell --json notify "key $AWS_ACCESS_KEY_ID done"
key [REDACTED] done
{"outcome":"accepted","redactions":1,"route":"stderr","schema_version":1}
```

Scanning precedes truncation so a cut-off secret cannot leak. `redactions`
counts occurrences in both fields (zero under `reject`). Detection has false
positives and misses; it is not proof a message is safe.

## Outcomes

| Exit | Outcome | Meaning | Resend? |
| --- | --- | --- | --- |
| 0 | accepted | printed, posted, or provider exited 0; not proof anyone read it, except alert (`acknowledged: true`) | no |
| 1 | config | config, input, or I/O error before delivery | after fixing |
| 2 | usage | invalid invocation | after fixing |
| 3 | not_sent | missing executable, unsupported platform, unattended desktop, unacknowledged alert, Herdr not shown, absent/invalid target, busy/blocked agent, no match | yes, or another route |
| 4 | failed | provider ran and failed; `partial` means some destinations got it | not blindly |
| 5 | uncertain | timed out or cancelled after the provider started | not without checking |

```console
$ tell --json notify --via apprise "Deploy done"
{"schema_version":1,"redactions":0,"outcome":"not_sent","route":"apprise","error":{"code":"not_sent","message":"provider executable was not found","reason":"missing_executable"}}
```

Reasons: `missing_executable`, `start_failed`, `unsupported_route`,
`unsupported_platform`, `unattended`, `unacknowledged`, `no_user_interaction`,
`unsupported_message`, `missing_route`,
`missing_target`, `missing_session`, `invalid_target`, `invalid_response`,
`no_match`, `busy`, `blocked`, `disabled`, `rate_limited`,
`no_foreground_client`, `not_shown`, `partial`, `provider_failure`, `timeout`,
`cancelled`, `secret_detected`, `secret_scan_failed`. Results never include the
body or provider output; unknown provider exits are `failed`.

- Cancellation before the provider starts, or while reading stdin, is `config`.
- The stderr route writes to stderr even with `--json`; a write error is `failed`.
- If writing an accepted JSON result fails, Tell warns and still exits 0, so callers don't resend.

## Desktop delivery

| Platform | Runs | Type |
| --- | --- | --- |
| macOS | `osascript` with fixed AppleScript; title and body as argv | ignored |
| Linux | `notify-send --urgency LEVEL -- TITLE BODY` | `failure` → critical, else normal |
| other | — | `not_sent/unsupported_platform` |

Text is never interpolated into code. macOS banners appear as **Script
Editor**; allow its notifications. Linux services need the session D-Bus
(`DBUS_SESSION_BUS_ADDRESS`), which Tell inherits but does not discover.

Before sending, Tell checks whether anyone could see the banner. A locked
screen or a session that is not the active one returns `not_sent/unattended`
(exit 3) without running the provider, so a cascade such as
`[desktop, apprise]` moves on to the phone. This applies to CLI notify,
listener notify actions, and MCP.

| Platform | Locked | Not the active session |
| --- | --- | --- |
| macOS | `IOConsoleLocked` from `ioreg -n Root -d1 -a` | own `IOConsoleUsers` entry has `kCGSSessionOnConsoleKey` false (switched user) |
| Linux | `LockedHint=yes` on the user's logind display session | `Active=no` on that session |

Detection fails open: a missing `ioreg`/`loginctl`, no matching session, or
unreadable output sends as before. `tell doctor --via desktop` shows
`presence`: `attended`, `locked`, `inactive_session`, or `unknown`.

Not detected: Focus or Do Not Disturb (no public API; its files need Full Disk
Access), idle time, and display sleep. Apple Silicon has no
`IODisplayWrangler`, and Linux DPMS state is compositor-specific, so a sleeping
but unlocked display still counts as attended. Displays that lock on sleep are
caught as locked.

## Alert delivery

An alert waits for someone at this machine to click **Acknowledge**, so a
cascade such as `[alert, apprise]` escalates only when nobody responds.

| Platform | Runs |
| --- | --- |
| macOS | `osascript` running fixed AppleScript `display dialog … buttons {"Acknowledge"} giving up after N`; title, body, and N as argv |
| Linux | `notify-send --wait --urgency critical --expire-time 0 --action ack=Acknowledge --print-id -- TITLE BODY`; on expiry, `gdbus` `CloseNotification` by ID |
| other | `not_sent/unsupported_platform` |

| Result | Outcome | Exit |
| --- | --- | --- |
| Acknowledge clicked | `accepted`, JSON `acknowledged: true` | 0 |
| wait elapsed, dismissed, or closed without the action | `not_sent/unacknowledged`; the dialog or notification is removed | 3 |
| locked or inactive session ([presence](#desktop-delivery)) | `not_sent/unattended`, without showing or waiting | 3 |
| macOS process outside the GUI session, such as ssh (`-1713`) | `not_sent/no_user_interaction` | 3 |
| provider error | `failed/provider_failure` | 4 |
| provider start or teardown exceeds `--timeout`, or cancellation | `uncertain` | 5 |

`wait` defaults to `2m`; set it per step with `alert: {wait: 30s}`. The wait is
human time outside the shared `--timeout` budget: `--timeout` covers only
starting and tearing down the provider, and later cascade steps keep their
remaining budget. One alert step runs at most `wait` (rounded up to whole
seconds) plus that budget. Listener action timeouts still cap the whole step;
an MCP call blocks for the wait unless the client cancels it.

Dialogs work from launchd agents in the GUI session, including Noon jobs.
From ssh they fail with `-1713`; Tell reports that instead of asking for
Automation permission to drive System Events. Linux needs libnotify 0.7.10 or
newer; `tell doctor --via alert` checks it. Linux behavior is not yet verified
by hand.

## Apprise delivery

```text
apprise --title TITLE --body BODY --notification-type TYPE [--config FILE] [--tag TAG] [--input-format FORMAT]
```

Unset options are omitted, so Apprise defaults apply (without `--tag`, only
untagged services fire). No URL arguments; `APPRISE_*` variables pass through.
Tell reads at most 1 MiB of output and never prints it. Failures show the exit
code and fixed hints, which never change the outcome.

| Apprise exit | Tell outcome |
| --- | --- |
| 0 | accepted |
| 3 | not_sent/no_match |
| 4 | failed/partial |
| 5 | uncertain/timeout |
| other (including 1, 2) | failed/provider_failure |

To debug, run `tell doctor --via apprise` with the same overrides (no send),
then `apprise -vv` with a harmless message (sends; logs are sensitive).
Verified against Apprise `8227752a16c630536daefc387a0e546580fed5ae`; older
versions may report no match as exit 1.

## Shoutrrr delivery

Tell links [Shoutrrr](https://github.com/nicholas-fedor/shoutrrr) `v0.22.1`
in-process; no executable is needed, so it is the natural fallback where
Apprise is not installed. `shoutrrr.urls_env` (or a step's `urls_env`) names an
environment variable holding whitespace-separated Shoutrrr URLs. Tell reads it
at send time; there is no plain `url` key because the URL is the credential.
The title and body are sent; `--type` is not mapped to service levels.

| Condition | Tell outcome |
| --- | --- |
| variable unset or empty | not_sent/missing_urls |
| URL parse failure or unknown service | not_sent/invalid_config |
| every service succeeds | accepted |
| any service times out or is cancelled | uncertain/timeout or cancelled |
| any other service error | failed/provider_failure |

With several URLs some may have been delivered, so a partial failure is
`failed` (the same rule as Apprise) and does not advance a cascade unless
`fallback_on` lists it. Shoutrrr errors and logs can embed URLs and tokens, so
Tell never prints library text: results carry fixed messages and fixed hints
(TLS or connection error) chosen by error type, and all library logging is
discarded. `tell doctor --via shoutrrr` parses the URLs without connecting or
sending.

## iMessage delivery

On a Mac signed in to Messages, `imessage` texts one handle with nothing to
install. Tell runs `osascript` with a fixed script and passes the recipient and
text as argv, never as script source. The text is the redacted body, with the
title as its first line when set. `accepted` means Messages took the message,
not that it was delivered or read. Presence checks do not apply: a text should
reach the phone when nobody is at the Mac.

| Condition | Tell outcome |
| --- | --- |
| not macOS | not_sent/unsupported_platform |
| `to` missing or not an E.164 number or email | config error before sending |
| no enabled iMessage account | not_sent/no_account |
| Automation permission denied (`-1743`) | not_sent/automation_denied |
| Messages took the message | accepted |
| `--timeout` exceeded, cancelled, or Apple event timeout | uncertain |
| any other Messages error | failed/provider_failure |

`not_sent` outcomes advance a cascade such as `[desktop, imessage]`. Tell keeps
only the AppleScript error number; it never prints the recipient, body, or
Messages error text. `tell doctor --via imessage` reads the account list without
sending; that may open Messages and, on first use, show the Automation prompt.
A message to your own phone number alerted the phone in testing; Apple ID
email handles reach only devices that receive iMessage at that address.

Not supported: SMS fallback, group chats, attachments, several recipients per
step, delivery status (reading `chat.db` needs Full Disk Access), receiving
messages, and a listener source.

## Herdr delivery

Requires an explicit running `herdr.session` (`default` for the default
session). Tell never launches, attaches, or switches sessions.

```text
herdr --session SESSION notification show TITLE --body BODY --sound none
```

All inherited `HERDR_*` variables are removed. Herdr's parser scans message
argv with no escape, so Tell refuses (`not_sent/unsupported_message`) a title
or body equal to, or starting with `=` after, `--session`, `--remote`,
`--remote-keybindings`, or `--handoff`, and a title of `help`, `--help`, or
`-h`. Verified against Herdr 0.9.5.

| Case | Outcome |
| --- | --- |
| shown, or exit 0 with unparseable output | `accepted` |
| notifications disabled, rate limited, no foreground client, or busy | `not_sent/disabled`, `rate_limited`, `no_foreground_client`, or `busy` |
| `shown: false` with another reason | `not_sent/not_shown` |
| no session configured | `not_sent/missing_target` |
| no executable | `not_sent/missing_executable` |
| unreachable session, other nonzero exit | `failed/provider_failure` |

Herdr exits 0 even when it declines to show, so Tell reads `result.reason`
from its JSON (capped at 1 MiB, never printed) and a `not_sent` advances the
cascade. Herdr 0.9.5 still reports `shown` for a session with no attached
client, so a detached session is `accepted` until Herdr reports it.

## Command delivery

```yaml
# personal config only (never tell.yaml)
notify:
  via: command
  command: [/Users/me/.local/bin/tell-speak]
```

`tell notify --type warning "Disk 91% full"` in `~/dev/app` gives the child:

```text
stdin         Disk 91% full        (no trailing newline, then EOF)
TELL_TITLE    app
TELL_TYPE     warning
TELL_PROJECT  /Users/me/dev/app
```

Argv is passed unchanged: no shell, splitting, templates, or added arguments.
Bare names use PATH; relative paths use the current directory, so prefer
absolute paths. The child never gets Tell's stdin. Output is discarded and argv
is never logged. On timeout Tell kills only the direct child, so wrappers should
`exec`.

| Child result | Outcome |
| --- | --- |
| exit 0 | accepted |
| exit 3 (nothing sent) | not_sent/no_match |
| other nonzero | failed/provider_failure |
| executable missing | not_sent/missing_executable |
| other start failure | not_sent/start_failed |
| `notify.command` unset | config/invalid_config |

This is trusted personal execution, not a sandbox. Inspection never runs it.

## Agent delivery

`send.via` is `hcom` or `herdr`; `send.to` is one recipient, never inferred.
`request` intent creates hcom reply obligations; neither intent waits. Every
body starts with a provenance header that counts toward the 16 KiB cap:

```text
[Tell automation: tell-automation; route: hcom; intent: inform]
Tests failed; see noon logs 42
```

Each send prints one body-free receipt on stderr; human failures add an
explanation, and `--json` adds a stdout result:

```console
$ tell --json send --send-via hcom --to reviewer "hi"
tell send: route="hcom" recipient="reviewer" intent="inform" outcome=not_sent reason=missing_executable
{"schema_version":1,"redactions":0,"outcome":"not_sent","route":"hcom","recipient":"reviewer","intent":"inform","error":{"code":"not_sent","message":"provider executable was not found","reason":"missing_executable"}}
```

### hcom

```text
hcom list --json                                              # one exact active name
hcom send @NAME --from SENDER --intent INTENT --json -- MESSAGE
```

The explicit target and `--` keep body mentions from changing the recipient.
Broadcasts, multiple targets, tag selectors, and remote addresses are refused.
hcom delivers at the recipient's next hook. A target missing or a probe timeout
at check time is `not_sent`; exit 1 at send time is `failed`. Verified against
hcom `669a06703aad3b503d6eac0ce9fd744bf3f7c771`.

### Herdr

```text
herdr --session SESSION agent get NAME            # require idle or done
herdr --session SESSION agent prompt PANE_ID MESSAGE
```

| Case | Outcome |
| --- | --- |
| working or unknown | not_sent/busy |
| dialog, or `agent_blocked` from prompt | not_sent/blocked |
| agent or executable missing | not_sent |
| no `herdr.session` | not_sent/missing_session |
| `agent get` timed out | not_sent/timeout |
| provider error | failed |
| prompt timed out, malformed receipt | uncertain |

**The check and prompt are not atomic**: the agent can start working between
them. Use hcom when interruption matters.

### Recipient listing

```text
hcom list --json                                  # hcom
herdr --session SESSION session list --json       # Herdr: same check as doctor
herdr --session SESSION agent list
```

Each row is the exact `send.to` value, the agent kind, a status, and whether
send would accept it now. hcom lists active, listening, and blocked names
(all ready); a name listed twice is `duplicate` and not ready. Herdr lists the
agent name, or the pane ID for an unnamed agent; only `idle` and `done` are
ready, and `working`, `blocked`, or `unknown` are not. Only these fields are
read; titles, working directories, session paths, and other provider fields are
never printed. Kinds that are not short identifiers print as `unknown`.

Failures match `send` and `doctor` (missing executable, `missing_session`,
`session_not_running`, `timeout`, `invalid_response` for malformed or
oversized output) and exit as `not_sent`. A completed listing exits 0, even
with no recipients; `--json` prints one schema_version 1 object.

## Scheduler hooks

A hook turns scheduler context into Tell arguments:

```sh
tell notify --type failure "$JOB failed (run $RUN_ID)"   # hook stdin stays with the hook
```

Pass `-` only when stdin is the message. Tell's 10 s default fits Noon's 30 s
hook budget. Never rerun a business job to resend an alert. Services do not
inherit an interactive Herdr session, so set PATH, config, and targets
explicitly. See the [Noon example](../examples/noon/README.md).
