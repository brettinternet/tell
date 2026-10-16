# Noon → repo hook → Tell

Copy [notify.sh](notify.sh) to your repo's `scripts/noon-notify.sh` and merge the
job-level hooks from [noon.yaml](noon.yaml). Replace the absolute script paths
and job command. The hook needs only POSIX `/bin/sh` and Tell. Review/approve
the resulting Noon job before scheduling it.

Requires Noon with NOON-31 (`2d3d376` or later), which supplies
`NOON_PREVIOUS_STATUS`: the last retained succeeded/failed/timed_out result for
the same project/job, snapshotted before this run finishes. Update older Noon
installations first; without this variable the hook stays quiet. The hook reads
Noon's environment, not its JSON stdin, and does not query history itself.
Only success → failure (including timeout) and failure → recovery notify.
Repeated failures, a change from failed to timed_out, and repeated successes
do not. With no predecessor (first run or pruned history), it sends nothing:
establish a baseline first. Cancelled/skipped/interrupted runs are not health
transitions. This unknown-history policy deliberately differs from Noon's
example wrappers, which treat unknown history as healthy.

Use serial jobs (`overlap: skip`) and avoid concurrent manual runs of the same
job if you need one ordered health sequence. Hooks execute outside scheduler
concurrency limits. This is best-effort transition reporting, not a durable
alert queue or exactly-once delivery. Missing current context skips with a
generic diagnostic; no local state file is created.

Messages include job, status, run ID, project, and a `noon logs RUN` pointer.
They never open the log file or copy excerpts, which could leak credentials to
Apprise's third-party destinations. Tell's redaction is extra protection, not a
reason to forward logs. The JSON hook summary is never the message.

## Service setup

Services do not inherit your interactive shell setup. In the **Noon service's**
environment, explicitly configure:

- `PATH` containing the installed `noon`, `tell`, job tools, and provider
  executable (`apprise`, `herdr`, or `notify-send`), plus system directories.
  Use stable installation paths, not an interactive shell alias or activation.
- `HOME` for the service user and, if used, `XDG_CONFIG_HOME`. Tell reads
  `$XDG_CONFIG_HOME/tell/config.yaml`, otherwise `$HOME/.config/tell/config.yaml`.
- Personal routing and provider-owned credentials. For example:

  ```yaml
  # service user's ~/.config/tell/config.yaml
  version: 1
  notify:
    via: desktop
  ```

For Apprise use `notify.via: apprise` and personal `apprise.config` pointing to
Apprise's native config. Keep its tokens there, never in this hook or shared
Tell config. For Herdr use `notify.via: herdr` and explicit
`herdr.session: work` (or explicitly `default`). The session must already be
running under the service user. Tell neither discovers nor starts sessions,
and ignores inherited `HERDR_*` context. Do not infer agent identity from a
service environment.

On Linux, install `notify-send` and supply the actual user's desktop-session
`DBUS_SESSION_BUS_ADDRESS` to the service. Tell does not discover D-Bus.
On macOS, run in the logged-in user's GUI session, allow Script Editor
notifications, and ensure `/usr/bin/osascript` is available. Focus / Do Not
Disturb can hide accepted banners; system daemons are not desktop sessions.

Verify with the **same user, working directory, PATH, HOME/XDG and desktop
environment as the service**, not just your interactive shell:

```sh
tell -C /absolute/path/to/repo doctor
tell -C /absolute/path/to/repo --json doctor
```

Doctor sends nothing. It checks executable/config readiness, not reachability,
credentials, GUI permission, delivery, or human acknowledgment. Afterward,
optionally send a clearly labeled test notification yourself. Bare `tell`
only inspects settings. For a service with an explicit Herdr target:

```sh
tell -C /absolute/path/to/repo --via herdr --herdr-session work doctor
```

Top-level Noon hook defaults belong only in user config or an explicit
`--config` file, never shared/private project Noon files. Job-level hooks may
live in `noon.yaml`, as this example does. Hook failures do not change the job's
recorded status or foreground exit code.

## Budgets and failures

Noon allows 30 seconds for a hook, then TERM to its process group, five seconds
of grace, then KILL. Tell's delivery budget is explicitly 10 seconds (also its
default), leaving room for startup, preparation, and cleanup inside Noon's hook
budget. No history subprocess is needed. Do not raise Tell's timeout past the
outer budget. Nothing automatically retries.

Tell exit codes are: **0** accepted (not acknowledgment), **1** input/config,
**2** usage, **3** not sent, **4** provider failure (possibly partial), **5**
uncertain after start. Fix setup for 1–3; inspect 4–5 before deciding whether
resending is safe. The hook ignores delivery exit codes with
`tell notify "context" || true`, returning success even if delivery fails. Noon can append diagnostics to its run log, but job outcome stays
separate from notification outcome. **Never rerun a business job because its
notification failed.** See [Tell outcomes](../../docs/design.md#outcomes).

## Isolated verification

`task smoke` builds Tell and runs the hook with a sanitized service-style
environment, synthetic Noon previous-status context/JSON stdin, and a local
capture provider. It exercises transitions, unknown history, literal context, and
notification failure without contacting Noon services or real destinations.
