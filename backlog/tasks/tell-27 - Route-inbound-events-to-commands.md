---
id: TELL-27
title: Route inbound events to commands
status: To Do
assignee: []
created_date: '2026-10-17 21:01'
labels:
  - listen
dependencies:
  - TELL-26
documentation:
  - >-
    backlog/decisions/decision-1 -
    Tell-is-the-local-integration-edge-in-both-directions-schedulers-own-background-work.md
  - docs/design.md
  - docs/configuration.md
  - ../noon/docs/triggers.md
modified_files:
  - internal/listen/**
  - internal/config/config.go
  - internal/config/yaml.go
  - internal/config/config_test.go
  - internal/delivery/delivery.go
  - internal/cli/cli.go
  - internal/cli/listen.go
  - internal/cli/listen_test.go
  - integration/smoke_test.go
  - tell.schema.json
  - README.md
  - docs/design.md
  - docs/configuration.md
  - examples/listen/**
  - examples/README.md
priority: high
type: feature
ordinal: 20000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: printing events (TELL-26) proves the connection; acting on them is the point. An `inc-*` Slack channel should queue a Noon job or ping a person. Routes match events and hand each match to a command that returns quickly. Durable, retried, or long work goes to a scheduler such as Noon (decision-1). The contract is source-agnostic so later sources reuse it unchanged.

Settled contract:
```yaml
# personal config only
listen:
  routes:
    - name: incident-kickoff
      source: slack
      type: channel_created
      match:
        event.channel.name: {prefix: "inc-"}
      run: [/Users/me/.local/bin/incident-to-noon]
```
- Fields: `name` (required, unique, `^[a-z0-9][a-z0-9_-]*$`), `source` (required, a configured source), `type` (required, exact envelope type), `match` (optional), and `run` (required argv list with the same rules as `notify.command`: literal, no shell, no templates, non-blank executable, no NUL). Unknown keys are errors. A higher-precedence file replaces `listen.routes` wholesale, like `notify.command`.
- `match` maps a dot-separated key path inside the envelope `data` to a predicate. A scalar (string, number, or bool) requires an equal value of the same JSON type. `{prefix: STRING}` requires a string value that starts with STRING. Every entry must hold. A missing key, or a non-object along the path, is no match. No array indexing, regex, or expression language.
- Every route whose source, type, and match hold runs, in config order. With `--print`, nothing runs; each matching route is logged with `outcome=dry_run`.
- Action contract: argv exactly as configured; the envelope JSON (one object, then EOF) on stdin; the `tell listen` stdin is never inherited; the working directory is the `tell listen` working directory. The environment is inherited minus every variable named by a listen `*_env` setting, plus `TELL_EVENT_SOURCE`, `TELL_EVENT_TYPE`, `TELL_EVENT_ID`, `TELL_ROUTE`, and `TELL_PROJECT`, which override inherited values. Child stdout and stderr go to the `tell listen` stderr unchanged; Tell log lines stay body-free.
- Each action runs under the configured `timeout` (default 10s). Outcomes mirror the command route and are logged once per action, for example `tell listen: route="incident-kickoff" id="Ev0123" outcome=accepted`: exit 0 is accepted; exit 3 is not_sent; other nonzero is failed with the exit code; a missing executable is not_sent/missing_executable; another start failure is not_sent/start_failed; timeout or cancellation after start is uncertain. Nothing retries.
- Bounds: at most 4 actions run at once and up to 64 more wait in memory. Beyond that, a matched action is dropped and logged as not_sent with reason busy. Reading and acknowledging provider events never waits on actions.
- Shutdown: the first SIGINT or SIGTERM stops receiving, drops waiting actions (each logged), waits for running actions within their timeout, then exits 0. A second signal cancels running actions (logged uncertain) and exits.
- Validation in `tell listen` (before connecting) and `tell config check`: unknown source, duplicate name, empty or invalid `run`, invalid path, unknown predicate, or non-string prefix is a config error naming the route. `tell` and `tell config` show route names and counts, never argv.
- Docs and examples/listen show an incident route whose wrapper runs `noon run incident-kickoff --detach --key "$TELL_EVENT_ID" --data -` against a Noon job with `requests: each`, and explain that actions must hand off rather than do long work.

Non-goals: built-in notify or send actions and message templates (see the draft on routing events to notify and send); shell strings; argv templating or payload interpolation; regex, jq, or another expression language; retries, durable queues, or history; per-route timeout or concurrency settings; reloading routes without a restart; correlating multiple events.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 With the TELL-26 fixture Slack server and local fixture executables, a matching event runs each matching route argv exactly once without a shell, with the envelope on stdin, the five TELL_ variables set, and the Slack token variable absent from the child environment; a type mismatch, predicate mismatch, missing key, or non-object path runs nothing.
- [ ] #2 Scalar equality (type-sensitive) and prefix predicates combine with AND; multiple matching routes run in config order; invalid routes (unknown source, duplicate name, empty run, NUL in argv, unknown predicate, non-string prefix) fail `tell listen` and `tell config check` with a config error naming the route before any fixture request; routes in shared tell.yaml are rejected.
- [ ] #3 Exit 0, exit 3, other nonzero exits, a missing executable, and a timeout log accepted, not_sent, failed with the exit code, not_sent/missing_executable, and uncertain; an action sleeping past the timeout does not delay acks of later envelopes; with every slot and the wait queue full, the next match logs not_sent busy; nothing is retried.
- [ ] #4 `--print` runs no action and logs dry_run for each matching route; on the first SIGINT, waiting actions are dropped and a running action finishes before exit 0; a second SIGINT cancels it and logs uncertain.
- [ ] #5 README, docs/design.md, docs/configuration.md, and examples/listen document routes, predicates, the action contract, and the incident-to-Noon handoff with the event id as idempotency key; `tell` and `tell config` never print route argv; `task ci` passes.
<!-- AC:END -->
