---
id: TELL-27
title: Route inbound events to commands
status: Done
assignee:
  - '@pi'
created_date: '2026-10-17 21:01'
updated_date: '2026-10-18 02:39'
labels:
  - listen
  - reviewed
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
- [x] #1 Local event/command fixtures prove exact argv (no shell/interpolation), envelope stdin and EOF, invocation CWD, five overridden TELL_ values, and absence of named listener credentials. Child stdout/stderr sentinels are discarded. Type/predicate/missing/null/non-object mismatches run nothing.
- [x] #2 Typed scalar equality (including large integers without float rounding), prefix AND, and whole-list replacement match docs/listening.md. Invalid paths/predicates, duplicate names, invalid argv, unconfigured sources, and shared routes fail config check/startup before inbound I/O. Multiple matching routes are admitted in config order without promising concurrent side-effect order.
- [x] #3 0/3/other exit, missing/start-failed executable, timeout and cancellation yield existing honest receipts. Four running plus 64 waiting bounds are enforced; excess logs not_sent/busy without retry, duplicate replay of an admitted event cannot repeat successful siblings, and slow actions do not delay provider acks.
- [x] #4 --print exports envelopes without matching/actions; --dry-run reports body-free matches without subprocess/target probes; conflicting or selector-free live preview flags exit 2. First SIGINT/SIGTERM drops queued work and drains running actions within remaining deadlines; second signal cancels running actions as uncertain; fatal source shutdown exits 1.
- [x] #5 Safe inspection/help/completion, both schemas, README/config/design docs, and examples/listen describe commands, predicates, admission versus execution order, discarded output, and truthful Noon handoff limits. No inspection emits argv or payloads. Focused fixture tests and task ci pass, with commands/results recorded.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend strict personal route configuration and exact typed matching, preserving safe inspection. 2. Reuse command outcomes in bounded dispatch with dry-run and two-stage shutdown. 3. Add critical fixture regressions and update schemas/help/docs/examples. 4. Run focused race tests and task ci; one independent acceptance review; commit, merge locally, finalize backlog and clean up.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented in 7c9da10; integrated current main in 9dbe1cf and fast-forward merged locally. Verification: go test -race ./internal/listen ./internal/config ./internal/cli ./internal/delivery ./cmd/tell passed; task ci passed before and after integration (security, vet, staticcheck, full unit/race, build, smoke). task check:staged passed before each commit. LSP routes_test diagnostics clean; schema JSON/Python syntax and diff checks passed. Independent verifier passed AC1/3/4/5 and reported signed zero for AC2; parent inspected canonicalNumber (already normalizes zero sign) and added a passing public Matches regression for -0 versus 0, disproving the finding without source changes. Matching/config fixtures cover exact large integers, typed AND predicates and whole-list replacement; executor/Slack fixtures cover argv/stdin/CWD/env isolation, outcomes, 4+64 capacity, duplicates and prompt acks; CLI fixtures cover preview safety and two-stage shutdown. Noon limits independently confirmed against sibling docs/triggers.md, store retention and key validation. Worklease expired during delayed tool return; user explicitly authorized recovery, all agents stopped, and same-loop claim reacquired. Owned worktree and branch removed by Worktrunk after receipt/list verification; associated workspace cleanup hook ran. No blockers or remaining implementation steps.

Implementation review: inspected strict route parsing/replacement, exact typed matching, literal argv/stdin/CWD/environment isolation, bounded executor, outcomes, and CLI lifecycle. P2 finding (shared by TELL-28/29): internal/listen/capture.go:162-166 waits for event dispatch to drain before OnStop stops the executor. Fatal source/output errors cancel only sourceCtx, not the CLI caller context, so queued events/actions may start new side effects during fatal shutdown rather than being dropped. Stop action admission before draining ingress; preserve diagnostics until running actions finish. A temporary TestReviewFatalSourceStopsBeforeDispatchDrain probe failed as expected, confirming dispatch remains enabled after fatal source failure; probe removed after review. Existing focused race tests, task smoke, go test ./..., and go vet ./... passed. Finding recorded only; implementation unchanged.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Shipped strict personal command routes, exact predicates, bounded execution, dry-run, and two-stage shutdown with safe receipts and Noon examples. Focused race tests and post-integration task ci pass. Implemented 7c9da10, merged to main at 9dbe1cf. Independent review finding disproved by public signed-zero regression. Worktree, branch and Herdr workspace removed; no outstanding work.
<!-- SECTION:FINAL_SUMMARY:END -->
