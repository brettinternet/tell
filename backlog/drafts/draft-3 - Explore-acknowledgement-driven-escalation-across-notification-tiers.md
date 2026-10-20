---
id: DRAFT-3
title: Explore acknowledgement-driven escalation across notification tiers
status: Draft
assignee: []
created_date: '2026-10-20 19:35'
labels:
  - notify
  - escalation
dependencies: []
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Context
TELL-30 adds an availability cascade: within one invocation, try the next route when the current one cannot deliver. That is not real escalation. Real escalation is time- and acknowledgement-driven: notify tier 1, wait for someone to acknowledge, and if nobody does within N minutes, notify tier 2 (louder or different people), repeat on a cadence, and stop on acknowledge or resolve. Accepted is not read, so only an explicit acknowledgement can stop escalation.

That needs durable incident state, timers that survive process exit, and an acknowledgement ingress. decision-1 says Tell keeps no durable state, queues, retries, or scheduling, and that durable/timed work goes to a scheduler such as Noon. Noon already has delayed runs (noon run JOB --detach --in 10m / --at), idempotency keys (--key returns the existing run for a reused key), payloads (--data), and noon cancel. Tell already has the inbound side: tell listen with Slack Socket Mode (TELL-26) and signed GitHub webhooks through cloudflared (TELL-29), routing events to commands (TELL-27).

## Candidate architecture (recommended direction, not settled)
Split responsibilities along decision-1:
- Caller raises an incident with a stable key and a policy name (CI failure, monitor, Noon hook). Raising the same key while open does not create a second incident.
- Noon owns incident state and timers: tier N runs tell notify for that tier (each tier may itself be a TELL-30 cascade), then schedules tier N+1 with --in WAIT and a key derived from incident+tier. Repeat and max-tier limits live in the policy.
- Tell owns delivery and acknowledgement ingress: messages carry the incident key and an ack affordance; tell listen receives the ack (Slack button or reaction, ntfy action button posting to a signed webhook, or a local CLI) and its route hands off to Noon to stop pending tiers. Resolve by the caller uses the same stop path.
- Tell stays stateless; no Tell-side timers.

## Alternatives to evaluate
- A. Noon-owned escalation as above (preferred if Noon can cancel or skip by key).
- B. Provider-native escalation: Pushover emergency priority (retries until acknowledged, receipt API), PagerDuty/Opsgenie through Apprise. Possibly zero new code; evaluate first for personal use.
- C. A dedicated escalation engine binary with its own store. Clean, but another service.
- D. Durable state inside Tell. Requires superseding decision-1; not recommended.

## Policy model to refine
policy name -> ordered tiers {notify targets or cascade, wait before next tier}; repeat count/interval for the last tier; quiet hours or working-hours schedule; severity mapping to policy; dedupe window per incident key; auto-resolve from caller; audit trail of tier deliveries and acks (body-free).

## Questions for refinement
- Can Noon cancel or skip pending runs by key, or must each tier job check an ack marker before notifying? If neither, what minimal Noon feature is needed?
- Which ack channels matter first: Slack interactive button (Socket Mode interactivity payloads are not handled by TELL-26 today), Slack reaction, ntfy action buttons via the cloudflared webhook, Pushover receipts, local tell ack/noon cancel?
- How are ack requests authenticated so a third party cannot silence an incident, and how is the incident key bound to the acknowledger?
- Where do policies live (Noon job config vs Tell personal config), and who validates them?
- What happens when Tell or Noon is down mid-escalation (missed ack, missed tier), and what reconciliation is acceptable for a personal developer machine?
- Is multi-person on-call in scope, or only one person with escalating loudness (desktop -> phone -> phone emergency)?

## Candidate acceptance for refinement
- Raising an incident delivers tier 1; without ack, tier 2 is delivered after the configured wait; ack before the wait prevents tier 2; resolve stops all pending tiers.
- Raising an already-open key does not duplicate tiers.
- An unauthenticated or mismatched ack is rejected and logged without message bodies.
- Restarting Tell or Noon does not lose pending tiers beyond documented limits; Tell itself holds no durable state.
- Docs and an example show an end-to-end personal policy.

## Boundaries and references
Likely surfaces: Noon (state, timers, cancel-by-key), Tell listen ack event mapping and route handoff, message ack affordances, docs/listening.md, docs/design.md, examples/noon. Depends on TELL-30 for per-tier cascades; related TELL-26, TELL-27, TELL-29, decision-1, ../noon/docs/triggers.md and ../noon/docs/cli.md. No implementation until refined and promoted.
<!-- SECTION:DESCRIPTION:END -->
