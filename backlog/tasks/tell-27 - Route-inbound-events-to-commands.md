---
id: TELL-27
title: Route inbound events to commands
status: To Do
assignee: []
created_date: '2026-10-17 21:01'
updated_date: '2026-10-17 21:14'
labels:
  - listen
dependencies:
  - TELL-26
documentation:
  - >-
    backlog/decisions/decision-1 -
    Tell-is-the-local-integration-edge-in-both-directions-schedulers-own-background-work.md
  - docs/listening.md
  - docs/design.md
  - docs/configuration.md
  - ../noon/docs/triggers.md
modified_files:
  - internal/listen/**
  - internal/config/**
  - internal/cli/**
  - integration/smoke_test.go
  - tell.schema.json
  - tell.shared.schema.json
  - README.md
  - docs/design.md
  - docs/configuration.md
  - docs/listening.md
  - internal/delivery/**
  - cmd/tell/main.go
  - examples/listen/**
  - examples/README.md
priority: high
type: feature
ordinal: 20000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Context
Event capture alone cannot connect local tools. Route a Slack incident event to a short local command or scheduler handoff, keeping Tell source-agnostic and free of durable work. Depends on TELL-26's event/ingress and lifecycle contract.

## Scope and settled interface
Implement the matching and command-action section of docs/listening.md:
- Personal `listen.routes` is a whole-list replacement with unique names, configured source, exact type, optional AND predicates, and exactly one `run` argv action. Reserve the one-action union for later native notify/send; do not implement them here.
- Exact typed scalar equality and string prefix match object paths inside data. No shell, expressions, array traversal, or numeric precision loss.
- All matches enter the common executor in configuration order, not guaranteed effect/completion order. Four actions run and 64 wait; excess is not_sent/busy, with no retries or dedup reset.
- Literal argv, envelope JSON on stdin, invocation CWD, five fixed event/project environment values, and inherited environment minus inbound credential references. Discard child output; never forward payloads or secrets to diagnostics.
- Add `tell listen --dry-run SOURCE...` (or --all): body-free match receipts, no executables/probes/actions. `--print` stays a separate envelope export with no matching. Bare listen stays safe.
- Use existing command outcomes, per-action timeout, and bounded first/second-signal shutdown. A failed action does not kill the listener or skip sibling routes.

## Non-goals
Native notify/send, templates, per-route timeout/concurrency, durable queues, retries, replay, stateful correlations, config reload, and scheduler-specific parsing. Brief local work is allowed; only long/durable work needs a scheduler.

## Next action and verification
After claiming, extend TELL-26's fixtures and reuse existing subprocess outcome/config validation patterns. Include a safe local command example and a Noon handoff recipe using source-qualified event ID, requests: each, and explicit payload/key-limit handling. Noon nests the Tell envelope in its data; do not claim identical formats. Record focused Go tests and task ci results before checking criteria.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Local event/command fixtures prove exact argv (no shell/interpolation), envelope stdin and EOF, invocation CWD, five overridden TELL_ values, and absence of named listener credentials. Child stdout/stderr sentinels are discarded. Type/predicate/missing/null/non-object mismatches run nothing.
- [ ] #2 Typed scalar equality (including large integers without float rounding), prefix AND, and whole-list replacement match docs/listening.md. Invalid paths/predicates, duplicate names, invalid argv, unconfigured sources, and shared routes fail config check/startup before inbound I/O. Multiple matching routes are admitted in config order without promising concurrent side-effect order.
- [ ] #3 0/3/other exit, missing/start-failed executable, timeout and cancellation yield existing honest receipts. Four running plus 64 waiting bounds are enforced; excess logs not_sent/busy without retry, duplicate replay of an admitted event cannot repeat successful siblings, and slow actions do not delay provider acks.
- [ ] #4 --print exports envelopes without matching/actions; --dry-run reports body-free matches without subprocess/target probes; conflicting or selector-free live preview flags exit 2. First SIGINT/SIGTERM drops queued work and drains running actions within remaining deadlines; second signal cancels running actions as uncertain; fatal source shutdown exits 1.
- [ ] #5 Safe inspection/help/completion, both schemas, README/config/design docs, and examples/listen describe commands, predicates, admission versus execution order, discarded output, and truthful Noon handoff limits. No inspection emits argv or payloads. Focused fixture tests and task ci pass, with commands/results recorded.
<!-- AC:END -->
