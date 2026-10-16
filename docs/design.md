# Design

## Boundary

Tell is a short-lived CLI between repo automation and delivery tools:

```text
human / script / scheduler hook
          │
         tell
          ├── notify: stderr / desktop / Herdr / Apprise (external services)
          └── send: hcom participant
```

Configuration, inspection, and notify via stderr and Apprise are implemented.
Message normalization is implemented. Desktop, Herdr, user-owned commands,
send, and secret redaction remain planned.
Backlog.md is the work queue; this document records boundaries, not a second task list.

The caller's scheduler (cron, launchd, CI, or Noon) owns schedules, queued runs,
concurrency, cancellation, and run history. Repo scripts decide what events
deserve notification and what agent work means. Tell owns routing for one
invocation, safe argv, bounded subprocess execution, and honest dispatch
results. It owns no message database or background worker, and it never reads
scheduler-specific environment or payloads: callers pass arguments or explicit
stdin.

Tell does not throttle, deduplicate, or rate-limit. That needs state shared
across invocations, only the caller knows whether a message repeats, and a
dropped alert costs more than a duplicate. Callers own it, for example a
scheduler hook that notifies only when a job's status changes.

## Commands and semantics

Commands retain the conversation's separation (send remains planned):
- notify requests human attention. stderr is the safe built-in default; it
  keeps a calling script's stdout data and --json output intact.
- send delivers a message to an existing agent through hcom or Herdr.

Send carries the sender's own information or instructions. Instructions are
legitimate when a person or their scheduled job authors them; the scheduler
owns when they run, missed runs, and overlap. Messages carry pointers (run IDs,
paths, URLs) rather than raw job output, so agents read untrusted data through
their own tools. Tell never waits for a reply or for work to finish: tracked
agent work belongs to a scheduler's agent jobs (such as Noon's), and
ask-and-wait flows belong to Unharness.

Notify routes are thin built-ins plus one advanced provider. Built-ins cover the
common personal cases with no extra runtime: stderr; desktop through osascript
on macOS or notify-send on Linux; and an explicit Herdr session. Apprise handles
fan-out to external services and owns their destinations and credentials. A
user-owned command route runs a personal executable with a fixed contract,
without shell strings or a plugin registry.

Message preparation processes titles and bodies before any route runs (see below).
Secret redaction with an embedded scanner ruleset remains planned. Tell is the
last point before third-party services and agent transcripts persist a message.

Delivery destinations are personal. Routing fields (notify.via, apprise.config,
herdr.session, hcom.to) now come only from user config, private
.tell.yaml, --config, environment, or flags. Shared tell.yaml keeps shared
semantics such as the Apprise tag and format. Repository scripts decide what
happened; personal configuration decides where it goes.

## Message preparation

Every notification route receives the same prepared title and body, whether from
literal arguments, explicit stdin, or the default project-directory title:

- NUL anywhere in either original field rejects the invocation with exit 1
  (`config/invalid_message`), even beyond the truncation limit. Nothing is sent.
- C0 controls (U+0001–U+001F), DEL, and C1 controls (U+0080–U+009F) are removed,
  except newline (LF) and tab. CR is removed, so CRLF becomes LF.
- Terminal escape sequences are removed in full: CSI, OSC, DCS, SOS, PM, APC,
  and ordinary ESC sequences, including their C1 forms. Control strings end at
  BEL or ST; incomplete sequences are discarded through end of input.
- Unicode bidi embedding/override controls U+202A–U+202E and isolate controls
  U+2066–U+2069 are removed. Other Unicode text is preserved; malformed UTF-8
  is replaced with U+FFFD.
- After filtering, titles are capped at **256 bytes**, bodies at **16,384 bytes**.
  Truncation keeps a UTF-8 boundary and appends **`[truncated]`** within that limit.
  These are output limits, not limits on bytes read from stdin.
- A body empty or whitespace-only after preparation fails with exit 1
  (`config/empty_message`) before dispatch.

No Markdown/HTML sanitization, secret scanning, or other reformatting occurs.

## Outcomes

Every notify invocation reports exactly one outcome. Callers decide
whether to resend; the outcome says whether that is safe.

| Exit | Outcome | Meaning | Resend? |
| --- | --- | --- | --- |
| 0 | accepted | The route accepted the message: printed, posted, or the provider exited 0. Not proof anyone read or acted on it. | No |
| 1 | config | Configuration, input, or I/O error before delivery started. Nothing sent. | After fixing |
| 2 | usage | Invalid invocation. Nothing sent. | After fixing |
| 3 | not_sent | Route unavailable: executable missing, platform unsupported, target absent, or the provider matched no destination. Nothing sent. | Yes, or try another route |
| 4 | failed | The provider ran and reported failure. With reason partial, some destinations received it. | Not blindly |
| 5 | uncertain | Timed out or cancelled after the provider started. It may have been delivered. | Not without checking |

With --json, stdout receives one object with schema_version, outcome, and route;
failures add error.code (the outcome), error.message, and a reason such as
missing_executable, unsupported_route, no_match, partial, timeout,
or cancelled. Planned routes may add unsupported_platform or secret_detected. Results never include the message body or raw provider output.
Unknown provider exit codes map to failed, never accepted. The route is empty
when invocation parsing or configuration fails before route resolution.
The built-in stderr route writes the message there even in JSON mode; stdout
contains only the result. A stderr write error is failed because a partial write
may already have occurred. If writing an accepted JSON result fails, Tell warns
on stderr and retains exit 0: output failure does not undo acceptance and must
not invite a retry. A broken stdout cannot carry the promised JSON object.
Cancellation while reading explicit stdin exits config/cancelled before delivery.

Apprise is an optional external executable. Tell maps its exits 0 to accepted,
3 to not_sent/no_match, 4 to failed/partial, and 5 to uncertain/timeout; all other
exits (including 1 and 2) are failed/provider_failure. This mapping was verified
against Apprise `8227752a16c630536daefc387a0e546580fed5ae`, `apprise/result.py`
and `apprise/cli.py`. Older versions may report generic failure for no match;
Tell does not infer success or safety from provider text. No destinations loaded
can also produce exit 1. Provider output is discarded, never echoed.

An accepted provider invocation does not prove a human read the notification,
nor that an agent completed work. hcom recipients must participate in hcom;
the wrapper must not silently target an arbitrary running terminal.

No automatic retries, agent spawning, fallback to another session, fan-out,
ACP/MCP server, or generic plugin registry in the initial delivery implementation.
Timeout/cancellation may leave delivery uncertain; never relabel that success
or blindly retry consequential agent instructions.

## Configuration ownership

Use the implemented [configuration contract](configuration.md). Keep native
provider configuration in that provider. Apprise owns destinations, credentials,
and tag expression semantics. Tell forwards explicitly selected common options;
unset provider options defer to provider behavior. Native provider files are not
parsed or merged by Tell.

Private routing and explicit invocation inputs are trusted policy, not a sandbox.
Shared tell.yaml rejects personal routing; no system layer is loaded.
Do not add shared-config
executable overrides, plugin-loading paths, command templates, or raw shell
strings. Provider subprocesses receive argv, never interpolated shell code.
Logs and errors must not echo credentials or raw message contents.

## Scheduler hooks

Schedulers call Tell like any script. A small hook script translates the
scheduler's context (job, status, run ID) into Tell arguments; Tell has no
scheduler-specific protocol. Literal notify arguments do not consume a hook's
stdin, so payloads such as Noon's JSON run summary stay with the hook.
Explicitly choose `-` only when its contents are the message.

Keep delivery inside the hook's budget: Noon allows 30 seconds, and Tell's
default 10-second delivery timeout leaves room for hook supervision.
A failed hook is not a durable notification-redelivery mechanism; do not rerun a
successful business job to resend an alert.

Scheduled services do not inherit an interactive Herdr session or agent identity.
Resolve executable paths/configuration from explicit per-user setup and pass
session/recipient targets deliberately. For scheduled agent work with
completion tracking, use the scheduler's agent execution (such as Noon agent
jobs), not successful hcom dispatch as a proxy for completion.

## Implementation shape

cmd/tell remains the process boundary. internal/cli handles arguments and
rendering; internal/config owns strict file resolution. internal/delivery owns
stderr, Apprise, dispatch outcomes, and bounded subprocess execution. No interface registry for three
known tools.

Verification should use real temporary files and local fixture executables.
Critical tests cover argument preservation, config provenance, process timeout/
cancellation, provider exit failures, and redaction. Never send real messages
from the automated suite.
