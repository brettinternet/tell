# Noon → repo hook → Tell

Two patterns: notify a person when a job breaks or recovers, and send an agent
a scheduled instruction.

## Failure and recovery notifications

1. Copy [notify.sh](notify.sh) to your repo as `scripts/noon-notify.sh`.
2. Merge the hooks from [noon.yaml](noon.yaml) and replace the absolute paths
   and job command.
3. Review and approve the Noon job.

```yaml
jobs:
  checks:
    run: [task, test]
    schedule: every 1h
    overlap: skip
    on_success: [/bin/sh, /absolute/path/to/repo/scripts/noon-notify.sh]
    on_failure: [/bin/sh, /absolute/path/to/repo/scripts/noon-notify.sh]
```

The hook needs only POSIX `/bin/sh` and Tell. It compares Noon's
`NOON_PREVIOUS_STATUS` with `NOON_STATUS` and notifies only on a transition:

| Previous → current | Notifies |
| --- | --- |
| succeeded → failed / timed_out | failure |
| failed / timed_out → succeeded | success |
| failed → failed, failed → timed_out, succeeded → succeeded | no |
| none (first run or pruned history) | no; establish a baseline first |
| cancelled, skipped, interrupted | no |

This deliberately treats unknown history as unknown, unlike Noon's example
wrappers, which treat it as healthy. It requires Noon with NOON-31 (`2d3d376`
or later); older Noon lacks `NOON_PREVIOUS_STATUS`, so the hook stays quiet.

The message is a pointer, never log content:

```text
Job: checks
Status: failed
Run: 42
Project: /Users/me/dev/app
Logs: run noon logs 42 from the project directory.
```

The hook reads Noon's environment, never its JSON stdin or log file, so neither
can leak to Apprise's third-party destinations. Tell's redaction is a backstop,
not a reason to forward logs. Missing context skips with a generic diagnostic.
No state file is created.

Use serial jobs (`overlap: skip`) and avoid concurrent manual runs if you want
one ordered sequence; hooks run outside Noon's concurrency limits. This is
best-effort reporting, not a durable alert queue.

### Budgets and failures

Noon gives a hook 30 seconds, then TERM to its process group, 5 seconds of
grace, then KILL. The hook passes `--timeout 10s`, leaving room for startup
and cleanup. Do not raise it past Noon's budget.

```sh
tell -C "$NOON_PROJECT" --timeout 10s notify \
    --title 'Noon job transition' --type "$kind" -- "$message" </dev/null || true
```

`|| true` makes the hook succeed even if delivery fails. Hook failures never
change the job's recorded status or exit code. Tell exit codes 1–3 mean fix
setup; 4–5 mean check before resending (see [outcomes](../../docs/design.md#outcomes)).
Nothing retries. **Never rerun a business job because its notification failed.**

## Service setup

Services do not inherit your shell. Give the Noon service:

- `PATH` with `noon`, `tell`, job tools, and the provider (`apprise`, `herdr`,
  or `notify-send`) at stable install paths, plus system directories.
- `HOME` and, if used, `XDG_CONFIG_HOME`, so Tell finds the service user's
  `tell/config.yaml`.
- Personal routing, with credentials kept in the provider's own config:

```yaml
# service user's ~/.config/tell/config.yaml
version: 1
notify:
  via: desktop     # or apprise (+ apprise.config), or herdr (+ herdr.session)
```

| Route | Service also needs |
| --- | --- |
| desktop, Linux | `notify-send` and the user's session `DBUS_SESSION_BUS_ADDRESS` |
| desktop, macOS | logged-in GUI session, `/usr/bin/osascript`, Script Editor notifications allowed |
| apprise | `apprise.config` pointing at Apprise's native config, which holds the tokens |
| herdr | an explicit `herdr.session` already running as the service user |

Tell ignores inherited `HERDR_*` context and never discovers D-Bus or sessions.
System daemons are not desktop sessions, and Focus can hide accepted banners.

Verify as the service would run, with the same user, directory, PATH, HOME/XDG,
and desktop environment:

```sh
tell -C /absolute/path/to/repo doctor
tell -C /absolute/path/to/repo --via herdr --herdr-session work doctor
```

Doctor sends nothing and checks setup, not credentials, reachability, or GUI
permission. Send a clearly labeled test notification yourself if needed.

Top-level Noon hook defaults belong only in user config or an explicit
`--config` file, never project Noon files. Job-level hooks may live in
`noon.yaml`, as here.

## Scheduled instructions

Merge [scheduled-send.yaml](scheduled-send.yaml) and approve the job. It sends
a weekday PR-triage instruction:

```yaml
jobs:
  triage:
    schedule: weekdays at 9am
    missed: skip       # don't replay a missed instruction after sleep
    overlap: skip
    timeout: 30s
    run: >-
      exec tell --timeout 10s send --intent request
      "Triage new PRs and report blockers. Scheduled run ${NOON_RUN_ID:?}; see noon logs ${NOON_RUN_ID}."
    on_failure: >-
      tell --timeout 10s notify --type failure
      "Agent dispatch failed for run ${NOON_RUN_ID:?}; inspect noon logs ${NOON_RUN_ID} before resending."
```

In the service user's Tell config:

```yaml
send:
  via: hcom          # or herdr, with send.to as name/pane ID and herdr.session: work
  to: reviewer
  from: nightly
notify:
  via: desktop       # for the on_failure notice to a person
```

Put `hcom` on the service PATH and make sure `reviewer` participates in hcom.
For Herdr, the session and agent must already exist. Run `tell doctor` in the
service environment before enabling the job.

Noon's shell expands `NOON_RUN_ID`; Tell never reads Noon's environment. A
failed send fails the run, and `on_failure` notifies a person on the first
failure. It does not use the transition-only `notify.sh`. Check the receipt in
`noon logs RUN` before retrying: timeout may mean delivered.

A successful run means **dispatched**, not that triage finished. hcom delivers
at the agent's next hook without interrupting. Herdr types a turn only after an
idle check, which can race with new work; busy agents are not prompted. When the
work needs a fresh context or tracked completion, use a Noon `prompt` or
`prompt_file` agent job instead.

## Isolated verification

`task smoke` builds Tell and runs the hook in a sanitized service-like
environment with synthetic Noon context and a local capture provider. It covers
transitions, unknown history, literal context, and delivery failure without
touching Noon or real destinations.
