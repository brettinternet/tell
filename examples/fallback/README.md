# Fallback chains: try another route only when it's safe

Tell has no fallback routes on purpose: whether trying elsewhere is safe
depends on the outcome. Exit 3 (`not_sent`) promises nothing was delivered;
exit 4 or 5 means it may have arrived. [deliver.sh](deliver.sh) builds the
chain in the caller, where that decision belongs.

```console
$ examples/fallback/deliver.sh "Review feat/rate-limit at 3f9c2e1; blockers first."
herdr reviewer: busy
hcom reviewer: accepted
```

| Step | Route | Moves on when |
| --- | --- | --- |
| 1 | Herdr pane: types a turn if the agent is idle | `busy`, `blocked`, missing agent, missing Herdr |
| 2 | hcom mailbox: queued for the next hook | agent not participating, hcom missing |
| 3 | `tell notify` to you | — |

`--json` gives the reason (`busy`, `missing_target`, `missing_executable`,
...) without parsing human text. The exit code alone decides whether to
continue.

## Setup

```yaml
# ~/.config/tell/config.yaml
version: 1
notify:
  via: desktop
herdr:
  session: work
send:
  from: dispatcher
```

Names can differ per transport: `HERDR_AGENT=claude-2 HCOM_AGENT=reviewer deliver.sh "..."`.

## Never fall back on

| Exit | Why not |
| --- | --- |
| 4 failed | some destinations may have it |
| 5 uncertain | it timed out after starting; it may have arrived |
| 1, 2 | your config or invocation is wrong; every route will fail the same way |

A request delivered twice can mean two agents doing the same work.
