# Slack event routes

Tell can match Slack events to literal local commands or native notify/send
actions. Configure the Slack
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
state outside Tell. See [the exact contract](../../docs/listening.md#github-webhooks-through-cloudflared--tell-29).

## Native notification and agent actions

A route can notify a person, inform one configured agent, or invoke a trusted
command. Use separate matching routes for intentional fan-out; each route has
exactly one action. Static selectors override personal outbound settings only
for that action:

```yaml
listen:
  routes:
    - name: notify-channel
      source: slack
      type: channel_created
      match:
        event.channel.name: {prefix: "inc-"}
      notify:
        via: desktop
        title: "New incident channel ${data.event.channel.name}"
        type: warning
        message: "Slack event ${id}; inspect channel ID ${data.event.channel.id}."
    - name: inform-reviewer
      source: slack
      type: channel_created
      match:
        event.channel.name: {prefix: "inc-"}
      send:
        via: hcom
        to: reviewer
        intent: inform
        message: "Inspect Slack channel ID ${data.event.channel.id} for event ${id}; use provider tools to retrieve context."
```

Only message and notify title allow single-pass `${path}` placeholders rooted at
the envelope. Use `$${` for a literal `${`; missing or compound values fail the
action. The combined rendered message/title is capped at 256 KiB before Tell's
shared preparation. `send` is inform-only and includes fixed external-event
source/ID provenance plus a warning that event content is untrusted. This is not
a prompt-injection sandbox: prefer stable IDs and provider links, not raw
user-authored text. `tell listen --dry-run slack` validates templates and message
policy without delivering or probing targets, and never prints rendered text.

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
