---
id: TELL-28
title: Deliver matched events through native notify and send actions
status: Done
assignee:
  - '@pi'
created_date: '2026-10-17 21:02'
updated_date: '2026-10-18 01:48'
labels:
  - listen
dependencies:
  - TELL-27
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
  - examples/listen/**
  - examples/README.md
priority: high
type: feature
ordinal: 21000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Context and promotion decision
Promote: the user wants Tell to be a local cross-tool integrations layer. Direct person/agent communication is a core use case, so requiring repeated wrapper scripts before building it is no longer the right gate. This is original draft 3; TELL-27 supplies the shared matcher/action lifecycle.

## Scope and settled interface
Implement the native-action section of docs/listening.md. Each personal route has exactly one of run, notify, or send; multiple routes provide explicit fan-out. Reuse existing in-process delivery, preparation, and honest outcomes.
- notify has message, optional title/type, and static via/herdr_session/apprise_tag selectors.
- send has message and optional static via/to/herdr_session; intent is inform only. Inherit omitted personal outbound selectors. No event-selected destinations, executable/config paths, sender identity, timeout, or secret policy.
- Only message/title accept single-pass `${path}` placeholders rooted in the envelope; `$${` escapes the opener. Scalar values only; missing/null/compound values fail that action, not empty substitution. No functions, expressions, shell, environment, or recursive expansion.
- Bound rendering before the existing normalization/secret-policy/truncation pipeline. Fixed send provenance marks external event data as untrusted; prefer IDs/links. inform is not a prompt-injection sandbox.
- Native actions share the existing executor bounds, credential stripping, cancellation, and no-retry rule. Dry-run validates templates/policy and reports body-free results without delivery or destination probes.

## Non-goals
New outbound transports, event-driven request intent, arbitrary agent work, target discovery, action lists, templated argv/selectors, scripts inside templates, fallback, retries, or new CLI/global flags. A trusted command wrapper can explicitly choose stronger agent authority.

## Next action and verification
After TELL-27 is complete and this task is claimed, inspect delivery entry points and extend existing local provider fixtures. Verify injection/trust boundaries and preparation parity through listener behavior, not mocks. Record focused Go checks and task ci results before checking criteria.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Matched notify/send routes dispatch once through existing local provider fixtures with inherited or static per-route selectors and normal outcomes. Exactly-one-action validation rejects mixed action keys, invalid selectors, request intent, unknown keys, and shared routing before inbound I/O; run routes retain their literal argv behavior.
- [x] #2 Single-pass scalar placeholders and escaped openers render as specified. Missing/null/object/array values produce not_sent/template_value, oversized expansion produces not_sent/template_size, and invalid syntax fails startup. Event values containing shell/template syntax cannot alter executable, argv, recipient, route, or policy and are never evaluated twice.
- [x] #3 Delivered text uses existing control normalization, secret redaction/rejection, truncation, and send provenance plus the fixed external-event warning. A secret/scanner failure sends nothing for that action, does not terminate the listener, and does not expose text in receipts. Provider/timeout failures remain failed/uncertain rather than accepted.
- [x] #4 --dry-run evaluates templates/preparation without delivery or target probes, prints no rendered text, and distinguishes a valid match from a render/policy failure. --print still exports only events and runs nothing. Human/JSON inspection does not leak templates or action configuration.
- [x] #5 Command, notify, and send share the four-running/64-waiting action bound, per-action timeout, shutdown behavior, and stripped credential environment. Local fixtures show no retry or fallback and no duplicate sibling delivery after one action fails.
- [x] #6 Both schemas, safe inspection, README/design/config docs, and examples/listen document the action union, exact template grammar, static overrides, inform-only trust boundary, and IDs/links examples. Existing outbound CLI/MCP semantics remain unchanged; focused tests and task ci pass with recorded evidence.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend strict personal route configuration to exactly one run/notify/send action, static selectors, and bounded single-pass envelope templates; update schemas.
2. Reuse delivery preparation and dispatch for native actions inside the shared executor, including inform-only external-event provenance, dry-run validation without probes, and existing timeout/shutdown/credential isolation.
3. Extend local listener/provider fixtures for template and policy failures, static/inherited delivery, mixed action bounds, preview isolation, and unchanged outbound behavior. Update shipped docs and IDs/links examples.
4. Run focused Go checks and task ci; perform one independent defect review focused on injection/privacy and lifecycle parity, fix concrete findings, and rerun affected checks.
5. Commit implementation, merge into main, record acceptance evidence and completion, then remove the verified session-owned worktree and release the claim.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Worklease claim 09493ff93079a8c01f7a1dadf7530261 acquired by loop 2cced948-bba5-4b53-aa82-e529d2004c86; authority 5c3be162a660fb7434fde16d059bdd47. Implementation checkout .worktrees/tell-28-native-actions created from 600d7cf; creation receipt in .git/worktrees/tell-28-native-actions/agent-creation.json. Executor cc5d2197-23d1-4fa7-a8c6-fd6cb51b038b implements source/tests/docs; parent owns provider and integration.

Verification before acceptance: executor ran go test ./internal/config ./internal/listen ./internal/delivery ./internal/cli; go test -race ./internal/listen ./internal/config ./internal/cli ./internal/delivery; task ci (security scans, formatting/vet/Staticcheck, unit/race, build and smoke); jq empty on both schemas; git diff --check. All passed. After review fixes, focused tests/race, task ci and gopls check passed again; parent independently reran all four focused package tests successfully.
AC1: real Apprise/hcom provider fixtures and strict config tests verify static/inherited selectors, exactly-one-action and safe parsing. AC2: template tests cover scalar paths, escapes, single-pass injection resistance, invalid/null/compound values and combined size cap. AC3: native provider and preparation tests verify controls, secrets/redaction/rejection/scanner failure, provenance/truncation, sibling continuation and honest timeout failures. AC4: CLI preview/inspection tests verify body-free dry-run policy/template results, no probes/delivery, and print isolation. AC5: mixed-action executor bounds/shutdown, duplicate suppression and credential-stripping fixtures pass. AC6: schemas and all requested docs/examples now describe implemented native actions, IDs/links and inform-only trust boundary; full CI passes including existing outbound CLI/MCP coverage.
Single independent review found three concrete issues: uncancellable repeated rendering, stderr enqueue acknowledged before write, and outbound selector flags invalidating dormant listener routes. All fixed with regressions: context-aware/cached rendering, bounded serialized acknowledged stderr writes, and listener validation isolated from one-shot flags. Parent checked corrected paths. No further general review. Delivery/integration pending.

Delivery: implementation commit 7c2d9ea7cff9f034d750abf0ad58d110b22ff3b6 fast-forward merged into main. Staged Gitleaks initially flagged synthetic test secrets; changed fixture construction to the existing split-literal pattern without changing runtime values, reran affected tests and task check:staged successfully. Worktrunk removed the receipt-verified session-owned checkout/branch; matching Herdr workspace wGB was closed by its hook. No retained checkout or remaining blocker. No push. Worklease release follows task-state commit.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented native notify/send routes with strict static selectors, bounded single-pass templates, shared preparation/execution, inform-only external-event provenance, and safe previews/inspection. Focused tests/race, task ci, gopls, schema parsing, and staged checks passed. Independent review findings fixed with regressions. Committed as 7c2d9ea and fast-forward merged into main; owned worktree/branch and matching Herdr workspace removed. No push.
<!-- SECTION:FINAL_SUMMARY:END -->
