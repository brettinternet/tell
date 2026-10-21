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
services; command runs one personal executable.

`send` gives one existing agent information (`--intent inform`) or an
instruction (`--intent request`) over one transport. It never falls through to
another agent and never waits for a reply. Tracked agent work belongs to the
scheduler's agent jobs (such as Noon's); ask-and-wait belongs to Unharness.
Listener routes may only `inform`, because event authors are untrusted.

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
reason. Labels are the Apprise tag, config basename, or `urls_env` name, never
URLs. JSON receipts list attempts:

```json
{"attempts":[{"route":"apprise","label":"apprise:phone","outcome":"not_sent","reason":"no_match"},{"route":"desktop","label":"desktop","outcome":"accepted","reason":""}]}
```

Apprise tags are ordered alternatives, not comma-separated fan-out. A tagged
step removes inherited `APPRISE_URLS` so the tag is honored. `urls_env` passes
a variable's value only as the child's `APPRISE_URLS`. Tell never parses
Apprise config or URLs.

`tell doctor` lists steps in order and is ready if any step is. Listener
`notify` actions without `via` use the cascade within the action timeout.

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
| exit codes | stable; 0 never means acknowledged, 5 never means safe to resend |
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
| 0 | accepted | printed, posted, or provider exited 0; not proof anyone read it | no |
| 1 | config | config, input, or I/O error before delivery | after fixing |
| 2 | usage | invalid invocation | after fixing |
| 3 | not_sent | missing executable, unsupported platform, absent/invalid target, busy/blocked agent, no match | yes, or another route |
| 4 | failed | provider ran and failed; `partial` means some destinations got it | not blindly |
| 5 | uncertain | timed out or cancelled after the provider started | not without checking |

```console
$ tell --json notify --via apprise "Deploy done"
{"schema_version":1,"redactions":0,"outcome":"not_sent","route":"apprise","error":{"code":"not_sent","message":"provider executable was not found","reason":"missing_executable"}}
```

Reasons: `missing_executable`, `start_failed`, `unsupported_route`,
`unsupported_platform`, `unsupported_message`, `missing_route`,
`missing_target`, `missing_session`, `invalid_target`, `invalid_response`,
`no_match`, `busy`, `blocked`, `partial`, `provider_failure`, `timeout`,
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
(`DBUS_SESSION_BUS_ADDRESS`), which Tell inherits but does not discover. Focus
or Do Not Disturb can hide an accepted banner.

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
| no session configured | `not_sent/missing_target` |
| no executable | `not_sent/missing_executable` |
| unreachable session, other nonzero exit | `failed/provider_failure` |

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

## Scheduler hooks

A hook turns scheduler context into Tell arguments:

```sh
tell notify --type failure "$JOB failed (run $RUN_ID)"   # hook stdin stays with the hook
```

Pass `-` only when stdin is the message. Tell's 10 s default fits Noon's 30 s
hook budget. Never rerun a business job to resend an alert. Services do not
inherit an interactive Herdr session, so set PATH, config, and targets
explicitly. See the [Noon example](../examples/noon/README.md).
