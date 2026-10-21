# Local event integrations

Tell receives Slack Socket Mode events and signed GitHub repository webhooks,
matches personal routes, and runs one bounded action per match: a local
command, a notification, or an inform-only agent message (TELL-26 through
TELL-29). It owns transient connections, in-memory buffers, and duplicate
suppression. Durable, retried, scheduled, or long-running work belongs to
callers such as Noon. Connections recover; actions never retry or replay.

## CLI

| Invocation | Network | Actions | Output |
| --- | --- | --- | --- |
| `tell listen` | none | none | inspection summary |
| `tell --json listen` | none | none | one body-free JSON object (`schema_version: 1`) |
| `tell listen slack` / `github` / `slack github` | selected sources | run | stderr receipts |
| `tell listen --dry-run SOURCE…` | selected sources | match and validate only | `dry_run` receipts |
| `tell listen --print SOURCE…` | selected sources | none, no matching | NDJSON envelopes on stdout |
| `tell listen --print --all` | every configured source | none | NDJSON envelopes |

| Exit | When |
| --- | --- |
| 2 | `--print`/`--dry-run` without sources or `--all`; `--all` with names; repeated or unknown names; conflicting modes; `--json` on a live run |
| 1 | selected source unconfigured or its credential unset/empty; `--all` with no sources; dispatch or dry-run with no route for a selected source; bind failure; fatal source or output error |
| 0 | graceful stop (not proof that actions succeeded) |

All checks above run before any network I/O. Configuration loads once; restart
after edits. `--timeout` bounds each setup request and each action, not the
listener's lifetime. Outbound flags (`--via`, `--to`, `--intent`) do not apply.
There are no `start`/`stop`, daemon, service-install, tunnel, or MCP listener
commands.

| Signal | Effect |
| --- | --- |
| first SIGINT/SIGTERM | stop sources, drop queued events and actions, let running actions finish within their timeout |
| second | cancel running actions as `uncertain` |

## Configuration

All `listen` keys are personal-only; shared `tell.yaml` rejects the block.

```yaml
listen:
  slack:
    app_token_env: TELL_SLACK_APP_TOKEN
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

| Key | Rule |
| --- | --- |
| `slack.app_token_env`, `github.secret_env` | environment variable name (`[A-Za-z_][A-Za-z0-9_]*`), never the value |
| `github.repository_ids` | required nonempty list of positive integers |
| `http.addr` | literal loopback IPv4/IPv6 and port 1–65535; default `127.0.0.1:8787`; no DNS, wildcard, LAN, or public addresses |
| `routes` | replaced as a whole across layers; `[]` disables inherited routes |

A provider block configures a source; `http` alone does not. Credential values
are read only for selected sources and only at startup.

Inspection (`tell`, `listen`, `config`, `config check`) shows names, counts,
transport settings, credential variable names, and origins. It never shows
credential values, argv, templates, or rendered text, and does not require the
variables to be set. `tell doctor` adds informational inbound readiness
(variable set, settings valid) without binding, connecting, or changing its
outbound exit status.

Every configured credential variable is removed from **all** Tell child
environments: outbound delivery, doctor probes, and listener actions.

## Envelope

```json
{"schema_version":1,"source":"slack","type":"channel_created","id":"Ev0123","received_at":"2026-10-17T20:59:00.123Z","data":{"event_id":"Ev0123","event":{"type":"channel_created","channel":{"id":"C0123","name":"inc-api"}}}}
```

`received_at` is local receipt time (UTC, milliseconds), not freshness proof.
`type` (≤128 bytes) and `id` (≤256 bytes) are required, control-free provider
metadata; Tell never synthesizes IDs. Malformed or oversized events and
duplicate JSON keys are rejected. Unknown event types can be printed and
matched.

## Bounds

| Limit | Value |
| --- | --- |
| raw Slack frame / GitHub body | 1 MiB |
| encoded envelope | 256 KiB |
| ingress buffer (shared) | 64 events; full → Slack `not_sent/busy`, HTTP 503 |
| duplicate suppression | `(source, id)` for 10 min after admission, 10,000 entries, oldest evicted first |
| action executor | 4 running, 64 waiting; overflow → `not_sent/busy` |
| diagnostic queue | 64 receipts |
| output write stall | 5 s, then exit 1 |
| rendered message + title | 256 KiB before preparation |

The identity is reserved atomically on admission, never for dropped events.
Duplicates are acknowledged but not routed or printed, and do not refresh the
window. Failed actions keep their identity reserved so redelivery cannot repeat
a successful sibling. The provider ack path never waits for actions,
rendering, or output. Nothing survives restart.

## Output

Normal and dry-run stdout stay empty. Stderr receipts carry source, event
ID/type, route/action, outcome, and a bounded reason; lifecycle receipts show
connecting, connected, reconnecting, and stopped. No-match, duplicate, and
dropped receipts are distinct. Receipts never contain payloads, headers,
ticket URLs, argv, templates, rendered text, raw provider errors, or child
output.

`--print` is **sensitive data export, not diagnostics**: one full envelope per
line, which may contain private text and secrets. Only Slack's legacy top-level
`token` is removed. Never pipe it into an untrusted log.

## Slack source

1. `apps.connections.open` with the app token (Authorization header), then connect to the returned `wss` URL. The token is never sent to the WebSocket or a redirected origin.
2. Ack every valid `envelope_id` promptly, including ignored, duplicate, and invalid deliveries. An ack means transport receipt, not action success.
3. Only `events_api` with an `event_callback` becomes an event: `type = payload.event.type`, `id = payload.event_id`, `data = payload` minus `token`. Interactive and slash-command envelopes are acked and ignored.
4. On loss or refresh, reconnect with a fresh URL: jittered backoff 1–60 s, reset after 60 s healthy, honoring 429 `Retry-After`. Network errors and 5xx retry; auth/setup errors and `link_disabled` are fatal. Only recognized error codes are logged.

Run one listener per Slack app: Slack splits deliveries among connections.
Sleep or disconnection can lose events. Tests use an internal transport seam
and local fixtures; there is no user-facing API URL option.

## Routes

```yaml
routes:
  - name: incident-kickoff          # unique, [a-z0-9][a-z0-9_-]*
    source: slack                   # configured source
    type: channel_created           # exact event type
    match:                          # optional; all must hold
      event.channel.name: {prefix: "inc-"}
      event.channel.is_private: false
    run: [/Users/me/.local/bin/incident-to-noon]   # exactly one of run | notify | send
```

`match` keys are dot paths inside `data` (segments `[A-Za-z_][A-Za-z0-9_-]*`).
Values are same-type equality on a string, finite number (no float64 precision
loss), or boolean, or `{prefix: STRING}`. Missing, null, mismatched types, and
non-object traversal do not match. No arrays, escaped dots, regex, jq, or
expressions.

Every matching route is admitted to the executor in config order; that is
admission order, not start or side-effect order. Fan-out means adding routes.
One failure never blocks siblings. Dry-run reports matches without resolving
executables, probing destinations, or using action slots.

### `run`

Literal argv (validated like `notify.command`): no shell, interpolation, or
appended arguments. The child gets:

| Input | Value |
| --- | --- |
| stdin | envelope JSON, then EOF |
| CWD | invocation directory (not `--project`) |
| environment | inherited, minus inbound credentials, plus `TELL_EVENT_SOURCE`, `TELL_EVENT_TYPE`, `TELL_EVENT_ID`, `TELL_ROUTE`, `TELL_PROJECT` |
| stdout/stderr | discarded |

| Child result | Outcome |
| --- | --- |
| exit 0 | accepted |
| exit 3 | not_sent |
| other nonzero | failed |
| missing executable / other start error | not_sent/missing_executable, not_sent/start_failed |
| timeout or cancellation after start | uncertain |

The timeout starts when the action starts, not while queued. Tell kills only
the direct child, so wrappers should `exec` and never background work.

### `notify` and `send`

```yaml
- name: notify-channel
  source: slack
  type: channel_created
  notify:
    message: "Slack event ${id}; inspect channel ${data.event.channel.id}"   # required
    title: "Channel ${data.event.channel.name} created"                     # default: project name
    type: info            # optional
    via: desktop          # optional; omit to use the notify cascade
    # herdr_session, apprise_tag also allowed
- name: inform-agent
  source: slack
  type: channel_created
  send:
    message: "Review Slack event ${id}; use provider tools for context."
    via: hcom             # optional; to, herdr_session also allowed
    to: reviewer
    intent: inform        # the default and only allowed value
```

Native actions use the in-process delivery pipeline. Omitted selectors inherit
personal outbound config; a notify without `via` uses the
[cascade](design.md#notify-fallback-cascades) within the action timeout.
Actions cannot set executables, config paths, sender, timeout, or secret policy.

| Template rule | Behavior |
| --- | --- |
| where | `message` and notify `title` only; never selectors, enums, or `run` |
| `${path}` | envelope path (`${id}`, `${data.event.channel.id}`), single pass, no functions or env lookup |
| `$${` | literal `${` |
| string, number, boolean | rendered as text |
| missing, null, object, array | `not_sent/template_value` |
| over 256 KiB combined | `not_sent/template_size` |
| malformed placeholder | config error at startup |

Rendered text then goes through normal [message preparation](design.md#message-preparation);
a secret-policy rejection fails only that action. `send` adds fixed source/ID
provenance and a warning that event content is untrusted. `inform` is not a
prompt-injection sandbox: prefer IDs and links over raw event text. Dry-run
validates rendering and preparation but never shows rendered text.

### Noon handoff

[`examples/listen/noon-handoff.py`](../examples/listen/noon-handoff.py) is a
`run` executable for a Noon job with `requests: each`. It keys requests by
`source:id` when that fits Noon's 1–200 character `[A-Za-z0-9._:-]` limit,
otherwise by its SHA-256. It `exec`s Noon and propagates failure. Noon caps
`--data` at 256 KiB (matching the envelope cap), keeps 20 queued due-now runs
per `requests: each` job (no `cooldown`), and remembers idempotency keys only
for its newest 50 finished runs. Tell adds no durable replay.

## GitHub webhooks

`tell listen github` binds `listen.http.addr` and serves only `POST /github`.
Inspection and Slack-only runs never bind. Scope is repository webhooks with
one secret and an allowlist of repositories; no GitHub Apps, organization
hooks, or generic HMAC.

A request is accepted only with:

- JSON content type (optional charset) and identity encoding;
- exactly one `X-Hub-Signature-256`, `X-GitHub-Delivery` (UUID), and `X-GitHub-Event` (`[a-z][a-z0-9_]{0,127}`);
- a valid `sha256=` HMAC over the raw body, checked in constant time **before** parsing;
- a signed `repository.id` in `repository_ids` (pings included).

There is no SHA-1 fallback, unsigned mode, proxy-header trust, or IP
authentication. The envelope is `source: github`, `type` from
`X-GitHub-Event`, `id` from `X-GitHub-Delivery`, and the unchanged body as
`data`. Headers are not copied.

| Status | Meaning |
| --- | --- |
| 202 | verified and admitted; actions run later |
| 200 | verified duplicate |
| 204 | verified `ping`; never routed |
| 400 | malformed or ambiguous metadata/JSON |
| 401 | missing or bad signature |
| 403 | repository not allowed |
| 404 / 405 | wrong path / method |
| 413 | body or envelope too large |
| 415 | wrong content type or encoding |
| 431 | headers over 16 KiB |
| 503 | ingress full, over 16 body readers, or shutting down |

Responses are body-free. 2xx means received in memory, not action success or
storage; action overflow can still drop admitted work. Only an admitted event
reserves an identity. Header/body reads and response writes time out after
5 s; idle connections after 30 s.

GitHub signs the body but not the headers or a timestamp. Tell therefore
deduplicates on both delivery ID and SHA-256 body digest for 10 minutes, so
identical bodies can collapse separate deliveries. Old valid requests can
replay after expiry, eviction, or restart. Security-sensitive actions should
check signed payload context and current provider state. GitHub does not
redeliver failures automatically; reconcile outside Tell.

Keep the tunnel and its credentials with cloudflared:

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

Point the repository webhook at `https://hooks.example.com/github` with JSON,
TLS verification, and the same secret. Do not add a transforming Worker or an
interactive Access login; cloudflared must pass the raw body and signature
through. Tell never starts, installs, or configures a tunnel. Automated tests
use local HTTP only.

## References

[Slack Socket Mode](https://docs.slack.dev/apis/events-api/using-socket-mode),
[apps.connections.open](https://docs.slack.dev/reference/methods/apps.connections.open),
[GitHub signature validation](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries),
[headers](https://docs.github.com/en/webhooks/webhook-events-and-payloads),
[best practices](https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks),
[redelivery](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks),
[Cloudflare published applications](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/routing-to-tunnel/).
