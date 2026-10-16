# Design

## Boundary

```text
person / script / scheduler hook
          │
         tell  (one invocation, then exit)
          ├── notify → stderr | desktop | Herdr | Apprise → services | your command
          └── send   → hcom mailbox | idle Herdr agent pane
```

| Owner | Owns |
| --- | --- |
| Scheduler (cron, launchd, CI, Noon) | schedules, queues, concurrency, cancellation, run history |
| Repo scripts | which events matter, what agent work means |
| Tell | routing, safe argv, bounded subprocesses, honest results |
| Provider | destinations, credentials, native config |

Tell keeps no state. It never reads scheduler-specific environment or payloads;
callers pass arguments or explicit stdin.

Tell does not throttle, deduplicate, or rate-limit. That needs state across
invocations, only the caller knows whether a message repeats, and a dropped
alert costs more than a duplicate. A hook that notifies only on status change
is the caller's job (see [examples/noon](../examples/noon/README.md)).

There are no retries, fallback routes, fan-out, agent spawning, plugin
registry, or ACP/MCP server. Backlog.md tracks planned work; this document
describes what exists.

## Commands

`notify` asks a person for attention. The default route, stderr, keeps a
script's stdout and `--json` output clean. Desktop and Herdr cover common
personal cases with no extra runtime. Apprise fans out to external services.
The command route runs one personal executable with a fixed contract.

`send` gives an existing agent information (`--intent inform`) or an
instruction (`--intent request`). Instructions are legitimate when a person or
their scheduled job wrote them. Send pointers (run IDs, paths, URLs), not raw
job output, so agents read untrusted data through their own tools. Tell never
waits for a reply or for the work to finish. Tracked agent work belongs to the
scheduler's agent jobs (such as Noon's); ask-and-wait belongs to Unharness.

## Compatibility

The first published stable release establishes the compatibility contract.
Within config `version: 1`, existing valid keys retain their meaning and
precedence/trust boundaries; incompatible configuration changes require a new
config version and migration guidance. Unknown keys remain errors.

JSON `schema_version: 1` preserves existing field types and meanings.
Additive fields may appear; consumers should ignore unknown fields and handle
unknown reason/route values conservatively. Removing or changing an existing
field incompatibly requires a new schema version.

The numeric exit codes and delivery guarantees in [Outcomes](#outcomes) are
stable: notably, 0 never means human acknowledgment and 5 never means safe
to resend. Incompatible CLI/exit behavior requires a new major release and
migration notes. Human-readable prose and diagnostic wording are not an API.
Security fixes may tighten validation or redaction without preserving unsafe
behavior. External provider behavior and availability remain outside Tell's
compatibility guarantee; Tell never turns an uncertain provider outcome into
success.

## Message preparation

Every route gets the same prepared title and body, in this order:

1. **Reject NUL** anywhere in the original text: exit 1, `config/invalid_message`.
2. **Strip controls**: C0, DEL, C1, and CR (so CRLF → LF). Newline and tab stay.
3. **Strip terminal sequences** in full: CSI, OSC, DCS, SOS, PM, APC, and plain
   ESC sequences, including C1 forms. Strings end at BEL or ST; an unterminated
   sequence is dropped to end of input.
4. **Strip bidi controls** U+202A–U+202E and U+2066–U+2069. Invalid UTF-8
   becomes U+FFFD. Other text is untouched; no Markdown/HTML sanitizing.
5. **Scan for secrets** with the embedded Gitleaks v8 default rules, offline.
   Matches become `[REDACTED]` wherever the value appears in the title or body. With personal `secrets.policy: reject`, exit 1
   `config/secret_detected` instead. A scanner error fails closed
   (`config/secret_scan_failed`). `gitleaks:allow` does not bypass it. No local
   rules, ignore files, or network checks.
6. **Truncate** titles to 256 bytes and bodies to 16,384 bytes on a UTF-8
   boundary, ending with `[truncated]` inside the limit. These cap output, not
   stdin reads.
7. **Reject empty**: a blank body exits 1, `config/empty_message`.

```console
$ tell --json notify "key $AWS_ACCESS_KEY_ID done"
key [REDACTED] done
{"outcome":"accepted","redactions":1,"route":"stderr","schema_version":1}
```

Scanning runs before truncation so a secret at the limit is not leaked as a
partial token. `redactions` counts replaced occurrences across both fields,
including ones later truncated. `reject` reports zero. Detection uses patterns,
entropy, and allowlists, so it has false positives and misses. It is not proof
a message is safe.

## Outcomes

| Exit | Outcome | Meaning | Resend? |
| --- | --- | --- | --- |
| 0 | accepted | Printed, posted, or the provider exited 0. Not proof anyone read or acted on it. | No |
| 1 | config | Configuration, input, or I/O error before delivery. Nothing sent. | After fixing |
| 2 | usage | Invalid invocation. Nothing sent. | After fixing |
| 3 | not_sent | Executable missing, platform unsupported, target absent/invalid, agent busy/blocked, or no destination matched. Nothing sent. | Yes, or another route |
| 4 | failed | The provider ran and failed. Reason `partial` means some destinations got it. | Not blindly |
| 5 | uncertain | Timed out or cancelled after the provider started. It may have arrived. | Not without checking |

```console
$ tell --json notify --via apprise "Deploy done"
{"schema_version":1,"redactions":0,"outcome":"not_sent","route":"apprise","error":{"code":"not_sent","message":"provider executable was not found","reason":"missing_executable"}}
```

Reasons include `missing_executable`, `start_failed`, `unsupported_route`,
`unsupported_platform`, `unsupported_message`, `missing_route`,
`missing_target`, `missing_session`, `invalid_target`, `invalid_response`,
`no_match`, `busy`, `blocked`, `partial`, `provider_failure`, `timeout`,
`cancelled`, `secret_detected`, and `secret_scan_failed`. Results never include the body or
provider output. Unknown provider exits are `failed`, never `accepted`.

- Cancellation before the provider starts, or while reading stdin, is `config`.
- The stderr route writes the message to stderr even with `--json`; stdout holds
  only the result. A stderr write error is `failed`, since part may be written.
- If writing an accepted JSON result fails, Tell warns on stderr and keeps exit 0.
  Output failure does not undo delivery and must not invite a resend.

Never relabel `uncertain` as success or blindly resend a consequential agent
instruction.

## Stderr

Writes the body to stderr. Nothing else.

## Desktop delivery

| Platform | Runs | Type |
| --- | --- | --- |
| macOS | `osascript` with fixed AppleScript; title and body as separate argv | ignored |
| Linux | `notify-send --urgency LEVEL -- TITLE BODY` | `failure` → critical, else normal |
| other | — | `not_sent/unsupported_platform` |

Text is never interpolated into code. On macOS, banners appear as **Script
Editor**; allow its notifications in System Settings. On Linux, install
libnotify's `notify-send` and run inside a desktop session. Scheduled services
need the user's session D-Bus, usually `DBUS_SESSION_BUS_ADDRESS`; Tell inherits
it and does not discover it.

Both tools resolve through PATH. Stdin is closed and output discarded.
Focus / Do Not Disturb can hide an accepted banner.

## Apprise delivery

Requires `apprise` on PATH. Apprise owns destinations, credentials, and tag
semantics; Tell never opens its config file.

```text
apprise --title TITLE --body BODY --notification-type TYPE [--config FILE] [--tag TAG] [--input-format FORMAT]
```

Unset options are omitted so Apprise defaults apply (without `--tag`, only
untagged services are notified). No URL arguments. Native `APPRISE_*`
variables pass through uninterpreted. Stdin is closed and output discarded.

| Apprise exit | Tell outcome |
| --- | --- |
| 0 | accepted |
| 3 | not_sent/no_match |
| 4 | failed/partial |
| 5 | uncertain/timeout |
| other (including 1, 2) | failed/provider_failure |

Verified against Apprise `8227752a16c630536daefc387a0e546580fed5ae`
(`apprise/result.py`, `apprise/cli.py`). Older versions may report no match as
a generic failure, and no loaded destinations can also exit 1. Tell never
infers success from provider text.

## Herdr delivery

Requires `herdr` on PATH and an explicit, already-running `herdr.session`
(use `default` for the default session). Tell never launches, attaches,
creates, discovers, or switches sessions.

```text
herdr --session SESSION notification show TITLE --body BODY --sound none
```

Every inherited `HERDR_*` variable is removed, including socket, session, pane,
remote, and config-path overrides. HOME and XDG_CONFIG_HOME still locate the
user's Herdr config. Type does not change sound or severity.

Herdr's global parser also scans message argv and has no positional escape.
So Tell refuses, as `not_sent/unsupported_message`, a title or body that equals
`--session`, `--remote`, `--remote-keybindings`, or `--handoff` or starts with
one followed by `=`, and a title equal to `help`, `--help`, or `-h`. The same
strings inside longer text are fine. Verified against Herdr 0.9.5
(`src/main.rs`, `src/session.rs`, `src/cli/notification.rs`): explicit
`--session` beats the socket environment, and notification dispatch never
enters the launch path.

| Case | Outcome |
| --- | --- |
| no session configured | `not_sent/missing_target` |
| no executable | `not_sent/missing_executable` |
| session unreachable, other nonzero exit | `failed/provider_failure` |

## Command delivery

Runs one personal executable. Configure argv as a list in user config,
`.tell.yaml`, or `--config` (never `tell.yaml`):

```yaml
notify:
  via: command
  command: [/Users/me/.local/bin/tell-speak]
```

```sh
#!/bin/sh
# /Users/me/.local/bin/tell-speak (chmod +x); speaks the body on macOS
exec /usr/bin/say -f -
```

The child sees:

```text
stdin         prepared body, no trailing newline, then EOF
TELL_TITLE    prepared title
TELL_TYPE     info | success | warning | failure
TELL_PROJECT  canonical absolute project directory
```

For `tell notify --type warning "Disk 91% full"` in `~/dev/app`:

```text
TELL_TITLE=app
TELL_TYPE=warning
TELL_PROJECT=/Users/me/dev/app
stdin: Disk 91% full
```

- argv is passed unchanged: no shell, splitting, templates, or added arguments.
  Bare names resolve through PATH. Relative paths resolve against the current
  directory, not the config file or project, so prefer absolute paths.
- The child never inherits Tell's stdin, even with `notify -`. The three
  variables override inherited ones of the same name.
- Output is discarded. Tell never logs argv. Keep credentials in the provider's
  own files or environment, not in argv.
- Tell kills the direct child on timeout, not its process tree. Wrappers should
  `exec` the provider and must not background deliveries.

| Child result | Outcome |
| --- | --- |
| exit 0 | accepted |
| exit 3 (promise: nothing sent) | not_sent/no_match |
| any other nonzero, including 5 | failed/provider_failure |
| executable missing | not_sent/missing_executable |
| other start failure | not_sent/start_failed |
| `notify.command` unset | config/invalid_config |

This is trusted personal execution, not a sandbox. Only `notify` runs it;
inspection never does.

## Agent delivery

`send.via` is `hcom` or `herdr`; `send.to` is exactly one recipient. Nothing is
inferred from the caller's agent context. `--intent` defaults to `inform`;
`request` is explicit and creates hcom reply obligations. Neither waits.

Every body starts with a provenance header:

```text
[Tell automation: tell-automation; route: hcom; intent: inform]
Tests failed; see noon logs 42
```

The header counts toward the 16 KiB body cap. Normalization and secret policy
apply as for notify. The header records origin; it does not make content
trusted.

Each invocation prints one body-free receipt to stderr; `--json` adds a result
on stdout:

```console
$ tell send "Tests failed; see noon logs 42"
tell send: route="hcom" recipient="reviewer" intent="inform" outcome=accepted

$ tell --json send --send-via hcom --to reviewer "hi"
tell send: route="hcom" recipient="reviewer" intent="inform" outcome=not_sent reason=missing_executable
{"schema_version":1,"redactions":0,"outcome":"not_sent","route":"hcom","recipient":"reviewer","intent":"inform","error":{"code":"not_sent","message":"provider executable was not found","reason":"missing_executable"}}
```

Unresolved fields are empty in the receipt and may be omitted from JSON. No
subprocess inherits Tell's stdin. A broken output stream cannot undo dispatch.
Bare `tell send` sends nothing.

### hcom

Tell checks `hcom list --json` for one exact active name, then runs:

```text
hcom send @NAME --from SENDER --intent INTENT --json -- MESSAGE
```

The explicit target and `--` stop stdin and body mentions from changing the
recipient; MESSAGE is one argv element. Broadcasts, multiple targets, tag
selectors, and remote addresses are refused. hcom delivers at the recipient's
next hook without steering its current turn.

hcom exit 1 covers both provider errors and a participant that vanished. A
target missing at probe time is `not_sent`; exit 1 at send time is
conservatively `failed`. Verified against hcom
`669a06703aad3b503d6eac0ce9fd744bf3f7c771` (`send.rs`, `list.rs`); hcom was not
installed during development.

### Herdr

Requires `herdr.session`. Against Herdr 0.9.5:

```text
herdr --session SESSION agent get NAME            # require agent_status idle or done
herdr --session SESSION agent prompt PANE_ID MESSAGE
```

| Case | Outcome |
| --- | --- |
| working or unknown | not_sent/busy |
| blocked on a dialog, or `agent_blocked` from prompt | not_sent/blocked |
| agent or executable missing | not_sent |
| no `herdr.session` | not_sent/missing_session |
| provider error | failed |
| timeout after start, malformed prompt receipt | uncertain |

**The check and the prompt are not atomic.** The agent can start working in
between, and the typed turn may steer it. Use hcom when interruption matters.
`HERDR_*` variables are removed. Tell never starts or resumes agents.

## Scheduler hooks

A small hook script turns scheduler context (job, status, run ID) into Tell
arguments. Literal message arguments never consume the hook's stdin, so
payloads like Noon's JSON run summary stay with the hook; pass `-` only when
stdin is the message.

Keep delivery inside the hook budget: Noon allows 30 seconds, and Tell's 10s
default leaves room for startup and cleanup. A failed hook is not a redelivery
mechanism; never rerun a business job to resend an alert.

Services do not inherit an interactive Herdr session or agent identity. Set
PATH, config, and targets explicitly per user. For agent work that needs
completion tracking, use the scheduler's agent jobs, not hcom dispatch.

See the [Noon example](../examples/noon/README.md).

## Implementation

| Package | Owns |
| --- | --- |
| `cmd/tell` | process boundary |
| `internal/cli` | arguments, rendering |
| `internal/config` | strict file resolution |
| `internal/delivery` | routes, outcomes, bounded subprocesses |

Routes are plain subprocess adapters with no interface registry. Tests use real
temp files and local fixture executables, and never send real messages.
