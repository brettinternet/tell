---
id: TELL-26
title: Listen to Slack events over Socket Mode with tell listen
status: Done
assignee:
  - '@pi'
created_date: '2026-10-17 21:01'
updated_date: '2026-10-18 00:30'
labels:
  - listen
  - slack
  - reviewed
dependencies: []
references:
  - 'https://docs.slack.dev/apis/events-api/using-socket-mode'
  - 'https://docs.slack.dev/reference/methods/apps.connections.open'
  - 'https://docs.slack.dev/reference/events/channel_created'
documentation:
  - >-
    backlog/decisions/decision-1 -
    Tell-is-the-local-integration-edge-in-both-directions-schedulers-own-background-work.md
  - docs/listening.md
  - docs/design.md
  - docs/configuration.md
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
  - go.mod
  - go.sum
  - docs/development.md
  - AGENTS.md
priority: high
type: feature
ordinal: 19000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Context
Tell is becoming a local integrations layer, not just an outbound notification wrapper. A Slack incident channel should trigger local behavior without requiring a public URL. This first slice provides safe inspection and real event capture; routing follows in TELL-27. It must be useful on its own through explicit print mode.

## Scope and settled interface
Implement the CLI, configuration/inspection, envelope/bounds, and Slack sections of docs/listening.md. That planned contract refines decision-1; do not carry forward its obsolete blanket one-shot/no-state wording.
- Bare `tell listen` inspects and exits; `tell --json listen` returns one safe snapshot. Only explicit source selectors or `--all` start connections.
- `tell listen --print slack` exports NDJSON and acknowledges deliveries, with no actions. Print is sensitive data export, not a dry-run alias. Dispatch without routes fails with a hint; do not advertise --dry-run until TELL-27.
- Load personal-only `listen.slack.app_token_env`; validate structure without requiring credentials for inspection/unrelated commands. Strip named inbound credentials from every Tell child, including existing outbound delivery and doctor children.
- Use the common bounded envelope, 64-event ingress, source-scoped atomic 10-minute/10,000-entry dedup, safe receipts, cancellable connection recovery, and no action retries.
- Production uses fixed Slack HTTPS/WSS endpoints; fixtures use internal injection, not a public api_url configuration option.

## Non-goals
Routes/actions, HTTP ingress, other providers, interactive Slack responses, outbound Slack posting, multiple Slack apps, service installation, persistence, scheduled work, and live-provider tests. Existing MCP remains unchanged.

## Next action and verification
After claiming, inspect existing config, CLI output, delivery child-environment, and signal patterns; implement this slice only. Use local HTTP/WebSocket fixtures for protocol behavior and isolated CLI fixtures for safe startup/privacy. Record focused Go test commands/results and `task ci` before checking criteria.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A local Slack HTTP/WebSocket fixture observes Authorization only on apps.connections.open and prompt acknowledgments for valid envelope IDs, including ignored and duplicate deliveries. --print emits the specified event_callback envelope minus legacy token. Malformed identity/JSON, duplicate keys, and size limits fail closed; bounded output/ingress cannot stall acknowledgments or grow without limit.
- [x] #2 Disconnect/refresh and network/5xx/429 fixtures recover with fresh connection URLs, cancellable backoff and Retry-After; fatal auth/setup errors and link_disabled exit 1 without raw response/URL leakage. Duplicate IDs are acked but emitted once; source-scoped identity, TTL expiry, capacity eviction, and unreserved admission drops follow docs/listening.md.
- [x] #3 Isolated CLI checks prove bare listen and --json listen inspect only, while explicit slack/--all print mode connects. Invalid selectors/mode combinations/live --json exit 2; missing selected configuration/credentials and dispatch without routes exit 1 before I/O. SIGINT/SIGTERM exit 0; fatal/output errors exit 1. Unselected missing credentials do not break unrelated commands.
- [x] #4 Sentinel app credentials, legacy verification tokens, ticket URLs, and raw provider errors never reach diagnostics/config snapshots; print removes the legacy token without promising arbitrary payload sanitization. Shared listen config is rejected; config/check/doctor/bare listen perform no inbound I/O. Local doctor readiness leaves existing outbound exit rules unchanged. Existing outbound child fixtures prove named listener credentials are stripped.
- [x] #5 Help/completion, README, design/configuration/development docs, AGENTS.md, and both schemas describe only shipped inspection/Slack behavior, safe selectors, print sensitivity, memory/loss limits, Slack app setup, and one listener per app. Local fixture tests and task ci pass, with commands/results recorded.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend strict personal configuration and safe inspection; strip named listener credentials at every child boundary. 2. Implement bounded Slack Socket Mode capture, acknowledgments, atomic admission/dedup, cancellable recovery, and bounded output. 3. Wire safe listen CLI, readiness, schemas and shipped-only docs. 4. Add local protocol/privacy/CLI regression fixtures after implementation; run focused tests and task ci. 5. Perform one focused security/acceptance review, commit, merge, record evidence, and clean up the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Validation passed:
- mise exec -- task ci (vet, Staticcheck, unit/race/smoke tests, Gitleaks, govulncheck; no called vulnerabilities).
- mise exec go -- go test -race ./internal/listen ./internal/config ./internal/cli ./internal/delivery
- mise exec go -- go test -race ./internal/listen -count=10
- Help and completion executed; only Slack and shipped flags offered. Both schemas parse as JSON. LSP diagnostics and git diff --check clean.

AC1/2 evidence: local HTTP/WebSocket fixtures verify header-only authentication, no credential forwarding, ignored, invalid, and duplicate acknowledgments, legacy field removal, malformed identity and duplicate-key rejection, raw/envelope bounds, and 100 acknowledgments despite stalled stdout. Atomic admission tests verify source isolation, TTL, oldest eviction, and no reservation on overflow. Recovery fixtures cover fresh URLs after refresh, HTTP 503/429, network failure, Retry-After, fatal handshake/authentication/link_disabled, and safe diagnostics.
AC3/4 evidence: isolated CLI/config tests cover inspection, explicit slack/all selection, usage/config failures, missing credentials, shared-block rejection, and existing command/doctor child environment isolation. Built-binary smoke checks SIGINT/SIGTERM exit 0 using a localhost-only rejecting proxy and closed stderr pipe exit 1.
AC5: README, design/configuration/development/listening docs, AGENTS, help/completion, and schemas distinguish shipped capture from planned routing and GitHub. No live Slack services or credentials used.

One direct acceptance/privacy review completed; delegation was unavailable and the user chose direct implementation. Fixed WebSocket 4xx retry classification and SIGPIPE handling; regression fixtures and task ci passed afterward.

Delivery: source commit 2501b32d926407c7085cb0d24733e4ab3de4696e fast-forward merged to main. Source task check:staged passed. Worktrunk removed the verified session-owned tell-26-slack checkout and branch; its Herdr workspace wG9 is confirmed absent. No push or PR. No remaining blocker or implementation step. Final notes were reworded after a Gitleaks false positive in prose; no scan bypass or allowlist was added.

Review: implementation matches docs/listening.md. Fixed non-event_callback payload receipts to report 'ignored' (was 'not_sent/ignored') and replaced string-matched invalid_connection_url with typed setupError. task ci passed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented safe Slack Socket Mode inspection and explicit sensitive NDJSON capture, bounded transport/admission/dedup/output and recovery, strict personal credentials with all-child stripping, CLI readiness/help, schemas and docs. Verified local HTTP/WebSocket privacy and failure fixtures, repeated race tests, real-process signals/broken pipe, and task ci. Committed as 2501b32, merged to main, and cleaned up the owned worktree/workspace. Routing/actions and GitHub remain planned.
<!-- SECTION:FINAL_SUMMARY:END -->
