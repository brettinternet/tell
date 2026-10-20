# Safe fallback chains

Notify supports an ordered fallback cascade; `send` deliberately remains
single-route because a repeated request can make two agents do the same work.
The safety rule is outcome-based: exit 3 (`not_sent`) promises nothing was
delivered; exit 4 (`failed`) may have reached some destinations; exit 5
(`uncertain`) may have arrived. `failed` is an explicit notify opt-in, and
`uncertain` never advances. [deliver.sh](deliver.sh) demonstrates caller-owned
send fallback, where it can decide which handoff semantics are safe.

Configure personal notify fallback in `~/.config/tell/config.yaml`:

```yaml
notify:
  via:
    - apprise: {tag: phone}
    - apprise: {tag: chat}
    - desktop
  fallback_on: [not_sent]
  fallback_notice: title
```

The steps run in order and share one `--timeout` budget. Apprise tag steps are
ordered alternatives, not comma-separated OR fan-out. A tagged step strips an
inherited `APPRISE_URLS` so Apprise honors the tag. To select URLs held by an
environment variable instead, use `apprise: {urls_env: PHONE_APPRISE_URLS}`;
Tell forwards the value only in the Apprise child environment, never argv or
output. Do not combine `urls_env` with a step's effective config or tag. Use
`fallback_notice: title` to prefix later titles with the first step label and
reason; `none` is the silent default.

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
