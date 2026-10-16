# Design

## Boundary

Tell is a short-lived CLI between repo automation and delivery tools:

```text
human / script / Noon job
          │
         tell
          ├── notify: stdout / desktop / Herdr / Apprise (external services)
          └── send: hcom participant
```

The configuration and inspection foundation is implemented. The adapters and
commands below are planned, not existing interfaces. Backlog.md is the work
queue; this document records the intended boundaries, not a second task list.

Noon owns schedules, queued runs, concurrency, cancellation, and run history.
Repo scripts decide what events deserve notification and what agent work means.
Tell owns routing for one invocation, safe argv, bounded subprocess execution,
and honest dispatch results. It owns no message database or background worker.

## Commands and semantics

Planned commands retain the conversation's separation:
- notify requests human attention. stderr is the safe built-in default; it
  keeps a calling script's stdout data and --json output intact.
- send delivers a message to an existing agent through hcom or Herdr.

Send carries the sender's own information or instructions. Instructions are
legitimate when a person or their scheduled job authors them; Noon owns when
they run, missed runs, and overlap. Messages carry pointers (run IDs, paths,
URLs) rather than raw job output, so agents read untrusted data through their
own tools. Tell never waits for a reply or for work to finish: tracked agent
work belongs to Noon agent jobs, and ask-and-wait flows belong to Unharness.

Notify routes are thin built-ins plus one advanced provider. Built-ins cover the
common personal cases with no extra runtime: stderr; desktop through osascript
on macOS or notify-send on Linux; and an explicit Herdr session. Apprise handles
fan-out to external services and owns their destinations and credentials. A
user-owned command route runs a personal executable with a fixed contract,
without shell strings or a plugin registry.

Before any route runs, titles and bodies pass one preparation step: control
characters removed, lengths capped with a visible marker, and secrets redacted
with an embedded scanner ruleset. Tell is the last point before third-party
services and agent transcripts persist a message.

Delivery destinations are personal. Routing fields (notify.via, apprise.config,
herdr.session, hcom.to) are planned to come only from user config, private
.tell.yaml, --config, environment, or flags. Shared tell.yaml keeps shared
semantics such as the Apprise tag and format. Repository scripts decide what
happened; personal configuration decides where it goes.

## Outcomes

Every delivery invocation (planned) reports exactly one outcome. Callers decide
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
missing_executable, unsupported_platform, no_match, partial, timeout,
cancelled, or secret_detected. Results never include the message body or raw provider output.
Unknown provider exit codes map to failed, never accepted.

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

Per-repo routing is trusted policy, not a sandbox. Do not add shared-config
executable overrides, plugin-loading paths, command templates, or raw shell
strings. Provider subprocesses receive argv, never interpolated shell code.
Logs and errors must not echo credentials or raw message contents.

## Noon integration

Noon's current lifecycle hooks receive environment values such as NOON_JOB,
NOON_STATUS, NOON_RUN_ID, NOON_PROJECT and NOON_LOG_FILE, plus a JSON run summary
on stdin. A small repo-owned script can translate those into Tell arguments once
delivery exists; no Noon-specific protocol is needed in Tell.

Noon's hooks have a 30-second budget and separate hook status from job status.
Tell's default 10-second delivery timeout leaves room for hook supervision.
A failed hook is not a durable notification-redelivery mechanism; do not rerun a
successful business job to resend an alert.

Scheduled services do not inherit an interactive Herdr session or agent identity.
Resolve executable paths/configuration from explicit per-user setup and pass
session/recipient targets deliberately. For actual scheduled agent work with
completion tracking, use Noon's agent execution, not successful hcom dispatch
as a proxy for completion.

## Implementation shape

cmd/tell remains the process boundary. internal/cli handles arguments and
rendering; internal/config owns strict file resolution. Add a concrete delivery
package only with its first adapter. Reuse a subprocess helper when a second
adapter needs the same lifecycle contract. No interface registry for three
known tools.

Verification should use real temporary files and local fixture executables.
Critical tests cover argument preservation, config provenance, process timeout/
cancellation, provider exit failures, and redaction. Never send real messages
from the automated suite.
