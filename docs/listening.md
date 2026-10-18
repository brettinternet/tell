# Local event integrations

**Implemented:** Slack inspection, explicit `--print` capture, personal
`run` routes and body-free `--dry-run` matching (TELL-26/27), bounded
ingress/dedup/action queues, and connection recovery. **Planned:** native
`notify`/`send` actions (TELL-28) and GitHub webhooks (TELL-29). Examples below
use only shipped Slack/command behavior unless labeled planned. This refines
decision-1; outbound commands retain their behavior.

Tell is a local integrations layer: receive external events, match personal
rules, and notify a person, inform an agent, or invoke a bounded local command.
It need not involve a scheduler. Durable, retried, scheduled, or long-running
work still belongs to callers such as Noon. Tell owns transient connections,
bounded buffers, and duplicate suppression, not a database or workflow engine.
Connection recovery is allowed; action retries and durable replay are not.

## CLI

| Invocation | Behavior |
| --- | --- |
| `tell listen` | Inspect configured sources and route names; exit without network, binding sockets, reading stdin, or running actions |
| `tell --json listen` | The same inspection as one body-free JSON object with `schema_version: 1` |
| `tell listen slack` | Start the foreground Slack listener and dispatch matching personal `run` routes |
| `tell listen github` (planned) | Start only GitHub's loopback HTTP source |
| `tell listen slack github` (planned) | Start those sources in one foreground process |
| `tell listen --print --all` | Explicitly capture every configured source (currently Slack only) |
| `tell listen --print slack` | Receive and acknowledge events; print NDJSON envelopes, but never run actions |
| `tell listen --dry-run slack` | Receive and acknowledge events; report matching routes without actions or payload output |

`--all`, `--print`, and `--dry-run` are listen-only flags, with no short aliases.
Require source names or `--all` for either preview mode: `tell listen --print`
is a usage error, not an implicit start-all.
`--all` cannot accompany source names; repeated/unknown names, conflicting modes,
and `--json` on any live invocation are usage errors (exit 2). Live output is a
stream, not the existing single-result JSON API. `--print` is not a JSON flag
alias: it explicitly releases sensitive event data. No `start`, `stop`, daemon,
service-install, tunnel, or provider-credential flags.

Global `--project`/`-C`, `--config`/`-c`, `--timeout`, and `--no-color` retain their
scope. `--timeout` bounds each network setup request and action, not the whole
listener lifetime; transport-specific limits below still apply. Outbound flags
such as `--via`, `--to`, and `--intent` do not apply to listen. Help and shell
completion show only flags and sources supported in the installed version.

Load configuration once, requiring restart after edits. Validate all configured
structure before opening anything; check credential values only for selected
sources. Known but unconfigured sources, missing/empty selected credentials,
`--all` with no sources, and dispatch/dry-run with no route for any selected
source fail before I/O (exit 1). Every selected source needs at least one route
in those modes. Print mode needs none and does not evaluate routes. Route and
credential validation happens before opening the provider connection.

The first SIGINT/SIGTERM stops sources, drops queued events/actions, and lets
running actions drain within their per-action deadlines; a second signal cancels
running children as uncertain. Graceful stop exits 0, not proof that actions
succeeded. Fatal source or output errors use the same bounded shutdown and exit
1; per-event/action failures are receipts, not process exits. Existing outbound
exit codes and MCP tools are unchanged. No new MCP listener tool.

## Configuration and inspection

All `listen` keys are personal-only. `listen.slack.app_token_env` selects the
Slack credential variable; `listen.routes` is a whole-list replacement (an
empty list disables inherited routes). Use user config, `.git/tell.yaml`, or an
explicit `--config` file. Shared `tell.yaml` rejects them even if overridden.
Keep the existing strict YAML rules and source origins.

```yaml
# Personal config; run commands are trusted local executables.
version: 1
listen:
  slack:
    app_token_env: TELL_SLACK_APP_TOKEN
  routes:
    - name: incident-kickoff
      source: slack
      type: channel_created
      match:
        event.channel.name: {prefix: "inc-"}
      run: [/Users/me/.local/bin/incident-to-noon]
```

A source is configured by its provider block. Credential references must be
nonempty environment names (`[A-Za-z_][A-Za-z0-9_]*`), never literal credentials.
`http` alone is not a source. `config check` validates structure, not environment
secret availability. Bare `tell`, `listen`, and `config` show names, counts,
safe transport settings, credential variable names, and origins, not credential
values, action argv, templates, or rendered text. JSON has the same privacy
boundary. Never resolve credential values into a printable config object.

CLI `doctor` adds local inbound readiness (selected variable set/nonempty, valid
settings), with no inbound network requests, port binds, tunnel probes, or
actions. These informational rows do not change its existing outbound-selected
exit rule and do not claim provider authentication or tunnel reachability.
Existing outbound doctor probes remain unchanged. Missing inbound credentials
must not prevent unrelated outbound commands. Remove every credential variable
named by the effective listen configuration from **all** Tell child environments,
including outbound delivery/doctor children, not just listen's command actions.

## Events, bounds, and receipts

```json
{"schema_version":1,"source":"slack","type":"channel_created","id":"Ev0123","received_at":"2026-10-17T20:59:00.123Z","data":{"event_id":"Ev0123","event":{"type":"channel_created","channel":{"id":"C0123","name":"inc-api"}}}}
```

`received_at` is Tell's receipt time, UTC RFC 3339 with milliseconds, not proof
of freshness. `source`, `type`, and `id` are nonempty provider metadata; `data`
is a JSON object. Cap raw frames/bodies at 1 MiB and the encoded envelope at
256 KiB. Reject malformed/oversized events, duplicate JSON keys, and missing
identity; never synthesize random IDs. Metadata used in receipts is bounded
(type 128 bytes, ID 256 bytes), control-free, and quoted. Unknown event types
can be printed/matched; they are not executable capabilities.

Suppress duplicates by `(source, id)` for 10 minutes after admission, with at
most 10,000 entries, oldest admitted evicted first. Duplicates do not refresh
the window. Admission and identity reservation are atomic across concurrent
receivers. Reserve only after admission to a bounded 64-event ingress buffer;
full ingress drops Slack events with `not_sent/busy` or rejects HTTP with 503,
without reserving their identities. A duplicate is acknowledged but not routed
or printed. Failed/dropped actions do not clear admitted identities: automatic
redelivery could otherwise repeat an already successful sibling action.
GitHub also uses a body digest as described below. No state survives restart.

The provider reader/ack path never waits for a command, rendering, or stdout.
Output has a 64-event ingress queue and a separate 64-receipt diagnostic queue;
each write has a 5-second stall budget. A broken or persistently stalled output stream
stops the listener with exit 1 rather than growing memory or blocking acks.
`--print` emits one complete envelope per stdout line. This is an explicit data
export, **not diagnostics**: payloads may contain private text and secrets.
Remove known transport credentials (Slack's legacy top-level payload `token`),
but do not promise arbitrary payload redaction or mutate data used for matching.

Normal and dry-run stdout stay empty. Body-free stderr receipts identify source,
event ID/type, route/action, outcome, and a bounded reason/exit code. Lifecycle
receipts include connecting, connected, reconnecting, and stopped. Do not print
raw provider errors, headers, URLs with tickets, payloads, argv, templates, or
child output. Preview receipts use `dry_run`, never `accepted`. No-match,
duplicate, and admission-drop receipts are distinguishable. Intentional message
delivery to the configured stderr notify route is not a diagnostic; it still
uses the ordinary preparation pipeline.

## Slack source — TELL-26

Socket Mode uses `apps.connections.open` with the named app token in the
Authorization header, then connects to its returned URL. Ack each valid
`envelope_id` promptly, including ignored/duplicate/invalid-payload deliveries;
acknowledgment means transport receipt, not completed action. Only `events_api`
with an `event_callback` object becomes an event: `type = payload.event.type`,
`id = payload.event_id`, `data = payload` minus top-level `token`. Ack and ignore
interactive/slash-command envelopes; handle `hello`/`disconnect` as lifecycle
messages. Bound frame reads before decoding; malformed frames never reach routes.

Reconnect with a fresh URL on loss/refresh, exponential backoff with jitter from
1s to 60s; reset after 60s of healthy connection. Respect HTTP 429 Retry-After;
network errors and 5xx retry connection setup, not actions. Non-retryable Slack
auth/setup errors and `link_disabled` are fatal; log only recognized safe error
codes (unknown codes get a fixed reason), never the raw response. All waits are
cancellable. Use `wss` and the fixed production Slack API endpoint. Do not add a
user-facing `api_url` merely for fixtures: use an internal test transport seam
and local fixture server; never send Authorization to a redirected origin or
the WebSocket connection. Production does not allow plaintext transport.

Document Socket Mode enablement, `connections:write` app token, subscribed bot
events/scopes (for example `channel_created`/`channels:read`), and app installation.
Run one listener per Slack app: Slack distributes deliveries among connections,
not a copy to every listener. Sleep/disconnection can lose events.

## Route matching and command actions — TELL-27

Each route has a unique `name` matching `[a-z0-9][a-z0-9_-]*`, a configured
`source`, exact `type`, optional `match`, and exactly one action. The only shipped
action is `run`; native `notify` and `send` may extend that union later. Unknown
keys and multiple actions fail config validation; there are no action lists or
implicit fallbacks.

`match` maps dot-separated object paths inside `data` to scalar equality
(string, finite JSON number, boolean; same type) or `{prefix: STRING}`. All
predicates must hold; omitted/empty match accepts the selected source/type.
Paths use nonempty `[A-Za-z_][A-Za-z0-9_-]*` segments. Missing/null values,
non-object traversal, and type mismatches do not match. Compare numbers without
float64 precision loss. No array indexing, escaped dotted keys, regex, jq,
expressions, or implicit string conversion.

Every matching route is admitted in config order to the shared action executor:
4 running actions, 64 waiting, overflow `not_sent/busy`. This is admission order,
not a promise of subprocess start, completion, or side-effect order. One route's
failure does not block siblings. Actions never retry. Duplicate source/ID events
are suppressed after event admission, so a failed action never replays successful
siblings. Dry-run reports matches without resolving executables, probing
destinations, or using action slots.

`run` is a nonempty literal argv list using existing `notify.command` validation.
No shell, interpolation, or appended args. CWD is the invocation CWD (not changed
by `--project`); bare executables use PATH and relative paths use that CWD.
Stdin receives the envelope JSON and EOF, never the listener's stdin. Inherit
the environment minus named inbound credentials, then override `TELL_EVENT_SOURCE`,
`TELL_EVENT_TYPE`, `TELL_EVENT_ID`, `TELL_ROUTE`, and `TELL_PROJECT`. Discard child
stdout/stderr as existing command delivery does; diagnostics are not a payload
escape hatch. Users can explicitly persist output in their own trusted wrapper.

Use existing command outcomes: 0 accepted, 3 not_sent, other nonzero failed,
missing executable not_sent/missing_executable, other start error
not_sent/start_failed, timeout/cancellation after start uncertain. Timeout starts
when an action starts, not while queued. Reuse direct-child cancellation limits;
wrappers must `exec` and not background work. First SIGINT/SIGTERM stops sources,
drops waiting events/actions with body-free receipts, and lets running actions
finish within their remaining timeout; a second signal cancels them. Fatal
source errors perform the same bounded shutdown but exit 1.

A Noon handoff can use the supplied `examples/listen/noon-handoff.py` as the
trusted route executable and a job configured with `requests: each`. It uses
`source:id` as the key when it fits Noon's 1–200 ASCII character limit
(`[A-Za-z0-9._:-]`), otherwise maps it to a stable SHA-256 key. The Tell
envelope is at most 256 KiB and Noon accepts `--data` up to 256 KiB; Noon
validates that limit itself and rejects oversized data. The wrapper `exec`s
Noon and propagates failure, never truncates or claims success on a failed
handoff. Noon stores the Tell envelope as its own event `data`; the formats are
not interchangeable. `requests: each` preserves each payload but caps a job at
20 queued due-now runs and cannot be combined with `cooldown`. Noon's idempotency
keys are retained only for its
newest 50 finished runs per job; Tell adds no durable replay guarantee. No
Noon-specific parsing is implemented in Tell.

## Planned: native notification and agent actions — TELL-28

Promote: direct cross-tool communication is core product scope, not contingent
on repeated wrapper scripts. Reuse the in-process delivery/preparation APIs;
do not spawn `tell` recursively or duplicate transports.

- `notify`: required `message`; optional `title`, `type` (default info), static
  `via`, `herdr_session`, and `apprise_tag` selectors. Inherit omitted selectors
  and the default project title from effective personal outbound configuration.
- `send`: required `message`; optional static `via`, `to`, `herdr_session`, and
  `intent` (only `inform`, also the default). Inherit omitted selectors from
  effective personal send configuration. No event-driven `request` intent in
  this slice; a trusted command wrapper can explicitly make that policy choice.
- Neither action can set executable/config paths, sender identity, timeout, or
  secret policy. Validate static selector syntax at config load; dynamic target
  readiness remains per-action and returns existing honest delivery outcomes.

Only message and notify title accept `${path}` placeholders rooted in the
**envelope**, for example `${data.event.channel.id}` or `${id}`. Paths use the
same grammar as matching. `$${` emits literal `${`; substitution is single-pass,
with no functions, expressions, shell, recursion, or environment lookup. String,
number, and boolean values render as text. Missing/null/object/array values fail
that route as `not_sent/template_value`; never silently send an empty value.
Malformed placeholder syntax is a startup config error. Reject placeholders in
all selectors and enum fields. Literal command argv is never templated.

Render inside the shared action budget with a 256 KiB pre-preparation cap;
exceeding it is `not_sent/template_size`, without partial delivery. Then apply
existing normalization, secret policy, provenance, and truncation. Secret-policy
rejection/scanner failure is the existing per-action config outcome, not fatal
to the listener. Receipts never contain rendered text. Send adds fixed provenance
identifying an external event and its source/ID, marking interpolated fields as
untrusted data; it does not grant event authors authority to instruct the agent.
Prefer IDs and links over raw user-authored text. `inform` is not a prompt-injection
sandbox. Dry-run validates rendering/policy without dispatch and reports only
validation results/errors, never rendered previews. All action kinds share capacity,
shutdown, credential stripping, and no-retry behavior.

## Planned: GitHub webhooks through cloudflared — TELL-29

Promote with the user's concrete scope: GitHub **repository** webhooks through a
separately managed Cloudflare Tunnel. One `listen.http.addr` (default
`127.0.0.1:8787`) serves the fixed `/github` path only when GitHub is selected.
Require a literal loopback IP and port 1–65535 (IPv4 or IPv6); refuse wildcard,
LAN/public addresses and DNS names. No insecure override or per-source ports.
No listener during inspection or Slack-only runs. Bind failure stops startup.

`listen.github.secret_env` is required, with a nonempty selected environment
value. Require a nonempty list of positive integer `repository_ids`, checked
against the signed body's `repository.id` before admission (including ping).
First scope is one secret, possibly several allowed repositories; not GitHub
Apps/organization hooks, multiple secrets, generic HMAC, or other providers.

Accept only POST `/github`, JSON content type (optional charset), identity
content encoding, and one each of `X-Hub-Signature-256`, `X-GitHub-Delivery`, and
`X-GitHub-Event`. Reject ambiguous repeated/comma-joined identity/signature
headers. Verify `sha256=` HMAC over the exact bounded raw bytes in constant time
**before** JSON parsing, deduplication, printing, or route matching. No SHA-1
fallback, unsigned mode, proxy-header trust, or IP-address authentication.
Require a UUID delivery ID and an event name matching `[a-z][a-z0-9_]{0,127}`; `source = github`,
`type = X-GitHub-Event`, `id = X-GitHub-Delivery`, `data = parsed body` unchanged.
Headers/signatures are not copied into `data`. `ping` is verified and acknowledged
with 204 but never dispatched; other valid event types use normal matching.

Responses are fixed and body-free: 404 wrong path; 405 wrong method; 415 wrong
content type/encoding; 413 raw/envelope size exceeded; 401 missing/bad signature;
400 malformed/ambiguous metadata or JSON; 403 disallowed repository; 503 full
ingress/shutdown. An admitted event returns 202 immediately, an already admitted
duplicate 200. No non-2xx response reserves an identity. 2xx means verified and
received in memory (or duplicate/ping), **not** action success or durable storage;
action overflow can still drop work after admission. Response failure after
admission cannot undo it. Enforce 16 KiB header limit, 5s header/body-read and
response-write timeouts, 30s idle timeout, and at most 16 active body-reading
handlers; reject excess with 503. Never wait for actions before responding.

GitHub signs the body, **not** the event/delivery headers or a timestamp. A
10-minute ID cache is duplicate suppression, not cryptographic freshness. Also
reserve a SHA-256 raw-body digest under the same bounded TTL for GitHub, so a
changed unsigned delivery ID cannot bypass suppression within that window.
Check/reserve ID and digest atomically; either duplicate suppresses dispatch.
Identical bodies can consequently collapse distinct deliveries within the window.
Old valid bodies can still replay after expiry, eviction, or restart, and event
headers remain unauthenticated metadata. Do not invent an expired-request check
or present `received_at`/payload timestamps as signed freshness. Security-sensitive
actions must validate signed payload context and reconcile current provider state.
Use dedicated webhook secrets and repository allowlists. A tunnel is transport,
not authorization. GitHub does not automatically redeliver failed deliveries;
manual redelivery/reconciliation belongs outside Tell.

The setup recipe uses a named Cloudflare Tunnel with a stable public HTTPS
hostname mapped to `http://127.0.0.1:8787`, with only `/github` forwarded and a
catch-all 404. Configure the repository webhook URL as
`https://hooks.example.com/github`, JSON content type, TLS verification enabled,
and the same secret. cloudflared must preserve raw bodies and signature headers;
no transforming Worker or interactive Access login on this machine-to-machine
path. Keep Cloudflare credentials/configuration with cloudflared. Tell never
starts, installs, authenticates, or configures a tunnel. Live setup is optional
manual verification; automated tests use local HTTP requests only.

## Evidence and delivery gates

Provider contracts checked during refinement against:

- [Slack Socket Mode](https://docs.slack.dev/apis/events-api/using-socket-mode)
  and [apps.connections.open](https://docs.slack.dev/reference/methods/apps.connections.open).
- [GitHub signature validation](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries),
  [headers](https://docs.github.com/en/webhooks/webhook-events-and-payloads),
  [best practices](https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks),
  and [redelivery](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks).
- [Cloudflare published applications](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/routing-to-tunnel/).

Implement Slack/envelope first, then command routing. Native actions and GitHub
are independent after routing; neither needs the other. Each task updates help,
completion, strict parsing, both editor schemas, safe inspection, and docs for
only the capabilities it ships. Verify with local protocol/provider fixtures,
focused Go tests, and `task ci`. Never claim a planned command works in current
README examples or enable it in current schemas before implementation.
