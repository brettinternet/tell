# Event routes

| Slack recipe | Trigger → action |
| --- | --- |
| [Incident channel](incident-channel/README.md) | Create `inc-*` → desktop notification, optional agent inform |
| [Mention an agent](mention-agent/README.md) | Mention the app in one channel → agent inform |
| [Reaction triage](reaction-triage/README.md) | Add 👀 to a message → agent inform |
| [Incident job](incident-noon/README.md) | Create `inc-*` → Noon job with event data |

[GitHub repository webhooks](#github-repository-webhook) are covered below.

## Slack setup

1. Create a Slack app and enable Socket Mode.
2. Generate an app-level token with `connections:write`. Set it securely in
   `TELL_SLACK_APP_TOKEN`; do not put the token in YAML.
3. Under **Event Subscriptions → Subscribe to bot events**, add the events
   for your recipe and the corresponding bot scopes.
4. Install the app in your workspace, or reinstall after changing scopes.
   Invite it to channels used by the mention and reaction recipes.

| Recipe | Bot event | Bot scope |
| --- | --- | --- |
| Incident channel / Noon job | `channel_created` | `channels:read` |
| Mention an agent | `app_mention` | `app_mentions:read` |
| Reaction triage | `reaction_added` | `reactions:read` |

Run recipe commands from the Tell checkout unless noted. The configs use
explicit `--config` paths so they do not load your normal config files.
Listener settings are personal; never merge them into shared `tell.yaml`.
To combine recipes, merge their routes into one list. A later `listen.routes`
setting replaces the whole list.

Run one listener per Slack app. Multiple connections split deliveries.
`tell listen` only inspects; `--dry-run slack` connects and consumes events
without actions. Trigger a new event after switching to dispatch.

Match paths start inside `data`, such as `event.channel.name`.
Templates start at the envelope root, such as `${data.event.channel.name}`.

Mention/reaction events carry channel IDs, not names. Tell does not look up
names, fetch history, or reply in Slack. Keep IDs as pointers rather than
forwarding message text. Use `--print slack` only for deliberate sensitive
event export, not routine logs.

## Local command

Add this route to personal config to record only the latest event identity:

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

## GitHub repository webhook

Configure the dedicated webhook secret's environment variable name and the
allowed repository IDs in personal config. Use the repository's numeric
`repository.id`, not its owner/name:

```yaml
listen:
  github:
    secret_env: TELL_GITHUB_WEBHOOK_SECRET
    repository_ids: [123456789]
  http:
    addr: 127.0.0.1:8787
  routes:
    - name: opened-pr
      source: github
      type: pull_request
      match: {action: opened}
      run: [/Users/me/.local/bin/handle-pr-event]
```

Set `TELL_GITHUB_WEBHOOK_SECRET` securely in the listener's environment; never
put its value in YAML. In GitHub repository **Settings → Webhooks**, register
`https://hooks.example.com/github`, choose `application/json`, enable TLS
verification, use the same secret, and select only needed repository events.

Create a named tunnel separately. Keep Cloudflare credentials with cloudflared
and route only the fixed path to Tell's loopback listener:

```sh
cloudflared tunnel create tell-github-hooks
cloudflared tunnel route dns tell-github-hooks hooks.example.com
```

```yaml
# cloudflared config.yaml
tunnel: <TUNNEL-UUID>
credentials-file: /Users/me/.cloudflared/<TUNNEL-UUID>.json
ingress:
  - hostname: hooks.example.com
    path: ^/github$
    service: http://127.0.0.1:8787
  - service: http_status:404
```

Run `cloudflared tunnel run tell-github-hooks` separately from
`tell listen github`. Do not put an interactive Access challenge or transforming Worker on
this machine-to-machine route; cloudflared must preserve the raw request body
and signature header. Tell never creates, configures, authenticates, or manages
the tunnel. `tell listen` remains offline. `tell doctor` does not bind or probe
this inbound source, but keeps its existing outbound readiness probes. Only
explicit `tell listen github` binds.

GitHub signs the body, not event/delivery headers or a timestamp. Tell checks
the body repository ID and verifies SHA-256 HMAC before parsing or matching.
A 202 response means in-memory admission, not action success or durable storage;
a 204 ping is never routed, and a 200 is a duplicate. Deduplication by delivery
ID and raw-body digest is bounded to 10 minutes and does not prove freshness;
old valid payloads can replay after expiry, eviction, or restart. GitHub does
not automatically redeliver failed deliveries. Reconcile important provider
state outside Tell. See [the exact contract](../../docs/listening.md#github-webhooks).

## Native notification and agent actions

Use [incident-channel](incident-channel/README.md) for a desktop notification
and optional agent fan-out, [mention-agent](mention-agent/README.md) for app
mentions, or [reaction-triage](reaction-triage/README.md) for emoji markers.

Each route has one action. Separate routes match independently; failures do
not block siblings. `send` supports only `inform`, with fixed external-event
provenance and an untrusted-content warning. It is not a prompt-injection sandbox.

## Noon handoff

[incident-noon](incident-noon/README.md) includes Tell config, a runnable Noon
job, and verification commands. It reuses [noon-handoff.py](noon-handoff.py)
and [record-event.py](record-event.py).

Tell owns the short handoff; Noon owns queued work and history. Neither a
Slack acknowledgment nor a successful handoff means the job finished.
