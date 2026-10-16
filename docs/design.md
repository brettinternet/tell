# Design

## Boundary

Tell is a short-lived CLI between repo automation and delivery tools:

```text
human / script / Noon job
          │
         tell
          ├── notify: stdout / Apprise desktop or external destinations / Herdr
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
- notify requests human attention. stdout is the safe built-in default.
- send delivers a message to a participating hcom agent.

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
