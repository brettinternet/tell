---
id: TELL-28
title: Deliver matched events through native notify and send actions
status: To Do
assignee: []
created_date: '2026-10-17 21:02'
updated_date: '2026-10-17 21:18'
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
- [ ] #1 Matched notify/send routes dispatch once through existing local provider fixtures with inherited or static per-route selectors and normal outcomes. Exactly-one-action validation rejects mixed action keys, invalid selectors, request intent, unknown keys, and shared routing before inbound I/O; run routes retain their literal argv behavior.
- [ ] #2 Single-pass scalar placeholders and escaped openers render as specified. Missing/null/object/array values produce not_sent/template_value, oversized expansion produces not_sent/template_size, and invalid syntax fails startup. Event values containing shell/template syntax cannot alter executable, argv, recipient, route, or policy and are never evaluated twice.
- [ ] #3 Delivered text uses existing control normalization, secret redaction/rejection, truncation, and send provenance plus the fixed external-event warning. A secret/scanner failure sends nothing for that action, does not terminate the listener, and does not expose text in receipts. Provider/timeout failures remain failed/uncertain rather than accepted.
- [ ] #4 --dry-run evaluates templates/preparation without delivery or target probes, prints no rendered text, and distinguishes a valid match from a render/policy failure. --print still exports only events and runs nothing. Human/JSON inspection does not leak templates or action configuration.
- [ ] #5 Command, notify, and send share the four-running/64-waiting action bound, per-action timeout, shutdown behavior, and stripped credential environment. Local fixtures show no retry or fallback and no duplicate sibling delivery after one action fails.
- [ ] #6 Both schemas, safe inspection, README/design/config docs, and examples/listen document the action union, exact template grammar, static overrides, inform-only trust boundary, and IDs/links examples. Existing outbound CLI/MCP semantics remain unchanged; focused tests and task ci pass with recorded evidence.
<!-- AC:END -->
