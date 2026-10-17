---
id: TELL-29
title: Receive signed GitHub webhooks through a cloudflared tunnel
status: To Do
assignee: []
created_date: '2026-10-17 21:02'
updated_date: '2026-10-17 21:18'
labels:
  - listen
  - github
  - webhooks
dependencies:
  - TELL-27
references:
  - >-
    https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
  - 'https://docs.github.com/en/webhooks/webhook-events-and-payloads'
  - >-
    https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks
  - >-
    https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks
  - >-
    https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/routing-to-tunnel/
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
ordinal: 22000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Context and promotion decision
Promote: the user selected GitHub webhooks and a cloudflared tunnel. Original draft 4's missing provider/ingress decision is now resolved. A repository PR event should trigger local commands or communication without turning Tell into a public service manager. This task depends on TELL-27 (and transitively TELL-26), not on native notify/send actions.

## Scope and settled interface
Implement the GitHub section and shared ingress contract of docs/listening.md:
- `tell listen github` selects the source; bare listen remains inspection. A shared personal listen.http.addr defaults to 127.0.0.1:8787 and permits literal loopback IPs only. POST /github is fixed; no new flags, public binding, or tunnel management.
- Personal listen.github.secret_env and repository_ids are required. Only allowed repository webhook bodies enter the pipeline. One secret; repository events only.
- Verify SHA-256 HMAC over bounded original bytes in constant time before decoding or routing. Reject unsigned/SHA-1-only, ambiguous metadata, invalid JSON, oversized bodies, and disallowed repositories. No raw headers/signatures enter data or logs.
- Map GitHub event/delivery headers to envelope type/id and preserve parsed body as data. Verified ping returns 204 without routing; admitted events return 202, duplicates 200, full/shutting-down ingress 503. Neither 2xx nor a tunnel means durable processing or authorization.
- GitHub signs no timestamp and does not authenticate delivery/event headers. Supplement ID dedup with a bounded raw-body digest; document identical-body collapse and replay after expiry/eviction/restart. Do not promise rejection of old-but-valid requests or automatic GitHub retries.
- Bound headers, request duration, active body readers, ingress, and action execution. Reuse existing matching/run actions unchanged, with atomic concurrent admission/dedup.
- Document a user-managed named cloudflared tunnel with a stable HTTPS hostname, /github-only ingress to loopback, catch-all 404, preserved bodies/headers, and no interactive Access challenge. Tell neither runs nor configures cloudflared.

## Non-goals
Other webhook providers, generic HMAC, GitHub Apps/organization hooks, secret rotation/multiple instances, TLS termination, public binds, provider registration, tunnel lifecycle, durability/replay, scheduling, or real-provider CI tests. Native actions are an independent follow-up, not a dependency.

## Next action and verification
After prerequisites and claim, recheck linked GitHub/Cloudflare contracts, extend source fixtures with signed loopback HTTP requests, and exercise a repository pull_request/opened route to a local command. Capture signature, duplicate/replay, overload, and no-bind inspection evidence. Record focused Go tests and task ci; live tunnel testing is optional and must not be claimed from fixtures.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Signed local repository webhook fixtures produce the common github envelope with header event/delivery metadata and unchanged parsed body, and execute a matching TELL-27 command. Verified ping returns 204 with no action; unrelated events match nothing; Slack-only runs and every inspection command never bind HTTP.
- [ ] #2 Constant-time raw-body HMAC verification rejects unsigned, SHA-1-only, malformed or wrong signatures and tampered bytes before any print/action. Disallowed repository IDs return 403. Tests exercise ambiguous/missing headers, malformed/duplicate-key JSON, media/encoding/path/method errors, and configured body/header limits with the documented status codes and no admission.
- [ ] #3 Concurrent identical deliveries route once; changing unsigned delivery ID cannot bypass the body-digest cache within its window. Source identities do not collide. TTL/eviction/restart allow replay as documented; payload/received timestamps are not treated as freshness proof. Verified duplicates return 200; no rejected admission reserves an identity.
- [ ] #4 Slow actions do not delay 202 responses. Full ingress or shutdown returns 503 without admission; active body readers and deadlines remain bounded. Later action overflow/failure cannot change a prior 202 or invite automatic replay of sibling actions. Signal/fatal shutdown closes HTTP and preserves existing listener exit semantics.
- [ ] #5 Non-loopback/DNS binds, invalid repository IDs, and missing selected secrets fail before listening; bind conflict exits 1 and stops other sources. Shared config is rejected. Config/doctor/receipts and child environments do not leak secret values, raw payloads, headers, or tunnel credentials; --print remains explicit sensitive export.
- [ ] #6 Both schemas, help/completion, configuration/design docs, and examples/listen include GitHub repository setup plus the user-owned named cloudflared HTTPS-to-loopback recipe. Docs explain no signed timestamp/headers, no automatic GitHub redelivery, 2xx versus action success, and no durable replay. Focused local tests and task ci pass with recorded evidence.
<!-- AC:END -->
