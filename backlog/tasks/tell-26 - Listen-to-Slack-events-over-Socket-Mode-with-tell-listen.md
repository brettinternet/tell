---
id: TELL-26
title: Listen to Slack events over Socket Mode with tell listen
status: To Do
assignee: []
created_date: '2026-10-17 21:01'
updated_date: '2026-10-17 21:01'
labels:
  - listen
  - slack
dependencies: []
references:
  - 'https://docs.slack.dev/apis/events-api/using-socket-mode'
  - 'https://docs.slack.dev/reference/methods/apps.connections.open'
  - 'https://docs.slack.dev/reference/events/channel_created'
documentation:
  - >-
    backlog/decisions/decision-1 -
    Tell-is-the-local-integration-edge-in-both-directions-schedulers-own-background-work.md
  - docs/design.md
  - docs/configuration.md
modified_files:
  - internal/listen/**
  - internal/config/config.go
  - internal/config/yaml.go
  - internal/config/config_test.go
  - internal/cli/cli.go
  - internal/cli/listen.go
  - internal/cli/listen_test.go
  - internal/cli/doctor.go
  - internal/cli/doctor_test.go
  - cmd/tell/main.go
  - integration/smoke_test.go
  - go.mod
  - go.sum
  - tell.schema.json
  - tell.shared.schema.json
  - README.md
  - docs/design.md
  - docs/configuration.md
  - docs/development.md
  - AGENTS.md
priority: high
type: feature
ordinal: 19000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: a Slack incident channel being created (and later, events from other providers) should start local work during working hours. decision-1 makes Tell the inbound integration edge as well as the outbound one, with only in-memory state and no background jobs. This task adds `tell listen`, the common event envelope every future source emits, and Slack Socket Mode as the first source. Socket Mode needs no public URL, which suits a laptop. Routing events to commands is TELL-27; until then the useful mode is `--print`, which shows real events so routes can be written against them.

Settled contract:
- `tell listen [--print] [SOURCE...]` runs in the foreground until SIGINT or SIGTERM, then closes connections and exits 0. With SOURCE arguments it starts only those sources. These fail before any network request: no configured source (config, exit 1); a SOURCE name Tell does not support (usage, exit 2); a supported but unconfigured SOURCE (config, exit 1); a missing or empty credential variable (config, exit 1); `--json` (usage, exit 2; stdout carries events). Without `--print` and with no routes configured it is a config error with a hint (routes arrive in TELL-27).
- Configuration loads once at startup through the normal layered sources (`--project`, `--config`, env, flags). Edits need a restart.
- `--print` writes one JSON envelope per line to stdout, flushed per event, and runs nothing:
  `{"schema_version":1,"source":"slack","type":"channel_created","id":"Ev0123","received_at":"2026-10-17T20:59:00.123Z","data":{...}}`
  `received_at` is RFC 3339 UTC with milliseconds. The shape matches Noon event files plus `type` and `id`, so an action can pass it to `noon run --data -` unchanged. For Slack, `data` is the Events API `event_callback` payload unchanged except that the legacy top-level `token` (verification token) is removed; `type` is `data.event.type` and `id` is `data.event_id`.
- Deduplication applies to every source: an event whose `id` this process already emitted in the last 10 minutes is dropped and logged as `duplicate`. At most 10,000 ids are remembered, oldest evicted first. Nothing persists across restarts.
- Logging: body-free lines on stderr in the existing receipt style, for example `tell listen: source="slack" type="channel_created" id="Ev0123" outcome=printed` and `tell listen: source="slack" state=connected`. Never log payloads, tokens, or WebSocket URLs (they carry a ticket).
- Slack Socket Mode: call `apps.connections.open` with the app-level token in the Authorization header, connect to the returned URL, and ack every envelope that has an `envelope_id` as soon as it is read, before any other processing (an ack means received, not handled). `events_api` envelopes become events. `hello` is logged. Other envelope types (`interactive`, `slash_commands`) are acked and ignored with a log line. A `disconnect` message or a dropped connection reconnects with a fresh URL, using capped exponential backoff with jitter (1s doubling to 60s, reset after a healthy connection). Slack auth or setup errors (for example `invalid_auth`, `not_authed`, `token_revoked`, `missing_scope`) and a `link_disabled` disconnect exit 1 naming the Slack error code. Network errors, HTTP 5xx, and rate limits retry.
- Configuration, personal only (shared `tell.yaml` rejects every `listen` key):
  ```yaml
  listen:
    slack:
      app_token_env: TELL_SLACK_APP_TOKEN
      api_url: https://slack.com/api/   # optional
  ```
  The Slack source is configured when `listen.slack.app_token_env` is set; the named variable is read at startup and must be nonempty. `api_url` exists so tests and proxies can use a local server: it must be https, or http only for a loopback host. WebSocket URLs must be `wss`, except `ws` to a loopback host when `api_url` is loopback http. Tell removes the token variable from every child environment it starts.
- Inspection: bare `tell` and `tell config` show configured listen sources, the `app_token_env` name, and its origin, never the value. `tell doctor` reports each listen source as ready or not ready (variable set and nonempty) without contacting Slack; listen readiness does not change the doctor exit code. `tell config check` validates the listen section.
- Docs cover Slack app setup (Socket Mode enabled, an app-level token with `connections:write`, bot event subscriptions such as `channel_created` with `channels:read`, app installed), state that events sent while Tell is disconnected or the machine is asleep may be lost, and warn that Slack spreads payloads across every open connection of an app, so run one `tell listen` per Slack app. Verify Slack details against current Slack docs while implementing.

Non-goals: routes and actions (TELL-27); an HTTP listener or the Slack Events API over HTTP; interactive components, slash commands, or response payloads; posting to Slack; multiple workspaces or apps; service installation; durable queues, history, or retries; reloading config without a restart; tests that contact real Slack.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 With a local fixture Slack server (apps.connections.open plus a WebSocket), `tell listen --print slack` prints exactly one envelope line per `events_api` delivery with the documented fields and `data` equal to the payload minus `token`, and the fixture observes an ack for every envelope_id, including ignored envelope types.
- [ ] #2 A fixture `disconnect` message and a dropped connection each cause a new apps.connections.open call and resumed delivery; an event id redelivered within the window is printed once and acked each time; a fatal Slack auth error and a `link_disabled` disconnect exit 1 naming the Slack error code.
- [ ] #3 No configured source, an unsupported SOURCE, an unconfigured SOURCE, a missing or empty token variable, `--json`, and running without `--print` while no routes exist fail with the documented exit codes before the fixture receives any request; SIGINT and SIGTERM close the connection and exit 0.
- [ ] #4 A sentinel token value never appears in stdout, stderr, `tell config`, `tell --json config`, or `tell doctor` output; `tell config` shows the variable name and origin; any `listen` key in shared tell.yaml is a config error; bare `tell`, `tell config`, and `tell doctor` send no request to the fixture.
- [ ] #5 README, docs/design.md, docs/configuration.md, docs/development.md, AGENTS.md, and both JSON schemas describe the listen boundary from decision-1 (the one long-running inbound command, in-memory state only, possibly lost events, one listener per Slack app, Slack app setup); `task ci` passes.
<!-- AC:END -->
