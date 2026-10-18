# Slack command routes

Tell can match Slack events to literal local command argv. Configure the Slack
app and credential environment variable as in the [README setup](../../README.md#slack-event-capture),
then add this to personal config:

```yaml
listen:
  slack:
    app_token_env: TELL_SLACK_APP_TOKEN
  routes:
    - name: remember-channel
      source: slack
      type: channel_created
      match:
        event.channel.name: {prefix: "inc-"}
      run: [python3, ./examples/listen/record-event.py]
```

`run` receives one JSON Tell envelope on stdin and EOF. This example validates
the envelope and keeps only the last event's source, type, ID, and receipt time
in `~/.local/state/tell/last-listen-event.json`; it never stores the event body.
Run it from the repository root because relative executables and paths use the
invocation CWD, not `--project`. Tell discards the child's stdout and stderr.
The route name appears in inspection, but argv is not printed.

Inspect without connecting, then preview matches without invoking the command:

```sh
tell listen
tell listen --dry-run slack
```

`--dry-run` requires the selected source's credential and routes, but performs no
subprocess or target probes. Start real dispatch explicitly with
`tell listen slack`. Every matching route is admitted in config order; four
commands run concurrently and 64 can wait. This does not order command starts or
side effects. Keep commands short and use `exec` in wrappers; Tell has no
persistent queue, action retry, or completion history.

## Noon handoff

For work that needs durable scheduling/history, configure and enable this Noon
job (and keep `noon serve` running to dispatch it):

```yaml
jobs:
  incident-kickoff:
    run: [./handle-event]
    requests: each
```

`requests: each` preserves each payload but permits at most 20 queued due-now
runs per job; configure no cooldown with it. The handoff route can use this
trusted executable:

```yaml
listen:
  routes:
    - name: kickoff-incident
      source: slack
      type: channel_created
      match:
        event.channel.name: {prefix: "inc-"}
      run: [python3, /absolute/path/to/examples/listen/noon-handoff.py]
```

The wrapper uses `TELL_EVENT_SOURCE:TELL_EVENT_ID` when the key is within Noon's
1–200 ASCII character limit (`[A-Za-z0-9._:-]`); otherwise it maps the same
source-qualified identity to a stable SHA-256 key. Tell caps its envelope at
256 KiB, the same maximum Noon accepts for `--data`; Noon validates its input
and rejects oversize payloads. The wrapper passes stdin unchanged, never
truncates it, and `exec`s Noon so a failed start/queue is a nonzero Tell action
outcome. It does not claim that a queued run completed. Noon retains idempotency
keys only for its newest 50 finished runs per job, so this is not permanent
replay protection.

Noon wraps the Tell envelope inside its own `data`; a Noon job reads its event
file via `NOON_EVENT_FILE`. They are different envelope formats. The job must
validate/map its own payload and reconcile important state. Tell does no
Noon-specific parsing, scheduling, retries, or durable storage. See
[Noon triggers](https://github.com/brettinternet/noon/blob/main/docs/triggers.md)
for the caller-owned queue and payload contract.
