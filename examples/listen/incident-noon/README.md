# Incident channel → Noon job

Queue each new public `inc-*` channel as a Noon job. The demo job records only
the latest event identity in `~/.local/state/tell/last-listen-event.json`.

Needs Tell, Noon 0.4.0 or newer, Python 3, and jq.
Noon 0.3.0 rejects `requests: each`.

```sh
noon --version
# Install 0.4.0 with mise if needed.
mise use -g github:brettinternet/noon@0.4.0
```

## Configure

1. Follow the [Slack setup](../README.md#slack-setup) with
   `channel_created` and `channels:read`.
2. Merge [noon.yaml](noon.yaml) into the target project's `noon.yaml`.
3. Replace the absolute paths in that job and [config.yaml](config.yaml)
   with your Tell checkout path.
4. Review the job and referenced scripts before enabling it.

From the target project, not the Tell checkout, run the following.

```sh
noon config check
noon enable
noon serve
```

In another terminal, stay in that same project.

```sh
tell --config /absolute/path/to/tell/examples/listen/incident-noon/config.yaml config check
tell --config /absolute/path/to/tell/examples/listen/incident-noon/config.yaml listen --dry-run slack
# Create inc-preview, then stop with Ctrl-C. No job is queued.
tell --config /absolute/path/to/tell/examples/listen/incident-noon/config.yaml listen slack
# Create inc-demo.
```

Tell's command runs in its invocation directory. `--project` does not change
that directory. Give both processes the required tools on `PATH`.

## Check the result

```sh
noon history
noon logs incident-kickoff
jq . ~/.local/state/tell/last-listen-event.json
```

| Slack action | Result |
| --- | --- |
| Create public `inc-demo` | One queued job; after dispatch, the identity file is updated |
| Create public `team-demo` | No matching route or job |
| Create `inc-preview` during Tell dry-run | Match receipt, no job |

[noon-handoff.py](../noon-handoff.py) passes the Tell envelope unchanged to
`noon run incident-kickoff --detach --key … --data -`. The key uses
`source:id`, or its stable hash when needed.

Noon wraps that envelope in its own `data`. The job unwraps it with jq and
passes it to [record-event.py](../record-event.py), which writes no message
body. Replace the job's `run` with your incident workflow when ready.

## Limits

- Tell's accepted receipt means Noon accepted the handoff, not that the job finished.
- `requests: each` keeps separate payloads, with at most 20 queued runs per job.
  Do not combine it with cooldown.
- Noon stores the full event payload. Treat its state as sensitive.
- Tell has no action retries or durable replay. Noon keys are retained for
  its newest 50 finished runs per job, not forever.
- Noon expires queued runs more than an hour overdue. Reconcile important
  incident state rather than relying only on events.

See [Noon triggers](https://github.com/brettinternet/noon/blob/main/docs/triggers.md)
for queue and payload behavior.
